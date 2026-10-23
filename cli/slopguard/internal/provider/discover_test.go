package provider

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverExecutableRejectsReviewedRepository(t *testing.T) {
	t.Parallel()

	repository := t.TempDir()
	path := filepath.Join(repository, "bin", "codex")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := discoverExecutable(path, repository, []string{"PATH=" + filepath.Dir(path)}); err == nil {
		t.Fatal("discoverExecutable() accepted a repository-local executable")
	}
	if _, err := discoverExecutable("codex", repository, []string{"PATH=" + filepath.Dir(path)}); err == nil {
		t.Fatal("discoverExecutable() accepted a repository-local PATH entry")
	}
}

// A PATH entry may spell the reviewed repository with a different case or
// through the /System/Volumes/Data firmlink; containment must still hold.
func TestDiscoverExecutableRejectsRepositoryAliasOnPATH(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	repository := filepath.Join(root, "Repo")
	path := filepath.Join(repository, "node_modules", ".bin", "claude")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	aliases := []string{filepath.Join(root, "repo", "node_modules", ".bin")}
	if resolved, err := filepath.EvalSymlinks(filepath.Dir(path)); err == nil {
		aliases = append(aliases, filepath.Join("/System/Volumes/Data", resolved))
	}
	tested := 0
	for _, alias := range aliases {
		if _, err := os.Stat(filepath.Join(alias, "claude")); err != nil {
			continue
		}
		tested++
		if found, err := discoverExecutable("claude", repository, []string{"PATH=" + alias}); err == nil {
			t.Errorf("discoverExecutable() accepted %q through PATH alias %q", found, alias)
		}
	}
	if tested == 0 {
		t.Skip("filesystem offers no case or firmlink alias")
	}
}

// A dotfiles repository at the home directory must not reject providers
// installed under it while a nested project is reviewed.
func TestDiscoverExecutableAcceptsEnclosingHomeWorktree(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
	repository := filepath.Join(home, "projects", "app")
	if err := os.MkdirAll(filepath.Join(repository, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(home, ".local", "bin", "claude")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := discoverExecutable("claude", repository, []string{"PATH=" + filepath.Dir(path)}); err != nil {
		t.Fatalf("discoverExecutable() rejected a provider under an enclosing home worktree: %v", err)
	}
}

func TestDiscoverExecutableResolvesExternalSymlink(t *testing.T) {
	t.Parallel()

	repository := t.TempDir()
	external := writeTestExecutable(t, "codex-real", "#!/bin/sh\nexit 0\n")
	link := filepath.Join(t.TempDir(), "codex")
	if err := os.Symlink(external, link); err != nil {
		t.Fatal(err)
	}
	resolved, err := discoverExecutable(link, repository, nil)
	if err != nil {
		t.Fatal(err)
	}
	want, err := filepath.EvalSymlinks(external)
	if err != nil {
		t.Fatal(err)
	}
	if resolved != want {
		t.Fatalf("resolved executable = %q, want %q", resolved, want)
	}
}

func TestDiscoverExecutableRejectsRepositorySymlinkToExternalBinary(t *testing.T) {
	t.Parallel()

	repository := t.TempDir()
	external := writeTestExecutable(t, "codex-real", "#!/bin/sh\nexit 0\n")
	link := filepath.Join(repository, "codex")
	if err := os.Symlink(external, link); err != nil {
		t.Fatal(err)
	}
	if _, err := discoverExecutable(link, repository, nil); err == nil {
		t.Fatal("discoverExecutable() accepted a repository-local symlink")
	}
}

func TestDiscoverExecutableSkipsRelativePathEntries(t *testing.T) {
	t.Parallel()

	repository := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "codex"), []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	workingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	relative, err := filepath.Rel(workingDirectory, outside)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := discoverExecutable("codex", repository, []string{"PATH=" + relative}); err == nil {
		t.Fatal("discoverExecutable() accepted a relative PATH entry")
	}
}
