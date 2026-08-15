package main

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestStorageResolutionDefaultsAndRelativeOverrides(t *testing.T) {
	repoDir := t.TempDir()
	runGit(t, repoDir, "init")
	withWorkingDirectory(t, repoDir)

	xdg := t.TempDir()
	t.Setenv("SLOPSHIPPER_DB", "")
	t.Setenv("XDG_DATA_HOME", xdg)
	doc, err := resolveStorage(true)
	if err != nil {
		t.Fatal(err)
	}
	wantDefault, err := physicalPath(filepath.Join(xdg, "slopshipper", "slopshipper.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	if doc.Path != wantDefault || doc.Source != "xdg-data" || doc.Scope != "user" || doc.Exists || doc.GitIgnored != nil {
		t.Fatalf("default storage=%+v want path=%q", doc, wantDefault)
	}
	t.Setenv("XDG_DATA_HOME", "relative-data")
	if _, err := resolveStorage(true); !errors.Is(err, errInvalidStateConfig) || !strings.Contains(err.Error(), "must be an absolute path") {
		t.Fatalf("relative XDG_DATA_HOME error=%v", err)
	}
	out, code := captureStdoutResult(t, func() int { return run([]string{"storage", "--json"}) })
	if code != 2 || !strings.Contains(out, `"kind": "invalid_state_config"`) {
		t.Fatalf("relative XDG_DATA_HOME exit=%d output=%s", code, out)
	}
	t.Setenv("XDG_DATA_HOME", xdg)

	blockingFile := filepath.Join(t.TempDir(), "not-a-directory")
	mustWrite(t, blockingFile, "blocked")
	t.Setenv("SLOPSHIPPER_DB", filepath.Join(blockingFile, "slopshipper.sqlite"))
	out, code = captureStdoutResult(t, func() int { return run([]string{"storage", "--json"}) })
	if code != 2 || !strings.Contains(out, `"kind": "state_unavailable"`) || !strings.Contains(out, blockingFile) {
		t.Fatalf("unpreparable storage exit=%d output=%s", code, out)
	}

	t.Setenv("SLOPSHIPPER_DB", filepath.Join(".slopshipper", "slopshipper.sqlite"))
	doc, err = resolveStorage(false)
	if err != nil {
		t.Fatal(err)
	}
	wantLocal, err := physicalPath(filepath.Join(repoDir, ".slopshipper", "slopshipper.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	if doc.Path != wantLocal || doc.Source != "environment" || doc.Scope != "repository" || doc.GitIgnored == nil || *doc.GitIgnored {
		t.Fatalf("unsafe local storage=%+v want path=%q", doc, wantLocal)
	}
	if _, err := resolveStorage(true); !errors.Is(err, errUnsafeStatePath) || !strings.Contains(err.Error(), "/.slopshipper/") {
		t.Fatalf("unsafe storage error=%v", err)
	}

	mustWrite(t, filepath.Join(repoDir, ".git", "info", "exclude"), "/.slopshipper/\n")
	doc, err = resolveStorage(true)
	if err != nil || doc.GitIgnored == nil || !*doc.GitIgnored {
		t.Fatalf("ignored local storage=%+v err=%v", doc, err)
	}

	subdir := filepath.Join(repoDir, "nested", "work")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatal(err)
	}
	withWorkingDirectory(t, subdir)
	doc, err = resolveStorage(true)
	if err != nil || doc.Path != wantLocal {
		t.Fatalf("subdirectory storage=%+v err=%v want=%q", doc, err, wantLocal)
	}
}

func TestPathWithinUsesFilesystemIdentity(t *testing.T) {
	root := t.TempDir()
	caseVariant := strings.ToUpper(root)
	if _, err := os.Stat(caseVariant); err != nil {
		t.Skip("filesystem is case-sensitive")
	}
	inside, relative, err := pathWithin(root, filepath.Join(caseVariant, ".slopshipper", "slopshipper.sqlite"))
	if err != nil || !inside || filepath.ToSlash(relative) != ".slopshipper/slopshipper.sqlite" {
		t.Fatalf("inside=%t relative=%q err=%v", inside, relative, err)
	}
}

func TestStorageRequiresSQLiteSidecarsToBeIgnored(t *testing.T) {
	repoDir := t.TempDir()
	runGit(t, repoDir, "init")
	withWorkingDirectory(t, repoDir)
	t.Setenv("SLOPSHIPPER_DB", filepath.Join(".slopshipper", "slopshipper.sqlite"))
	mustWrite(t, filepath.Join(repoDir, ".git", "info", "exclude"), "/.slopshipper/slopshipper.sqlite\n")

	_, err := resolveStorage(true)
	if !errors.Is(err, errUnsafeStatePath) || !strings.Contains(err.Error(), "slopshipper.sqlite-wal") {
		t.Fatalf("sidecar error=%v", err)
	}
}

func TestStorageRejectsTrackedDatabaseEvenWhenPatternMatches(t *testing.T) {
	repoDir := t.TempDir()
	runGit(t, repoDir, "init")
	runGit(t, repoDir, "config", "user.email", "t@example.com")
	runGit(t, repoDir, "config", "user.name", "t")
	if err := os.Mkdir(filepath.Join(repoDir, ".slopshipper"), 0o700); err != nil {
		t.Fatal(err)
	}
	database := filepath.Join(repoDir, ".slopshipper", "slopshipper.sqlite")
	mustWrite(t, database, "tracked")
	runGit(t, repoDir, "add", ".slopshipper/slopshipper.sqlite")
	runGit(t, repoDir, "commit", "-m", "track unsafe state")
	mustWrite(t, filepath.Join(repoDir, ".git", "info", "exclude"), "/.slopshipper/\n")
	withWorkingDirectory(t, repoDir)
	t.Setenv("SLOPSHIPPER_DB", filepath.Join(".slopshipper", "slopshipper.sqlite"))

	_, err := resolveStorage(true)
	if !errors.Is(err, errUnsafeStatePath) || !strings.Contains(err.Error(), "slopshipper.sqlite") {
		t.Fatalf("tracked database error=%v", err)
	}
}

func TestStorageUsesPhysicalPathForRepositoryScope(t *testing.T) {
	repoDir := t.TempDir()
	runGit(t, repoDir, "init")
	withWorkingDirectory(t, repoDir)

	external := t.TempDir()
	if err := os.Symlink(external, filepath.Join(repoDir, "external-state")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SLOPSHIPPER_DB", filepath.Join("external-state", "slopshipper.sqlite"))
	doc, err := resolveStorage(true)
	wantExternal, pathErr := physicalPath(filepath.Join(external, "slopshipper.sqlite"))
	if pathErr != nil {
		t.Fatal(pathErr)
	}
	if err != nil || doc.Scope != "custom" || doc.Path != wantExternal {
		t.Fatalf("outbound symlink storage=%+v err=%v", doc, err)
	}

	localDir := filepath.Join(repoDir, ".slopshipper")
	if err := os.Mkdir(localDir, 0o700); err != nil {
		t.Fatal(err)
	}
	inbound := filepath.Join(t.TempDir(), "repo-state")
	if err := os.Symlink(localDir, inbound); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SLOPSHIPPER_DB", filepath.Join(inbound, "slopshipper.sqlite"))
	doc, err = resolveStorage(false)
	if err != nil || doc.Scope != "repository" || doc.GitIgnored == nil || *doc.GitIgnored {
		t.Fatalf("inbound symlink storage=%+v err=%v", doc, err)
	}
}

func TestStorageCommandAndUnsafeJSONError(t *testing.T) {
	h := newCLIHarness(t)
	out := h.must("storage", "--json")
	var doc storageDocument
	wantDatabase, pathErr := physicalPath(h.db)
	if pathErr != nil {
		t.Fatal(pathErr)
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil || doc.Source != "environment" || doc.Scope != "custom" || doc.Path != wantDatabase {
		t.Fatalf("storage JSON=%+v err=%v output=%s", doc, err, out)
	}
	if out := h.must("storage"); !strings.Contains(out, "scope=custom") || !strings.Contains(out, "git_ignored=n/a") {
		t.Fatalf("storage human=%s", out)
	}

	h.env = replaceEnvironment(h.env, "SLOPSHIPPER_DB", filepath.Join(".slopshipper", "slopshipper.sqlite"))
	out, code := h.run("init", "--run", "unsafe", "--json")
	if code != 2 || !strings.Contains(out, `"kind": "unsafe_state_path"`) {
		t.Fatalf("unsafe init exit=%d output=%s", code, out)
	}
	if _, err := os.Stat(filepath.Join(h.repoDir, ".slopshipper")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unsafe init created state: %v", err)
	}

	mustWrite(t, filepath.Join(h.repoDir, ".git", "info", "exclude"), "/.slopshipper/\n")
	out = h.must("storage", "--json")
	if err := json.Unmarshal([]byte(out), &doc); err != nil || doc.GitIgnored == nil || !*doc.GitIgnored || doc.Scope != "repository" {
		t.Fatalf("safe storage JSON=%+v err=%v output=%s", doc, err, out)
	}
	h.must("init", "--run", "safe")
	h.mustInput(`{"run":"safe","delivery_mode":"pr-hold","required_reviewers":["bugbot"],"series_bound":1,"units":[{"id":"u1","title":"repository-local dry run","blockers":[]}]}`, "intake", "--input", "-", "--json")
	h.must("release", "--revision", "2", "--run", "safe", "--json")
	dryRun := h.must("build", "--run", "safe", "--dry-run", "--json")
	if !strings.Contains(dryRun, `"dry_run": true`) || !strings.Contains(dryRun, `"state": "BUILD"`) {
		t.Fatalf("repository-local dry run=%s", dryRun)
	}
	if status := gitOutput(t, h.repoDir, "status", "--porcelain"); status != "" {
		t.Fatalf("ignored local state dirtied worktree: %q", status)
	}
}

func TestStorageHumanOutputIncludesUnsafeRecovery(t *testing.T) {
	repoDir := t.TempDir()
	runGit(t, repoDir, "init")
	withWorkingDirectory(t, repoDir)
	t.Setenv("SLOPSHIPPER_DB", filepath.Join(".slopshipper", "slopshipper.sqlite"))

	out := captureStdout(t, func() int { return run([]string{"storage"}) })
	if !strings.Contains(out, "git_ignored=false") || !strings.Contains(out, "recovery=use a path outside the worktree") || !strings.Contains(out, "/.slopshipper/") {
		t.Fatalf("unsafe storage recovery=%s", out)
	}
}

func TestStorageCommandDirectOutput(t *testing.T) {
	repoDir := t.TempDir()
	runGit(t, repoDir, "init")
	withWorkingDirectory(t, repoDir)
	database := filepath.Join(t.TempDir(), "state.sqlite")
	t.Setenv("SLOPSHIPPER_DB", database)

	out := captureStdout(t, func() int { return run([]string{"storage", "--json"}) })
	var doc storageDocument
	if err := json.Unmarshal([]byte(out), &doc); err != nil || doc.Source != "environment" || doc.Scope != "custom" {
		t.Fatalf("direct storage JSON=%+v err=%v output=%s", doc, err, out)
	}
	out = captureStdout(t, func() int { return run([]string{"storage"}) })
	if !strings.Contains(out, "slopshipper storage") || !strings.Contains(out, "git_ignored=n/a") {
		t.Fatalf("direct storage human=%s", out)
	}
	if code := run([]string{"storage", "--unexpected", "value"}); code != 2 {
		t.Fatalf("direct storage invalid flag=%d", code)
	}
}

func TestReadJSONRejectsInteractiveStdin(t *testing.T) {
	var input initInput
	err := readJSONFrom("-", &input, strings.NewReader(`{"run":"terminal"}`), true)
	if err == nil || !strings.Contains(err.Error(), "interactive terminal") || !strings.Contains(err.Error(), "slopshipper schema") {
		t.Fatalf("interactive stdin error=%v", err)
	}
	if err := readJSONFrom("-", &input, strings.NewReader(`{"run":"pipe"}`), false); err != nil || input.Run != "pipe" {
		t.Fatalf("piped stdin input=%+v err=%v", input, err)
	}
}

func withWorkingDirectory(t *testing.T, directory string) {
	t.Helper()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(directory); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })
}

func replaceEnvironment(environment []string, name, value string) []string {
	prefix := name + "="
	result := make([]string, 0, len(environment)+1)
	for _, item := range environment {
		if !strings.HasPrefix(item, prefix) {
			result = append(result, item)
		}
	}
	return append(result, prefix+value)
}

func gitOutput(t *testing.T, directory string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = directory
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return strings.TrimSpace(string(out))
}
