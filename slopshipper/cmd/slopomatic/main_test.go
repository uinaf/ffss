package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/uinaf/slopomatic/internal/machine"
	"github.com/uinaf/slopomatic/internal/repo"
	"github.com/uinaf/slopomatic/internal/store"
)

type cliHarness struct {
	t       *testing.T
	bin     string
	repoDir string
	env     []string
}

func newCLIHarness(t *testing.T) *cliHarness {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "slopomatic")
	buildArgs := []string{"build", "-o", bin, "."}
	if os.Getenv("SLOPOMATIC_COVERAGE_DIR") != "" {
		buildArgs = []string{"build", "-cover", "-covermode=atomic", "-coverpkg=github.com/uinaf/slopomatic/...", "-o", bin, "."}
	}
	build := exec.Command("go", buildArgs...)
	build.Dir = "."
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}

	db := filepath.Join(t.TempDir(), "t.sqlite")
	repoDir := t.TempDir()
	runGit(t, repoDir, "init")
	runGit(t, repoDir, "config", "user.email", "t@example.com")
	runGit(t, repoDir, "config", "user.name", "t")
	if err := os.WriteFile(filepath.Join(repoDir, "README"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, repoDir, "add", ".")
	runGit(t, repoDir, "commit", "-m", "init")

	return &cliHarness{
		t:       t,
		bin:     bin,
		repoDir: repoDir,
		env:     append(os.Environ(), "SLOPOMATIC_DB="+db),
	}
}

func (h *cliHarness) run(args ...string) (string, int) {
	return h.runInput("", args...)
}

func (h *cliHarness) runInput(input string, args ...string) (string, int) {
	h.t.Helper()
	cmd := exec.Command(h.bin, args...)
	cmd.Dir = h.repoDir
	cmd.Env = h.env
	if input != "" {
		cmd.Stdin = strings.NewReader(input)
	}
	out, err := cmd.CombinedOutput()
	code := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else {
			h.t.Fatalf("%v: %s", err, out)
		}
	}
	return string(out), code
}

func (h *cliHarness) must(args ...string) string {
	h.t.Helper()
	out, code := h.run(args...)
	if code != 0 {
		h.t.Fatalf("%v: exit %d\n%s", args, code, out)
	}
	return out
}

func TestCLINorthStarSmoke(t *testing.T) {
	h := newCLIHarness(t)
	h.must("init", "--run", "smoke")

	intake := filepath.Join(t.TempDir(), "intake.json")
	mustWrite(t, intake, `{
		"review_consent":"autoreview",
		"series_bound":1,
		"units":[{"id":"u1","title":"one"}]
	}`)
	h.must("intake", "--file", intake, "--run", "smoke")

	out := h.must("status", "--json", "--run", "smoke")
	var st struct {
		IntakeRevision int64 `json:"intake_revision"`
	}
	if err := json.Unmarshal([]byte(out), &st); err != nil {
		t.Fatalf("json: %v\n%s", err, out)
	}
	h.must("release", "--revision", itoa(st.IntakeRevision), "--run", "smoke")
	h.must("build", "--run", "smoke")
	h.must("verify", "--cmd", "true", "--run", "smoke")

	rev := filepath.Join(t.TempDir(), "review.json")
	mustWrite(t, rev, `{"reviewer":"autoreview","verdict":"clean","artifact_ref":"test://1"}`)
	h.must("review", "--evidence", rev, "--run", "smoke")

	del := filepath.Join(t.TempDir(), "deliver.json")
	mustWrite(t, del, `{"delivery_mode":"pr-hold","pr_url":"https://example.com/pr/1"}`)
	h.must("deliver", "--evidence", del, "--run", "smoke")

	out = h.must("status", "--json", "--run", "smoke")
	var fin struct {
		State string `json:"state"`
	}
	if err := json.Unmarshal([]byte(out), &fin); err != nil {
		t.Fatal(err)
	}
	if fin.State != "RUN_DONE" {
		t.Fatalf("want RUN_DONE got %s (%s)", fin.State, out)
	}

	// Terminal status remains readable without --run.
	out = h.must("status", "--json")
	if err := json.Unmarshal([]byte(out), &fin); err != nil || fin.State != "RUN_DONE" {
		t.Fatalf("status without --run: %v %s", err, out)
	}
}

