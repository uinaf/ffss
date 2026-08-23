package repository

import (
	"context"
	"fmt"
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
	root              string
	gitPath           string
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
	gitPath, err := trustedexec.Resolve(
		ctx,
		"git",
		options.GitPath,
		requested,
		environment,
		trustedexec.GitProbe(os.TempDir()),
	)
	if err != nil {
		return nil, fmt.Errorf("find git: %w", err)
	}
	absolute, resolved, err := resolveRequested(requested)
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
	return &Context{
		requestedAbsolute: absolute,
		requestedResolved: resolved,
		root:              root,
		gitPath:           gitPath,
	}, nil
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
	if repository == nil || repository.requestedAbsolute == "" || repository.requestedResolved == "" || repository.root == "" || repository.gitPath == "" {
		return fmt.Errorf("repository context is unavailable")
	}
	return requireContained(repository.root, repository.requestedResolved)
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
