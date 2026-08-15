package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDirectRepoCommand exercises cmdRepo in-process so package coverage
// reflects the profile paths the harness tests drive through a child binary.
func TestDirectRepoCommand(t *testing.T) {
	repoDir := t.TempDir()
	runGit(t, repoDir, "init")
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repoDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldDir) })
	t.Setenv("SLOPSHIPPER_DB", filepath.Join(t.TempDir(), "direct-repo.sqlite"))

	if code := run([]string{"repo", "--json"}); code != 0 {
		t.Fatalf("show unregistered=%d", code)
	}
	if code := run([]string{"repo", "destroy"}); code != 2 {
		t.Fatalf("unknown subcommand=%d", code)
	}
	if code := run([]string{"repo", "show", "--trust", "low"}); code != 2 {
		t.Fatalf("show with flags=%d", code)
	}
	if code := run([]string{"repo", "--dry-run"}); code != 2 {
		t.Fatalf("dry-run show=%d", code)
	}
	if code := run([]string{"repo", "register", "--bind", "review=slopzapper"}); code != 2 {
		t.Fatalf("unregistered binding=%d", code)
	}
	if code := run([]string{"repo", "register", "--bind", "review"}); code != 2 {
		t.Fatalf("malformed bind=%d", code)
	}
	if code := run([]string{"repo", "update", "--trust", "low"}); code != 5 {
		t.Fatalf("update unregistered=%d", code)
	}
	if code := run([]string{"reviewers", "--add", "slopzapper"}); code != 0 {
		t.Fatalf("reviewers add=%d", code)
	}
	if code := run([]string{"repo", "register", "--dry-run", "--json",
		"--forge", "github", "--verify-cmd", "true"}); code != 0 {
		t.Fatalf("dry-run register=%d", code)
	}
	if code := run([]string{"repo", "register",
		"--forge", "github", "--trust", "low", "--verify-cmd", "echo 'x'",
		"--delivery", "pr-hold", "--readiness", "ready",
		"--bind", "review=slopguard,review=slopzapper,qa=slopscouter"}); code != 0 {
		t.Fatalf("register=%d", code)
	}
	if code := run([]string{"repo"}); code != 0 {
		t.Fatalf("show compact=%d", code)
	}
	if code := run([]string{"repo", "register"}); code != 2 {
		t.Fatalf("double register=%d", code)
	}
	if code := run([]string{"repo", "update", "--trust", "total"}); code != 2 {
		t.Fatalf("bad trust=%d", code)
	}
	if code := run([]string{"repo", "update", "--trust", "medium", "--json"}); code != 0 {
		t.Fatalf("update=%d", code)
	}
	if code := run([]string{"init", "--run", "direct-repo"}); code != 0 {
		t.Fatalf("init=%d", code)
	}
	if code := run([]string{"status", "--json", "--run", "direct-repo"}); code != 0 {
		t.Fatalf("status=%d", code)
	}
	if code := run([]string{"repo", "unregister"}); code != 0 {
		t.Fatalf("unregister=%d", code)
	}
	if code := run([]string{"repo", "unregister", "--json"}); code != 0 {
		t.Fatalf("unregister idempotent=%d", code)
	}
}

func decodeRepoDoc(t *testing.T, out string) repoProfileDocument {
	t.Helper()
	var doc repoProfileDocument
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("json: %v\n%s", err, out)
	}
	return doc
}

func TestRepoProfileLifecycle(t *testing.T) {
	h := newCLIHarness(t)

	out := h.must("repo", "--json")
	if doc := decodeRepoDoc(t, out); doc.Registered {
		t.Fatalf("fresh repo must be unregistered: %s", out)
	}

	// Review bindings must exist as reviewer identities first.
	out, code := h.run("repo", "register", "--bind", "review=slopzapper")
	if code != 2 || !strings.Contains(out, "slopshipper reviewers --add") {
		t.Fatalf("unregistered review binding must fail actionably: exit %d\n%s", code, out)
	}
	h.must("reviewers", "--add", "slopzapper")

	out = h.must("repo", "register",
		"--forge", "github", "--trust", "low", "--verify-cmd", "true",
		"--delivery", "pr-merge-when-ready", "--readiness", "ready",
		"--bind", "review=slopguard,review=slopzapper,qa=slopscouter", "--json")
	doc := decodeRepoDoc(t, out)
	if !doc.Registered || doc.ForgeKind != "github" || doc.VerifyCommand != "true" ||
		doc.DeliveryMode != "pr-merge-when-ready" || len(doc.Bindings["review"]) != 2 {
		t.Fatalf("register did not record the profile: %s", out)
	}

	out, code = h.run("repo", "register", "--json")
	if code != 2 || !strings.Contains(out, "already registered") {
		t.Fatalf("double register must fail: exit %d\n%s", code, out)
	}

	out = h.must("repo", "update", "--trust", "medium", "--json")
	doc = decodeRepoDoc(t, out)
	if doc.TrustTier != "medium" || doc.VerifyCommand != "true" {
		t.Fatalf("update must patch only passed fields: %s", out)
	}

	out, code = h.run("repo", "update", "--forge", "gitlab")
	if code != 2 || !strings.Contains(out, "github") {
		t.Fatalf("unknown forge must fail closed: exit %d\n%s", code, out)
	}
	out, code = h.run("repo", "update", "--bind", "reviewer=slopzapper")
	if code != 2 {
		t.Fatalf("unknown role must fail closed: exit %d\n%s", code, out)
	}
	out, code = h.run("repo", "update", "--bind", "review")
	if code != 2 || !strings.Contains(out, "role=name") {
		t.Fatalf("malformed bind must fail with the format: exit %d\n%s", code, out)
	}

	out, code = h.run("repo", "update", "--forge=")
	if code != 2 || !strings.Contains(out, "non-empty value") {
		t.Fatalf("explicitly empty policy flag must not silently clear: exit %d\n%s", code, out)
	}
	if after := decodeRepoDoc(t, h.must("repo", "--json")); after.ForgeKind != "github" {
		t.Fatalf("rejected update must leave the profile unchanged: %+v", after)
	}

	line := h.must("repo")
	if !strings.Contains(line, "registered=true") || !strings.Contains(line, "trust=medium") {
		t.Fatalf("compact repo line: %s", line)
	}

	out = h.must("repo", "unregister", "--json")
	if doc := decodeRepoDoc(t, out); doc.Registered {
		t.Fatalf("unregister must clear the profile: %s", out)
	}
	out, code = h.run("repo", "update", "--trust", "low")
	if code != 5 || !strings.Contains(out, "slopshipper repo register") {
		t.Fatalf("update without a profile must point at register: exit %d\n%s", code, out)
	}
}

