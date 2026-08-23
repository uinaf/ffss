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
	gitMetadataPath   string
	gitMetadata       *boundaryIdentity
	trustedBoundaries *trustedexec.RepositoryBoundarySet
	gitPath           string
	gitIdentity       *executableIdentity
}

type executableIdentity struct {
	info   os.FileInfo
	digest [sha256.Size]byte
}

type boundaryIdentity struct {
	info      os.FileInfo
	digest    [sha256.Size]byte
	hasDigest bool
	target    *boundaryIdentity
}

func Resolve(ctx context.Context, options Options) (*Context, error) {
	requested := options.Path
	if requested == "" {
		requested = "."
	}
	absolute, resolved, err := resolveRequested(requested)
	if err != nil {
		return nil, err
	}
	requestedInfo, err := os.Stat(resolved)
	if err != nil {
		return nil, fmt.Errorf("inspect requested repository path: %w", err)
	}
	expectedRoot, expectedRootInfo, gitMetadataPath, gitMetadata, err := discoverWorktreeRoot(resolved)
	if err != nil {
		return nil, err
	}
	trustedBoundaries, err := trustedexec.CaptureRepositoryBoundariesForPaths(absolute, resolved)
	if err != nil {
		return nil, err
	}
	if err := validateRepositoryIdentity(absolute, resolved, requestedInfo, expectedRoot, expectedRootInfo, gitMetadataPath, gitMetadata); err != nil {
		return nil, err
	}
	environment := options.Environment
	if environment == nil {
		environment = os.Environ()
	}
	gitProbe := trustedexec.GitProbe(os.TempDir())
	var gitIdentity *executableIdentity
	gitPath, err := trustedexec.ResolveWithBoundaries(
		ctx,
		"git",
		options.GitPath,
		trustedBoundaries,
		environment,
		func(ctx context.Context, path string) error {
			if err := validateRepositoryIdentity(absolute, resolved, requestedInfo, expectedRoot, expectedRootInfo, gitMetadataPath, gitMetadata); err != nil {
				return trustedexec.AbortCheck(err)
			}
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
	if err := validateRepositoryIdentity(absolute, resolved, requestedInfo, expectedRoot, expectedRootInfo, gitMetadataPath, gitMetadata); err != nil {
		return nil, err
	}
	output, err := queryRepositoryRoot(ctx, gitPath, gitIdentity, absolute)
	if err != nil {
		return nil, err
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
	rootInfo, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("inspect repository root: %w", err)
	}
	if !os.SameFile(expectedRootInfo, rootInfo) {
		return nil, fmt.Errorf("repository root changed during discovery")
	}
	if err := validateRepositoryIdentity(absolute, resolved, requestedInfo, expectedRoot, expectedRootInfo, gitMetadataPath, gitMetadata); err != nil {
		return nil, err
	}
	repository := &Context{
		requestedAbsolute: absolute,
		requestedResolved: resolved,
		requestedInfo:     requestedInfo,
		root:              root,
		rootInfo:          expectedRootInfo,
		gitMetadataPath:   gitMetadataPath,
		gitMetadata:       gitMetadata,
		trustedBoundaries: trustedBoundaries,
		gitPath:           gitPath,
		gitIdentity:       gitIdentity,
	}
	if err := repository.ValidateGit(ctx); err != nil {
		return nil, err
	}
	return repository, nil
}

func queryRepositoryRoot(ctx context.Context, gitPath string, gitIdentity *executableIdentity, repository string) ([]byte, error) {
	if err := validateExecutableIdentity(ctx, gitPath, gitIdentity); err != nil {
		return nil, fmt.Errorf("validate trusted Git executable before root discovery: %w", err)
	}
	return runRepositoryRootQuery(ctx, gitPath, repository)
}

func runRepositoryRootQuery(ctx context.Context, executable, repository string) ([]byte, error) {
	command := exec.CommandContext(ctx, executable, "-C", repository, "-c", "core.hooksPath=/dev/null", "rev-parse", "--show-toplevel")
	command.Dir = os.TempDir()
	command.Env = trustedexec.GitEnvironment()
	output, err := command.Output()
	if ctxErr := ctx.Err(); ctxErr != nil {
		return nil, ctxErr
	}
	if err != nil {
		return nil, fmt.Errorf("find repository root: %w", err)
	}
	return output, nil
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

func (repository *Context) RequestedPath() string {
	if repository == nil {
		return ""
	}
	return repository.requestedAbsolute
}

func (repository *Context) TrustedBoundaries() *trustedexec.RepositoryBoundarySet {
	if repository == nil {
		return nil
	}
	return repository.trustedBoundaries
}

func (repository *Context) Validate() error {
	if repository == nil || repository.requestedAbsolute == "" || repository.requestedResolved == "" || repository.requestedInfo == nil || repository.root == "" || repository.rootInfo == nil || repository.gitMetadataPath == "" || repository.gitMetadata == nil || repository.trustedBoundaries == nil || repository.gitPath == "" || repository.gitIdentity == nil {
		return fmt.Errorf("repository context is unavailable")
	}
	return validateRepositoryIdentity(repository.requestedAbsolute, repository.requestedResolved, repository.requestedInfo, repository.root, repository.rootInfo, repository.gitMetadataPath, repository.gitMetadata)
}

func (repository *Context) ValidateGit(ctx context.Context) error {
	if err := repository.Validate(); err != nil {
		return err
	}
	return validateExecutableIdentity(ctx, repository.gitPath, repository.gitIdentity)
}

func validateExecutableIdentity(ctx context.Context, path string, expected *executableIdentity) error {
	current, err := captureExecutableIdentity(ctx, path)
	if err != nil {
		return fmt.Errorf("validate trusted Git executable identity: %w", err)
	}
	if !sameExecutableIdentity(expected, current) {
		return fmt.Errorf("trusted Git executable changed after validation")
	}
	return nil
}

func sameExecutableIdentity(left, right *executableIdentity) bool {
	return left != nil && right != nil && os.SameFile(left.info, right.info) && left.info.Mode() == right.info.Mode() && left.info.Size() == right.info.Size() && left.info.ModTime().Equal(right.info.ModTime()) && left.digest == right.digest
}

func sameBoundaryIdentity(left, right *boundaryIdentity) bool {
	if left == nil || right == nil || !os.SameFile(left.info, right.info) || left.info.Mode() != right.info.Mode() || left.hasDigest != right.hasDigest {
		return false
	}
	if !left.hasDigest {
		return sameOptionalBoundaryIdentity(left.target, right.target)
	}
	return left.info.Size() == right.info.Size() && left.info.ModTime().Equal(right.info.ModTime()) && left.digest == right.digest && sameOptionalBoundaryIdentity(left.target, right.target)
}

func sameOptionalBoundaryIdentity(left, right *boundaryIdentity) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return sameBoundaryIdentity(left, right)
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

func validateRepositoryIdentity(requestedAbsolute, requestedResolved string, requestedInfo os.FileInfo, root string, rootInfo os.FileInfo, gitMetadataPath string, gitMetadata *boundaryIdentity) error {
	if err := requireContained(root, requestedResolved); err != nil {
		return err
	}
	resolved, err := filepath.EvalSymlinks(requestedAbsolute)
	if err != nil {
		return fmt.Errorf("resolve requested repository path: %w", err)
	}
	if resolved != requestedResolved {
		return fmt.Errorf("repository argument changed; resolve a new repository context")
	}
	currentRequestedInfo, err := os.Stat(requestedResolved)
	if err != nil {
		return fmt.Errorf("inspect requested repository path: %w", err)
	}
	currentRootInfo, err := os.Stat(root)
	if err != nil {
		return fmt.Errorf("inspect repository root: %w", err)
	}
	if !os.SameFile(requestedInfo, currentRequestedInfo) || !os.SameFile(rootInfo, currentRootInfo) {
		return fmt.Errorf("repository worktree changed after validation")
	}
	currentGitMetadata, err := captureBoundaryIdentity(gitMetadataPath)
	if err != nil {
		return fmt.Errorf("inspect Git metadata boundary: %w", err)
	}
	if !sameBoundaryIdentity(gitMetadata, currentGitMetadata) {
		return fmt.Errorf("Git metadata boundary changed after validation")
	}
	return nil
}

func discoverWorktreeRoot(requested string) (string, os.FileInfo, string, *boundaryIdentity, error) {
	info, err := os.Stat(requested)
	if err != nil {
		return "", nil, "", nil, fmt.Errorf("inspect requested repository path: %w", err)
	}
	directory := requested
	if !info.IsDir() {
		directory = filepath.Dir(directory)
	}
	for {
		gitMetadataPath := filepath.Join(directory, ".git")
		if _, err := os.Lstat(gitMetadataPath); err == nil {
			root, err := filepath.EvalSymlinks(directory)
			if err != nil {
				return "", nil, "", nil, fmt.Errorf("resolve repository root: %w", err)
			}
			rootInfo, err := os.Stat(root)
			if err != nil {
				return "", nil, "", nil, fmt.Errorf("inspect repository root: %w", err)
			}
			gitMetadata, err := captureBoundaryIdentity(gitMetadataPath)
			if err != nil {
				return "", nil, "", nil, fmt.Errorf("inspect Git metadata boundary: %w", err)
			}
			return root, rootInfo, gitMetadataPath, gitMetadata, nil
		} else if !os.IsNotExist(err) {
			return "", nil, "", nil, fmt.Errorf("inspect repository boundary: %w", err)
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			return "", nil, "", nil, fmt.Errorf("reviewed path is not inside a Git worktree")
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

func captureBoundaryIdentity(path string) (*boundaryIdentity, error) {
	before, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if before.Mode().IsRegular() {
		return captureStableRegularBoundaryIdentity(path)
	}
	identity := &boundaryIdentity{info: before}
	var content []byte
	var resolvedTarget string
	switch {
	case before.Mode()&os.ModeSymlink != 0:
		var target string
		target, err = os.Readlink(path)
		content = []byte(target)
		if err == nil {
			if !filepath.IsAbs(target) {
				target = filepath.Join(filepath.Dir(path), target)
			}
			resolvedTarget, err = filepath.EvalSymlinks(target)
			if err == nil {
				identity.target, err = captureBoundaryIdentity(resolvedTarget)
			}
		}
	case before.IsDir():
	default:
		return nil, fmt.Errorf("Git metadata boundary is not a file, symlink, or directory")
	}
	if err != nil {
		return nil, err
	}
	after, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !os.SameFile(before, after) || before.Mode() != after.Mode() {
		return nil, fmt.Errorf("Git metadata boundary changed while reading identity")
	}
	identity.info = after
	if !after.IsDir() {
		identity.hasDigest = true
		identity.digest = sha256.Sum256(content)
	}
	if identity.target != nil {
		currentResolvedTarget, err := filepath.EvalSymlinks(path)
		if err != nil {
			return nil, err
		}
		if currentResolvedTarget != resolvedTarget {
			return nil, fmt.Errorf("Git metadata symlink target changed while reading identity")
		}
		currentTarget, err := captureBoundaryIdentity(currentResolvedTarget)
		if err != nil {
			return nil, err
		}
		if !sameBoundaryIdentity(identity.target, currentTarget) {
			return nil, fmt.Errorf("Git metadata symlink target changed while reading identity")
		}
	}
	return identity, nil
}

func captureStableRegularBoundaryIdentity(path string) (*boundaryIdentity, error) {
	first, err := captureRegularBoundaryIdentity(path)
	if err != nil {
		return nil, err
	}
	second, err := captureRegularBoundaryIdentity(path)
	if err != nil {
		return nil, err
	}
	if !sameBoundaryIdentity(first, second) {
		return nil, fmt.Errorf("Git metadata file changed between identity reads")
	}
	return second, nil
}

func captureRegularBoundaryIdentity(path string) (_ *boundaryIdentity, returnErr error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			returnErr = errors.Join(returnErr, closeErr)
		}
	}()
	before, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !before.Mode().IsRegular() || before.Size() > 64<<10 {
		return nil, fmt.Errorf("Git metadata file exceeds safe identity limit")
	}
	content, err := io.ReadAll(io.LimitReader(file, (64<<10)+1))
	if err != nil {
		return nil, err
	}
	if int64(len(content)) != before.Size() {
		return nil, fmt.Errorf("Git metadata file changed while reading identity")
	}
	after, err := file.Stat()
	if err != nil {
		return nil, err
	}
	pathInfo, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !os.SameFile(before, after) || !os.SameFile(after, pathInfo) || before.Mode() != after.Mode() || after.Mode() != pathInfo.Mode() || before.Size() != after.Size() || after.Size() != pathInfo.Size() || !before.ModTime().Equal(after.ModTime()) || !after.ModTime().Equal(pathInfo.ModTime()) {
		return nil, fmt.Errorf("Git metadata file changed while reading identity")
	}
	identity := &boundaryIdentity{info: after, digest: sha256.Sum256(content), hasDigest: true}
	if target, ok, err := gitDirectoryTarget(path, content); err != nil {
		return nil, err
	} else if ok {
		firstTarget, err := captureBoundaryIdentity(target)
		if err != nil {
			return nil, err
		}
		secondTarget, err := captureBoundaryIdentity(target)
		if err != nil {
			return nil, err
		}
		if !sameBoundaryIdentity(firstTarget, secondTarget) {
			return nil, fmt.Errorf("Git metadata target changed while reading identity")
		}
		identity.target = secondTarget
	}
	return identity, nil
}

func gitDirectoryTarget(path string, content []byte) (string, bool, error) {
	value := strings.TrimSpace(string(content))
	const prefix = "gitdir:"
	if !strings.HasPrefix(value, prefix) {
		return "", false, nil
	}
	target := strings.TrimSpace(strings.TrimPrefix(value, prefix))
	if target == "" {
		return "", false, fmt.Errorf("Git metadata file has an empty gitdir target")
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(path), target)
	}
	resolved, err := filepath.EvalSymlinks(target)
	if err != nil {
		return "", false, err
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", false, err
	}
	if !info.IsDir() {
		return "", false, fmt.Errorf("Git metadata gitdir target is not a directory")
	}
	return resolved, true, nil
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
