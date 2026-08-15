package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const emptyThreadsJSON = `{"data":{"repository":{"pullRequest":{"reviewThreads":{"pageInfo":{"hasNextPage":false,"endCursor":""},"nodes":[]}}}}}`

// installFakeGH puts a gh stub first on PATH that serves canned view and
// GraphQL payloads, so watch exercises the real adapter without the network.
func installFakeGH(t *testing.T, h *cliHarness, viewJSON, threadsJSON string) (viewFile string) {
	t.Helper()
	binDir := t.TempDir()
	viewFile = filepath.Join(binDir, "view.json")
	threadsFile := filepath.Join(binDir, "threads.json")
	mustWrite(t, viewFile, viewJSON)
	mustWrite(t, threadsFile, threadsJSON)
	script := fmt.Sprintf(`#!/bin/bash
case "$1" in
  pr) cat %q ;;
  api) cat %q ;;
  *) echo "unexpected gh invocation: $*" >&2; exit 1 ;;
esac
`, viewFile, threadsFile)
	ghPath := filepath.Join(binDir, "gh")
	if err := os.WriteFile(ghPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	h.env = append(h.env, "PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return viewFile
}

func deliverWatchableRun(t *testing.T, h *cliHarness, runID string) {
	t.Helper()
	h.must("init", "--run", runID)
	intake := filepath.Join(t.TempDir(), "intake.json")
	mustWrite(t, intake, `{
		"required_reviewers":["autoreview"],
		"series_bound":1,
		"units":[{"id":"u1","title":"one"}]
	}`)
	h.must("intake", "--file", intake, "--run", runID)
	h.must("release", "--revision", "2", "--run", runID)
	h.must("build", "--run", runID)
	h.must("verify", "--cmd", "true", "--run", runID)
	review := filepath.Join(t.TempDir(), "review.json")
	mustWrite(t, review, `{"reviewer":"autoreview","verdict":"clean","artifact_ref":"test://1"}`)
	h.must("review", "--evidence", review, "--run", runID)
	deliver := filepath.Join(t.TempDir(), "deliver.json")
	mustWrite(t, deliver, `{"delivery_mode":"pr-hold","pr_url":"https://github.com/o/r/pull/1","commit_sha":"aaaa1111aaaa1111"}`)
	h.must("deliver", "--evidence", deliver, "--run", runID)
}

func decodeWatchDoc(t *testing.T, out string) watchDocument {
	t.Helper()
	var doc watchDocument
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("json: %v\n%s", err, out)
	}
	return doc
}

func TestWatchOnceSettlesMergedUnit(t *testing.T) {
	h := newCLIHarness(t)
	deliverWatchableRun(t, h, "w1")
	installFakeGH(t, h,
		`{"headRefOid":"aaaa1111aaaa1111","state":"MERGED","mergeable":"UNKNOWN","statusCheckRollup":[]}`,
		emptyThreadsJSON)

	out := h.must("watch", "--once", "--json", "--run", "w1")
	doc := decodeWatchDoc(t, out)
	if len(doc.Observations) != 1 || doc.Observations[0].Signal != "merged" || !doc.Observations[0].Recorded {
		t.Fatalf("merged must be recorded: %s", out)
	}
	if doc.State != "RUN_DONE" {
		t.Fatalf("single-unit run must finish after merge: %s", out)
	}

	// Nothing is delivered anymore: another pass records nothing.
	out = h.must("watch", "--json", "--run", "w1")
	doc = decodeWatchDoc(t, out)
	if len(doc.Observations) != 0 || !strings.Contains(doc.Stopped, "no delivered unit") {
		t.Fatalf("idempotent pass must record nothing: %s", out)
	}
}

func TestWatchRecordsFailedChecksAsReworkCause(t *testing.T) {
	h := newCLIHarness(t)
	deliverWatchableRun(t, h, "w2")
	installFakeGH(t, h,
		`{"headRefOid":"aaaa1111aaaa1111","state":"OPEN","mergeable":"MERGEABLE","statusCheckRollup":[{"status":"COMPLETED","conclusion":"FAILURE"}]}`,
		emptyThreadsJSON)

	out := h.must("watch", "--once", "--json", "--run", "w2")
	doc := decodeWatchDoc(t, out)
	if len(doc.Observations) != 1 || doc.Observations[0].Signal != "checks_failed" || !doc.Observations[0].Recorded {
		t.Fatalf("failed checks must be recorded: %s", out)
	}
	if !strings.Contains(doc.NextAction, "slopshipper build") {
		t.Fatalf("post-observation next_action must be executable recovery: %s", out)
	}

	var st struct {
		Units []struct {
			ID          string `json:"id"`
			Phase       string `json:"phase"`
			ReworkCause string `json:"rework_cause"`
		} `json:"units"`
	}
	statusOut := h.must("status", "--json", "--run", "w2")
	if err := json.Unmarshal([]byte(statusOut), &st); err != nil {
		t.Fatal(err)
	}
	if st.Units[0].Phase != "rework" || !strings.Contains(st.Units[0].ReworkCause, "checks_failed") {
		t.Fatalf("unit must be rework-eligible with the observation as cause: %s", statusOut)
	}
}

func TestWatchDetectsMovedHead(t *testing.T) {
	h := newCLIHarness(t)
	deliverWatchableRun(t, h, "w3")
	installFakeGH(t, h,
		`{"headRefOid":"bbbb2222bbbb2222","state":"OPEN","mergeable":"MERGEABLE","statusCheckRollup":[{"status":"COMPLETED","conclusion":"SUCCESS"}]}`,
		emptyThreadsJSON)

	out := h.must("watch", "--json", "--run", "w3")
	doc := decodeWatchDoc(t, out)
	if len(doc.Observations) != 1 || doc.Observations[0].Signal != "head_moved" || !doc.Observations[0].Recorded {
		t.Fatalf("moved head must invalidate delivery evidence: %s", out)
	}
}

func TestWatchWaitingStateRecordsNothingTwice(t *testing.T) {
	h := newCLIHarness(t)
	deliverWatchableRun(t, h, "w4")
	installFakeGH(t, h,
		`{"headRefOid":"aaaa1111aaaa1111","state":"OPEN","mergeable":"MERGEABLE","statusCheckRollup":[{"status":"IN_PROGRESS","conclusion":""}]}`,
		emptyThreadsJSON)

	for pass := 0; pass < 2; pass++ {
		out := h.must("watch", "--json", "--run", "w4")
		doc := decodeWatchDoc(t, out)
		if len(doc.Observations) != 1 || doc.Observations[0].Recorded || doc.Observations[0].Signal != "" {
			t.Fatalf("unchanged pending state must record nothing: %s", out)
		}
		if doc.State != "AWAITING_SIGNALS" {
			t.Fatalf("run must keep waiting: %s", out)
		}
	}
}

const oneThreadJSON = `{"data":{"repository":{"pullRequest":{"reviewThreads":{"pageInfo":{"hasNextPage":false,"endCursor":""},"nodes":[{"isResolved":false,"path":"a.go","line":3,"comments":{"nodes":[{"author":{"login":"rev"},"body":"fix this"}]}}]}}}}}`

func TestWatchDoesNotLoopOnUnchangedFeedback(t *testing.T) {
	h := newCLIHarness(t)
	deliverWatchableRun(t, h, "w8")
	installFakeGH(t, h,
		`{"headRefOid":"aaaa1111aaaa1111","state":"OPEN","mergeable":"MERGEABLE","statusCheckRollup":[{"status":"COMPLETED","conclusion":"SUCCESS"}]}`,
		oneThreadJSON)

	out := h.must("watch", "--json", "--run", "w8")
	doc := decodeWatchDoc(t, out)
	if doc.Observations[0].Signal != "review_feedback" || !doc.Observations[0].Recorded {
		t.Fatalf("first unresolved feedback must be recorded: %s", out)
	}

	// The unit re-delivers with the same thread still unresolved on the
	// forge; identical feedback must not pull it back again.
	h.must("build", "--run", "w8")
	h.must("verify", "--cmd", "true", "--run", "w8")
	review := filepath.Join(t.TempDir(), "review.json")
	mustWrite(t, review, `{"reviewer":"autoreview","verdict":"clean","artifact_ref":"test://2"}`)
	h.must("review", "--evidence", review, "--run", "w8")
	deliver := filepath.Join(t.TempDir(), "deliver.json")
	mustWrite(t, deliver, `{"delivery_mode":"pr-hold","pr_url":"https://github.com/o/r/pull/1","commit_sha":"aaaa1111aaaa1111"}`)
	h.must("deliver", "--evidence", deliver, "--run", "w8")

	out = h.must("watch", "--json", "--run", "w8")
	doc = decodeWatchDoc(t, out)
	if doc.Observations[0].Recorded || doc.Observations[0].Signal != "" {
		t.Fatalf("identical feedback must not re-record: %s", out)
	}
	if !strings.Contains(doc.Observations[0].Note, "already recorded") {
		t.Fatalf("skip must be explained: %s", out)
	}
	if doc.State != "AWAITING_SIGNALS" {
		t.Fatalf("unit must stay delivered: %s", out)
	}
}

func threadsJSON(threads ...string) string {
	return `{"data":{"repository":{"pullRequest":{"reviewThreads":{"pageInfo":{"hasNextPage":false,"endCursor":""},"nodes":[` + strings.Join(threads, ",") + `]}}}}}`
}

func threadNode(id, path string, line int, commentID, body string) string {
	return fmt.Sprintf(`{"id":%q,"isResolved":false,"path":%q,"line":%d,"comments":{"nodes":[{"id":%q,"author":{"login":"rev"},"body":%q}]}}`, id, path, line, commentID, body)
}

func TestWatchFeedbackSubsetDoesNotRetrigger(t *testing.T) {
	h := newCLIHarness(t)
	deliverWatchableRun(t, h, "w9")
	view := `{"headRefOid":"aaaa1111aaaa1111","state":"OPEN","mergeable":"MERGEABLE","statusCheckRollup":[{"status":"COMPLETED","conclusion":"SUCCESS"}]}`
	viewFile := installFakeGH(t, h, view, threadsJSON(
		threadNode("t1", "a.go", 3, "c1", "fix this"),
		threadNode("t2", "b.go", 9, "c2", "and this"),
	))
	threadsFile := filepath.Join(filepath.Dir(viewFile), "threads.json")

	out := h.must("watch", "--json", "--run", "w9")
	if doc := decodeWatchDoc(t, out); !doc.Observations[0].Recorded {
		t.Fatalf("two-thread feedback must record: %s", out)
	}

	redeliver := func() {
		h.must("build", "--run", "w9")
		h.must("verify", "--cmd", "true", "--run", "w9")
		review := filepath.Join(t.TempDir(), "review.json")
		mustWrite(t, review, `{"reviewer":"autoreview","verdict":"clean","artifact_ref":"test://r"}`)
		h.must("review", "--evidence", review, "--run", "w9")
		deliver := filepath.Join(t.TempDir(), "deliver.json")
		mustWrite(t, deliver, `{"delivery_mode":"pr-hold","pr_url":"https://github.com/o/r/pull/1","commit_sha":"aaaa1111aaaa1111"}`)
		h.must("deliver", "--evidence", deliver, "--run", "w9")
	}
	redeliver()

	// One thread resolved: the remaining set is a subset — nothing new.
	mustWrite(t, threadsFile, threadsJSON(threadNode("t1", "a.go", 3, "c1", "fix this")))
	out = h.must("watch", "--json", "--run", "w9")
	doc := decodeWatchDoc(t, out)
	if doc.Observations[0].Recorded || !strings.Contains(doc.Observations[0].Note, "no new feedback") {
		t.Fatalf("subset feedback must not re-trigger rework: %s", out)
	}

	// A new comment on the old thread is new feedback.
	mustWrite(t, threadsFile, threadsJSON(threadNode("t1", "a.go", 3, "c3", "still broken")))
	out = h.must("watch", "--json", "--run", "w9")
	if doc := decodeWatchDoc(t, out); !doc.Observations[0].Recorded {
		t.Fatalf("a new comment must re-trigger: %s", out)
	}
}

func TestWatchFeedbackBeyondSampleBoundStillTriggers(t *testing.T) {
	h := newCLIHarness(t)
	deliverWatchableRun(t, h, "w10")
	view := `{"headRefOid":"aaaa1111aaaa1111","state":"OPEN","mergeable":"MERGEABLE","statusCheckRollup":[{"status":"COMPLETED","conclusion":"SUCCESS"}]}`
	tenThreads := make([]string, 0, 11)
	for i := 0; i < 10; i++ {
		tenThreads = append(tenThreads, threadNode(fmt.Sprintf("t%d", i), fmt.Sprintf("f%d.go", i), i+1, fmt.Sprintf("c%d", i), "fix"))
	}
	viewFile := installFakeGH(t, h, view, threadsJSON(tenThreads...))
	threadsFile := filepath.Join(filepath.Dir(viewFile), "threads.json")

	out := h.must("watch", "--json", "--run", "w10")
	if doc := decodeWatchDoc(t, out); !doc.Observations[0].Recorded {
		t.Fatalf("ten-thread feedback must record: %s", out)
	}
	h.must("build", "--run", "w10")
	h.must("verify", "--cmd", "true", "--run", "w10")
	review := filepath.Join(t.TempDir(), "review.json")
	mustWrite(t, review, `{"reviewer":"autoreview","verdict":"clean","artifact_ref":"test://r"}`)
	h.must("review", "--evidence", review, "--run", "w10")
	deliver := filepath.Join(t.TempDir(), "deliver.json")
	mustWrite(t, deliver, `{"delivery_mode":"pr-hold","pr_url":"https://github.com/o/r/pull/1","commit_sha":"aaaa1111aaaa1111"}`)
	h.must("deliver", "--evidence", deliver, "--run", "w10")

	// An 11th thread lands outside the 10-thread sample: the sampled tokens
	// are all previously recorded, but the count changed — must re-trigger.
	eleven := append(append([]string{}, tenThreads...), threadNode("t10", "f10.go", 11, "c10", "new"))
	mustWrite(t, threadsFile, threadsJSON(eleven...))
	out = h.must("watch", "--json", "--run", "w10")
	if doc := decodeWatchDoc(t, out); !doc.Observations[0].Recorded {
		t.Fatalf("feedback beyond the sample bound must still trigger: %s", out)
	}
}

func TestDeliverRejectsCallerSuppliedUnitField(t *testing.T) {
	h := newCLIHarness(t)
	h.must("init", "--run", "d1")
	intake := filepath.Join(t.TempDir(), "intake.json")
	mustWrite(t, intake, `{"required_reviewers":["autoreview"],"series_bound":1,"units":[{"id":"u1","title":"one"}]}`)
	h.must("intake", "--file", intake, "--run", "d1")
	h.must("release", "--revision", "2", "--run", "d1")
	h.must("build", "--run", "d1")
	h.must("verify", "--cmd", "true", "--run", "d1")
	review := filepath.Join(t.TempDir(), "review.json")
	mustWrite(t, review, `{"reviewer":"autoreview","verdict":"clean","artifact_ref":"test://1"}`)
	h.must("review", "--evidence", review, "--run", "d1")

	for unitField, want := range map[string]string{
		`"unit":""`:   "must not name a unit",
		`"unit":"u1"`: "must not name a unit",
		`"unit":null`: "null JSON value",
	} {
		deliver := filepath.Join(t.TempDir(), "deliver.json")
		mustWrite(t, deliver, `{`+unitField+`,"delivery_mode":"pr-hold","pr_url":"https://github.com/o/r/pull/1"}`)
		out, code := h.run("deliver", "--evidence", deliver, "--run", "d1")
		if code != 2 || !strings.Contains(out, want) {
			t.Fatalf("%s must be rejected: exit %d\n%s", unitField, code, out)
		}
	}
}

func TestWatchDryRunReportsWithoutRecording(t *testing.T) {
	h := newCLIHarness(t)
	deliverWatchableRun(t, h, "w5")
	installFakeGH(t, h,
		`{"headRefOid":"aaaa1111aaaa1111","state":"MERGED","mergeable":"UNKNOWN","statusCheckRollup":[]}`,
		emptyThreadsJSON)

	out := h.must("--dry-run", "watch", "--json", "--run", "w5")
	doc := decodeWatchDoc(t, out)
	if !doc.DryRun || len(doc.Observations) != 1 || doc.Observations[0].Signal != "merged" || doc.Observations[0].Recorded {
		t.Fatalf("dry run must report the signal without recording: %s", out)
	}
	// The document projects what a real pass would leave behind...
	if doc.State != "RUN_DONE" {
		t.Fatalf("dry run must project the would-be closing state: %s", out)
	}
	// ...while the store stays untouched.
	var st struct {
		State string `json:"state"`
	}
	statusOut := h.must("status", "--json", "--run", "w5")
	if err := json.Unmarshal([]byte(statusOut), &st); err != nil {
		t.Fatal(err)
	}
	if st.State != "AWAITING_SIGNALS" {
		t.Fatalf("dry run must not settle the unit: %s", statusOut)
	}
}

func TestWatchIntervalRecoversFromRateLimit(t *testing.T) {
	h := newCLIHarness(t)
	deliverWatchableRun(t, h, "wi")
	binDir := t.TempDir()
	viewFile := filepath.Join(binDir, "view.json")
	mustWrite(t, viewFile, `{"headRefOid":"aaaa1111aaaa1111","state":"MERGED","mergeable":"UNKNOWN","statusCheckRollup":[]}`)
	threadsFile := filepath.Join(binDir, "threads.json")
	mustWrite(t, threadsFile, emptyThreadsJSON)
	marker := filepath.Join(binDir, "failed-once")
	// The first invocation rate-limits and drops a marker; every later one
	// serves healthy payloads, so pass one aborts and pass two recovers.
	script := fmt.Sprintf(`#!/bin/bash
if [ ! -f %q ]; then
  touch %q
  echo 'HTTP 429: rate limit exceeded' >&2
  exit 1
fi
case "$1" in
  pr) cat %q ;;
  api) cat %q ;;
  *) exit 1 ;;
esac
`, marker, marker, viewFile, threadsFile)
	if err := os.WriteFile(filepath.Join(binDir, "gh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	h.env = append(h.env, "PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	out, code := h.run("--json", "watch", "--interval", "5", "--iterations", "2", "--run", "wi")
	if code != 0 {
		t.Fatalf("a failure recovered by a later pass must exit clean: exit %d\n%s", code, out)
	}
	doc := decodeWatchDoc(t, out)
	if doc.ErrorKind != "" || doc.Iterations != 2 || doc.State != "RUN_DONE" {
		t.Fatalf("recovery must record on the second pass: %s", out)
	}
	if doc.Observations[0].ErrorKind != "rate_limit" || !doc.Observations[1].Recorded {
		t.Fatalf("the first pass failure and second pass recording must both be reported: %s", out)
	}
}

func TestWatchIntervalExhaustsBoundsCleanly(t *testing.T) {
	h := newCLIHarness(t)
	deliverWatchableRun(t, h, "wb")
	installFakeGH(t, h,
		`{"headRefOid":"aaaa1111aaaa1111","state":"OPEN","mergeable":"MERGEABLE","statusCheckRollup":[{"status":"IN_PROGRESS","conclusion":""}]}`,
		emptyThreadsJSON)

	out := h.must("--json", "watch", "--interval", "5", "--iterations", "2", "--run", "wb")
	doc := decodeWatchDoc(t, out)
	if doc.Iterations != 2 || doc.Stopped != "iterations exhausted" || doc.ErrorKind != "" {
		t.Fatalf("bounded interval mode must exhaust cleanly while waiting: %s", out)
	}
	if len(doc.Observations) != 2 || doc.Observations[1].Recorded {
		t.Fatalf("each pass must observe without recording: %s", out)
	}
}

func TestWatchFlagValidation(t *testing.T) {
	h := newCLIHarness(t)
	deliverWatchableRun(t, h, "w6")
	for _, args := range [][]string{
		{"watch", "--once", "--interval", "10"},
		{"watch", "--interval", "1"},
		{"watch", "--interval", "9999"},
		{"watch", "--iterations", "5"},
		{"watch", "--interval", "10", "--iterations", "0"},
		{"watch", "--once=true"},
	} {
		if out, code := h.run(args...); code != 2 {
			t.Fatalf("%v must fail validation: exit %d\n%s", args, code, out)
		}
	}
}

// TestDirectWatchCommand exercises cmdWatch in-process so package coverage
// reflects the watch paths the harness tests drive through a child binary.
func TestDirectWatchCommand(t *testing.T) {
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
	t.Setenv("SLOPSHIPPER_DB", filepath.Join(t.TempDir(), "direct-watch.sqlite"))

	binDir := t.TempDir()
	viewFile := filepath.Join(binDir, "view.json")
	mustWrite(t, viewFile, `{"headRefOid":"aaaa1111aaaa1111","state":"OPEN","mergeable":"MERGEABLE","statusCheckRollup":[{"status":"COMPLETED","conclusion":"FAILURE"}]}`)
	threadsFile := filepath.Join(binDir, "threads.json")
	mustWrite(t, threadsFile, emptyThreadsJSON)
	script := fmt.Sprintf("#!/bin/bash\ncase \"$1\" in\n  pr) cat %q ;;\n  api) cat %q ;;\n  *) exit 1 ;;\nesac\n", viewFile, threadsFile)
	if err := os.WriteFile(filepath.Join(binDir, "gh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	for _, args := range [][]string{
		{"init", "--run", "dw"},
		{"release", "--revision", "2", "--run", "dw"}, // fails: no units yet
	} {
		run(args)
	}
	intake := filepath.Join(t.TempDir(), "intake.json")
	mustWrite(t, intake, `{"required_reviewers":["autoreview"],"series_bound":1,"units":[{"id":"u1","title":"one"}]}`)
	steps := [][]string{
		{"intake", "--file", intake, "--run", "dw"},
		{"release", "--revision", "2", "--run", "dw"},
		{"build", "--run", "dw"},
		{"verify", "--cmd", "true", "--run", "dw"},
	}
	for _, args := range steps {
		if code := run(args); code != 0 {
			t.Fatalf("%v failed", args)
		}
	}
	review := filepath.Join(t.TempDir(), "review.json")
	mustWrite(t, review, `{"reviewer":"autoreview","verdict":"clean","artifact_ref":"test://1"}`)
	if code := run([]string{"review", "--evidence", review, "--run", "dw"}); code != 0 {
		t.Fatal("review failed")
	}
	deliver := filepath.Join(t.TempDir(), "deliver.json")
	mustWrite(t, deliver, `{"delivery_mode":"pr-hold","pr_url":"https://github.com/o/r/pull/9","commit_sha":"aaaa1111aaaa1111"}`)
	if code := run([]string{"deliver", "--evidence", deliver, "--run", "dw"}); code != 0 {
		t.Fatal("deliver failed")
	}

	if code := run([]string{"watch", "--once", "--interval", "10"}); code != 2 {
		t.Fatalf("flag conflict=%d", code)
	}
	if code := run([]string{"watch", "--iterations", "3"}); code != 2 {
		t.Fatalf("iterations without interval=%d", code)
	}
	if code := run([]string{"--dry-run", "watch", "--json", "--run", "dw"}); code != 0 {
		t.Fatalf("dry-run watch failed")
	}
	if code := run([]string{"watch", "--once", "--run", "dw"}); code != 0 {
		t.Fatalf("watch failed")
	}
	// The unit is rework now; a second pass has nothing delivered to observe.
	if code := run([]string{"--json", "watch", "--run", "dw"}); code != 0 {
		t.Fatalf("idempotent watch failed")
	}

	// A second run covers the per-unit degradation branches: one observable
	// unit, one with a URL the GitHub adapter cannot parse.
	if code := run([]string{"init", "--run", "dw2"}); code != 0 {
		t.Fatal("init dw2")
	}
	intake2 := filepath.Join(t.TempDir(), "intake2.json")
	mustWrite(t, intake2, `{"required_reviewers":["autoreview"],"series_bound":2,"units":[{"id":"u1","title":"one"},{"id":"u2","title":"two"}]}`)
	if code := run([]string{"intake", "--file", intake2, "--run", "dw2"}); code != 0 {
		t.Fatal("intake dw2")
	}
	if code := run([]string{"release", "--revision", "2", "--run", "dw2"}); code != 0 {
		t.Fatal("release dw2")
	}
	for i, url := range []string{"https://gitlab.example/o/r/-/merge_requests/2", "https://github.com/o/r/pull/2"} {
		for _, args := range [][]string{
			{"build", "--run", "dw2"},
			{"verify", "--cmd", "true", "--run", "dw2"},
			{"review", "--evidence", review, "--run", "dw2"},
		} {
			if code := run(args); code != 0 {
				t.Fatalf("unit %d: %v failed", i, args)
			}
		}
		deliverN := filepath.Join(t.TempDir(), "deliver2.json")
		mustWrite(t, deliverN, `{"delivery_mode":"pr-hold","pr_url":"`+url+`","commit_sha":"cccc3333cccc3333"}`)
		if code := run([]string{"deliver", "--evidence", deliverN, "--run", "dw2"}); code != 0 {
			t.Fatalf("deliver unit %d failed", i)
		}
	}
	// u1's URL never parses: the unit is unobservable data, so every pass
	// classifies it and exits 7; u2 exercises the forge branches while
	// pending checks keep it delivered.
	ghPath := filepath.Join(binDir, "gh")
	mustWrite(t, viewFile, `{"headRefOid":"cccc3333cccc3333","state":"OPEN","mergeable":"MERGEABLE","statusCheckRollup":[{"status":"IN_PROGRESS","conclusion":""}]}`)
	if code := run([]string{"watch", "--once", "--run", "dw2"}); code != 7 {
		t.Fatal("an unparseable delivery URL must classify the pass")
	}
	if err := os.WriteFile(ghPath, []byte("#!/bin/bash\necho 'connection timed out' >&2\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Transient failures no longer read as a clean wait: the pass completes
	// (other units still observed) but exits 7 with the failure classified.
	if code := run([]string{"watch", "--run", "dw2"}); code != 7 {
		t.Fatal("an unobservable unit must exit 7")
	}
	if err := os.WriteFile(ghPath, []byte("#!/bin/bash\necho 'HTTP 401: authentication required' >&2\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if code := run([]string{"watch", "--run", "dw2"}); code != 7 {
		t.Fatal("auth failure must abort with exit 7")
	}
}

// TestDirectWatchProjectionAndAbort covers the dry-run projection loop and
// the forge-abort accumulation path in-process for the package coverage gate.
func TestDirectWatchProjectionAndAbort(t *testing.T) {
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
	t.Setenv("SLOPSHIPPER_DB", filepath.Join(t.TempDir(), "direct-watch2.sqlite"))

	binDir := t.TempDir()
	viewFile := filepath.Join(binDir, "view.json")
	mustWrite(t, viewFile, `{"headRefOid":"aaaa1111aaaa1111","state":"MERGED","mergeable":"UNKNOWN","statusCheckRollup":[]}`)
	threadsFile := filepath.Join(binDir, "threads.json")
	mustWrite(t, threadsFile, emptyThreadsJSON)
	script := fmt.Sprintf("#!/bin/bash\ncase \"$1\" in\n  pr) cat %q ;;\n  api) cat %q ;;\n  *) exit 1 ;;\nesac\n", viewFile, threadsFile)
	ghPath := filepath.Join(binDir, "gh")
	if err := os.WriteFile(ghPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	if code := run([]string{"init", "--run", "dp"}); code != 0 {
		t.Fatal("init")
	}
	intake := filepath.Join(t.TempDir(), "intake.json")
	mustWrite(t, intake, `{"required_reviewers":["autoreview"],"series_bound":1,"units":[{"id":"u1","title":"one"}]}`)
	for _, args := range [][]string{
		{"intake", "--file", intake, "--run", "dp"},
		{"release", "--revision", "2", "--run", "dp"},
		{"build", "--run", "dp"},
		{"verify", "--cmd", "true", "--run", "dp"},
	} {
		if code := run(args); code != 0 {
			t.Fatalf("%v failed", args)
		}
	}
	review := filepath.Join(t.TempDir(), "review.json")
	mustWrite(t, review, `{"reviewer":"autoreview","verdict":"clean","artifact_ref":"test://1"}`)
	if code := run([]string{"review", "--evidence", review, "--run", "dp"}); code != 0 {
		t.Fatal("review")
	}
	deliver := filepath.Join(t.TempDir(), "deliver.json")
	mustWrite(t, deliver, `{"delivery_mode":"pr-hold","pr_url":"https://github.com/o/r/pull/3","commit_sha":"aaaa1111aaaa1111"}`)
	if code := run([]string{"deliver", "--evidence", deliver, "--run", "dp"}); code != 0 {
		t.Fatal("deliver")
	}

	// Unresolved feedback records in-process, then dedups after re-delivery.
	mustWrite(t, viewFile, `{"headRefOid":"aaaa1111aaaa1111","state":"OPEN","mergeable":"MERGEABLE","statusCheckRollup":[{"status":"COMPLETED","conclusion":"SUCCESS"}]}`)
	mustWrite(t, threadsFile, threadsJSON(threadNode("t1", "a.go", 3, "c1", "fix")))
	if code := run([]string{"watch", "--run", "dp"}); code != 0 {
		t.Fatal("feedback watch")
	}
	for _, args := range [][]string{
		{"build", "--run", "dp"},
		{"verify", "--cmd", "true", "--run", "dp"},
		{"review", "--evidence", review, "--run", "dp"},
		{"deliver", "--evidence", deliver, "--run", "dp"},
	} {
		if code := run(args); code != 0 {
			t.Fatalf("%v failed", args)
		}
	}
	if code := run([]string{"--json", "watch", "--run", "dp"}); code != 0 {
		t.Fatal("dedup watch")
	}

	// Dry-run projects the would-be merged state without saving.
	mustWrite(t, viewFile, `{"headRefOid":"aaaa1111aaaa1111","state":"MERGED","mergeable":"UNKNOWN","statusCheckRollup":[]}`)
	mustWrite(t, threadsFile, emptyThreadsJSON)
	if code := run([]string{"--dry-run", "--json", "watch", "--run", "dp"}); code != 0 {
		t.Fatal("dry-run watch")
	}
	// A rate-limited forge aborts with exit 7 but still emits the document.
	if err := os.WriteFile(ghPath, []byte("#!/bin/bash\necho 'HTTP 429: rate limit exceeded' >&2\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if code := run([]string{"watch", "--run", "dp"}); code != 7 {
		t.Fatalf("rate limit must abort with 7, got %d", code)
	}
	if code := run([]string{"--json", "watch", "--run", "dp"}); code != 7 {
		t.Fatal("json abort must also exit 7")
	}
	// Manual observation settles the unit without the forge.
	if code := run([]string{"observe", "--signal", "merged", "--unit", "u1", "--reference", "https://github.com/o/r/pull/3", "--run", "dp"}); code != 0 {
		t.Fatal("manual observe")
	}
	if code := run([]string{"--json", "observe", "--signal", "merged", "--run", "dp"}); code == 0 {
		t.Fatal("no delivered unit remains; observe must fail")
	}
}

func TestWatchReportsAuthFailure(t *testing.T) {
	h := newCLIHarness(t)
	deliverWatchableRun(t, h, "w7")
	binDir := t.TempDir()
	script := "#!/bin/bash\necho 'HTTP 401: authentication required' >&2\nexit 1\n"
	if err := os.WriteFile(filepath.Join(binDir, "gh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	h.env = append(h.env, "PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	out, code := h.run("--json", "watch", "--run", "w7")
	if code != 7 {
		t.Fatalf("auth failure must abort with exit 7: exit %d\n%s", code, out)
	}
	doc := decodeWatchDoc(t, out)
	if doc.ErrorKind != "observation_auth" {
		t.Fatalf("kind must classify the forge failure: %s", out)
	}
	if len(doc.Observations) != 1 || doc.Observations[0].ErrorKind != "auth" {
		t.Fatalf("the failing observation must still be reported: %s", out)
	}
}
