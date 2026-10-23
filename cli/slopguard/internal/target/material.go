package target

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/uinaf/ffss/cli/slopguard/internal/protocol"
)

const maximumIndexBytes = int64(1 << 30)

type nulRecordWriter struct {
	buffer []byte
	handle func([]byte) error
	err    error
}

type lineRecordWriter struct {
	buffer []byte
	handle func([]byte) error
	err    error
}

func (writer *lineRecordWriter) Write(data []byte) (int, error) {
	for _, character := range data {
		if writer.err != nil {
			continue
		}
		if character == '\n' {
			writer.err = writer.handle(writer.buffer)
			writer.buffer = writer.buffer[:0]
			continue
		}
		if len(writer.buffer) >= 512 {
			writer.err = fmt.Errorf("git line record exceeds safe maximum")
			continue
		}
		writer.buffer = append(writer.buffer, character)
	}
	return len(data), nil
}

func (writer *lineRecordWriter) Err() error {
	if writer.err != nil {
		return writer.err
	}
	if len(writer.buffer) != 0 {
		return fmt.Errorf("git returned an unterminated line record")
	}
	return nil
}

func (writer *nulRecordWriter) Write(data []byte) (int, error) {
	for _, character := range data {
		if writer.err != nil {
			continue
		}
		if character == 0 {
			writer.err = writer.handle(writer.buffer)
			writer.buffer = writer.buffer[:0]
			continue
		}
		if len(writer.buffer) >= 4*protocol.MaxPathCharacters+128 {
			writer.err = fmt.Errorf("git path record exceeds protocol maximum")
			continue
		}
		writer.buffer = append(writer.buffer, character)
	}
	return len(data), nil
}

func (writer *nulRecordWriter) Err() error {
	if writer.err != nil {
		return writer.err
	}
	if len(writer.buffer) != 0 {
		return fmt.Errorf("git returned unterminated path record")
	}
	return nil
}

func (collector *Collector) validateTrackedWorktree(ctx context.Context, root string, plan *targetPlan) error {
	flags := &nulRecordWriter{handle: func(raw []byte) error {
		if len(raw) < 3 || raw[1] != ' ' {
			return fmt.Errorf("git returned malformed index flag record")
		}
		tag := raw[0]
		if tag == 'S' || (tag >= 'a' && tag <= 'z') {
			return fmt.Errorf("index flags that hide worktree changes are unsupported for %q", raw[2:])
		}
		return nil
	}}
	if err := collector.git.runSandboxWithAttributesTo(ctx, root, plan.sandbox, nil, flags, "ls-files", "-v", "-z"); err != nil {
		return fmt.Errorf("inspect index flags: %w", err)
	}
	if err := flags.Err(); err != nil {
		return err
	}
	parser := &nulRecordWriter{handle: func(raw []byte) error {
		path := string(raw)
		if err := protocolPath(path); err != nil {
			return fmt.Errorf("tracked path %q: %w", path, err)
		}
		if err := validateTrackedWorktreePath(root, path); err != nil {
			return fmt.Errorf("inspect tracked path %q: %w", path, err)
		}
		return nil
	}}
	if err := collector.git.runSandboxWithAttributesTo(ctx, root, plan.sandbox, nil, parser, "ls-files", "--modified", "--deleted", "-z"); err != nil {
		return fmt.Errorf("list tracked paths: %w", err)
	}
	return parser.Err()
}

