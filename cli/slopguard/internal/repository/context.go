package repository

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/uinaf/ffss/cli/slopguard/internal/trustedexec"
)

type Options struct {
	Path        string
	GitPath     string
	Environment []string
}

type Context struct {
	requestedAbsolute string
	requestedResolved string
	requestedInfo     os.FileInfo
	root              string
	rootInfo          os.FileInfo
	gitPath           string
	gitIdentity       *executableIdentity
}

type executableIdentity struct {
	info   os.FileInfo
	digest [sha256.Size]byte
}

func Resolve(ctx context.Context, options Options) (*Context, error) {
	requested := options.Path
	if requested == "" {
		requested = "."
	}
	environment := options.Environment
	if environment == nil {
		environment = os.Environ()
	}
	gitProbe := trustedexec.GitProbe(os.TempDir())
	var gitIdentity *executableIdentity
	gitPath, err := trustedexec.Resolve(
		ctx,
		"git",
		options.GitPath,
		requested,
		environment,
		func(ctx context.Context, path string) error {
			before, err := captureExecutableIdentity(ctx, path)
			if err != nil {
				return err
			}
			if err := gitProbe(ctx, path); err != nil {
				return err
			}
			after, err := captureExecutableIdentity(ctx, path)
			if err != nil {
				return err
			}
			if !sameExecutableIdentity(before, after) {
				return fmt.Errorf("trusted Git executable changed during capability probe")
			}
			gitIdentity = after
			return nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("find git: %w", err)
	}
	if gitIdentity == nil {
		return nil, fmt.Errorf("trusted Git executable identity is unavailable after capability probe")
	}
	absolute, resolved, err := resolveRequested(requested)
	if err != nil {
		return nil, err
	}
	requestedInfo, err := os.Stat(resolved)
	if err != nil {
		return nil, fmt.Errorf("inspect requested repository path: %w", err)
	}
	expectedRoot, expectedRootInfo, err := discoverWorktreeRoot(resolved)
	if err != nil {
		return nil, err
	}
	command := exec.CommandContext(ctx, gitPath, "-C", absolute, "-c", "core.hooksPath=/dev/null", "rev-parse", "--show-toplevel")
	command.Dir = os.TempDir()
	command.Env = trustedexec.GitEnvironment()
	output, err := command.Output()
	if ctxErr := ctx.Err(); ctxErr != nil {
		return nil, ctxErr
	}
	if err != nil {
		return nil, fmt.Errorf("find repository root: %w", err)
	}
	root := strings.TrimSpace(string(output))
	if root == "" || !filepath.IsAbs(root) {
		return nil, fmt.Errorf("git returned an invalid repository root")
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return nil, fmt.Errorf("resolve repository root: %w", err)
	}
	if err := requireContained(root, resolved); err != nil {
		return nil, err
	}
	if root != expectedRoot {
		return nil, fmt.Errorf("Git repository root changed during discovery")
	}
	currentRequestedInfo, err := os.Stat(resolved)
	if err != nil {
		return nil, fmt.Errorf("inspect requested repository path: %w", err)
	}
	if !os.SameFile(requestedInfo, currentRequestedInfo) {
		return nil, fmt.Errorf("requested repository path changed during root discovery")
	}
	rootInfo, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("inspect repository root: %w", err)
	}
	if !os.SameFile(expectedRootInfo, rootInfo) {
		return nil, fmt.Errorf("repository root changed during discovery")
	}
	repository := &Context{
		requestedAbsolute: absolute,
		requestedResolved: resolved,
		requestedInfo:     requestedInfo,
		root:              root,
		rootInfo:          rootInfo,
		gitPath:           gitPath,
		gitIdentity:       gitIdentity,
	}
	if err := repository.ValidateGit(ctx); err != nil {
		return nil, err
	}
	return repository, nil
}

func (repository *Context) Root() string {
	if repository == nil {
		return ""
	}
	return repository.root
}

func (repository *Context) GitPath() string {
	if repository == nil {
		return ""
	}
	return repository.gitPath
}

func (repository *Context) Validate() error {
	if repository == nil || repository.requestedAbsolute == "" || repository.requestedResolved == "" || repository.requestedInfo == nil || repository.root == "" || repository.rootInfo == nil || repository.gitPath == "" || repository.gitIdentity == nil {
		return fmt.Errorf("repository context is unavailable")
	}
	if err := requireContained(repository.root, repository.requestedResolved); err != nil {
		return err
	}
	resolved, err := filepath.EvalSymlinks(repository.requestedAbsolute)
	if err != nil {
		return fmt.Errorf("resolve requested repository path: %w", err)
	}
	if resolved != repository.requestedResolved {
		return fmt.Errorf("repository argument changed; resolve a new repository context")
	}
	requestedInfo, err := os.Stat(repository.requestedResolved)
	if err != nil {
		return fmt.Errorf("inspect requested repository path: %w", err)
	}
	rootInfo, err := os.Stat(repository.root)
	if err != nil {
		return fmt.Errorf("inspect repository root: %w", err)
	}
	if !os.SameFile(repository.requestedInfo, requestedInfo) || !os.SameFile(repository.rootInfo, rootInfo) {
		return fmt.Errorf("repository worktree changed after validation")
	}
	return nil
}

