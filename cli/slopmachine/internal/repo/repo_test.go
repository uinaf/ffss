package repo

import (
	"os/exec"
	"strings"
	"testing"
)

func TestRedactRemoteStripsUserinfo(t *testing.T) {
	got := redactRemote("https://TOKEN@github.com/uinaf/ffsstack/cli/slopmachine.git")
	want := "https://github.com/uinaf/ffsstack/cli/slopmachine.git"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if got := redactRemote("git@github.com:uinaf/slopmachine.git"); got != "github.com:uinaf/slopmachine.git" {
		t.Fatalf("scp form changed: %q", got)
	}
	if got := redactRemote("TOKEN@github.com:org/repo.git?secret=yes#fragment"); got != "github.com:org/repo.git" {
		t.Fatalf("scp form leaked userinfo or suffix: %q", got)
	}
	if got := redactRemote("TOKEN@host:org/repo@v1?secret=yes#fragment"); got != "host:org/repo@v1" {
		t.Fatalf("scp path @ confused with userinfo: %q", got)
	}
	if got := redactRemote("TOKEN%40host:org/repo@v1?secret=yes#fragment"); got != "host:org/repo@v1" {
		t.Fatalf("encoded scp userinfo retained: %q", got)
	}
	if got := redactRemote("oauth2:SECRET@host:org/repo.git"); strings.Contains(got, "SECRET") {
		t.Fatalf("colon-bearing credential retained: %q", got)
	}
	if got := redactRemote("user_name:SECRET@host:org/repo.git"); got != "host:org/repo.git" {
		t.Fatalf("invalid-scheme colon credential retained: %q", got)
	}
	if got := redactRemote("host:org/repo@v1?secret=yes#fragment"); got != "host:org/repo@v1" {
		t.Fatalf("scp path @ changed: %q", got)
	}
	if got := redactRemote("host?credential=secret:repo"); got != "host" {
		t.Fatalf("suffix truncation used stale separator: %q", got)
	}
	if got := redactRemote("https:TOKEN@host/path?secret=yes#fragment"); got != "https:host/path" {
		t.Fatalf("opaque URL leaked userinfo or suffix: %q", got)
	}
	if got := redactRemote("custom:TOKEN@host/repo?secret=yes#fragment"); got != "custom:host/repo" {
		t.Fatalf("custom opaque URL leaked userinfo or suffix: %q", got)
	}
	if got := redactRemote("https:host/org/repo@v1?secret=yes#fragment"); got != "https:host/org/repo@v1" {
		t.Fatalf("opaque path @ changed: %q", got)
	}
	if got := redactRemote("https:/host/repo@v1?credential=secret#fragment"); got != "https:/host/repo@v1" {
		t.Fatalf("single-slash URL retained suffix or changed path @: %q", got)
	}
	if got := redactRemote("https:/TOKEN@host/repo?credential=secret#fragment"); got != "https:/host/repo" {
		t.Fatalf("single-slash URL retained credentials: %q", got)
	}
	if got := redactRemote("https:///user:password@host/repo?credential=secret#fragment"); got != "https:///host/repo" {
		t.Fatalf("triple-slash URL retained credentials: %q", got)
	}
	if got := redactRemote("https:/TOKEN%40host/repo"); got != "https:/host/repo" {
		t.Fatalf("encoded single-slash URL retained credentials: %q", got)
	}
	if got := redactRemote("custom:TOKEN%40host/repo"); got != "custom:host/repo" {
		t.Fatalf("encoded opaque URL retained credentials: %q", got)
	}
	if got := redactRemote("https//TOKEN%40host/repo"); got != "https//host/repo" {
		t.Fatalf("encoded malformed URL retained credentials: %q", got)
	}
	if got := redactRemote("//user:password@host/repo?credential=secret#fragment"); got != "//host/repo" {
		t.Fatalf("scheme-relative URL retained credentials: %q", got)
	}
	if got := redactRemote("https//TOKEN@host/repo.git?secret=yes#fragment"); got != "https//host/repo.git" {
		t.Fatalf("malformed scheme leaked userinfo or suffix: %q", got)
	}
	if got := redactRemote("https//host/repo.git?credential=secret#fragment"); got != "https//host/repo.git" {
		t.Fatalf("malformed scheme retained suffix: %q", got)
	}
	if got := redactRemote("https//TOKEN@host/repo@v1?credential=secret#fragment"); got != "https//host/repo@v1" {
		t.Fatalf("malformed scheme confused path @ with userinfo: %q", got)
	}
	if got := redactRemote("local@path"); got != "local@path" {
		t.Fatalf("local path changed: %q", got)
	}
	if got := redactRemote("/tmp/local@path"); got != "/tmp/local@path" {
		t.Fatalf("local absolute path changed: %q", got)
	}
	for _, local := range []string{"/tmp/repo?copy", "/tmp/repo:name?copy", "./repo:name#copy", "../repo#copy", "local@path?copy", `C:\repo:name?copy`, "C:repo?copy"} {
		if got := redactRemote(local); got != local {
			t.Errorf("local path %q changed to %q", local, got)
		}
	}
	for _, local := range []string{"/srv/repo?access_token=synthetic", "/srv/repo?api_key=synthetic", "/srv/repo?%61ccess_token=synthetic", `C:\repo%3Ftoken%3Dsynthetic`, "./repo%3Ftoken=synthetic", "local-path#password=synthetic", "C:repo?token=synthetic"} {
		if got := redactRemote(local); strings.Contains(got, "synthetic") {
			t.Errorf("local path credential retained: %q", got)
		}
	}
	if got := redactRemote("https://user@example.com/repo.git?credential=value#fragment"); got != "https://example.com/repo.git" {
		t.Fatalf("query or fragment retained: %q", got)
	}
	if got := redactRemote("x://user:password@host/repo"); got != "x://host/repo" {
		t.Fatalf("one-character scheme retained credentials: %q", got)
	}
	if got := redactRemote("https://host/repo%3Faccess_token=SECRET"); got != "https://host/repo" {
		t.Fatalf("encoded query suffix retained: %q", got)
	}
	if got := redactRemote("https://host/repo%23token=SECRET"); got != "https://host/repo" {
		t.Fatalf("encoded fragment suffix retained: %q", got)
	}
	if got := redactRemote("https://host/repo.git;access_token=synthetic"); got != "https://host/repo.git" {
		t.Fatalf("path credential parameter retained: %q", got)
	}
	if got := redactRemote("https://host/repo.git%3Baccess_token=synthetic"); got != "https://host/repo.git" {
		t.Fatalf("encoded path credential parameter retained: %q", got)
	}
	if got := redactRemote("https://SECRET%3Fignored@host/repo.git"); got != "https://host/repo.git" {
		t.Fatalf("encoded userinfo became hostname: %q", got)
	}
	if got := redactRemote("https://SECRET@/repo.git"); strings.Contains(got, "SECRET") {
		t.Fatalf("empty-host URL retained userinfo: %q", got)
	}
	if got := redactRemote("https:://TOKEN@host/repo"); got != "https:://host/repo" {
		t.Fatalf("nested opaque authority retained userinfo: %q", got)
	}
	if got := redactRemote("https:https://TOKEN@host/repo"); got != "https:https://host/repo" {
		t.Fatalf("deeply nested opaque authority retained userinfo: %q", got)
	}
	if got := redactRemote("https:TOKEN@outer:https://INNER@host/repo"); got != "https:outer:https://host/repo" {
		t.Fatalf("multiple opaque authorities retained userinfo: %q", got)
	}
	if got := redactRemote("outer:https//TOKEN@host/repo.git"); got != "outer:https//host/repo.git" {
		t.Fatalf("nested malformed authority retained userinfo: %q", got)
	}
	if got := redactRemote("outer:https//TOKEN%40host/repo.git"); got != "outer:https//host/repo.git" {
		t.Fatalf("nested encoded malformed authority retained userinfo: %q", got)
	}
	if got := redactRemote(" https://user@host:bad/path?secret=yes#fragment "); got != "https://host:bad/path" {
		t.Fatalf("malformed URL fallback: %q", got)
	}
	if got := redactRemote("ssh://host:bad"); got != "ssh://host:bad" {
		t.Fatalf("malformed URL without path: %q", got)
	}
	if got := redactRemote("https://TOKEN@host:bad?secret=yes#fragment"); got != "https://host:bad" {
		t.Fatalf("malformed no-path URL leaked userinfo: %q", got)
	}
}