func (collector *Collector) newGitSandbox(ctx context.Context, root string, includeWorktree bool) (_ *gitSandbox, returnErr error) {
	objectFormatOutput, err := collector.git.run(ctx, root, nil, 128<<10, "rev-parse", "--show-object-format")
	if err != nil {
		return nil, fmt.Errorf("resolve Git object format: %w", err)
	}
	objectFormat := strings.TrimSpace(string(objectFormatOutput))
	if objectFormat != "sha1" && objectFormat != "sha256" {
		return nil, fmt.Errorf("unsupported Git object format %q", objectFormat)
	}
	originalObjects, err := collector.gitMetadataPath(ctx, root, "objects")
	if err != nil {
		return nil, err
	}
	var originalIndex string
	var originalExclude string
	if includeWorktree {
		originalIndex, err = collector.gitMetadataPath(ctx, root, "index")
		if err != nil {
			return nil, err
		}
		originalExclude, err = collector.gitMetadataPath(ctx, root, "info/exclude")
		if err != nil {
			return nil, err
		}
	}

	directory, err := os.MkdirTemp("", "slopguard-git-")
	if err != nil {
		return nil, fmt.Errorf("create isolated Git directory: %w", err)
	}
	defer func() {
		if returnErr != nil {
			_ = os.RemoveAll(directory)
		}
	}()
	if _, err := collector.git.run(ctx, filepath.Dir(directory), nil, 128<<10, "init", "--quiet", "--bare", "--object-format="+objectFormat, directory); err != nil {
		return nil, fmt.Errorf("initialize isolated Git directory: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(directory, "objects", "info"), 0o700); err != nil {
		return nil, fmt.Errorf("create isolated object database: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(directory, "info"), 0o700); err != nil {
		return nil, fmt.Errorf("create isolated Git metadata: %w", err)
	}
	alternate := filepath.Join(directory, "original-objects")
	if err := os.Symlink(originalObjects, alternate); err != nil {
		return nil, fmt.Errorf("link original Git objects: %w", err)
	}
	if err := os.WriteFile(filepath.Join(directory, "objects", "info", "alternates"), []byte(alternate+"\n"), 0o600); err != nil {
		return nil, fmt.Errorf("configure isolated Git objects: %w", err)
	}
	config := "[core]\n\trepositoryFormatVersion = 0\n\tbare = false\n"
	if objectFormat == "sha256" {
		config = "[core]\n\trepositoryFormatVersion = 1\n\tbare = false\n"
	}
	if includeWorktree {
		excludes, err := collector.userGlobalExcludes(ctx, root)
		if err != nil {
			return nil, err
		}
		if len(excludes) > 0 {
			excludesPath := filepath.Join(directory, "info", "global-exclude")
			if err := os.WriteFile(excludesPath, excludes, 0o600); err != nil {
				return nil, fmt.Errorf("write global Git excludes: %w", err)
			}
			config += "\texcludesFile = " + gitConfigQuote(excludesPath) + "\n"
		}
	}
	if objectFormat == "sha256" {
		config += "[extensions]\n\tobjectFormat = sha256\n"
	}
	if err := os.WriteFile(filepath.Join(directory, "config"), []byte(config), 0o600); err != nil {
		return nil, fmt.Errorf("write isolated Git config: %w", err)
	}
	if err := os.WriteFile(filepath.Join(directory, "HEAD"), []byte("ref: refs/heads/slopguard\n"), 0o600); err != nil {
		return nil, fmt.Errorf("initialize isolated Git HEAD: %w", err)
	}
	if includeWorktree {
		// Preserve the original index mtime. A naive copy advances the file's
		// timestamp and closes Git's racy-git window, which can hide same-size
		// worktree edits (including symlink retargets) on the verification collect.
		var indexModTime time.Time
		var hasIndex bool
		if info, statErr := os.Stat(originalIndex); statErr == nil {
			indexModTime = info.ModTime()
			hasIndex = true
		} else if !errors.Is(statErr, os.ErrNotExist) {
			return nil, fmt.Errorf("inspect Git index: %w", statErr)
		}
		copiedIndex := filepath.Join(directory, "index")
		if err := copyStableFile(filepath.Dir(originalIndex), filepath.Base(originalIndex), copiedIndex, maximumIndexBytes); err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				return nil, fmt.Errorf("copy Git index: %w", err)
			}
		} else if hasIndex {
			if err := os.Chtimes(copiedIndex, indexModTime, indexModTime); err != nil {
				return nil, fmt.Errorf("preserve Git index mtime: %w", err)
			}
		}
		excludeRoot := filepath.Dir(filepath.Dir(originalExclude))
		excludeRelative, err := filepath.Rel(excludeRoot, originalExclude)
		if err != nil {
			return nil, fmt.Errorf("resolve Git exclude path: %w", err)
		}
		if err := copyStableFile(excludeRoot, filepath.ToSlash(excludeRelative), filepath.Join(directory, "info", "exclude"), metadataLimit); err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("copy Git excludes: %w", err)
		}
	}

	sandbox := &gitSandbox{
		directory: directory,
		environment: []string{
			"GIT_DIR=" + directory,
			"GIT_WORK_TREE=" + root,
			"GIT_INDEX_FILE=" + filepath.Join(directory, "index"),
			"GIT_NO_REPLACE_OBJECTS=1",
		},
	}
	if includeWorktree {
		if err := rejectSplitIndex(filepath.Join(directory, "index"), objectFormat); err != nil {
			return nil, err
		}
	}
	emptyTree, err := collector.git.runSandbox(ctx, root, sandbox, nil, 128<<10, "hash-object", "-w", "-t", "tree", "--stdin")
	if err != nil {
		return nil, fmt.Errorf("write isolated empty tree: %w", err)
	}
	sandbox.attributeSource = strings.TrimSpace(string(emptyTree))
	if !validObjectID(sandbox.attributeSource) {
		return nil, fmt.Errorf("git returned invalid empty-tree object ID")
	}
	if includeWorktree {
		head, unborn, err := collector.resolveHEAD(ctx, root)
		if err != nil {
			return nil, err
		}
		if unborn {
			head = sandbox.attributeSource
		}
		if err := os.WriteFile(filepath.Join(directory, "HEAD"), []byte(head+"\n"), 0o600); err != nil {
			return nil, fmt.Errorf("write isolated Git HEAD: %w", err)
		}
	}
	return sandbox, nil
}

// gitConfigQuote quotes a value for a Git config file, escaping the
// characters its syntax cannot hold literally.
func gitConfigQuote(value string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`, "\t", `\t`, "\b", `\b`).Replace(value) + `"`
}

// globalExcludes holds the user's global excludes file, read once per
// collector so each isolated Git directory gets the same rules.
type globalExcludes struct {
	once    sync.Once
	content []byte
	err     error
}

func (collector *Collector) userGlobalExcludes(ctx context.Context, root string) ([]byte, error) {
	cache := collector.globalExcludes
	if cache == nil {
		cache = &globalExcludes{}
	}
	cache.once.Do(func() { cache.content, cache.err = collector.readGlobalExcludes(ctx, root) })
	return cache.content, cache.err
}

// readGlobalExcludes reads the excludes file the user's own Git applies. The
// setting comes from the user's global config only; the reviewed repository
// cannot choose it.
func (collector *Collector) readGlobalExcludes(ctx context.Context, root string) ([]byte, error) {
	lookup := collector.lookupEnv
	if lookup == nil {
		lookup = os.LookupEnv
	}
	environment := []string{"GIT_CONFIG_SYSTEM=/dev/null", "GIT_CONFIG_NOSYSTEM=1", "GIT_NO_LAZY_FETCH=1", "LANG=C", "LC_ALL=C"}
	user := false
	for _, name := range []string{"HOME", "XDG_CONFIG_HOME", "GIT_CONFIG_GLOBAL"} {
		// An empty GIT_CONFIG_GLOBAL deliberately disables the user's config.
		if value, ok := lookup(name); ok && (value != "" || name == "GIT_CONFIG_GLOBAL") {
			environment = append(environment, name+"="+value)
			user = true
		}
	}
	if !user {
		// No home and no global config: Git has no global excludes either.
		return nil, nil
	}
	// --global turns include processing off unless asked; running in the
	// reviewed root lets includeIf "gitdir:" rules match as they do for the user.
	command := exec.CommandContext(ctx, collector.git.path, "-c", "core.hooksPath=/dev/null", "-c", "core.fsmonitor=false", "config", "--global", "--includes", "--null", "--get", "--path", "core.excludesFile")
	command.Dir = root
	command.Env = environment
	output, err := command.Output()
	var exitErr *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &exitErr) && exitErr.ExitCode() == 1:
		output = nil
	default:
		return nil, fmt.Errorf("read global Git excludes setting: %w", err)
	}
	excludes := strings.TrimSuffix(string(output), "\x00")
	if excludes == "" {
		if xdg, ok := lookup("XDG_CONFIG_HOME"); ok && xdg != "" {
			excludes = filepath.Join(xdg, "git", "ignore")
		} else if home, ok := lookup("HOME"); ok && home != "" {
			excludes = filepath.Join(home, ".config", "git", "ignore")
		}
	}
	// A relative path would resolve inside the reviewed worktree.
	if !filepath.IsAbs(excludes) {
		return nil, nil
	}
	resolved, err := filepath.EvalSymlinks(excludes)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("resolve global Git excludes file: %w", err)
	}
	// A dotfiles checkout under review could otherwise hide its own files.
	if inside, err := directoryContains(root, resolved); err != nil || inside {
		return nil, err
	}
	file, info, err := openRegularFile(filepath.Dir(resolved), filepath.Base(resolved))
	if err != nil {
		return nil, fmt.Errorf("open global Git excludes file: %w", err)
	}
	defer func() { _ = file.Close() }()
	if info.Size() > metadataLimit {
		return nil, fmt.Errorf("global Git excludes file exceeds %d bytes", metadataLimit)
	}
	content, err := io.ReadAll(io.LimitReader(file, metadataLimit+1))
	if err != nil {
		return nil, fmt.Errorf("read global Git excludes file: %w", err)
	}
	if int64(len(content)) > metadataLimit {
		return nil, fmt.Errorf("global Git excludes file exceeds %d bytes", metadataLimit)
	}
	return content, nil
}

