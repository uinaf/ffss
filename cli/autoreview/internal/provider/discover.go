package provider

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func discoverExecutable(name, repository string, environment []string) (string, error) {
	candidates, err := discoverExecutableCandidates(name, repository, environment)
	if err != nil {
		return "", err
	}
	return candidates[0], nil
}

func discoverExecutableCandidates(name, repository string, environment []string) ([]string, error) {
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("provider executable is required")
	}
	if strings.ContainsRune(name, filepath.Separator) {
		path, err := filepath.Abs(name)
		if err != nil {
			return nil, fmt.Errorf("resolve provider executable: %w", err)
		}
		validated, err := validateExecutable(path, repository)
		if err != nil {
			return nil, err
		}
		return []string{validated}, nil
	}
	candidates := make([]string, 0, 1)
	seen := make(map[string]struct{})
	pathValue := environmentValue(environment, "PATH")
	for _, directory := range filepath.SplitList(pathValue) {
		if !filepath.IsAbs(directory) {
			continue
		}
		candidate := filepath.Join(directory, name)
		path, err := validateExecutable(candidate, repository)
		if err != nil {
			continue
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		candidates = append(candidates, path)
	}
	if len(candidates) == 0 {
		return nil, fmt.Errorf("provider executable %q was not found outside the reviewed repository", name)
	}
	return candidates, nil
}

func validateExecutable(path, repository string) (string, error) {
	original, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(original)
	if err != nil {
		return "", err
	}
	resolved, err = filepath.Abs(resolved)
	if err != nil {
		return "", err
	}
	originalRoot, err := filepath.Abs(repository)
	if err != nil {
		return "", fmt.Errorf("resolve reviewed repository: %w", err)
	}
	root, err := filepath.EvalSymlinks(originalRoot)
	if err != nil {
		return "", fmt.Errorf("resolve reviewed repository: %w", err)
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve reviewed repository: %w", err)
	}
	if pathInside(originalRoot, original) || pathInside(root, original) || pathInside(root, resolved) {
		return "", fmt.Errorf("provider executable %q is inside the reviewed repository", resolved)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
		return "", fmt.Errorf("provider executable %q is not an executable regular file", resolved)
	}
	return resolved, nil
}

func pathInside(root, candidate string) bool {
	relative, err := filepath.Rel(root, candidate)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)
}

func environmentValue(environment []string, name string) string {
	for index := len(environment) - 1; index >= 0; index-- {
		key, value, found := strings.Cut(environment[index], "=")
		if found && key == name {
			return value
		}
	}
	return ""
}
