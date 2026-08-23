package repository

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveProducesReusableValidatedContext(t *testing.T) {
	repository := testRepository(t)
	nested := filepath.Join(repository, "nested")
	if err := os.Mkdir(nested, 0o700); err != nil {
		t.Fatal(err)
	}
	gitPath, log, marker := testGitWrapper(t)
	resolved, err := Resolve(context.Background(), Options{Path: nested, GitPath: gitPath})
	if err != nil {
		t.Fatal(err)
	}
	expectedGit, err := filepath.EvalSymlinks(gitPath)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Root() != repository || resolved.GitPath() != expectedGit {
		t.Fatalf("context root=%q git=%q", resolved.Root(), resolved.GitPath())
	}
	if err := resolved.ValidateRequested(nested); err != nil {
		t.Fatal(err)
	}
	if err := resolved.ValidateRequested(repository); err == nil || !strings.Contains(err.Error(), "argument changed") {
		t.Fatalf("changed repository error = %v", err)
	}
	content, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	if lines := strings.Fields(string(content)); len(lines) != 2 || lines[0] != "probe" || lines[1] != "root" {
		t.Fatalf("Git invocations = %q", content)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("credential environment crossed Git boundary: %v", err)
	}
}

func TestValidateRequestedRejectsRetargetedSymlink(t *testing.T) {
	repository := testRepository(t)
	outside := t.TempDir()
	alias := filepath.Join(t.TempDir(), "repository")
	if err := os.Symlink(repository, alias); err != nil {
		t.Fatal(err)
	}
	gitPath, _, _ := testGitWrapper(t)
	resolved, err := Resolve(context.Background(), Options{Path: alias, GitPath: gitPath})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(alias); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, alias); err != nil {
		t.Fatal(err)
	}
	if err := resolved.Validate(); err == nil || !strings.Contains(err.Error(), "argument changed") {
		t.Fatalf("retargeted symlink Validate() error = %v", err)
	}
	if err := resolved.ValidateRequested(alias); err == nil || !strings.Contains(err.Error(), "argument changed") {
		t.Fatalf("retargeted symlink error = %v", err)
	}
}

func TestValidateGitRejectsInPlaceExecutableReplacement(t *testing.T) {
	repository := testRepository(t)
	gitPath, _, _ := testGitWrapper(t)
	resolved, err := Resolve(context.Background(), Options{Path: repository, GitPath: gitPath})
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(resolved.GitPath())
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(resolved.GitPath())
	if err != nil {
		t.Fatal(err)
	}
	mutated := strings.Replace(string(content), "probe", "qrobe", 1)
	if len(mutated) != len(content) || mutated == string(content) {
		t.Fatal("test mutation did not preserve executable size")
	}
	if err := os.WriteFile(resolved.GitPath(), []byte(mutated), info.Mode().Perm()); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(resolved.GitPath(), info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	if err := resolved.ValidateGit(context.Background()); err == nil || !strings.Contains(err.Error(), "changed after validation") {
		t.Fatalf("ValidateGit() error = %v", err)
	}
}

func TestValidateRequestedRejectsSamePathWorktreeReplacement(t *testing.T) {
	parent := t.TempDir()
	repository := filepath.Join(parent, "repository")
	if err := os.Mkdir(repository, 0o700); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("git", "init", "-q", "-b", "main")
	command.Dir = repository
	command.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, output)
	}
	gitPath, _, _ := testGitWrapper(t)
	resolved, err := Resolve(context.Background(), Options{Path: repository, GitPath: gitPath})
	if err != nil {
		t.Fatal(err)
	}
	backup := filepath.Join(parent, "original")
	if err := os.Rename(repository, backup); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(repository, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := resolved.ValidateRequested(repository); err == nil || !strings.Contains(err.Error(), "worktree changed") {
		t.Fatalf("ValidateRequested() error = %v", err)
	}
}