// directoryContains compares file identity, so case and firmlink spellings of
// root still count.
func directoryContains(root, candidate string) (bool, error) {
	rootInfo, err := os.Stat(root)
	if err != nil {
		return false, err
	}
	for current := candidate; ; current = filepath.Dir(current) {
		if info, err := os.Stat(current); err == nil && os.SameFile(rootInfo, info) {
			return true, nil
		}
		if parent := filepath.Dir(current); parent == current {
			return false, nil
		}
	}
}

func rejectSplitIndex(path, objectFormat string) error {
	checksumBytes := int64(20)
	if objectFormat == "sha256" {
		checksumBytes = 32
	}
	index, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("open copied Git index: %w", err)
	}
	defer func() { _ = index.Close() }()
	info, err := index.Stat()
	if err != nil {
		return fmt.Errorf("inspect copied Git index: %w", err)
	}
	contentBytes := info.Size() - checksumBytes
	if contentBytes < 12 {
		return fmt.Errorf("copied Git index is truncated")
	}
	reader := bufio.NewReader(io.LimitReader(index, contentBytes))
	header := make([]byte, 12)
	if _, err := io.ReadFull(reader, header); err != nil {
		return fmt.Errorf("read copied Git index header: %w", err)
	}
	if string(header[:4]) != "DIRC" {
		return fmt.Errorf("copied Git index has an invalid signature")
	}
	version := binary.BigEndian.Uint32(header[4:8])
	if version < 2 || version > 4 {
		return fmt.Errorf("copied Git index has unsupported version %d", version)
	}
	entryCount := binary.BigEndian.Uint32(header[8:12])
	fixedBytes := int64(42) + checksumBytes
	if uint64(entryCount) > uint64(contentBytes-12)/uint64(fixedBytes+1) {
		return fmt.Errorf("copied Git index entry count exceeds its size")
	}
	offset := int64(12)
	fixed := make([]byte, fixedBytes)
	for entry := uint32(0); entry < entryCount; entry++ {
		if _, err := io.ReadFull(reader, fixed); err != nil {
			return fmt.Errorf("read copied Git index entry %d: %w", entry, err)
		}
		offset += fixedBytes
		flags := binary.BigEndian.Uint16(fixed[len(fixed)-2:])
		extendedBytes := int64(0)
		if flags&0x4000 != 0 {
			if version == 2 {
				return fmt.Errorf("copied Git index version 2 entry has extended flags")
			}
			var extended [2]byte
			if _, err := io.ReadFull(reader, extended[:]); err != nil {
				return fmt.Errorf("read copied Git index entry %d flags: %w", entry, err)
			}
			offset += int64(len(extended))
			extendedBytes = int64(len(extended))
		}
		if version == 4 {
			for encodedBytes := 0; ; encodedBytes++ {
				if encodedBytes >= 10 {
					return fmt.Errorf("copied Git index entry %d has an invalid path prefix", entry)
				}
				value, err := reader.ReadByte()
				if err != nil {
					return fmt.Errorf("read copied Git index entry %d path prefix: %w", entry, err)
				}
				offset++
				if value&0x80 == 0 {
					break
				}
			}
		}
		pathBytes := int64(0)
		for {
			value, err := reader.ReadByte()
			if err != nil {
				return fmt.Errorf("read copied Git index entry %d path: %w", entry, err)
			}
			offset++
			if value == 0 {
				break
			}
			pathBytes++
			if pathBytes > 4*protocol.MaxPathCharacters+128 {
				return fmt.Errorf("copied Git index entry %d path exceeds protocol maximum", entry)
			}
		}
		if version != 4 {
			entryBytes := fixedBytes + extendedBytes + pathBytes
			paddingBytes := int64(8) - entryBytes%8
			remainingPadding := paddingBytes - 1
			for padding := int64(0); padding < remainingPadding; padding++ {
				value, err := reader.ReadByte()
				if err != nil {
					return fmt.Errorf("read copied Git index entry %d padding: %w", entry, err)
				}
				offset++
				if value != 0 {
					return fmt.Errorf("copied Git index entry %d has invalid padding", entry)
				}
			}
		}
	}
	for offset < contentBytes {
		if contentBytes-offset < 8 {
			return fmt.Errorf("copied Git index has a truncated extension")
		}
		var extension [8]byte
		if _, err := io.ReadFull(reader, extension[:]); err != nil {
			return fmt.Errorf("read copied Git index extension: %w", err)
		}
		offset += int64(len(extension))
		size := int64(binary.BigEndian.Uint32(extension[4:]))
		if size > contentBytes-offset {
			return fmt.Errorf("copied Git index extension exceeds its size")
		}
		if string(extension[:4]) == "link" {
			return fmt.Errorf("split Git indexes are unsupported")
		}
		if _, err := io.CopyN(io.Discard, reader, size); err != nil {
			return fmt.Errorf("read copied Git index extension: %w", err)
		}
		offset += size
	}
	return nil
}

func (collector *Collector) gitMetadataPath(ctx context.Context, root, name string) (string, error) {
	output, err := collector.git.run(ctx, root, nil, 128<<10, "rev-parse", "--path-format=absolute", "--git-path", name)
	if err != nil {
		return "", fmt.Errorf("resolve Git metadata path %q: %w", name, err)
	}
	path := strings.TrimSpace(string(output))
	if path == "" || !filepath.IsAbs(path) {
		return "", fmt.Errorf("git metadata path %q is not absolute", name)
	}
	return path, nil
}