func TestRepoSubcommandValidation(t *testing.T) {
	h := newCLIHarness(t)
	out, code := h.run("repo", "destroy")
	if code != 2 || !strings.Contains(out, "unknown repo subcommand") {
		t.Fatalf("exit %d\n%s", code, out)
	}
	out, code = h.run("repo", "show", "--trust", "low")
	if code != 2 || !strings.Contains(out, "does not accept") {
		t.Fatalf("show must reject mutation flags: exit %d\n%s", code, out)
	}
	out, code = h.run("--dry-run", "repo")
	if code != 2 || !strings.Contains(out, "mutating repo subcommand") {
		t.Fatalf("dry-run show must fail: exit %d\n%s", code, out)
	}
	out, code = h.run("repo", "register", "--forge", "gitlab", "--forge", "github")
	if code != 2 || !strings.Contains(out, "only once") {
		t.Fatalf("duplicate flags must fail closed, not last-value-win: exit %d\n%s", code, out)
	}
}

func TestRepoDryRunProjectsWithoutPersisting(t *testing.T) {
	h := newCLIHarness(t)
	// A fresh installation (no database yet) projects against empty state,
	// matching what the real command would create.
	out := h.must("--dry-run", "repo", "register", "--trust", "high", "--bind", "review=slopguard", "--json")
	doc := decodeRepoDoc(t, out)
	if !doc.DryRun || !doc.Registered || doc.TrustTier != "high" {
		t.Fatalf("dry-run register must project the profile: %s", out)
	}
	if after := decodeRepoDoc(t, h.must("repo", "--json")); after.Registered {
		t.Fatalf("dry run must not persist: %+v", after)
	}
	// With state present, dry runs project through the read-only store.
	h.must("init", "--run", "seed")
	out = h.must("--dry-run", "repo", "register", "--trust", "low", "--json")
	if doc := decodeRepoDoc(t, out); !doc.DryRun || doc.TrustTier != "low" {
		t.Fatalf("dry-run register with state must project: %s", out)
	}
}

func TestRegisteredRepoDrivesContractDefaultsAndGates(t *testing.T) {
	h := newCLIHarness(t)
	h.must("reviewers", "--add", "slopzapper")
	h.must("repo", "register",
		"--verify-cmd", "true", "--delivery", "pr-merge-when-ready",
		"--bind", "review=slopguard,review=slopzapper")

	// init inherits the profile's delivery policy.
	h.must("init", "--run", "prof")
	var st struct {
		DeliveryMode   string `json:"delivery_mode"`
		IntakeRevision int64  `json:"intake_revision"`
		NextAction     string `json:"next_action"`
		VerifyCommand  string `json:"verify_command"`
		RepoRegistered bool   `json:"repo_registered"`
		State          string `json:"state"`
	}
	out := h.must("status", "--json", "--run", "prof")
	if err := json.Unmarshal([]byte(out), &st); err != nil {
		t.Fatal(err)
	}
	if st.DeliveryMode != "pr-merge-when-ready" {
		t.Fatalf("init must inherit delivery mode: %s", out)
	}
	if !st.RepoRegistered || st.VerifyCommand != "true" {
		t.Fatalf("status must surface the profile defaults: %s", out)
	}

	// The two-reviewer gate: contract requires both bound reviewers.
	intake := filepath.Join(t.TempDir(), "intake.json")
	mustWrite(t, intake, `{
		"required_reviewers":["slopguard","slopzapper"],
		"series_bound":1,
		"units":[{"id":"u1","title":"one"}]
	}`)
	h.must("intake", "--file", intake, "--run", "prof")
	out = h.must("status", "--json", "--run", "prof")
	if err := json.Unmarshal([]byte(out), &st); err != nil {
		t.Fatal(err)
	}

	// Narrowing the review bindings blocks the release fail-closed.
	h.must("repo", "update", "--bind", "review=slopguard")
	out, code := h.run("release", "--revision", itoa(st.IntakeRevision), "--run", "prof")
	if code != 3 || !strings.Contains(out, "slopzapper") {
		t.Fatalf("release must fail when a required reviewer loses its binding: exit %d\n%s", code, out)
	}

	// Restoring the binding satisfies the two-reviewer gate.
	h.must("repo", "update", "--bind", "review=slopguard,review=slopzapper")
	h.must("release", "--revision", itoa(st.IntakeRevision), "--run", "prof")
	h.must("build", "--run", "prof")

	// At BUILD the next action names the canonical verify command verbatim.
	out = h.must("status", "--json", "--run", "prof")
	if err := json.Unmarshal([]byte(out), &st); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(st.NextAction, "slopshipper verify --cmd 'true'") {
		t.Fatalf("verify next_action must carry the canonical command: %s", st.NextAction)
	}
}
