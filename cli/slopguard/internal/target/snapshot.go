package target

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
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
	return collector.exportRevision(ctx, bundle.repository, sandbox, revision, writer)
}

// exportRevision reads tree entries and blobs directly rather than through
// git archive, whose export-ignore and export-subst attributes would let the
// reviewed revision hide or rewrite files in its own snapshot.
func (collector *Collector) exportRevision(ctx context.Context, root string, sandbox *gitSandbox, revision string, writer *snapshotWriter) error {
	if !validObjectID(revision) {
		return fmt.Errorf("snapshot revision is not a resolved object ID")
	}
	var entries []snapshotEntry
	parser := &nulRecordWriter{handle: func(raw []byte) error {
		metadata, path, ok := bytes.Cut(raw, []byte{'\t'})
		fields := strings.Fields(string(metadata))
		if !ok || len(fields) != 3 {
			return fmt.Errorf("git returned a malformed tree entry")
		}
		relative := string(path)
		if (fields[0] != "100644" && fields[0] != "100755") || fields[1] != "blob" || protocolPath(relative) != nil || !writer.wanted(relative) {
			return nil
		}
		if !validObjectID(fields[2]) {
			return fmt.Errorf("git returned an invalid blob ID for %q", relative)
		}
		entries = append(entries, snapshotEntry{path: relative, oid: fields[2]})
		return nil
	}}
	if err := collector.git.runSandboxTo(ctx, root, sandbox, nil, parser, "ls-tree", "-r", "-z", "--full-tree", revision); err != nil {
		return fmt.Errorf("list snapshot tree: %w", err)
	}
	if err := parser.Err(); err != nil {
		return err
	}
	if len(entries) == 0 {
		return nil
	}
	var input bytes.Buffer
	for _, entry := range entries {
		_, _ = input.WriteString(entry.oid + "\n")
	}
	batch := &snapshotBatchWriter{writer: writer, entries: entries, remaining: -1}
	if err := collector.git.runSandboxTo(ctx, root, sandbox, input.Bytes(), batch, "cat-file", "--batch"); err != nil {
		return fmt.Errorf("read snapshot blobs: %w", err)
	}
	return batch.Err()
}

type snapshotEntry struct {
	path string
	oid  string
}

// snapshotBatchWriter parses git cat-file --batch output and writes each blob
// as it completes.
type snapshotBatchWriter struct {
	writer    *snapshotWriter
	entries   []snapshotEntry
	index     int
	header    []byte
	content   []byte
	remaining int64
	err       error
}

func (batch *snapshotBatchWriter) Write(data []byte) (int, error) {
	input := data
	for len(input) > 0 && batch.err == nil {
		if batch.index >= len(batch.entries) {
			batch.err = fmt.Errorf("git returned excess snapshot blobs")
			break
		}
		entry := batch.entries[batch.index]
		if batch.remaining < 0 {
			newline := bytes.IndexByte(input, '\n')
			if newline < 0 {
				if len(batch.header)+len(input) > 512 {
					batch.err = fmt.Errorf("git snapshot blob header exceeds safe maximum")
				}
				batch.header = append(batch.header, input...)
				break
			}
			batch.header = append(batch.header, input[:newline]...)
			input = input[newline+1:]
			fields := strings.Fields(string(batch.header))
			batch.header = batch.header[:0]
			if len(fields) != 3 || fields[0] != entry.oid || fields[1] != "blob" {
				batch.err = fmt.Errorf("git returned an invalid snapshot blob header for %q", entry.path)
				break
			}
			size, err := strconv.ParseInt(fields[2], 10, 64)
			if err != nil || size < 0 {
				batch.err = fmt.Errorf("git returned an invalid snapshot blob size for %q", entry.path)
				break
			}
			if size > batch.writer.remaining {
				batch.err = fmt.Errorf("repository snapshot exceeds %d bytes", MaximumSnapshotBytes)
				break
			}
			batch.content = make([]byte, 0, size)
			batch.remaining = size
		}
		consume := min(int64(len(input)), batch.remaining)
		batch.content = append(batch.content, input[:consume]...)
		input = input[consume:]
		batch.remaining -= consume
		if batch.remaining == 0 && len(input) > 0 {
			if input[0] != '\n' {
				batch.err = fmt.Errorf("git returned an incomplete snapshot blob for %q", entry.path)
				break
			}
			input = input[1:]
			batch.err = batch.writer.write(entry.path, bytes.NewReader(batch.content))
			batch.content = nil
			batch.remaining = -1
			batch.index++
		}
	}
	return len(data), nil
}

func (batch *snapshotBatchWriter) Err() error {
	if batch.err != nil {
		return batch.err
	}
	if batch.index != len(batch.entries) || batch.remaining != -1 || len(batch.header) != 0 {
		return fmt.Errorf("git returned incomplete snapshot blobs")
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

func (writer *snapshotWriter) wanted(relative string) bool {
	return reviewablePath(relative)
}

func (writer *snapshotWriter) write(relative string, source io.Reader) error {
	content, err := io.ReadAll(io.LimitReader(source, writer.remaining+1))
	if err != nil {
		return fmt.Errorf("read snapshot file %q: %w", relative, err)
	}
	if int64(len(content)) > writer.remaining {
		return fmt.Errorf("repository snapshot exceeds %d bytes", MaximumSnapshotBytes)
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
