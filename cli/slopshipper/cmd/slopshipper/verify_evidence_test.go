package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/uinaf/slopshipper/internal/machine"
	"github.com/uinaf/slopshipper/internal/store"
)

// verifyGHStub lets a test rewrite the stub's payloads and failure between
// invocations without touching PATH again (a second PATH entry would be
// shadowed by the first).
type verifyGHStub struct {
	t           *testing.T
	viewFile    string
	reviewsFile string
	failFile    string
}

func (s verifyGHStub) setView(viewJSON string)       { mustWrite(s.t, s.viewFile, viewJSON) }
func (s verifyGHStub) setReviews(reviewsJSON string) { mustWrite(s.t, s.reviewsFile, reviewsJSON) }
func (s verifyGHStub) setFailure(message string)     { mustWrite(s.t, s.failFile, message) }

// installVerifyGH stubs gh with distinct payloads for the observation view,
// the reviews view, and review threads, so deliver verification and review
// corroboration exercise the real adapter offline. A non-empty failure makes
// every invocation exit 1 with that stderr.
func installVerifyGH(t *testing.T, h *cliHarness, viewJSON, reviewsJSON, failure string) verifyGHStub {
	t.Helper()
	binDir := t.TempDir()
	stub := verifyGHStub{
		t:           t,
		viewFile:    filepath.Join(binDir, "view.json"),
		reviewsFile: filepath.Join(binDir, "reviews.json"),
		failFile:    filepath.Join(binDir, "failure.txt"),
	}
	threadsFile := filepath.Join(binDir, "threads.json")
	stub.setView(viewJSON)
	stub.setReviews(reviewsJSON)
	stub.setFailure(failure)
	mustWrite(t, threadsFile, emptyThreadsJSON)
	script := fmt.Sprintf(`#!/bin/bash
fail=$(cat %q)
if [ -n "$fail" ]; then echo "$fail" >&2; exit 1; fi
for arg in "$@"; do
  if [ "$arg" = "reviews" ]; then cat %q; exit 0; fi
done
case "$1" in
  pr) cat %q ;;
  api) cat %q ;;
  *) echo "unexpected gh invocation: $*" >&2; exit 1 ;;
esac
`, stub.failFile, stub.reviewsFile, stub.viewFile, threadsFile)
	if err := os.WriteFile(filepath.Join(binDir, "gh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	h.env = append(h.env, "PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return stub
}

// forgeBoundRun registers a forge-bound profile and walks a run to DELIVER
// with slopzapper mapped as a forge-resident reviewer.
func forgeBoundRun(t *testing.T, h *cliHarness, runID string) {
	t.Helper()
	h.must("reviewers", "--add", "slopzapper")
	h.must("repo", "register", "--forge", "github",
		"--bind", "review=slopguard,review=slopzapper",
		"--forge-reviewer", "slopzapper=zapbot")
	h.must("init", "--run", runID)
	intake := filepath.Join(t.TempDir(), "intake.json")
	mustWrite(t, intake, `{
		"required_reviewers":["slopguard","slopzapper"],
		"series_bound":1,
		"units":[{"id":"u1","title":"one"}]
	}`)
	h.must("intake", "--file", intake, "--run", runID)
	h.must("release", "--revision", "2", "--run", runID)
	h.must("build", "--run", runID)
	h.must("verify", "--cmd", "true", "--run", runID)
	review := filepath.Join(t.TempDir(), "review.json")
	mustWrite(t, review, `{"reviewer":"slopguard","verdict":"clean","artifact_ref":"test://1"}`)
	h.must("review", "--evidence", review, "--run", runID)
}

func latestDelivery(t *testing.T, h *cliHarness, runID string) machine.DeliverEvidence {
	t.Helper()
	s, err := store.OpenReadOnly(h.db)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	deliveries, err := s.LatestDeliveries(runID)
	if err != nil {
		t.Fatal(err)
	}
	delivery, ok := deliveries["u1"]
	if !ok {
		t.Fatalf("no delivery recorded for u1: %+v", deliveries)
	}
	return delivery.Evidence
}

func corroboratedReview(t *testing.T, h *cliHarness, runID string) {
	t.Helper()
	review := filepath.Join(t.TempDir(), "zapper.json")
	mustWrite(t, review, `{"reviewer":"slopzapper","verdict":"clean","artifact_ref":"https://github.com/o/r/pull/7"}`)
	h.must("review", "--evidence", review, "--run", runID)
}

const openHeadView = `{"headRefOid":"bbbb2222bbbb2222","state":"OPEN","mergeable":"MERGEABLE","statusCheckRollup":[]}`
const zapbotApproved = `{"reviews":[{"author":{"login":"zapbot"},"state":"APPROVED","submittedAt":"2026-08-15T00:00:00Z"}]}`

func TestDeliverVerifiedAgainstForge(t *testing.T) {
	h := newCLIHarness(t)
	forgeBoundRun(t, h, "v1")
	installVerifyGH(t, h, openHeadView, zapbotApproved, "")
	corroboratedReview(t, h, "v1")

	deliver := filepath.Join(t.TempDir(), "deliver.json")
	mustWrite(t, deliver, `{"delivery_mode":"pr-hold","pr_url":"https://github.com/o/r/pull/7","commit_sha":"bbbb2222bbbb2222"}`)
	h.must("deliver", "--evidence", deliver, "--run", "v1")

	ev := latestDelivery(t, h, "v1")
	if ev.Verification != machine.VerificationObserved {
		t.Fatalf("matching head must record observed evidence: %+v", ev)
	}
}

func TestDeliverHeadMismatchFailsClosed(t *testing.T) {
	h := newCLIHarness(t)
	forgeBoundRun(t, h, "v2")
	installVerifyGH(t, h, openHeadView, zapbotApproved, "")
	corroboratedReview(t, h, "v2")

	deliver := filepath.Join(t.TempDir(), "deliver.json")
	mustWrite(t, deliver, `{"delivery_mode":"pr-hold","pr_url":"https://github.com/o/r/pull/7","commit_sha":"aaaa1111aaaa1111"}`)
	out, code := h.run("deliver", "--evidence", deliver, "--json", "--run", "v2")
	if code != 3 || !strings.Contains(out, "head mismatch") {
		t.Fatalf("head mismatch must fail with exit 3: code=%d %s", code, out)
	}
}

func TestDeliverAdoptsObservedHead(t *testing.T) {
	h := newCLIHarness(t)
	forgeBoundRun(t, h, "v3")
	installVerifyGH(t, h, openHeadView, zapbotApproved, "")
	corroboratedReview(t, h, "v3")

	deliver := filepath.Join(t.TempDir(), "deliver.json")
	mustWrite(t, deliver, `{"delivery_mode":"pr-hold","pr_url":"https://github.com/o/r/pull/7"}`)
	h.must("deliver", "--evidence", deliver, "--run", "v3")

	ev := latestDelivery(t, h, "v3")
	if ev.CommitSHA != "bbbb2222bbbb2222" || ev.Verification != machine.VerificationObserved {
		t.Fatalf("verified delivery must adopt the observed head: %+v", ev)
	}
}

func TestDeliverNotFoundAndUnreachable(t *testing.T) {
	h := newCLIHarness(t)
	forgeBoundRun(t, h, "v4")
	stub := installVerifyGH(t, h, openHeadView, zapbotApproved, "HTTP 404: Not Found")
	corroborated := filepath.Join(t.TempDir(), "zapper.json")
	mustWrite(t, corroborated, `{"reviewer":"slopzapper","verdict":"clean","artifact_ref":"https://github.com/o/r/pull/7"}`)
	if out, code := h.run("review", "--evidence", corroborated, "--json", "--run", "v4"); code != 3 {
		t.Fatalf("corroboration of a missing change request must fail: code=%d %s", code, out)
	}
	if out, code := h.run("review", "--evidence", corroborated, "--unverified", "--reason", "forge fixture is offline", "--run", "v4"); code != 0 {
		t.Fatalf("override must record the review: code=%d %s", code, out)
	}

	deliver := filepath.Join(t.TempDir(), "deliver.json")
	mustWrite(t, deliver, `{"delivery_mode":"pr-hold","pr_url":"https://github.com/o/r/pull/7"}`)
	out, code := h.run("deliver", "--evidence", deliver, "--json", "--run", "v4")
	if code != 3 || !strings.Contains(out, "not found") {
		t.Fatalf("missing change request must refute evidence with exit 3: code=%d %s", code, out)
	}

	// Unreachable is unprovable, not refuted: exit 7 with a structured kind.
	stub.setFailure("dial tcp: i/o timeout")
	out, code = h.run("deliver", "--evidence", deliver, "--json", "--run", "v4")
	if code != 7 || !strings.Contains(out, "observation_transient") {
		t.Fatalf("unreachable forge must exit 7 with observation kind: code=%d %s", code, out)
	}
}

func TestDeliverUnverifiedOverrideRecorded(t *testing.T) {
	h := newCLIHarness(t)
	forgeBoundRun(t, h, "v5")
	// No usable gh: an override must not touch the forge.
	installVerifyGH(t, h, openHeadView, zapbotApproved, "must not be called")
	review := filepath.Join(t.TempDir(), "zapper.json")
	mustWrite(t, review, `{"reviewer":"slopzapper","verdict":"clean","artifact_ref":"https://github.com/o/r/pull/7"}`)
	h.must("review", "--evidence", review, "--unverified", "--reason", "forge outage, review confirmed by hand", "--run", "v5")

	deliver := filepath.Join(t.TempDir(), "deliver.json")
	mustWrite(t, deliver, `{"delivery_mode":"pr-hold","pr_url":"https://github.com/o/r/pull/7","commit_sha":"bbbb2222bbbb2222"}`)
	h.must("deliver", "--evidence", deliver, "--unverified", "--reason", "forge outage, PR checked by hand", "--run", "v5")

	ev := latestDelivery(t, h, "v5")
	if ev.Verification != machine.VerificationOverridden || !strings.Contains(ev.UnverifiedReason, "forge outage") {
		t.Fatalf("override must be recorded in evidence: %+v", ev)
	}
}

func TestUnverifiedRejectedWhenNothingWouldVerify(t *testing.T) {
	h := newCLIHarness(t)
	deliverWatchableRunToReview(t, h, "v6")
	review := filepath.Join(t.TempDir(), "review.json")
	mustWrite(t, review, `{"reviewer":"slopguard","verdict":"clean","artifact_ref":"test://1"}`)
	if out, code := h.run("review", "--evidence", review, "--unverified", "--reason", "why", "--json", "--run", "v6"); code != 2 {
		t.Fatalf("override on a recorded-input review must be rejected: code=%d %s", code, out)
	}
	h.must("review", "--evidence", review, "--run", "v6")

	deliver := filepath.Join(t.TempDir(), "deliver.json")
	mustWrite(t, deliver, `{"delivery_mode":"pr-hold","pr_url":"https://github.com/o/r/pull/7"}`)
	if out, code := h.run("deliver", "--evidence", deliver, "--unverified", "--reason", "why", "--json", "--run", "v6"); code != 2 {
		t.Fatalf("override on a recorded-input delivery must be rejected: code=%d %s", code, out)
	}
}

// deliverWatchableRunToReview walks a profile-less run to REVIEW.
func deliverWatchableRunToReview(t *testing.T, h *cliHarness, runID string) {
	t.Helper()
	h.must("init", "--run", runID)
	intake := filepath.Join(t.TempDir(), "intake.json")
	mustWrite(t, intake, `{"required_reviewers":["slopguard"],"series_bound":1,"units":[{"id":"u1","title":"one"}]}`)
	h.must("intake", "--file", intake, "--run", runID)
	h.must("release", "--revision", "2", "--run", runID)
	h.must("build", "--run", runID)
	h.must("verify", "--cmd", "true", "--run", runID)
}

func TestUnverifiedFlagValidation(t *testing.T) {
	h := newCLIHarness(t)
	forgeBoundRun(t, h, "v7")
	deliver := filepath.Join(t.TempDir(), "deliver.json")
	mustWrite(t, deliver, `{"delivery_mode":"pr-hold","pr_url":"https://github.com/o/r/pull/7"}`)
	if out, code := h.run("deliver", "--evidence", deliver, "--unverified", "--run", "v7"); code != 2 || !strings.Contains(out, "--reason") {
		t.Fatalf("--unverified without --reason must fail: code=%d %s", code, out)
	}
	if out, code := h.run("deliver", "--evidence", deliver, "--reason", "why", "--run", "v7"); code != 2 || !strings.Contains(out, "--unverified") {
		t.Fatalf("--reason without --unverified must fail: code=%d %s", code, out)
	}
}

func TestEvidenceRejectsCallerSuppliedVerification(t *testing.T) {
	h := newCLIHarness(t)
	forgeBoundRun(t, h, "v8")
	review := filepath.Join(t.TempDir(), "review.json")
	mustWrite(t, review, `{"reviewer":"slopguard","verdict":"clean","artifact_ref":"test://1","verification":"observed"}`)
	if out, code := h.run("review", "--evidence", review, "--run", "v8"); code != 2 || !strings.Contains(out, "machine stamps it") {
		t.Fatalf("caller-supplied review verification must be rejected: code=%d %s", code, out)
	}
	deliver := filepath.Join(t.TempDir(), "deliver.json")
	mustWrite(t, deliver, `{"delivery_mode":"pr-hold","pr_url":"https://github.com/o/r/pull/7","unverified_reason":"spoof"}`)
	if out, code := h.run("deliver", "--evidence", deliver, "--run", "v8"); code != 2 || !strings.Contains(out, "machine stamps it") {
		t.Fatalf("caller-supplied unverified_reason must be rejected: code=%d %s", code, out)
	}
}

func TestReviewCorroborationOutcomes(t *testing.T) {
	h := newCLIHarness(t)
	forgeBoundRun(t, h, "v9")

	// A pending-only review set does not corroborate.
	stub := installVerifyGH(t, h, openHeadView, `{"reviews":[{"author":{"login":"zapbot"},"state":"PENDING","submittedAt":""}]}`, "")
	review := filepath.Join(t.TempDir(), "zapper.json")
	mustWrite(t, review, `{"reviewer":"slopzapper","verdict":"clean","artifact_ref":"https://github.com/o/r/pull/7"}`)
	if out, code := h.run("review", "--evidence", review, "--json", "--run", "v9"); code != 3 || !strings.Contains(out, "no submitted review") {
		t.Fatalf("pending-only reviews must not corroborate: code=%d %s", code, out)
	}

	// A non-URL artifact_ref cannot be corroborated.
	local := filepath.Join(t.TempDir(), "local.json")
	mustWrite(t, local, `{"reviewer":"slopzapper","verdict":"clean","artifact_ref":"test://1"}`)
	if out, code := h.run("review", "--evidence", local, "--json", "--run", "v9"); code != 3 || !strings.Contains(out, "change request URL") {
		t.Fatalf("non-URL artifact_ref must fail for forge reviewers: code=%d %s", code, out)
	}

	// The REST-style [bot] suffix in the profile matches the GraphQL login.
	h.must("repo", "update", "--forge-reviewer", "slopzapper=Zapbot[bot]")
	stub.setReviews(zapbotApproved)
	h.must("review", "--evidence", review, "--run", "v9")
}

func TestStatusStatesEvidenceVerification(t *testing.T) {
	h := newCLIHarness(t)
	forgeBoundRun(t, h, "v10")
	var doc map[string]any
	if err := json.Unmarshal([]byte(h.must("status", "--json", "--run", "v10")), &doc); err != nil {
		t.Fatal(err)
	}
	if doc["evidence_verification"] != "observed" {
		t.Fatalf("forge-bound repo must state observed evidence: %v", doc["evidence_verification"])
	}

	h2 := newCLIHarness(t)
	deliverWatchableRunToReview(t, h2, "v11")
	if err := json.Unmarshal([]byte(h2.must("status", "--json", "--run", "v11")), &doc); err != nil {
		t.Fatal(err)
	}
	if doc["evidence_verification"] != "recorded" {
		t.Fatalf("profile-less repo must state recorded evidence plainly: %v", doc["evidence_verification"])
	}
}

func TestDeliverDryRunVerifies(t *testing.T) {
	h := newCLIHarness(t)
	forgeBoundRun(t, h, "v12")
	installVerifyGH(t, h, openHeadView, zapbotApproved, "")
	corroboratedReview(t, h, "v12")

	deliver := filepath.Join(t.TempDir(), "deliver.json")
	mustWrite(t, deliver, `{"delivery_mode":"pr-hold","pr_url":"https://github.com/o/r/pull/7","commit_sha":"aaaa1111aaaa1111"}`)
	out, code := h.run("deliver", "--evidence", deliver, "--dry-run", "--json", "--run", "v12")
	if code != 3 || !strings.Contains(out, "head mismatch") {
		t.Fatalf("dry run must project the same verification failure: code=%d %s", code, out)
	}

	good := filepath.Join(t.TempDir(), "good.json")
	mustWrite(t, good, `{"delivery_mode":"pr-hold","pr_url":"https://github.com/o/r/pull/7","commit_sha":"bbbb2222bbbb2222"}`)
	out, code = h.run("deliver", "--evidence", good, "--dry-run", "--json", "--run", "v12")
	if code != 0 || !strings.Contains(out, `"dry_run": true`) {
		t.Fatalf("dry run of a verifiable delivery must project success: code=%d %s", code, out)
	}
}

func TestRawInputOverride(t *testing.T) {
	h := newCLIHarness(t)
	forgeBoundRun(t, h, "v13")
	installVerifyGH(t, h, openHeadView, zapbotApproved, "must not be called")
	if out, code := h.runInput(`{"run":"v13","reviewer":"slopzapper","verdict":"clean","artifact_ref":"https://github.com/o/r/pull/7","unverified":true,"unverified_reason":"offline fixture"}`, "review", "--input", "-"); code != 0 {
		t.Fatalf("raw-input override must pass: code=%d %s", code, out)
	}
	if out, code := h.runInput(`{"run":"v13","delivery_mode":"pr-hold","pr_url":"https://github.com/o/r/pull/7","unverified_reason":"orphan"}`, "deliver", "--input", "-"); code != 2 || !strings.Contains(out, "unverified") {
		t.Fatalf("raw-input reason without unverified must fail: code=%d %s", code, out)
	}
	if out, code := h.runInput(`{"run":"v13","delivery_mode":"pr-hold","pr_url":"https://github.com/o/r/pull/7","unverified":false,"unverified_reason":"orphan"}`, "deliver", "--input", "-"); code != 2 {
		t.Fatalf("explicit false with a reason must fail: code=%d %s", code, out)
	}
	if out, code := h.runInput(`{"run":"v13","delivery_mode":"pr-hold","pr_url":"https://github.com/o/r/pull/7","unverified":true,"unverified_reason":"offline fixture"}`, "deliver", "--input", "-"); code != 0 {
		t.Fatalf("raw-input deliver override must pass: code=%d %s", code, out)
	}
	ev := latestDelivery(t, h, "v13")
	if ev.Verification != machine.VerificationOverridden || ev.UnverifiedReason != "offline fixture" {
		t.Fatalf("raw-input override must be recorded: %+v", ev)
	}
}

func TestRepoForgeReviewerFlag(t *testing.T) {
	h := newCLIHarness(t)
	if out, code := h.run("repo", "register", "--forge", "github", "--forge-reviewer", "ghost=ghostbot", "--json"); code != 2 || !strings.Contains(out, "ghost") {
		t.Fatalf("unregistered identity must be rejected: code=%d %s", code, out)
	}
	if out, code := h.run("repo", "register", "--forge-reviewer", "slopguard=bot"); code != 2 || !strings.Contains(out, "forge kind") {
		t.Fatalf("mapping without forge kind must be rejected: code=%d %s", code, out)
	}
	if out, code := h.run("repo", "register", "--forge", "github", "--forge-reviewer", "slopguard=a,slopguard=b"); code != 2 || !strings.Contains(out, "twice") {
		t.Fatalf("duplicate mapping must be rejected: code=%d %s", code, out)
	}

	out := h.must("repo", "register", "--forge", "github", "--forge-reviewer", "slopguard=autobot", "--json")
	var doc repoProfileDocument
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatal(err)
	}
	if doc.ForgeReviewers["slopguard"] != "autobot" {
		t.Fatalf("mapping must round-trip: %s", out)
	}
	if text := h.must("repo", "show"); !strings.Contains(text, "forge-reviewers=slopguard=autobot") {
		t.Fatalf("text output must show the mapping: %s", text)
	}
	if text := h.must("repo", "update", "--forge-reviewer", ""); strings.Contains(text, "forge-reviewers=") {
		t.Fatalf("empty value must clear the mapping: %s", text)
	}
}

func TestDirectEvidenceVerification(t *testing.T) {
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
	t.Setenv("SLOPSHIPPER_DB", filepath.Join(t.TempDir(), "direct-verify.sqlite"))

	binDir := t.TempDir()
	viewFile := filepath.Join(binDir, "view.json")
	reviewsFile := filepath.Join(binDir, "reviews.json")
	threadsFile := filepath.Join(binDir, "threads.json")
	mustWrite(t, viewFile, openHeadView)
	mustWrite(t, reviewsFile, zapbotApproved)
	mustWrite(t, threadsFile, emptyThreadsJSON)
	script := fmt.Sprintf(`#!/bin/bash
for arg in "$@"; do
  if [ "$arg" = "reviews" ]; then cat %q; exit 0; fi
done
case "$1" in
  pr) cat %q ;;
  api) cat %q ;;
  *) exit 1 ;;
esac
`, reviewsFile, viewFile, threadsFile)
	if err := os.WriteFile(filepath.Join(binDir, "gh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	if code := run([]string{"reviewers", "--add", "slopzapper"}); code != 0 {
		t.Fatal("reviewers --add failed")
	}
	if code := run([]string{"repo", "register", "--forge", "github",
		"--bind", "review=slopguard,review=slopzapper",
		"--forge-reviewer", "slopzapper=zapbot"}); code != 0 {
		t.Fatal("repo register failed")
	}
	intake := filepath.Join(t.TempDir(), "intake.json")
	mustWrite(t, intake, `{"required_reviewers":["slopguard","slopzapper"],"series_bound":1,"units":[{"id":"u1","title":"one"}]}`)
	steps := [][]string{
		{"init", "--run", "dv"},
		{"intake", "--file", intake, "--run", "dv"},
		{"release", "--revision", "2", "--run", "dv"},
		{"build", "--run", "dv"},
		{"verify", "--cmd", "true", "--run", "dv"},
	}
	for _, args := range steps {
		if code := run(args); code != 0 {
			t.Fatalf("%v failed", args)
		}
	}
	local := filepath.Join(t.TempDir(), "auto.json")
	mustWrite(t, local, `{"reviewer":"slopguard","verdict":"clean","artifact_ref":"test://1"}`)
	if code := run([]string{"review", "--evidence", local, "--run", "dv"}); code != 0 {
		t.Fatal("local review failed")
	}
	zapper := filepath.Join(t.TempDir(), "zap.json")
	mustWrite(t, zapper, `{"reviewer":"slopzapper","verdict":"clean","artifact_ref":"test://1"}`)
	if code := run([]string{"review", "--evidence", zapper, "--run", "dv"}); code != 3 {
		t.Fatal("forge-corroborated review must require a change-request artifact_ref")
	}
	mustWrite(t, zapper, `{"reviewer":"slopzapper","verdict":"clean","artifact_ref":"https://github.com/o/r/pull/7"}`)
	if code := run([]string{"review", "--evidence", zapper, "--run", "dv"}); code != 0 {
		t.Fatal("corroborated review failed")
	}

	deliver := filepath.Join(t.TempDir(), "deliver.json")
	mustWrite(t, deliver, `{"delivery_mode":"pr-hold","pr_url":"https://github.com/o/r/pull/7","commit_sha":"aaaa1111aaaa1111"}`)
	if code := run([]string{"deliver", "--evidence", deliver, "--run", "dv"}); code != 3 {
		t.Fatal("head mismatch must fail with exit 3")
	}
	if code := run([]string{"deliver", "--evidence", deliver, "--unverified", "--run", "dv"}); code != 2 {
		t.Fatal("--unverified without --reason must fail")
	}
	if code := run([]string{"deliver", "--evidence", deliver, "--unverified", "--reason", "override for fixture", "--dry-run", "--run", "dv"}); code != 0 {
		t.Fatal("dry-run override must project")
	}
	mustWrite(t, deliver, `{"delivery_mode":"pr-hold","pr_url":"https://github.com/o/r/pull/7"}`)
	if code := run([]string{"deliver", "--evidence", deliver, "--run", "dv"}); code != 0 {
		t.Fatal("verified deliver failed")
	}
	if code := run([]string{"status", "--json", "--run", "dv"}); code != 0 {
		t.Fatal("status failed")
	}
}

func TestIncompleteForgePayloadsFailClosed(t *testing.T) {
	h := newCLIHarness(t)
	forgeBoundRun(t, h, "v14")
	stub := installVerifyGH(t, h, openHeadView,
		`{"reviews":[{"author":{"login":"zapbot"},"submittedAt":"2026-08-15T00:00:00Z"}]}`, "")

	// A mapped author whose review record carries no state does not
	// corroborate a submitted review.
	review := filepath.Join(t.TempDir(), "zapper.json")
	mustWrite(t, review, `{"reviewer":"slopzapper","verdict":"clean","artifact_ref":"https://github.com/o/r/pull/7"}`)
	if out, code := h.run("review", "--evidence", review, "--json", "--run", "v14"); code != 3 || !strings.Contains(out, "no submitted review") {
		t.Fatalf("stateless review record must not corroborate: code=%d %s", code, out)
	}
	stub.setReviews(zapbotApproved)
	h.must("review", "--evidence", review, "--run", "v14")

	// A view without a usable head is an incomplete observation, not a pass.
	stub.setView(`{"state":"OPEN","mergeable":"MERGEABLE","statusCheckRollup":[]}`)
	deliver := filepath.Join(t.TempDir(), "deliver.json")
	mustWrite(t, deliver, `{"delivery_mode":"pr-hold","pr_url":"https://github.com/o/r/pull/7"}`)
	out, code := h.run("deliver", "--evidence", deliver, "--json", "--run", "v14")
	if code != 7 || !strings.Contains(out, "no usable head") {
		t.Fatalf("headless observation must exit 7: code=%d %s", code, out)
	}
}

func TestDeliverVerificationIgnoresThreadFaults(t *testing.T) {
	h := newCLIHarness(t)
	forgeBoundRun(t, h, "v15")
	// Review threads are watch's read; a graphql fault must not reject a
	// delivery whose existence and head are provable.
	binDir := t.TempDir()
	viewFile := filepath.Join(binDir, "view.json")
	reviewsFile := filepath.Join(binDir, "reviews.json")
	mustWrite(t, viewFile, openHeadView)
	mustWrite(t, reviewsFile, zapbotApproved)
	script := fmt.Sprintf(`#!/bin/bash
for arg in "$@"; do
  if [ "$arg" = "reviews" ]; then cat %q; exit 0; fi
done
case "$1" in
  pr) cat %q ;;
  api) echo "GraphQL: something exploded" >&2; exit 1 ;;
  *) exit 1 ;;
esac
`, reviewsFile, viewFile)
	if err := os.WriteFile(filepath.Join(binDir, "gh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	h.env = append(h.env, "PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	corroboratedReview(t, h, "v15")

	deliver := filepath.Join(t.TempDir(), "deliver.json")
	mustWrite(t, deliver, `{"delivery_mode":"pr-hold","pr_url":"https://github.com/o/r/pull/7","commit_sha":"bbbb2222bbbb2222"}`)
	h.must("deliver", "--evidence", deliver, "--run", "v15")
	if ev := latestDelivery(t, h, "v15"); ev.Verification != machine.VerificationObserved {
		t.Fatalf("delivery proof must not depend on thread reads: %+v", ev)
	}
}

func TestLocalValidationPrecedesForgeContact(t *testing.T) {
	h := newCLIHarness(t)
	forgeBoundRun(t, h, "v16")
	// Forge unusable: locally invalid evidence must still fail with the
	// deterministic local guard, not exit 7.
	installVerifyGH(t, h, openHeadView, zapbotApproved, "must not be called")
	review := filepath.Join(t.TempDir(), "zapper.json")
	mustWrite(t, review, `{"reviewer":"slopzapper","verdict":"suspicious","artifact_ref":"https://github.com/o/r/pull/7"}`)
	if out, code := h.run("review", "--evidence", review, "--json", "--run", "v16"); code != 3 || !strings.Contains(out, "verdict") {
		t.Fatalf("invalid verdict must fail locally before forge contact: code=%d %s", code, out)
	}
	mustWrite(t, review, `{"reviewer":"slopzapper","verdict":"clean","artifact_ref":"https://github.com/o/r/pull/7"}`)
	h.must("review", "--evidence", review, "--unverified", "--reason", "offline fixture", "--run", "v16")

	deliver := filepath.Join(t.TempDir(), "deliver.json")
	mustWrite(t, deliver, `{"delivery_mode":"pr-hold","pr_url":"https://github.com/o/r/pull/7","commit_sha":"nothex!"}`)
	if out, code := h.run("deliver", "--evidence", deliver, "--json", "--run", "v16"); code != 3 || !strings.Contains(out, "commit_sha") {
		t.Fatalf("invalid commit_sha must fail locally before forge contact: code=%d %s", code, out)
	}
	mustWrite(t, deliver, `{"delivery_mode":"direct-trunk","pr_url":"https://github.com/o/r/pull/7","commit_sha":"bbbb2222bbbb2222"}`)
	if out, code := h.run("deliver", "--evidence", deliver, "--json", "--run", "v16"); code != 3 || !strings.Contains(out, "delivery_mode") {
		t.Fatalf("mode conflict must fail locally before forge contact: code=%d %s", code, out)
	}
}

func TestRawInputOrphanReasonRejected(t *testing.T) {
	h := newCLIHarness(t)
	forgeBoundRun(t, h, "v17")
	if out, code := h.runInput(`{"run":"v17","delivery_mode":"pr-hold","pr_url":"https://github.com/o/r/pull/7","unverified_reason":""}`, "deliver", "--input", "-"); code != 2 || !strings.Contains(out, "unverified_reason requires") {
		t.Fatalf("empty orphan unverified_reason must be rejected by presence: code=%d %s", code, out)
	}
	if out, code := h.runInput(`{"run":"v17","reviewer":"slopzapper","verdict":"clean","artifact_ref":"https://github.com/o/r/pull/7","unverified":true,"unverified_reason":"  "}`, "review", "--input", "-"); code != 2 {
		t.Fatalf("blank reason with unverified must be rejected: code=%d %s", code, out)
	}
}

func TestFreshInstallInitDryRunStatesRecordedEvidence(t *testing.T) {
	h := newCLIHarness(t)
	out := h.must("init", "--dry-run", "--json")
	var doc map[string]any
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatal(err)
	}
	if doc["evidence_verification"] != "recorded" {
		t.Fatalf("fresh-install projection must state recorded evidence: %s", out)
	}
}