func TestCLIMultiUnitAskDecide(t *testing.T) {
	h := newCLIHarness(t)
	h.must("init", "--run", "multi")

	intake := filepath.Join(t.TempDir(), "intake.json")
	mustWrite(t, intake, `{
		"review_consent":"bugbot",
		"series_bound":2,
		"units":[
			{"id":"u1","title":"first"},
			{"id":"u2","title":"second","blockers":["u1"]}
		]
	}`)
	h.must("intake", "--file", intake, "--run", "multi")

	out := h.must("status", "--json", "--run", "multi")
	var st struct {
		IntakeRevision int64 `json:"intake_revision"`
	}
	if err := json.Unmarshal([]byte(out), &st); err != nil {
		t.Fatal(err)
	}
	h.must("release", "--revision", itoa(st.IntakeRevision), "--run", "multi")

	// Interrupt before first build.
	h.must("ask", "--question", "keep pr-hold?", "--run", "multi")
	out = h.must("status", "--json", "--run", "multi")
	var parked struct {
		State            string `json:"state"`
		DecisionQuestion string `json:"decision_question"`
	}
	if err := json.Unmarshal([]byte(out), &parked); err != nil {
		t.Fatal(err)
	}
	if parked.State != "NEEDS_DECISION" || parked.DecisionQuestion == "" {
		t.Fatalf("ask park: %s", out)
	}
	h.must("decide", "--answer", "yes pr-hold", "--run", "multi")

	walkUnit := func(unit string, n int) {
		t.Helper()
		h.must("build", "--run", "multi")
		h.must("verify", "--cmd", "true", "--run", "multi")
		rev := filepath.Join(t.TempDir(), "review-"+unit+".json")
		mustWrite(t, rev, `{"reviewer":"bugbot","verdict":"clean","artifact_ref":"test://`+unit+`"}`)
		h.must("review", "--evidence", rev, "--run", "multi")
		del := filepath.Join(t.TempDir(), "deliver-"+unit+".json")
		mustWrite(t, del, `{"delivery_mode":"pr-hold","pr_url":"https://example.com/pr/`+itoa(int64(n))+`"}`)
		h.must("deliver", "--evidence", del, "--run", "multi")
	}
	walkUnit("u1", 1)
	walkUnit("u2", 2)

	out = h.must("status", "--json", "--run", "multi")
	var fin struct {
		State          string `json:"state"`
		CompletedUnits int    `json:"completed_units"`
	}
	if err := json.Unmarshal([]byte(out), &fin); err != nil {
		t.Fatal(err)
	}
	if fin.State != "RUN_DONE" || fin.CompletedUnits != 2 {
		t.Fatalf("want RUN_DONE completed=2 got %s", out)
	}
}

