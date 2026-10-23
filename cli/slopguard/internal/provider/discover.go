package provider

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/uinaf/ffss/cli/slopguard/internal/protocol"
	"github.com/uinaf/ffss/cli/slopguard/internal/trustedexec"
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

func selectCompatibleExecutable(candidates []string, preflight func(string) (string, error)) (preparedExecutable, error) {
	var lastCapabilityError error
	for _, candidate := range candidates {
		version, err := preflight(candidate)
		if err == nil {
			return preparedExecutable{Path: candidate, Version: version}, nil
		}
		var failure *Error
		if !errors.As(err, &failure) || failure.Class != protocol.FailureCapability {
			return preparedExecutable{}, err
		}
		lastCapabilityError = err
	}
	if lastCapabilityError != nil {
		return preparedExecutable{}, lastCapabilityError
	}
	return preparedExecutable{}, fmt.Errorf("provider has no usable executable candidate")
}

func validateExecutable(path, repository string) (string, error) {
	resolved, err := trustedexec.ValidateOutsideRepository(path, repository)
	if err != nil {
		return "", fmt.Errorf("provider %w", err)
	}
	return resolved, nil
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