func copyStableFile(root, relative, destination string, limit int64) error {
	input, before, err := openRegularFile(root, relative)
	if err != nil {
		return err
	}
	defer func() { _ = input.Close() }()
	if before.Size() > limit {
		return fmt.Errorf("file exceeds safe copy limit of %d bytes", limit)
	}
	output, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	written, copyErr := io.CopyN(output, input, limit+1)
	closeErr := output.Close()
	if copyErr != nil && !errors.Is(copyErr, io.EOF) {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if written > limit {
		return fmt.Errorf("file exceeds safe copy limit of %d bytes", limit)
	}
	after, err := input.Stat()
	if err != nil {
		return err
	}
	if !os.SameFile(before, after) || before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) {
		return fmt.Errorf("file changed while copying")
	}
	return nil
}

type deletedBlob struct {
	path    string
	oid     string
	section int
}

// changedPaths returns the changed paths of each diff command in section order.
func (collector *Collector) changedPaths(ctx context.Context, root string, plan *targetPlan, maxBytes int64) ([][]string, []deletedBlob, error) {
	var commandPaths [][]string
	binaries := map[string]struct{}{}
	var binarySides []changedSide
	var inventoryBytes int64
	for section, arguments := range diffCommands(plan, "--numstat", "-z") {
		var paths []string
		parser := &nulRecordWriter{handle: func(record []byte) error {
			fields := bytes.SplitN(record, []byte{'\t'}, 3)
			if len(fields) != 3 {
				return fmt.Errorf("git numstat returned malformed record")
			}
			if string(fields[0]) == "-" && string(fields[1]) == "-" {
				binaries[string(fields[2])] = struct{}{}
				binarySides = append(binarySides, changedSide{path: string(fields[2]), section: section})
			} else if _, err := strconv.ParseInt(string(fields[0]), 10, 64); err != nil {
				return fmt.Errorf("git numstat returned invalid addition count")
			} else if _, err := strconv.ParseInt(string(fields[1]), 10, 64); err != nil {
				return fmt.Errorf("git numstat returned invalid deletion count")
			}
			path := string(fields[2])
			inventoryBytes += int64(len(path) + 1)
			if inventoryBytes > maxBytes {
				return &SizeError{Limit: maxBytes, Actual: inventoryBytes, Contributors: []Contributor{{Name: "framing", Bytes: inventoryBytes}}}
			}
			paths = append(paths, path)
			return nil
		}}
		if err := collector.git.runSandboxWithAttributesTo(ctx, root, plan.sandbox, nil, parser, arguments...); err != nil {
			return nil, nil, fmt.Errorf("list changed paths: %w", collector.explainMissingObject(ctx, root, err))
		}
		if err := parser.Err(); err != nil {
			return nil, nil, err
		}
		commandPaths = append(commandPaths, paths)
	}
	deleted, modes, err := collector.inspectRawModes(ctx, root, plan)
	if err != nil {
		return nil, nil, err
	}
	if err := collector.rejectWithheldCode(ctx, root, plan, binarySides, deleted, modes); err != nil {
		return nil, nil, err
	}
	textDeleted := deleted[:0]
	for _, blob := range deleted {
		if _, binary := binaries[blob.path]; !binary {
			textDeleted = append(textDeleted, blob)
		}
	}
	return commandPaths, textDeleted, nil
}

var missingObjectPattern = regexp.MustCompile(`unable to read [0-9a-f]{40,64}`)

// slopguard runs Git without a credential helper, an SSH agent, or a terminal,
// so a blobless partial clone cannot lazily fetch the objects a diff needs. The
// bare "unable to read <oid>" that Git reports gives no hint of that, and the
// object is genuinely absent, so the failure looks like corruption.
func (collector *Collector) explainMissingObject(ctx context.Context, root string, err error) error {
	if err == nil || collector.git == nil || !missingObjectPattern.MatchString(err.Error()) {
		return err
	}
	output, configErr := collector.git.run(ctx, root, nil, 4<<10, "config", "--get", "remote.origin.promisor")
	if configErr != nil || strings.TrimSpace(string(output)) != "true" {
		return err
	}
	return fmt.Errorf("%w; the repository is a partial clone and review runs Git without network access, "+
		"so lazily fetched objects are unavailable (hydrate it with: git -C %s fetch --refetch origin)", err, root)
}

func (collector *Collector) inspectRawModes(ctx context.Context, root string, plan *targetPlan) ([]deletedBlob, map[changedSide]string, error) {
	var deleted []deletedBlob
	modes := map[changedSide]string{}
	for section, arguments := range diffCommands(plan, "--raw", "--abbrev=64", "-z") {
		var rawHeader []byte
		parser := &nulRecordWriter{handle: func(record []byte) error {
			if rawHeader == nil {
				rawHeader = append([]byte(nil), record...)
				return nil
			}
			header := strings.Fields(string(rawHeader))
			rawHeader = nil
			if len(header) < 5 || !strings.HasPrefix(header[0], ":") {
				return fmt.Errorf("git raw diff returned malformed record")
			}
			oldMode := strings.TrimPrefix(header[0], ":")
			newMode := header[1]
			path := string(record)
			if gitlinkMode(oldMode) || gitlinkMode(newMode) {
				return fmt.Errorf("gitlink input %q is unsupported", path)
			}
			if newMode == "000000" {
				deleted = append(deleted, deletedBlob{path: path, oid: header[2], section: section})
			}
			modes[changedSide{path: path, section: section}] = newMode
			return nil
		}}
		if err := collector.git.runSandboxWithAttributesTo(ctx, root, plan.sandbox, nil, parser, arguments...); err != nil {
			return nil, nil, fmt.Errorf("inspect changed modes: %w", err)
		}
		if err := parser.Err(); err != nil {
			return nil, nil, err
		}
		if rawHeader != nil {
			return nil, nil, fmt.Errorf("git raw diff returned incomplete record")
		}
	}
	return deleted, modes, nil
}

type changedSide struct {
	path    string
	section int
}

