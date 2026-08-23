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

func TestRepositoryRootQueryUsesValidatedExecutableSnapshot(t *testing.T) {
	repository := testRepository(t)
	gitPath, _, _ := testGitWrapper(t)
	gitPath, err := filepath.EvalSymlinks(gitPath)
	if err != nil {
		t.Fatal(err)
	}
	identity, err := captureExecutableIdentity(context.Background(), gitPath)
	if err != nil {
		t.Fatal(err)
	}
	executable, cleanup, err := snapshotValidatedExecutable(context.Background(), gitPath, identity)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	marker := filepath.Join(filepath.Dir(gitPath), "replacement-ran")
	replacement := filepath.Join(filepath.Dir(gitPath), "replacement")
	if err := os.WriteFile(replacement, []byte(fmt.Sprintf("#!/bin/sh\n: > %q\nexit 0\n", marker)), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(replacement, gitPath); err != nil {
		t.Fatal(err)
	}
	output, err := runRepositoryRootQuery(context.Background(), executable, repository)
	if err != nil {
		t.Fatal(err)
	}
	if root := strings.TrimSpace(string(output)); root != repository {
		t.Fatalf("repository root = %q, want %q", root, repository)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("replacement executable ran: %v", err)
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

func TestResolveRejectsWorktreeReplacementDuringCapabilityProbe(t *testing.T) {
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
	backup := filepath.Join(parent, "original")
	script := fmt.Sprintf("#!/bin/sh\nset -eu\nlast=''\nfor argument in \"$@\"; do last=\"$argument\"; done\nif [ \"$last\" = '--version' ]; then %q \"$@\"; /bin/mv %q %q; /bin/mkdir %q; exit 0; fi\nexec %q \"$@\"\n", realGit, repository, backup, repository, realGit)
	if err := os.WriteFile(wrapper, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := Resolve(context.Background(), Options{Path: repository, GitPath: wrapper}); err == nil || !strings.Contains(err.Error(), "worktree changed") {
		t.Fatalf("Resolve() error = %v", err)
	}
}

func TestValidateRejectsLinkedWorktreeGitFileMutation(t *testing.T) {
	repository := testRepository(t)
	if err := os.WriteFile(filepath.Join(repository, "file.txt"), []byte("base\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runTestGit(t, repository, "config", "user.name", "Slopguard Test")
	runTestGit(t, repository, "config", "user.email", "slopguard@example.invalid")
	runTestGit(t, repository, "add", "file.txt")
	runTestGit(t, repository, "commit", "-q", "-m", "base")
	worktree := filepath.Join(t.TempDir(), "linked")
	runTestGit(t, repository, "worktree", "add", "-q", "-b", "linked", worktree)
	gitPath, _, _ := testGitWrapper(t)
	resolved, err := Resolve(context.Background(), Options{Path: worktree, GitPath: gitPath})
	if err != nil {
		t.Fatal(err)
	}
	metadata := filepath.Join(worktree, ".git")
	info, err := os.Stat(metadata)
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(metadata)
	if err != nil {
		t.Fatal(err)
	}
	mutated := strings.Replace(string(content), "gitdir", "gitdix", 1)
	if len(mutated) != len(content) || mutated == string(content) {
		t.Fatal("test mutation did not preserve Git metadata size")
	}
	if err := os.WriteFile(metadata, []byte(mutated), info.Mode().Perm()); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(metadata, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	if err := resolved.Validate(); err == nil || !strings.Contains(err.Error(), "metadata boundary changed") {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestValidateRejectsLinkedWorktreeGitDirectoryReplacement(t *testing.T) {
	repository := testRepository(t)
	if err := os.WriteFile(filepath.Join(repository, "file.txt"), []byte("base\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runTestGit(t, repository, "config", "user.name", "Slopguard Test")
	runTestGit(t, repository, "config", "user.email", "slopguard@example.invalid")
	runTestGit(t, repository, "add", "file.txt")
	runTestGit(t, repository, "commit", "-q", "-m", "base")
	worktree := filepath.Join(t.TempDir(), "linked")
	runTestGit(t, repository, "worktree", "add", "-q", "-b", "linked-target", worktree)
	gitPath, _, _ := testGitWrapper(t)
	resolved, err := Resolve(context.Background(), Options{Path: worktree, GitPath: gitPath})
	if err != nil {
		t.Fatal(err)
	}
	metadata := filepath.Join(worktree, ".git")
	content, err := os.ReadFile(metadata)
	if err != nil {
		t.Fatal(err)
	}
	target, ok, err := gitDirectoryTarget(metadata, content)
	if err != nil || !ok {
		t.Fatalf("gitDirectoryTarget() target=%q ok=%t error=%v", target, ok, err)
	}
	oldTarget := target + "-old"
	if err := os.Rename(target, oldTarget); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := resolved.Validate(); err == nil || !strings.Contains(err.Error(), "metadata boundary changed") {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestResolveRejectsGitInsideRootForNestedRequest(t *testing.T) {
	repository := testRepository(t)
	nested := filepath.Join(repository, "src")
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
	repositoryBin := filepath.Join(repository, "bin")
	if err := os.Mkdir(repositoryBin, 0o700); err != nil {
		t.Fatal(err)
	}
	insideGit := filepath.Join(repositoryBin, "git")
	if err := os.WriteFile(insideGit, []byte(fmt.Sprintf("#!/bin/sh\nexec %q \"$@\"\n", realGit)), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := Resolve(context.Background(), Options{Path: nested, GitPath: insideGit}); err == nil || !strings.Contains(err.Error(), "inside the reviewed repository") {
		t.Fatalf("explicit repository Git error = %v", err)
	}
	externalBin := t.TempDir()
	if err := os.Symlink(realGit, filepath.Join(externalBin, "git")); err != nil {
		t.Fatal(err)
	}
	resolved, err := Resolve(context.Background(), Options{Path: nested, Environment: []string{"PATH=" + repositoryBin + string(os.PathListSeparator) + externalBin}})
	if err != nil {
		t.Fatal(err)
	}
	wantGit, err := filepath.EvalSymlinks(realGit)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.GitPath() != wantGit {
		t.Fatalf("Git path = %q, want %q", resolved.GitPath(), wantGit)
	}
}

func TestResolvePreservesLexicalAndResolvedWorktreeBoundaries(t *testing.T) {
	lexicalRepository := testRepository(t)
	resolvedRepository := testRepository(t)
	alias := filepath.Join(lexicalRepository, "linked")
	if err := os.Symlink(resolvedRepository, alias); err != nil {
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
	for _, repository := range []string{lexicalRepository, resolvedRepository} {
		bin := filepath.Join(repository, "bin")
		if err := os.Mkdir(bin, 0o700); err != nil {
			t.Fatal(err)
		}
		gitPath := filepath.Join(bin, "git")
		if err := os.WriteFile(gitPath, []byte(fmt.Sprintf("#!/bin/sh\nexec %q \"$@\"\n", realGit)), 0o700); err != nil {
			t.Fatal(err)
		}
		if _, err := Resolve(context.Background(), Options{Path: alias, GitPath: gitPath}); err == nil || !strings.Contains(err.Error(), "inside the reviewed repository") {
			t.Fatalf("repository Git %q error = %v", gitPath, err)
		}
	}
}

func TestValidateRejectsSymlinkedGitDirectoryTargetReplacement(t *testing.T) {
	repository := testRepository(t)
	metadata := filepath.Join(repository, ".git")
	target := filepath.Join(repository, ".git-target")
	if err := os.Rename(metadata, target); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Base(target), metadata); err != nil {
		t.Fatal(err)
	}
	gitPath, _, _ := testGitWrapper(t)
	resolved, err := Resolve(context.Background(), Options{Path: repository, GitPath: gitPath})
	if err != nil {
		t.Fatal(err)
	}
	oldTarget := filepath.Join(repository, ".git-old")
	if err := os.Rename(target, oldTarget); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := resolved.Validate(); err == nil || !strings.Contains(err.Error(), "metadata boundary changed") {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestGitDirectoryTargetRejectsCycle(t *testing.T) {
	directory := t.TempDir()
	metadata := filepath.Join(directory, ".git")
	if err := os.WriteFile(metadata, []byte("gitdir: .git\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := captureBoundaryIdentity(metadata); err == nil || !strings.Contains(err.Error(), "not a directory") {
		t.Fatalf("captureBoundaryIdentity() error = %v", err)
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

func runTestGit(t *testing.T, repository string, arguments ...string) {
	t.Helper()
	command := exec.Command("git", arguments...)
	command.Dir = repository
	command.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", arguments, err, output)
	}
}
