package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/uinaf/ffsstack/cli/slopmachine/internal/repo"
	"github.com/uinaf/ffsstack/cli/slopmachine/internal/store"
)

var (
	errUnsafeStatePath    = errors.New("unsafe state path")
	errInvalidStateConfig = errors.New("invalid state configuration")
)

type storageDocument struct {
	SchemaVersion int    `json:"schema_version"`
	Path          string `json:"path"`
	Source        string `json:"source"`
	Scope         string `json:"scope"`
	Exists        bool   `json:"exists"`
	GitIgnored    *bool  `json:"git_ignored"`
}

func resolveStorage(requireGitSafe bool) (storageDocument, error) {
	override := os.Getenv("SLOPMACHINE_DB")
	if override == "" {
		xdgDataHome := os.Getenv("XDG_DATA_HOME")
		if xdgDataHome != "" && !filepath.IsAbs(xdgDataHome) {
			return storageDocument{}, fmt.Errorf("%w: XDG_DATA_HOME must be an absolute path (got %q)", errInvalidStateConfig, xdgDataHome)
		}
		// HOME is only needed when XDG_DATA_HOME does not decide the base;
		// containers configured solely through XDG must not require it.
		home := ""
		if xdgDataHome == "" {
			var err error
			home, err = os.UserHomeDir()
			if err != nil {
				return storageDocument{}, err
			}
		}
		path, err := physicalPath(store.DefaultPath(xdgDataHome, home))
		if err != nil {
			return storageDocument{}, err
		}
		exists, err := pathExists(path)
		if err != nil {
			return storageDocument{}, fmt.Errorf("inspect database %q: %w", path, err)
		}
		doc := storageDocument{
			SchemaVersion: 1,
			Path:          path,
			Source:        "xdg-data",
			Scope:         "user",
			Exists:        exists,
		}
		// An absolute XDG_DATA_HOME can point inside the current worktree;
		// the default path earns no exemption from containment safety.
		cwd, err := os.Getwd()
		if err != nil {
			return storageDocument{}, err
		}
		_, root, repoErr := repo.Keys(cwd)
		if repoErr != nil {
			// Only "definitely not a repository" may skip containment;
			// a broken git inspection must fail closed, not fail open.
			if requireGitSafe && !errors.Is(repoErr, repo.ErrNotRepository) {
				return storageDocument{}, fmt.Errorf("inspect Git worktree for state safety: %w", repoErr)
			}
			return doc, nil
		}
		return applyGitSafety(doc, root, path, requireGitSafe)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return storageDocument{}, err
	}
	_, root, repoErr := repo.Keys(cwd)
	if !filepath.IsAbs(override) {
		if repoErr != nil {
			return storageDocument{}, fmt.Errorf("%w: resolve relative SLOPMACHINE_DB: %v", errInvalidStateConfig, repoErr)
		}
		override = filepath.Join(root, override)
	}
	path, err := physicalPath(override)
	if err != nil {
		return storageDocument{}, fmt.Errorf("resolve SLOPMACHINE_DB %q: %w", override, err)
	}
	exists, err := pathExists(path)
	if err != nil {
		return storageDocument{}, fmt.Errorf("inspect database %q: %w", path, err)
	}
	doc := storageDocument{
		SchemaVersion: 1,
		Path:          path,
		Source:        "environment",
		Scope:         "custom",
		Exists:        exists,
	}
	if repoErr != nil {
		if requireGitSafe && !errors.Is(repoErr, repo.ErrNotRepository) {
			return storageDocument{}, fmt.Errorf("inspect Git worktree for state safety: %w", repoErr)
		}
		return doc, nil
	}
	return applyGitSafety(doc, root, path, requireGitSafe)
}

// applyGitSafety enforces worktree containment for any resolved database
// path, override or default alike.
func applyGitSafety(doc storageDocument, root, path string, requireGitSafe bool) (storageDocument, error) {
	physicalRoot, err := physicalPath(root)
	if err != nil {
		return storageDocument{}, fmt.Errorf("resolve Git worktree root %q: %w", root, err)
	}
	inside, rel, err := pathWithin(physicalRoot, path)
	if err != nil {
		return storageDocument{}, err
	}
	if !inside {
		return doc, nil
	}

	doc.Scope = "repository"
	ignored, unsafePath, err := sqlitePathsIgnored(root, rel)
	if err != nil {
		return storageDocument{}, err
	}
	doc.GitIgnored = &ignored
	if requireGitSafe && !ignored {
		return storageDocument{}, fmt.Errorf(
			"%w: database path %q is inside Git worktree %q but %q is tracked or not ignored; use a path outside the worktree, or set SLOPMACHINE_DB=.slopmachine/slopmachine.sqlite after adding /.slopmachine/ to $(git rev-parse --git-path info/exclude)",
			errUnsafeStatePath, path, root, unsafePath,
		)
	}
	return doc, nil
}