// rejectWithheldCode fails the target when Git classified a changed file as
// binary but the file looks like code. Its content would be withheld from the
// reviewer while the review could still report clean.
func (collector *Collector) rejectWithheldCode(ctx context.Context, root string, plan *targetPlan, sides []changedSide, deleted []deletedBlob, modes map[changedSide]string) error {
	removed := make(map[changedSide]struct{}, len(deleted))
	for _, blob := range deleted {
		removed[changedSide{path: blob.path, section: blob.section}] = struct{}{}
	}
	var live []changedSide
	var objects bytes.Buffer
	for _, side := range sides {
		if _, gone := removed[side]; gone {
			continue
		}
		if err := protocolPath(side.path); err != nil {
			return fmt.Errorf("changed path %q: %w", side.path, err)
		}
		live = append(live, side)
		if !plan.local || side.section != 1 {
			object := ":" + side.path
			if !plan.local {
				object = plan.newRevision + ":" + side.path
			}
			_, _ = objects.WriteString(object + "\n")
		}
	}
	// One cat-file session reads every committed or staged side.
	blobs := &batchHeadWriter{limit: binarySniffBytes}
	if objects.Len() > 0 {
		if err := collector.git.runSandboxTo(ctx, root, plan.sandbox, objects.Bytes(), blobs, "cat-file", "--batch"); err != nil {
			return fmt.Errorf("read changed binary blobs: %w", err)
		}
		if blobs.err != nil {
			return blobs.err
		}
		if blobs.remaining != 0 || len(blobs.header) != 0 || len(blobs.heads) != bytes.Count(objects.Bytes(), []byte{'\n'}) {
			return fmt.Errorf("git returned incomplete binary blob output")
		}
	}
	next := 0
	for _, side := range live {
		var head []byte
		if plan.local && side.section == 1 {
			file, _, err := openRegularFile(root, side.path)
			if err != nil {
				return fmt.Errorf("read changed file %q: %w", side.path, err)
			}
			head, err = io.ReadAll(io.LimitReader(file, binarySniffBytes))
			_ = file.Close()
			if err != nil {
				return fmt.Errorf("read changed file %q: %w", side.path, err)
			}
		} else {
			if next >= len(blobs.heads) {
				return fmt.Errorf("git returned too few binary blobs")
			}
			head = blobs.heads[next]
			next++
		}
		if reason := withheldCodeReason(side.path, modes[side] == "100755", head); reason != "" {
			return fmt.Errorf("changed file %q %s, but Git treats it as binary, so its content would be withheld from review", side.path, reason)
		}
	}
	return nil
}

// withheldCodeReason reports why a file Git treats as binary may be code.
// Metadata checks hold even when junk bytes defeat the content check.
func withheldCodeReason(name string, executable bool, head []byte) string {
	switch _, source := sourceExtensions[strings.ToLower(path.Ext(name))]; {
	case bytes.IndexByte(head, 0) < 0:
		// Git calls the pair binary only because the old side was.
		return "is text"
	case textWithNULs(head):
		return "is text with NUL bytes"
	case bytes.HasPrefix(head, []byte("#!")):
		return "starts with #!"
	case source:
		return "has a source extension"
	case executable:
		return "is executable"
	}
	return ""
}

// textWithNULs reports whether content is valid UTF-8 apart from a few NUL
// bytes. UTF-16 text and real binaries stay binary.
func textWithNULs(head []byte) bool {
	nuls := bytes.Count(head, []byte{0})
	if nuls == 0 || nuls*8 >= len(head) {
		return false
	}
	text := bytes.ReplaceAll(head, []byte{0}, nil)
	if len(head) == binarySniffBytes {
		// The sniff window may end inside a multi-byte character.
		for trim := 0; trim < utf8.UTFMax && len(text) > 0 && !utf8.Valid(text); trim++ {
			text = text[:len(text)-1]
		}
	}
	return utf8.Valid(text)
}

// batchHeadWriter parses cat-file --batch output and keeps the first limit
// bytes of each blob.
type batchHeadWriter struct {
	limit     int
	heads     [][]byte
	header    []byte
	remaining int64
	err       error
}

func (writer *batchHeadWriter) Write(data []byte) (int, error) {
	consumed := len(data)
	for len(data) > 0 && writer.err == nil {
		if writer.remaining > 0 {
			take := min(int64(len(data)), writer.remaining)
			// The last byte of each object is cat-file's record newline.
			content := min(take, writer.remaining-1)
			head := &writer.heads[len(writer.heads)-1]
			if room := writer.limit - len(*head); room > 0 && content > 0 {
				*head = append(*head, data[:min(int64(room), content)]...)
			}
			writer.remaining -= take
			data = data[take:]
			continue
		}
		newline := bytes.IndexByte(data, '\n')
		if newline < 0 {
			writer.header = append(writer.header, data...)
			if len(writer.header) > 4096 {
				writer.err = fmt.Errorf("git returned an oversized blob header")
			}
			break
		}
		header := string(append(writer.header, data[:newline]...))
		writer.header = writer.header[:0]
		data = data[newline+1:]
		fields := strings.Fields(header)
		if len(fields) != 3 || fields[1] != "blob" {
			writer.err = fmt.Errorf("git returned an unexpected object for a changed binary file")
			break
		}
		size, err := strconv.ParseInt(fields[2], 10, 64)
		if err != nil || size < 0 {
			writer.err = fmt.Errorf("git returned an invalid blob size")
			break
		}
		writer.heads = append(writer.heads, nil)
		writer.remaining = size + 1
	}
	return consumed, nil
}

func diffCommands(plan *targetPlan, format ...string) [][]string {
	common := []string{"diff", "--no-ext-diff", "--no-textconv", "--no-renames"}
	common = append(common, format...)
	if plan.local {
		staged := append(append([]string(nil), common...), "--cached", plan.oldRevision, "--")
		unstaged := append(append([]string(nil), common...), "--")
		return [][]string{staged, unstaged}
	}
	arguments := append(append([]string(nil), common...), plan.oldRevision)
	if plan.newRevision != "" {
		arguments = append(arguments, plan.newRevision)
	}
	return [][]string{append(arguments, "--")}
}

func gitlinkMode(mode string) bool {
	return mode == "160000"
}