func (repository *Context) ValidateGit(ctx context.Context) error {
	if err := repository.Validate(); err != nil {
		return err
	}
	current, err := captureExecutableIdentity(ctx, repository.gitPath)
	if err != nil {
		return fmt.Errorf("validate trusted Git executable identity: %w", err)
	}
	expected := repository.gitIdentity
	if !sameExecutableIdentity(expected, current) {
		return fmt.Errorf("trusted Git executable changed after validation")
	}
	return nil
}

func sameExecutableIdentity(left, right *executableIdentity) bool {
	return left != nil && right != nil && os.SameFile(left.info, right.info) && left.info.Mode() == right.info.Mode() && left.info.Size() == right.info.Size() && left.info.ModTime().Equal(right.info.ModTime()) && left.digest == right.digest
}

func (repository *Context) ValidateRequested(path string) error {
	if err := repository.Validate(); err != nil {
		return err
	}
	if path == "" {
		path = "."
	}
	absolute, resolved, err := resolveRequested(path)
	if err != nil {
		return err
	}
	if absolute != repository.requestedAbsolute || resolved != repository.requestedResolved {
		return fmt.Errorf("repository argument changed; resolve a new repository context")
	}
	return requireContained(repository.root, resolved)
}

func resolveRequested(path string) (string, string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", "", fmt.Errorf("resolve repository path: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", "", fmt.Errorf("resolve requested repository path: %w", err)
	}
	return absolute, resolved, nil
}

func requireContained(root, requested string) error {
	relative, err := filepath.Rel(root, requested)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return fmt.Errorf("git worktree does not contain requested repository path")
	}
	return nil
}

func discoverWorktreeRoot(requested string) (string, os.FileInfo, error) {
	info, err := os.Stat(requested)
	if err != nil {
		return "", nil, fmt.Errorf("inspect requested repository path: %w", err)
	}
	directory := requested
	if !info.IsDir() {
		directory = filepath.Dir(directory)
	}
	for {
		if _, err := os.Lstat(filepath.Join(directory, ".git")); err == nil {
			root, err := filepath.EvalSymlinks(directory)
			if err != nil {
				return "", nil, fmt.Errorf("resolve repository root: %w", err)
			}
			rootInfo, err := os.Stat(root)
			if err != nil {
				return "", nil, fmt.Errorf("inspect repository root: %w", err)
			}
			return root, rootInfo, nil
		} else if !os.IsNotExist(err) {
			return "", nil, fmt.Errorf("inspect repository boundary: %w", err)
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			return "", nil, fmt.Errorf("reviewed path is not inside a Git worktree")
		}
		directory = parent
	}
}

func captureExecutableIdentity(ctx context.Context, path string) (_ *executableIdentity, returnErr error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			returnErr = errors.Join(returnErr, closeErr)
		}
	}()
	return captureOpenExecutableIdentity(ctx, path, file)
}

func captureOpenExecutableIdentity(ctx context.Context, path string, file *os.File) (*executableIdentity, error) {
	before, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !before.Mode().IsRegular() || before.Mode().Perm()&0o111 == 0 {
		return nil, fmt.Errorf("trusted Git executable is not an executable regular file")
	}
	hash := sha256.New()
	buffer := make([]byte, 64<<10)
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		read, readErr := file.Read(buffer)
		if read > 0 {
			if _, err := hash.Write(buffer[:read]); err != nil {
				return nil, err
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return nil, readErr
		}
	}
	after, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !os.SameFile(before, after) || before.Mode() != after.Mode() || before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) {
		return nil, fmt.Errorf("trusted Git executable changed while reading identity")
	}
	pathInfo, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	resolvedPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return nil, err
	}
	if resolvedPath != path {
		return nil, fmt.Errorf("trusted Git executable path changed while reading identity")
	}
	if !os.SameFile(after, pathInfo) || after.Mode() != pathInfo.Mode() || after.Size() != pathInfo.Size() || !after.ModTime().Equal(pathInfo.ModTime()) {
		return nil, fmt.Errorf("trusted Git executable path changed while reading identity")
	}
	identity := &executableIdentity{info: after}
	copy(identity.digest[:], hash.Sum(nil))
	return identity, nil
}