func TestMatchesKey(t *testing.T) {
	root := "/work/repo"
	current := "https://host/repo|" + root
	for _, candidate := range []string{
		"https://host/repo?access_token=OLD|" + root,
		"https://TOKEN%40host/repo|" + root,
	} {
		if !MatchesKey(candidate, current, root) {
			t.Errorf("credential alias did not match: %q", candidate)
		}
	}
	for _, candidate := range []string{
		"https://other/repo?access_token=OLD|" + root,
		"https://host/repo|/tmp|" + root,
		"https://host/repo|/work/other",
	} {
		if MatchesKey(candidate, current, root) {
			t.Errorf("unrelated identity matched: %q", candidate)
		}
	}
}

func TestKeysFromRepository(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init")
	root := strings.TrimSpace(runGitOutput(t, dir, "rev-parse", "--show-toplevel"))
	key, rootFromKeys, err := Keys(dir)
	if err != nil || key != root || rootFromKeys != root {
		t.Fatalf("no remote: key=%q root=%q err=%v", key, rootFromKeys, err)
	}
	runGit(t, dir, "remote", "add", "origin", "https://user@example.com/org/repo.git?credential=value#fragment")
	key, rootFromKeys, err = Keys(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(key, "https://example.com/org/repo.git|") || strings.Contains(key, "credential") {
		t.Fatalf("sanitized key: %q", key)
	}
	if rootFromKeys != root {
		t.Fatalf("root=%q want %q", rootFromKeys, root)
	}
	if got, err := Key(dir); err != nil || got != key {
		t.Fatalf("Key=%q err=%v", got, err)
	}
}

func TestKeyRejectsNonRepository(t *testing.T) {
	if _, err := Key(t.TempDir()); err == nil || !strings.Contains(err.Error(), "not a git repository") {
		t.Fatalf("got %v", err)
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func runGitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}