func (collector *Collector) untrackedFiles(ctx context.Context, root string, plan *targetPlan, budget *byteBudget) (map[string][]byte, map[string][]byte, error) {
	files := map[string][]byte{}
	binaries := map[string][]byte{}
	parser := &nulRecordWriter{handle: func(rawPath []byte) error {
		path := string(rawPath)
		if err := protocolPath(path); err != nil {
			return fmt.Errorf("untracked path %q: %w", path, err)
		}
		if !reviewablePath(path) {
			return fmt.Errorf("sensitive path %q is not reviewable", path)
		}
		if !budget.Exceeded() {
			summary, binary, err := summarizeBinaryFile(ctx, root, path)
			if errors.Is(err, errWithheldCode) && environmentFilePath(path) {
				return fmt.Errorf("binary environment file %q is not reviewable", path)
			}
			if err != nil {
				return fmt.Errorf("read untracked file %q: %w", path, err)
			}
			if binary {
				if environmentFilePath(path) {
					return fmt.Errorf("binary environment file %q is not reviewable", path)
				}
				budget.Add("binary:"+path, int64(len(summary)))
				budget.AddFraming(sectionFramingBytes("UNTRUSTED-BINARY-FILE", path, int64(len(summary))))
				if !budget.Exceeded() {
					binaries[path] = summary
				}
				return nil
			}
		}
		content, size, err := budget.Read(root, path, "untracked:"+path)
		if err != nil {
			return fmt.Errorf("read untracked file %q: %w", path, err)
		}
		budget.AddFraming(sectionFramingBytes("UNTRUSTED-UNTRACKED-FILE", path, size))
		if !budget.Exceeded() {
			files[path] = content
		}
		return nil
	}}
	if err := collector.git.runSandboxWithAttributesTo(ctx, root, plan.sandbox, nil, parser, "ls-files", "--others", "--exclude-standard", "-z"); err != nil {
		return nil, nil, fmt.Errorf("list untracked files: %w", err)
	}
	return files, binaries, parser.Err()
}

func (collector *Collector) deletedFiles(ctx context.Context, root string, plan *targetPlan, blobs []deletedBlob, budget *byteBudget) (map[string][]byte, error) {
	files := map[string][]byte{}
	if len(blobs) == 0 {
		return files, nil
	}
	var checkInput bytes.Buffer
	seen := make(map[string]struct{})
	for _, blob := range blobs {
		if err := protocolPath(blob.path); err != nil {
			return nil, fmt.Errorf("deleted path %q: %w", blob.path, err)
		}
		if !reviewablePath(blob.path) {
			return nil, fmt.Errorf("sensitive path %q is not reviewable", blob.path)
		}
		if !validObjectID(blob.oid) {
			return nil, fmt.Errorf("deleted path %q has invalid blob ID", blob.path)
		}
		if _, exists := seen[blob.path]; exists {
			return nil, fmt.Errorf("deleted path %q appears more than once", blob.path)
		}
		seen[blob.path] = struct{}{}
		_, _ = checkInput.WriteString(blob.oid)
		_ = checkInput.WriteByte('\n')
	}
	materials := make([]deletedBlobMaterial, 0, len(blobs))
	metadataIndex := 0
	metadata := &lineRecordWriter{handle: func(record []byte) error {
		if metadataIndex >= len(blobs) {
			return fmt.Errorf("git returned excess deleted blob metadata")
		}
		blob := blobs[metadataIndex]
		metadataIndex++
		fields := strings.Fields(string(record))
		if len(fields) != 3 || fields[0] != blob.oid || fields[1] != "blob" {
			return fmt.Errorf("git returned invalid deleted blob metadata for %q", blob.path)
		}
		size, err := strconv.ParseInt(fields[2], 10, 64)
		if err != nil || size < 0 {
			return fmt.Errorf("git returned invalid deleted blob size for %q", blob.path)
		}
		budget.Add("deleted:"+blob.path, size)
		budget.AddFraming(sectionFramingBytes("UNTRUSTED-DELETED-FILE", blob.path, size))
		if !budget.Exceeded() {
			materials = append(materials, deletedBlobMaterial{deletedBlob: blob, size: size})
		}
		return nil
	}}
	if err := collector.git.runSandboxTo(ctx, root, plan.sandbox, checkInput.Bytes(), metadata, "cat-file", "--batch-check=%(objectname) %(objecttype) %(objectsize)"); err != nil {
		return nil, fmt.Errorf("inspect deleted blobs: %w", err)
	}
	if err := metadata.Err(); err != nil {
		return nil, err
	}
	if metadataIndex != len(blobs) {
		return nil, fmt.Errorf("git returned incomplete deleted blob metadata")
	}
	if len(materials) == 0 {
		return files, nil
	}
	var contentInput bytes.Buffer
	for _, material := range materials {
		_, _ = contentInput.WriteString(material.oid)
		_ = contentInput.WriteByte('\n')
	}
	content := newDeletedBatchWriter(materials, files)
	if err := collector.git.runSandboxTo(ctx, root, plan.sandbox, contentInput.Bytes(), content, "cat-file", "--batch"); err != nil {
		return nil, fmt.Errorf("read deleted blobs: %w", err)
	}
	if err := content.Err(); err != nil {
		return nil, err
	}
	return files, nil
}

type deletedBlobMaterial struct {
	deletedBlob
	size int64
}

type deletedBatchWriter struct {
	materials []deletedBlobMaterial
	files     map[string][]byte
	index     int
	header    []byte
	content   []byte
	remaining int64
	err       error
}

func newDeletedBatchWriter(materials []deletedBlobMaterial, files map[string][]byte) *deletedBatchWriter {
	return &deletedBatchWriter{materials: materials, files: files, remaining: -1}
}

func (writer *deletedBatchWriter) Write(data []byte) (int, error) {
	input := data
	for len(input) > 0 && writer.err == nil {
		if writer.index >= len(writer.materials) {
			writer.err = fmt.Errorf("git returned excess deleted blob content")
			break
		}
		material := writer.materials[writer.index]
		if writer.remaining < 0 {
			newline := bytes.IndexByte(input, '\n')
			if newline < 0 {
				if len(writer.header)+len(input) > 512 {
					writer.err = fmt.Errorf("git deleted blob header exceeds safe maximum")
					break
				}
				writer.header = append(writer.header, input...)
				break
			}
			writer.header = append(writer.header, input[:newline]...)
			input = input[newline+1:]
			fields := strings.Fields(string(writer.header))
			if len(fields) != 3 || fields[0] != material.oid || fields[1] != "blob" || fields[2] != strconv.FormatInt(material.size, 10) {
				writer.err = fmt.Errorf("git returned invalid deleted blob header for %q", material.path)
				break
			}
			writer.content = make([]byte, 0, int(material.size))
			writer.remaining = material.size
			writer.header = writer.header[:0]
		}
		if writer.remaining > 0 {
			consume := int64(len(input))
			if consume > writer.remaining {
				consume = writer.remaining
			}
			writer.content = append(writer.content, input[:int(consume)]...)
			input = input[int(consume):]
			writer.remaining -= consume
		}
		if writer.remaining == 0 && len(input) > 0 {
			if input[0] != '\n' {
				writer.err = fmt.Errorf("git returned incomplete deleted blob %q", material.path)
				break
			}
			input = input[1:]
			if !utf8.Valid(writer.content) || bytes.IndexByte(writer.content, 0) >= 0 {
				writer.err = fmt.Errorf("deleted blob %q contains binary or invalid UTF-8 content", material.path)
				break
			}
			writer.files[material.path] = writer.content
			writer.content = nil
			writer.remaining = -1
			writer.index++
		}
	}
	return len(data), nil
}

