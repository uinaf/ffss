package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type Runtime struct {
	Workspace string
	root      string
	env       []string
	removeAll func(string) error
	closeMu   sync.Mutex
}

func PrepareRuntime(effective Effective, parentEnvironment []string) (*Runtime, error) {
	if err := effective.Validate(); err != nil {
		return nil, err
	}
	root, err := os.MkdirTemp("", "slopguard-provider-")
	if err != nil {
		return nil, fmt.Errorf("create provider runtime: %w", err)
	}
	runtime := &Runtime{root: root, Workspace: filepath.Join(root, "workspace"), removeAll: os.RemoveAll}
	if err := os.Mkdir(runtime.Workspace, 0o700); err != nil {
		_ = os.RemoveAll(root)
		return nil, fmt.Errorf("create provider workspace: %w", err)
	}
	runtime.env = append([]string(nil), parentEnvironment...)
	return runtime, nil
}

func (runtime *Runtime) Environment() []string {
	return append([]string(nil), runtime.env...)
}

func (runtime *Runtime) Close() error {
	runtime.closeMu.Lock()
	defer runtime.closeMu.Unlock()
	if runtime.root == "" {
		return nil
	}
	removeAll := runtime.removeAll
	if removeAll == nil {
		removeAll = os.RemoveAll
	}
	if err := removeAll(runtime.root); err != nil {
		return fmt.Errorf("remove provider runtime: %w", err)
	}
	runtime.root = ""
	return nil
}