func TestCaptureExecutableIdentityRejectsAtomicPathReplacement(t *testing.T) {
	gitPath, _, _ := testGitWrapper(t)
	file, err := os.Open(gitPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = file.Close() }()
	replacement := filepath.Join(filepath.Dir(gitPath), "replacement")
	if err := os.WriteFile(replacement, []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(replacement, gitPath); err != nil {
		t.Fatal(err)
	}
	if _, err := captureOpenExecutableIdentity(context.Background(), gitPath, file); err == nil || !strings.Contains(err.Error(), "path changed") {
		t.Fatalf("captureOpenExecutableIdentity() error = %v", err)
	}
}

func TestResolveRejectsWorktreeReplacementDuringRootDiscovery(t *testing.T) {
	parent := t.TempDir()
	repository := filepath.Join(parent, "repository")
	if err := os.Mkdir(repository, 0o700); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("git", "init", "-q", "-b", "main")
	command.Dir = repository
	command.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, output)
	}
	nested := filepath.Join(repository, "nested")
	if err := os.Mkdir(nested, 0o700); err != nil {
		t.Fatal(err)
	}
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	realGit, err = filepath.Abs(realGit)
	if err != nil {
		t.Fatal(err)
	}
	wrapperDir := t.TempDir()
	wrapper := filepath.Join(wrapperDir, "git")
	backup := filepath.Join(parent, "original")
	script := fmt.Sprintf("#!/bin/sh\nset -eu\ncase \" $* \" in *' rev-parse --show-toplevel '*) output=$(%q \"$@\"); /bin/mv %q %q; /bin/mkdir %q; /bin/mv %q %q; printf '%%s\\n' \"$output\"; exit 0;; esac\nexec %q \"$@\"\n", realGit, repository, backup, repository, filepath.Join(backup, "nested"), nested, realGit)
	if err := os.WriteFile(wrapper, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := Resolve(context.Background(), Options{Path: nested, GitPath: wrapper}); err == nil || !strings.Contains(err.Error(), "changed during discovery") {
		t.Fatalf("Resolve() error = %v", err)
	}
}

func TestResolveRejectsGitReplacementDuringCapabilityProbe(t *testing.T) {
	repository := testRepository(t)
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	realGit, err = filepath.Abs(realGit)
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	wrapper := filepath.Join(directory, "git")
	replacement := filepath.Join(directory, "replacement")
	if err := os.WriteFile(replacement, []byte(fmt.Sprintf("#!/bin/sh\nexec %q \"$@\"\n", realGit)), 0o700); err != nil {
		t.Fatal(err)
	}
	script := fmt.Sprintf("#!/bin/sh\nset -eu\nlast=''\nfor argument in \"$@\"; do last=\"$argument\"; done\nif [ \"$last\" = '--version' ]; then %q \"$@\"; /bin/mv %q \"$0\"; exit 0; fi\nexec %q \"$@\"\n", realGit, replacement, realGit)
	if err := os.WriteFile(wrapper, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := Resolve(context.Background(), Options{Path: repository, GitPath: wrapper}); err == nil {
		t.Fatal("expected Git replacement during capability probe to fail")
	}
}

func testRepository(t *testing.T) string {
	t.Helper()
	repository := t.TempDir()
	command := exec.Command("git", "init", "-q", "-b", "main")
	command.Dir = repository
	command.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, output)
	}
	resolved, err := filepath.EvalSymlinks(repository)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

func testGitWrapper(t *testing.T) (string, string, string) {
	t.Helper()
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	realGit, err = filepath.Abs(realGit)
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	path := filepath.Join(directory, "git")
	log := filepath.Join(directory, "calls")
	marker := filepath.Join(directory, "credential-leaked")
	script := fmt.Sprintf("#!/bin/sh\nset -eu\nlast=''\nfor argument in \"$@\"; do last=\"$argument\"; done\nif [ \"$last\" = '--version' ]; then printf 'probe\\n' >> %q; else printf 'root\\n' >> %q; fi\nif [ -n \"${SSH_AUTH_SOCK:-}\" ]; then : > %q; fi\nexec %q \"$@\"\n", log, log, marker, realGit)
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SSH_AUTH_SOCK", "/tmp/agent.sock")
	return path, log, marker
}
