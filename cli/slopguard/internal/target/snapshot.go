package target

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/uinaf/ffss/cli/slopguard/internal/protocol"
)

// MaximumSnapshotBytes bounds the file content a repository snapshot writes.
const MaximumSnapshotBytes = int64(512 << 20)

// ExportSnapshot writes the reviewed side of the target into directory as plain
// files without Git metadata, so history cannot reveal what the snapshot
// omits. Sensitive paths are left out, environment files are redacted, and
// symlinks and special files are skipped. Local targets copy the working tree,
// whose later drift VerifyUnchanged reports.
func (bundle *Bundle) ExportSnapshot(ctx context.Context, directory string) error {
	if err := outsideRepository(directory, bundle.repository); err != nil {
		return err
	}
	writer := &snapshotWriter{directory: directory, remaining: MaximumSnapshotBytes}
	collector := bundle.collector
	local := bundle.request.Mode == protocol.TargetLocal
	sandbox, err := collector.newGitSandbox(ctx, bundle.repository, local)
	if err != nil {
		return err
	}
	defer func() { _ = sandbox.Close() }()
	if local {
		return collector.exportWorktree(ctx, bundle.repository, sandbox, writer)
	}
	revision := bundle.target.CommitRevision
	if revision == "" {
		revision = bundle.target.HeadRevision
	}
	if !validObjectID(revision) {
		return fmt.Errorf("snapshot revision is not a resolved object ID")
	}
	reader, pipe := io.Pipe()
	extracted := make(chan error, 1)
	go func() {
		err := writer.extract(tar.NewReader(reader))
		if err == nil {
			// Git pads the archive past the end-of-archive marker.
			_, err = io.Copy(io.Discard, reader)
		}
		_ = reader.CloseWithError(err)
		extracted <- err
	}()
	archiveErr := collector.git.runSandboxWithAttributesTo(ctx, bundle.repository, sandbox, nil, pipe, "archive", "--format=tar", revision)
	_ = pipe.CloseWithError(archiveErr)
	if err := <-extracted; err != nil {
		return err
	}
	if archiveErr != nil {
		return fmt.Errorf("archive snapshot: %w", archiveErr)
	}
	return nil
}

func (collector *Collector) exportWorktree(ctx context.Context, root string, sandbox *gitSandbox, writer *snapshotWriter) error {
	var paths []string
	parser := &nulRecordWriter{handle: func(raw []byte) error {
		paths = append(paths, string(raw))
		return nil
	}}
	if err := collector.git.runSandboxWithAttributesTo(ctx, root, sandbox, nil, parser, "ls-files", "--cached", "--others", "--exclude-standard", "-z"); err != nil {
		return fmt.Errorf("list snapshot files: %w", err)
	}
	if err := parser.Err(); err != nil {
		return err
	}
	for _, relative := range uniqueSorted(paths) {
		if err := ctx.Err(); err != nil {
			return err
		}
		if protocolPath(relative) != nil || !writer.wanted(relative) {
			continue
		}
		info, err := os.Lstat(filepath.Join(root, filepath.FromSlash(relative)))
		if errors.Is(err, os.ErrNotExist) || (err == nil && !info.Mode().IsRegular()) {
			continue
		}
		if err != nil {
			return fmt.Errorf("inspect snapshot file %q: %w", relative, err)
		}
		file, _, err := openRegularFile(root, relative)
		if err != nil {
			return fmt.Errorf("open snapshot file %q: %w", relative, err)
		}
		err = writer.write(relative, file)
		_ = file.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

type snapshotWriter struct {
	directory string
	remaining int64
}

func (writer *snapshotWriter) extract(archive *tar.Reader) error {
	for {
		header, err := archive.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read snapshot archive: %w", err)
		}
		if header.Typeflag != tar.TypeReg || protocolPath(header.Name) != nil || !writer.wanted(header.Name) {
			continue
		}
		if err := writer.write(header.Name, archive); err != nil {
			return err
		}
	}
}

func (writer *snapshotWriter) wanted(relative string) bool {
	return reviewablePath(relative)
}

func (writer *snapshotWriter) write(relative string, source io.Reader) error {
	content, err := io.ReadAll(io.LimitReader(source, writer.remaining+1))
	if err != nil {
		return fmt.Errorf("read snapshot file %q: %w", relative, err)
	}
	if environmentFilePath(relative) {
		if bytes.IndexByte(content, 0) >= 0 {
			return nil
		}
		content = redactEnvironment(content)
	}
	if int64(len(content)) > writer.remaining {
		return fmt.Errorf("repository snapshot exceeds %d bytes", MaximumSnapshotBytes)
	}
	writer.remaining -= int64(len(content))
	destination := filepath.Join(writer.directory, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
		return fmt.Errorf("create snapshot directory for %q: %w", relative, err)
	}
	file, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create snapshot file %q: %w", relative, err)
	}
	_, writeErr := file.Write(content)
	if closeErr := file.Close(); writeErr == nil {
		writeErr = closeErr
	}
	if writeErr != nil {
		return fmt.Errorf("write snapshot file %q: %w", relative, writeErr)
	}
	return nil
}

// outsideRepository rejects a snapshot inside the reviewed checkout, where its
// files would read as untracked changes and fail unchanged verification.
func outsideRepository(directory, repository string) error {
	resolvedDirectory, err := filepath.EvalSymlinks(directory)
	if err != nil {
		return fmt.Errorf("resolve snapshot directory: %w", err)
	}
	resolvedRepository, err := filepath.EvalSymlinks(repository)
	if err != nil {
		return fmt.Errorf("resolve repository: %w", err)
	}
	relative, err := filepath.Rel(resolvedRepository, resolvedDirectory)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return nil
	}
	return fmt.Errorf("snapshot directory %q is inside the reviewed repository; point TMPDIR outside it", directory)
}