func TestCLIFailClosedGuards(t *testing.T) {
	h := newCLIHarness(t)
	h.must("init", "--run", "a")
	h.must("init", "--run", "b")

	_, code := h.run("status", "--json")
	if code != 5 {
		t.Fatalf("ambiguous status want exit 5 got %d", code)
	}

	intake := filepath.Join(t.TempDir(), "intake.json")
	mustWrite(t, intake, `{
		"review_consent":"autoreview",
		"series_bound":1,
		"units":[{"id":"u1","title":"one"}]
	}`)
	h.must("intake", "--file", intake, "--run", "a")

	_, code = h.run("build", "--run", "a")
	if code != 3 {
		t.Fatalf("build before release want exit 3 got %d", code)
	}

	// Internal state fields are rejected at the intake boundary.
	var out string
	for _, spoofed := range []struct{ field, value string }{{"attempt", "9"}, {"done", "true"}} {
		field, value := spoofed.field, spoofed.value
		spoof := filepath.Join(t.TempDir(), "spoof-"+field+".json")
		mustWrite(t, spoof, fmt.Sprintf(`{
			"review_consent":"autoreview",
			"series_bound":1,
			"units":[{"id":"u1","title":"one",%q:%s}]
		}`, field, value))
		out, code = h.run("intake", "--file", spoof, "--run", "a")
		if code != 2 || !strings.Contains(out, fmt.Sprintf(`unknown JSON field %q`, field)) {
			t.Fatalf("spoofed %s: want strict rejection, got exit %d %s", field, code, out)
		}
	}
	out = h.must("status", "--json", "--run", "a")
	var after struct {
		Released       bool     `json:"released"`
		Frontier       []string `json:"frontier"`
		IntakeRevision int64    `json:"intake_revision"`
	}
	if err := json.Unmarshal([]byte(out), &after); err != nil {
		t.Fatal(err)
	}
	if after.Released {
		t.Fatal("intake should leave run unreleased")
	}
	if len(after.Frontier) != 1 || after.Frontier[0] != "u1" {
		t.Fatalf("rejected intake changed frontier: %s", out)
	}

	h.must("release", "--revision", itoa(after.IntakeRevision), "--run", "a")
	h.must("build", "--run", "a")
	h.must("verify", "--cmd", "true", "--run", "a")

	emptyRev := filepath.Join(t.TempDir(), "empty-review.json")
	mustWrite(t, emptyRev, `{"reviewer":"autoreview","verdict":"clean"}`)
	out, code = h.run("review", "--evidence", emptyRev, "--run", "a")
	if code != 3 {
		t.Fatalf("empty artifact_ref want exit 3 got %d %s", code, out)
	}

	// Consent mismatch: bugbot evidence under autoreview consent.
	bad := filepath.Join(t.TempDir(), "bad-review.json")
	mustWrite(t, bad, `{"reviewer":"bugbot","verdict":"clean","artifact_ref":"x"}`)
	out, code = h.run("review", "--evidence", bad, "--run", "a")
	if code != 3 {
		t.Fatalf("consent mismatch want exit 3 got %d %s", code, out)
	}
}