func (writer *deletedBatchWriter) Err() error {
	if writer.err != nil {
		return writer.err
	}
	if writer.index != len(writer.materials) || writer.remaining != -1 || len(writer.header) != 0 {
		return fmt.Errorf("git returned incomplete deleted blob content")
	}
	return nil
}

func (collector *Collector) sourceStateHash(ctx context.Context, root string, plan *targetPlan, contexts, untrackedEnvironment map[string][]byte) (string, error) {
	if !plan.local {
		return immutableStateHash(ctx, plan, contexts)
	}
	return collector.localSourceStateHash(ctx, root, plan, untrackedEnvironment)
}

func immutableStateHash(ctx context.Context, plan *targetPlan, contexts map[string][]byte) (string, error) {
	hash := sha256.New()
	sections := []struct {
		label string
		value string
	}{
		{label: "mode", value: string(plan.target.Mode)},
		{label: "requested-base", value: plan.requestedBaseRevision},
		{label: "merge-base", value: plan.target.BaseRevision},
		{label: "head", value: plan.target.HeadRevision},
		{label: "commit", value: plan.target.CommitRevision},
	}
	for _, section := range sections {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		value := section.value
		if err := hashSourceSection(hash, section.label, func(output io.Writer) error {
			_, err := io.WriteString(output, value)
			return err
		}); err != nil {
			return "", fmt.Errorf("fingerprint %s: %w", section.label, err)
		}
	}
	for _, path := range sortedKeys(contexts) {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		path := path
		content := contexts[path]
		if err := hashSourceSection(hash, "context-path", func(output io.Writer) error {
			_, err := io.WriteString(output, path)
			return err
		}); err != nil {
			return "", fmt.Errorf("fingerprint context path: %w", err)
		}
		if err := hashSourceSection(hash, "context-content", func(output io.Writer) error {
			return writeContextHash(ctx, output, content)
		}); err != nil {
			return "", fmt.Errorf("fingerprint context %q: %w", path, err)
		}
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return "sha256:" + hex.EncodeToString(hash.Sum(nil)), nil
}

func writeContextHash(ctx context.Context, output io.Writer, content []byte) error {
	const chunkBytes = 64 << 10
	for len(content) > 0 {
		if err := ctx.Err(); err != nil {
			return err
		}
		length := min(len(content), chunkBytes)
		if _, err := output.Write(content[:length]); err != nil {
			return err
		}
		content = content[length:]
	}
	return ctx.Err()
}

// The local state hash covers raw index, diff, and untracked environment bytes
// while the payload carries their redaction. The hash reaches the provider; a
// secret key stops it from confirming guessed values. This per-process key
// stands in when no persistent Options.StateKey is supplied.
var environmentStateKey = func() []byte {
	key := make([]byte, 32)
	_, _ = rand.Read(key)
	return key
}()

func (collector *Collector) localSourceStateHash(ctx context.Context, root string, plan *targetPlan, untrackedEnvironment map[string][]byte) (string, error) {
	headRevision, unborn, err := collector.resolveHEAD(ctx, root)
	if err != nil {
		return "", err
	}
	head := []byte(headRevision)
	worktreeBase := headRevision
	if unborn {
		head = []byte("unborn")
		worktreeBase = plan.attributes
	}
	hash := sha256.New()
	if err := hashSourceSection(hash, "key", func(output io.Writer) error {
		key := collector.stateKey
		if len(key) == 0 {
			key = environmentStateKey
		}
		_, err := output.Write(key)
		return err
	}); err != nil {
		return "", fmt.Errorf("fingerprint key: %w", err)
	}
	if err := hashSourceSection(hash, "head", func(output io.Writer) error {
		_, err := output.Write(head)
		return err
	}); err != nil {
		return "", fmt.Errorf("fingerprint head: %w", err)
	}
	if err := hashSourceSection(hash, "index", func(output io.Writer) (returnErr error) {
		index, err := os.Open(filepath.Join(plan.sandbox.directory, "index"))
		if errors.Is(err, os.ErrNotExist) {
			_, err = io.WriteString(output, "no-index")
			return err
		}
		if err != nil {
			return err
		}
		defer func() {
			if closeErr := index.Close(); closeErr != nil {
				returnErr = errors.Join(returnErr, closeErr)
			}
		}()
		_, err = io.Copy(output, index)
		return err
	}); err != nil {
		return "", fmt.Errorf("fingerprint copied index: %w", err)
	}
	if err := hashSourceSection(hash, "status", func(output io.Writer) error {
		return collector.git.runSandboxWithAttributesTo(ctx, root, plan.sandbox, nil, output, "status", "--porcelain=v2", "-z", "--untracked-files=all")
	}); err != nil {
		return "", fmt.Errorf("fingerprint worktree: %w", err)
	}
	fingerprintPlan := &targetPlan{oldRevision: worktreeBase, local: true, sandbox: plan.sandbox, attributes: plan.attributes}
	for index, arguments := range diffCommands(fingerprintPlan, "--binary", "--full-index") {
		if err := hashSourceSection(hash, "diff-"+strconv.Itoa(index), func(output io.Writer) error {
			return collector.git.runSandboxWithAttributesTo(ctx, root, plan.sandbox, nil, output, arguments...)
		}); err != nil {
			return "", fmt.Errorf("fingerprint tracked worktree: %w", err)
		}
	}
	if len(untrackedEnvironment) > 0 {
		if err := hashSourceSection(hash, "untracked-environment", func(output io.Writer) error {
			for _, path := range sortedKeys(untrackedEnvironment) {
				content := untrackedEnvironment[path]
				_, _ = fmt.Fprintf(output, "%d:%s%d:", len(path), path, len(content))
				_, _ = output.Write(content)
			}
			return nil
		}); err != nil {
			return "", fmt.Errorf("fingerprint untracked environment files: %w", err)
		}
	}
	if err := hashSourceSection(hash, "config", func(output io.Writer) error {
		return collector.git.runConfiguredTo(ctx, root, nil, output, "", nil, "config", "list", "--local", "--null", "--includes")
	}); err != nil {
		return "", fmt.Errorf("fingerprint repository Git config: %w", err)
	}
	return "sha256:" + hex.EncodeToString(hash.Sum(nil)), nil
}

type sourceSectionWriter struct {
	hash  io.Writer
	bytes uint64
}

func (writer *sourceSectionWriter) Write(data []byte) (int, error) {
	written, err := writer.hash.Write(data)
	writer.bytes += uint64(written)
	return written, err
}

func hashSourceSection(destination io.Writer, label string, write func(io.Writer) error) error {
	sectionHash := sha256.New()
	section := &sourceSectionWriter{hash: sectionHash}
	if err := write(section); err != nil {
		return err
	}
	var frame [12]byte
	binary.BigEndian.PutUint32(frame[:4], uint32(len(label)))
	binary.BigEndian.PutUint64(frame[4:], section.bytes)
	if _, err := destination.Write(frame[:]); err != nil {
		return err
	}
	if _, err := io.WriteString(destination, label); err != nil {
		return err
	}
	_, err := destination.Write(sectionHash.Sum(nil))
	return err
}

func (collector *Collector) validateRepositoryConfig(ctx context.Context, root string) error {
	output, err := collector.git.run(ctx, root, nil, metadataLimit, "config", "list", "--local", "--name-only", "--null", "--includes")
	if err != nil {
		return fmt.Errorf("inspect repository Git config: %w", err)
	}
	for _, rawName := range bytes.Split(output, []byte{0}) {
		name := strings.ToLower(string(rawName))
		if name == "core.excludesfile" {
			return fmt.Errorf("repository Git config contains unsupported external excludes file")
		}
		if strings.HasPrefix(name, "filter.") || (strings.HasPrefix(name, "diff.") && (strings.HasSuffix(name, ".command") || strings.HasSuffix(name, ".textconv"))) {
			return fmt.Errorf("repository Git config contains executable filter or diff driver %q", name)
		}
	}
	return nil
}

func readContainedFile(root, relative string, retainLimit int64) ([]byte, int64, error) {
	if err := protocolPath(relative); err != nil {
		return nil, 0, err
	}
	file, before, err := openRegularFile(root, relative)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = file.Close() }()
	if before.Size() > retainLimit {
		after, err := file.Stat()
		if err != nil {
			return nil, 0, err
		}
		if !os.SameFile(before, after) || before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) {
			return nil, 0, fmt.Errorf("file changed while counting")
		}
		return nil, before.Size(), nil
	}
	content, err := io.ReadAll(io.LimitReader(file, retainLimit+1))
	if err != nil {
		return nil, 0, err
	}
	after, err := file.Stat()
	if err != nil {
		return nil, 0, err
	}
	if !os.SameFile(before, after) || before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) {
		return nil, 0, fmt.Errorf("file changed while reading")
	}
	if int64(len(content)) != before.Size() {
		return nil, 0, fmt.Errorf("incomplete file read")
	}
	if !utf8.Valid(content) || bytes.IndexByte(content, 0) >= 0 {
		return nil, 0, fmt.Errorf("binary or invalid UTF-8 content")
	}
	return content, before.Size(), nil
}