func physicalPath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	current := filepath.Clean(abs)
	missing := make([]string, 0)
	for {
		_, err := os.Lstat(current)
		if err == nil {
			resolved, err := filepath.EvalSymlinks(current)
			if err != nil {
				return "", err
			}
			for i := len(missing) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, missing[i])
			}
			return filepath.Clean(resolved), nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", err
		}
		missing = append(missing, filepath.Base(current))
		current = parent
	}
}

func pathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}

func pathWithin(root, path string) (bool, string, error) {
	rootInfo, err := os.Stat(root)
	if err != nil {
		return false, "", fmt.Errorf("inspect worktree root %q: %w", root, err)
	}
	current := filepath.Clean(path)
	parts := make([]string, 0)
	for {
		info, statErr := os.Stat(current)
		if statErr == nil {
			if os.SameFile(rootInfo, info) {
				if len(parts) == 0 {
					return true, ".", nil
				}
				relative := make([]string, len(parts))
				for i := range parts {
					relative[len(parts)-1-i] = parts[i]
				}
				return true, filepath.Join(relative...), nil
			}
		} else if !errors.Is(statErr, os.ErrNotExist) {
			return false, "", fmt.Errorf("inspect database ancestor %q: %w", current, statErr)
		}
		parent := filepath.Dir(current)
		if parent == current {
			return false, path, nil
		}
		parts = append(parts, filepath.Base(current))
		current = parent
	}
}

func sqlitePathsIgnored(root, relativeDatabase string) (bool, string, error) {
	for _, candidate := range []string{relativeDatabase, relativeDatabase + "-wal", relativeDatabase + "-shm"} {
		tracked, err := gitPathTracked(root, candidate)
		if err != nil {
			return false, "", err
		}
		if tracked {
			return false, filepath.ToSlash(candidate), nil
		}
		ignored, err := gitCheckIgnored(root, candidate)
		if err != nil {
			return false, "", err
		}
		if !ignored {
			return false, filepath.ToSlash(candidate), nil
		}
	}
	return true, "", nil
}

func gitPathTracked(root, relativePath string) (bool, error) {
	cmd := exec.Command("git", "ls-files", "--error-unmatch", "--", filepath.ToSlash(relativePath))
	cmd.Dir = root
	err := cmd.Run()
	if err == nil {
		return true, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return false, nil
	}
	return false, fmt.Errorf("check Git tracking for %q: %w", relativePath, err)
}

func gitCheckIgnored(root, relativePath string) (bool, error) {
	cmd := exec.Command("git", "check-ignore", "--quiet", "--no-index", "--", filepath.ToSlash(relativePath))
	cmd.Dir = root
	err := cmd.Run()
	if err == nil {
		return true, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return false, nil
	}
	return false, fmt.Errorf("check Git exclusion for %q: %w", relativePath, err)
}

func cmdStorage(args []string, opts runOptions) int {
	if _, code := requireFlags("storage", args, opts); code != 0 {
		return code
	}
	doc, err := resolveStorage(false)
	if err != nil {
		code := 10
		switch {
		case errors.Is(err, errInvalidStateConfig):
			code = 2
		default:
			// Match databasePath: resolution failures outside the named
			// config kinds are recoverable state unavailability everywhere,
			// including this inspection command.
			err = fmt.Errorf("%w: %w", store.ErrStateUnavailable, err)
			code = 2
		}
		return writeFailure(opts, code, err)
	}
	if opts.json {
		if err := writeJSON(doc); err != nil {
			return writeFailure(opts, 10, err)
		}
		return 0
	}
	ignored := "n/a"
	if doc.GitIgnored != nil {
		ignored = fmt.Sprintf("%t", *doc.GitIgnored)
	}
	fmt.Fprintf(os.Stdout, "slopmachine storage path=%q source=%s scope=%s exists=%t git_ignored=%s\n", doc.Path, doc.Source, doc.Scope, doc.Exists, ignored)
	if doc.GitIgnored != nil && !*doc.GitIgnored {
		fmt.Fprintln(os.Stdout, "recovery=use a path outside the worktree, or set SLOPMACHINE_DB=.slopmachine/slopmachine.sqlite after adding /.slopmachine/ to $(git rev-parse --git-path info/exclude)")
	}
	return 0
}