func TestParseFlags(t *testing.T) {
	_, err := parseFlagsWith([]string{"--revision", "--run", "smoke"}, commandFlags["release"])
	if err == nil {
		t.Fatal("expected error when --revision has no value")
	}
	got, err := parseFlagsWith([]string{"--run=smoke", "--revision=2"}, commandFlags["release"])
	if err != nil {
		t.Fatal(err)
	}
	if got["run"] != "smoke" || got["revision"] != "2" {
		t.Fatalf("got %#v", got)
	}
	got, err = parseFlagsWith([]string{"--run=--json"}, commandFlags["build"])
	if err != nil || got["run"] != "--json" {
		t.Fatalf("flag-like run value: %#v %v", got, err)
	}
	_, err = parseFlagsWith([]string{"--nope", "x"}, commandFlags["release"])
	if err == nil {
		t.Fatal("expected unknown flag error")
	}
	_, err = parseFlagsWith([]string{"--addr", "127.0.0.1:7780"}, commandFlags["status"])
	if err == nil {
		t.Fatal("expected --addr unknown outside serve")
	}
	got, err = parseFlagsWith([]string{"--addr", "127.0.0.1:7780"}, commandFlags["serve"])
	if err != nil || got["addr"] != "127.0.0.1:7780" {
		t.Fatalf("serve flags: %#v %v", got, err)
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

func mustWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func itoa(v int64) string {
	return strconv.FormatInt(v, 10)
}

func TestUsageMentionsServe(t *testing.T) {
	got := usage()
	if !strings.Contains(got, "slopomatic serve") {
		t.Fatalf("usage missing serve:\n%s", got)
	}
}

func TestCommandHelpAndScopedFlags(t *testing.T) {
	help, ok := commandUsage("review")
	if !ok || !strings.Contains(help, "Verdicts: clean, findings, ambiguous") {
		t.Fatalf("review help: %q", help)
	}
	if _, err := parseFlagsWith([]string{"--evidence", "x"}, commandFlags["init"]); err == nil {
		t.Fatal("init accepted review evidence")
	}
	if _, err := parseFlagsWith([]string{"--json=false"}, commandFlags["status"]); err == nil {
		t.Fatal("boolean flag accepted a value")
	}
}

func TestVerifyPreflightAndRecovery(t *testing.T) {
	h := newCLIHarness(t)
	h.must("init", "--run", "verify")
	marker := filepath.Join(t.TempDir(), "should-not-exist")
	out, code := h.run("verify", "--cmd", "touch "+marker, "--run", "verify")
	if code != 3 || !strings.Contains(out, "not allowed from INTAKE") {
		t.Fatalf("preflight: exit %d %s", code, out)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("illegal verify executed command: %v", err)
	}

}

func TestVerifyFailureReturnsNonzeroAndCanRetry(t *testing.T) {
	h := newCLIHarness(t)
	h.must("init", "--run", "retry")
	intake := `{"series_bound":1,"units":[{"id":"u1","title":"one","blockers":[]}]}`
	out, code := h.runInput(intake, "intake", "--file", "-", "--run", "retry")
	if code != 0 {
		t.Fatalf("stdin intake: %d %s", code, out)
	}
	statusJSON := h.must("status", "--json", "--run", "retry")
	var statusDoc struct {
		IntakeRevision int64 `json:"intake_revision"`
	}
	if err := json.Unmarshal([]byte(statusJSON), &statusDoc); err != nil {
		t.Fatal(err)
	}
	h.must("release", "--revision", itoa(statusDoc.IntakeRevision), "--run", "retry")
	h.must("build", "--run", "retry")
	out, code = h.run("verify", "--cmd", "false", "--run", "retry")
	if code != 6 || !strings.Contains(out, "state=BLOCKED") || !strings.Contains(out, "slopomatic retry") {
		t.Fatalf("failed verify: exit %d %s", code, out)
	}
	out = h.must("retry", "--reason", "fixed the failure", "--run", "retry")
	if !strings.Contains(out, "state=BUILD") {
		t.Fatalf("retry: %s", out)
	}
}

func TestVerifyEvidenceRequiresExplicitExitCode(t *testing.T) {
	h := newCLIHarness(t)
	h.must("init", "--run", "evidence")
	intake := filepath.Join(t.TempDir(), "intake.json")
	mustWrite(t, intake, `{"units":[{"id":"u1","title":"one"}]}`)
	h.must("intake", "--file", intake, "--run", "evidence")
	out := h.must("status", "--json", "--run", "evidence")
	var statusDoc struct {
		IntakeRevision int64 `json:"intake_revision"`
	}
	if err := json.Unmarshal([]byte(out), &statusDoc); err != nil {
		t.Fatal(err)
	}
	h.must("release", "--revision", itoa(statusDoc.IntakeRevision), "--run", "evidence")
	h.must("build", "--run", "evidence")
	missing := filepath.Join(t.TempDir(), "verify.json")
	mustWrite(t, missing, `{"command":"true"}`)
	out, code := h.run("verify", "--evidence", missing, "--run", "evidence")
	if code != 2 || !strings.Contains(out, "verify.exit_code is required") {
		t.Fatalf("missing exit code: %d %s", code, out)
	}
}

func TestVerifyEvidenceRejectsBlankFailedCommandWithoutBlocking(t *testing.T) {
	h := newCLIHarness(t)
	h.must("init", "--run", "blank-failure")
	intake := filepath.Join(t.TempDir(), "intake.json")
	mustWrite(t, intake, `{"units":[{"id":"u1","title":"one"}]}`)
	h.must("intake", "--file", intake, "--run", "blank-failure")
	h.must("release", "--revision", "2", "--run", "blank-failure")
	h.must("build", "--run", "blank-failure")
	evidence := filepath.Join(t.TempDir(), "verify.json")
	mustWrite(t, evidence, `{"command":"   ","exit_code":1}`)
	out, code := h.run("verify", "--evidence", evidence, "--run", "blank-failure")
	if code != 3 || !strings.Contains(out, "verify.command required") {
		t.Fatalf("blank failure: exit %d %s", code, out)
	}
	out = h.must("status", "--json", "--run", "blank-failure")
	var statusDoc struct {
		State string `json:"state"`
	}
	if err := json.Unmarshal([]byte(out), &statusDoc); err != nil || statusDoc.State != "BUILD" {
		t.Fatalf("blank failure changed state: err=%v %s", err, out)
	}
}

func TestRunEntryPointsAndFullRecoveryFlow(t *testing.T) {
	for _, args := range [][]string{
		nil, {"--help"}, {"help"}, {"version"}, {"--version"},
		{"init", "--help"}, {"intake", "-h"}, {"release", "--help"},
		{"build", "--help"}, {"verify", "--help"}, {"review", "--help"},
		{"rework", "--help"}, {"deliver", "--help"}, {"ask", "--help"},
		{"decide", "--help"}, {"retry", "--help"}, {"block", "--help"},
		{"status", "--help"}, {"serve", "--help"},
		{"version", "--help"},
	} {
		if code := run(args); code != 0 {
			t.Fatalf("run(%v)=%d", args, code)
		}
	}
	if code := run([]string{"unknown", "--help"}); code != 2 {
		t.Fatalf("unknown help=%d", code)
	}
	if code := run([]string{"version", "--run", "unexpected"}); code != 2 {
		t.Fatalf("version accepted scoped flag: %d", code)
	}
	if code := run([]string{"--version", "unexpected"}); code != 2 {
		t.Fatalf("--version accepted argument: %d", code)
	}

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
	t.Setenv("SLOPOMATIC_DB", filepath.Join(t.TempDir(), "direct.sqlite"))

	if code := run([]string{"unknown"}); code != 2 {
		t.Fatalf("unknown=%d", code)
	}
	if code := run([]string{"init", "--run", "direct"}); code != 0 {
		t.Fatalf("init=%d", code)
	}
	for _, args := range [][]string{
		{"intake", "--run", "direct"},
		{"release", "--run", "direct"},
		{"release", "--revision", "nope", "--run", "direct"},
		{"verify", "--run", "direct"},
		{"verify", "--cmd", "true", "--evidence", "x", "--run", "direct"},
		{"review", "--run", "direct"}, {"deliver", "--run", "direct"},
		{"ask", "--run", "direct"}, {"decide", "--run", "direct"},
		{"retry", "--run", "direct"}, {"block", "--run", "direct"},
	} {
		if code := run(args); code != 2 {
			t.Fatalf("missing args %v=%d", args, code)
		}
	}

	intake := filepath.Join(t.TempDir(), "intake.json")
	mustWrite(t, intake, `{"delivery_mode":"direct-trunk","review_consent":"both","series_bound":1,"units":[{"id":"u1","title":"one"}]}`)
	if code := run([]string{"intake", "--file", intake, "--run", "direct"}); code != 0 {
		t.Fatalf("intake=%d", code)
	}
	if code := run([]string{"status", "--json", "--run", "direct"}); code != 0 {
		t.Fatalf("status=%d", code)
	}
	if code := run([]string{"release", "--revision", "2", "--run", "direct"}); code != 0 {
		t.Fatalf("release=%d", code)
	}
	if code := run([]string{"build", "--run", "direct"}); code != 0 {
		t.Fatalf("build=%d", code)
	}
	if code := run([]string{"block", "--reason", "dependency unavailable", "--run", "direct"}); code != 0 {
		t.Fatalf("block=%d", code)
	}
	if code := run([]string{"retry", "--reason", "dependency restored", "--run", "direct"}); code != 0 {
		t.Fatalf("retry=%d", code)
	}
	verify := filepath.Join(t.TempDir(), "verify.json")
	mustWrite(t, verify, `{"command":"go test ./...","exit_code":0,"output_digest":"sha256:test"}`)
	if code := run([]string{"verify", "--evidence", verify, "--run", "direct"}); code != 0 {
		t.Fatalf("verify=%d", code)
	}
	if code := run([]string{"rework", "--run", "direct"}); code != 0 {
		t.Fatalf("rework=%d", code)
	}
	if code := run([]string{"build", "--run", "direct"}); code != 0 {
		t.Fatalf("rebuild=%d", code)
	}
	if code := run([]string{"verify", "--cmd", "true", "--run", "direct"}); code != 0 {
		t.Fatalf("reverify=%d", code)
	}
	if code := runReviewInput(t, "direct", `{"reviewer":"autoreview","verdict":"ambiguous","artifact_ref":"review://ambiguous"}`); code != 0 {
		t.Fatalf("ambiguous review=%d", code)
	}
	if code := run([]string{"decide", "--answer", "continue review", "--run", "direct"}); code != 0 {
		t.Fatalf("decide=%d", code)
	}
	if code := runReviewInput(t, "direct", `{"reviewer":"autoreview","verdict":"clean","artifact_ref":"review://autoreview"}`); code != 0 {
		t.Fatalf("autoreview=%d", code)
	}
	if code := runReviewInput(t, "direct", `{"reviewer":"bugbot","verdict":"clean","artifact_ref":"review://bugbot"}`); code != 0 {
		t.Fatalf("bugbot=%d", code)
	}
	delivery := filepath.Join(t.TempDir(), "delivery.json")
	mustWrite(t, delivery, `{"delivery_mode":"direct-trunk","commit_sha":"abc123"}`)
	if code := run([]string{"deliver", "--evidence", delivery, "--run", "direct"}); code != 0 {
		t.Fatalf("deliver=%d", code)
	}
}

func TestRunAutoIDAndDefaultDataPath(t *testing.T) {
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
	t.Setenv("SLOPOMATIC_DB", "")
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	if code := run([]string{"init"}); code != 0 {
		t.Fatalf("auto init=%d", code)
	}
}

func TestDirectErrorMappingJSONAndServeBindFailure(t *testing.T) {
	for err, want := range map[error]int{
		machine.ErrBadArgs: 2, machine.ErrIllegalTransition: 3, machine.ErrUnmetGuard: 3,
		machine.ErrRevisionConflict: 4, machine.ErrAmbiguousRun: 5, machine.ErrNotFound: 5,
		machine.ErrCorruptState: 5, errors.New("storage failed"): 10,
	} {
		if got := mapErr(fmt.Errorf("context: %w", err)); got != want {
			t.Errorf("mapErr(%v)=%d want %d", err, got, want)
		}
	}
	if !flagHas([]string{"--json"}, "--json") || !flagHas([]string{"--json=true"}, "--json") || flagHas(nil, "--json") {
		t.Fatal("flagHas contract")
	}

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
	t.Setenv("SLOPOMATIC_DB", filepath.Join(t.TempDir(), "errors.sqlite"))

	if code := run([]string{"status", "--run", "missing"}); code != 5 {
		t.Fatalf("missing status=%d", code)
	}
	if code := run([]string{"init", "--run", "errors"}); code != 0 {
		t.Fatalf("init=%d", code)
	}
	if code := run([]string{"status", "--run", "errors"}); code != 0 {
		t.Fatalf("plain status=%d", code)
	}
	if code := run([]string{"build", "--run", "errors"}); code != 3 {
		t.Fatalf("illegal build=%d", code)
	}
	if code := run([]string{"init", "--run", "errors"}); code != 10 {
		t.Fatalf("duplicate init=%d", code)
	}
	for _, args := range [][]string{
		{"intake", "--file", "missing.json", "--run", "errors"},
		{"review", "--evidence", "missing.json", "--run", "errors"},
		{"deliver", "--evidence", "missing.json", "--run", "errors"},
	} {
		if code := run(args); code != 2 {
			t.Fatalf("missing JSON %v=%d", args, code)
		}
	}
	badJSON := filepath.Join(t.TempDir(), "bad.json")
	mustWrite(t, badJSON, `{} {}`)
	if code := run([]string{"intake", "--file", badJSON, "--run", "errors"}); code != 2 {
		t.Fatalf("trailing JSON=%d", code)
	}
	nullJSON := filepath.Join(t.TempDir(), "null.json")
	mustWrite(t, nullJSON, `null`)
	if code := run([]string{"intake", "--file", nullJSON, "--run", "errors"}); code != 2 {
		t.Fatalf("null JSON=%d", code)
	}
	nestedNullJSON := filepath.Join(t.TempDir(), "nested-null.json")
	mustWrite(t, nestedNullJSON, `{"series_bound": null}`)
	if code := run([]string{"intake", "--file", nestedNullJSON, "--run", "errors"}); code != 2 {
		t.Fatalf("nested null JSON=%d", code)
	}
	duplicateJSON := filepath.Join(t.TempDir(), "duplicate.json")
	mustWrite(t, duplicateJSON, `{"series_bound": 1, "series_bound": 2}`)
	if code := run([]string{"intake", "--file", duplicateJSON, "--run", "errors"}); code != 2 {
		t.Fatalf("duplicate JSON=%d", code)
	}
	caseFoldedJSON := filepath.Join(t.TempDir(), "case-folded.json")
	mustWrite(t, caseFoldedJSON, `{"Units": []}`)
	if code := run([]string{"intake", "--file", caseFoldedJSON, "--run", "errors"}); code != 2 {
		t.Fatalf("case-folded JSON=%d", code)
	}
	caseAliasJSON := filepath.Join(t.TempDir(), "case-alias.json")
	mustWrite(t, caseAliasJSON, `{"units": [], "Units": []}`)
	if code := run([]string{"intake", "--file", caseAliasJSON, "--run", "errors"}); code != 2 {
		t.Fatalf("case-alias JSON=%d", code)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	if code := run([]string{"serve", "--addr", ln.Addr().String()}); code != 10 {
		t.Fatalf("serve bind failure=%d", code)
	}
	if code := run([]string{"serve", "--addr", "0.0.0.0:7780"}); code != 2 {
		t.Fatalf("serve unsafe addr=%d", code)
	}
}

func TestResolveRepoKeyMigratesMalformedVersionOneIdentity(t *testing.T) {
	repoDir := t.TempDir()
	runGit(t, repoDir, "init")
	runGit(t, repoDir, "remote", "add", "origin", "https://TOKEN@host:bad?keep=yes")
	key, root, err := repo.Keys(repoDir)
	if err != nil {
		t.Fatal(err)
	}
	legacyKey := "https://TOKEN@host:bad?keep=yes|" + root
	if strings.Contains(key, "TOKEN") {
		t.Fatalf("key=%q", key)
	}

	st, err := store.Open(filepath.Join(t.TempDir(), "legacy.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.CreateRun(machine.NewRun("legacy", legacyKey), nil); err != nil {
		t.Fatal(err)
	}
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repoDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldDir) })
	if got, err := resolveRepoKey(st); err != nil || got != key {
		t.Fatalf("resolveRepoKey=%q err=%v", got, err)
	}
	if got, _, err := st.ResolveStatusRun(key, "legacy"); err != nil || got.RepoKey != key {
		t.Fatalf("migrated run=%+v err=%v", got, err)
	}
	if oldRuns, err := st.ListRuns(legacyKey); err != nil || len(oldRuns) != 0 {
		t.Fatalf("legacy rows=%+v err=%v", oldRuns, err)
	}
}

func runReviewInput(t *testing.T, runID, body string) int {
	t.Helper()
	path := filepath.Join(t.TempDir(), "review.json")
	mustWrite(t, path, body)
	return run([]string{"review", "--evidence", path, "--run", runID})
}