var errWithheldCode = errors.New("code in a binary file would be withheld from review")

// binarySniffBytes matches Git's buffer_is_binary heuristic: a NUL byte in the
// first 8000 bytes marks the content as binary.
const binarySniffBytes = 8000

func summarizeBinaryFile(ctx context.Context, root, relative string) ([]byte, bool, error) {
	if err := protocolPath(relative); err != nil {
		return nil, false, err
	}
	file, before, err := openRegularFile(root, relative)
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = file.Close() }()
	head := make([]byte, binarySniffBytes)
	read, err := io.ReadFull(file, head)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return nil, false, err
	}
	if bytes.IndexByte(head[:read], 0) < 0 {
		return nil, false, nil
	}
	if reason := withheldCodeReason(relative, before.Mode().Perm()&0o111 != 0, head[:read]); reason != "" {
		return nil, false, fmt.Errorf("file %s: %w", reason, errWithheldCode)
	}
	if before.Size() > MaximumMaxBytes {
		return nil, false, fmt.Errorf("binary file of %d bytes exceeds hashing limit of %d bytes", before.Size(), MaximumMaxBytes)
	}
	hash := sha256.New()
	_, _ = hash.Write(head[:read])
	var rest int64
	chunk := make([]byte, 64<<10)
	for {
		if err := ctx.Err(); err != nil {
			return nil, false, err
		}
		count, err := file.Read(chunk)
		_, _ = hash.Write(chunk[:count])
		rest += int64(count)
		if rest > MaximumMaxBytes {
			return nil, false, fmt.Errorf("file changed while reading")
		}
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, false, err
		}
	}
	after, err := file.Stat()
	if err != nil {
		return nil, false, err
	}
	if !os.SameFile(before, after) || before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) || int64(read)+rest != before.Size() {
		return nil, false, fmt.Errorf("file changed while reading")
	}
	summary := fmt.Sprintf("binary content omitted; %d bytes; sha256:%s", before.Size(), hex.EncodeToString(hash.Sum(nil)))
	return []byte(summary), true, nil
}

func protocolPath(value string) error {
	return protocol.ValidatePath(value)
}

func sensitivePath(value string) bool {
	lower := strings.ToLower(value)
	base := path.Base(lower)
	if base == ".env" || strings.HasPrefix(base, ".env.") || base == ".envrc" || base == ".npmrc" || base == ".pypirc" || base == ".netrc" || base == ".pgpass" || base == ".git-credentials" || base == "credentials.json" || base == "id_rsa" || base == "id_ed25519" || base == "id_ecdsa" || base == "id_dsa" || strings.HasSuffix(base, ".tfstate") || strings.HasSuffix(base, ".tfstate.backup") {
		return true
	}
	switch path.Ext(base) {
	case ".pem", ".key", ".p12", ".pfx", ".jks", ".keystore":
		return true
	default:
		return false
	}
}

func environmentTemplatePath(value string) bool {
	base := strings.ToLower(path.Base(value))
	if !strings.HasPrefix(base, ".env.") {
		return false
	}
	return strings.HasSuffix(base, ".example") || strings.HasSuffix(base, ".sample") || strings.HasSuffix(base, ".template")
}

func lineCount(content []byte) (int, error) {
	lines := bytes.Count(content, []byte{'\n'})
	if len(content) == 0 || content[len(content)-1] != '\n' {
		lines++
	}
	if lines < 1 {
		return 1, nil
	}
	if lines > protocol.MaxLineNumber {
		return 0, fmt.Errorf("line count exceeds protocol maximum")
	}
	return lines, nil
}
