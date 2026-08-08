package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
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
	build := exec.Command("go", "build", "-o", bin, ".")
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
	h.t.Helper()
	cmd := exec.Command(h.bin, args...)
	cmd.Dir = h.repoDir
	cmd.Env = h.env
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

	// Spoofed done/attempt in intake JSON must not skip the unit.
	spoof := filepath.Join(t.TempDir(), "spoof.json")
	mustWrite(t, spoof, `{
		"review_consent":"autoreview",
		"series_bound":1,
		"units":[{"id":"u1","title":"one","done":true,"attempt":9}]
	}`)
	h.must("intake", "--file", spoof, "--run", "a")
	out := h.must("status", "--json", "--run", "a")
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
		t.Fatalf("spoofed done kept unit off frontier: %s", out)
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
	_, err := parseFlags([]string{"--revision", "--run", "smoke"})
	if err == nil {
		t.Fatal("expected error when --revision has no value")
	}
	got, err := parseFlags([]string{"--run=smoke", "--revision=2"})
	if err != nil {
		t.Fatal(err)
	}
	if got["run"] != "smoke" || got["revision"] != "2" {
		t.Fatalf("got %#v", got)
	}
	_, err = parseFlags([]string{"--nope", "x"})
	if err == nil {
		t.Fatal("expected unknown flag error")
	}
	_, err = parseFlags([]string{"--addr", "127.0.0.1:7780"})
	if err == nil {
		t.Fatal("expected --addr unknown outside serve")
	}
	got, err = parseFlagsWith([]string{"--addr", "127.0.0.1:7780"}, serveFlags)
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
