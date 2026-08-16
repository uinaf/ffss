package main

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/uinaf/ffsstack/cli/slopmachine/internal/selfupdate"
)

func TestSelfupdateRefusesDevBuild(t *testing.T) {
	h := newCLIHarness(t)
	out, code := h.run("selfupdate", "--json")
	if code != 2 || !strings.Contains(out, "not a release build") {
		t.Fatalf("dev build must be refused: code=%d %s", code, out)
	}
	out, code = h.run("selfupdate", "--check")
	if code != 2 || !strings.Contains(out, "not a release build") {
		t.Fatalf("check on a dev build must be refused: code=%d %s", code, out)
	}
}

func TestSelfupdateFlagValidation(t *testing.T) {
	h := newCLIHarness(t)
	if out, code := h.run("selfupdate", "--check", "value"); code != 2 {
		t.Fatalf("positional argument must be rejected: code=%d %s", code, out)
	}
	if out, code := h.run("selfupdate", "--release"); code != 2 || !strings.Contains(out, "requires a value") {
		t.Fatalf("--release without a value must be rejected: code=%d %s", code, out)
	}
}

func TestSelfupdateErrorExitMapping(t *testing.T) {
	if got := selfupdateErrorExit(selfupdate.ErrNotRelease); got != 2 {
		t.Fatalf("non-release refusal must exit 2, got %d", got)
	}
	if got := selfupdateErrorExit(fmt.Errorf("brew: %w", selfupdate.ErrBrewManaged)); got != 2 {
		t.Fatalf("brew refusal must exit 2, got %d", got)
	}
	if got := selfupdateErrorExit(errors.New("checksum mismatch")); got != 3 {
		t.Fatalf("verification failure must exit 3, got %d", got)
	}
}

func TestWriteSelfupdateResultExitCodes(t *testing.T) {
	upToDate := selfupdate.Result{From: "v1.0.0", To: "v1.0.0", Path: "/x"}
	available := selfupdate.Result{From: "v1.0.0", To: "v1.2.3", Path: "/x"}
	updated := selfupdate.Result{From: "v1.0.0", To: "v1.2.3", Updated: true, Path: "/x"}
	for _, opts := range []runOptions{{}, {json: true}} {
		if got := writeSelfupdateResult(upToDate, false, opts); got != 0 {
			t.Fatalf("up-to-date must exit 0, got %d", got)
		}
		if got := writeSelfupdateResult(upToDate, true, opts); got != 0 {
			t.Fatalf("check up-to-date must exit 0, got %d", got)
		}
		if got := writeSelfupdateResult(available, true, opts); got != 8 {
			t.Fatalf("check with update available must exit 8, got %d", got)
		}
		if got := writeSelfupdateResult(updated, false, opts); got != 0 {
			t.Fatalf("performed update must exit 0, got %d", got)
		}
	}
}

func TestCmdSelfupdateInProcessPaths(t *testing.T) {
	if got := cmdSelfupdate([]string{"--bogus"}, runOptions{json: true}); got != 2 {
		t.Fatalf("unknown flag must exit 2, got %d", got)
	}
	// A dev test binary is not a release build; both modes refuse with 2.
	if got := cmdSelfupdate([]string{"--check"}, runOptions{json: true}); got != 2 {
		t.Fatalf("in-process check on a dev build must exit 2, got %d", got)
	}
	if got := cmdSelfupdate(nil, runOptions{json: true}); got != 2 {
		t.Fatalf("in-process update on a dev build must exit 2, got %d", got)
	}
}

func TestSelfupdateDryRunProjectsLikeCheck(t *testing.T) {
	h := newCLIHarness(t)
	// A dev build refuses either way; the projection must not be rejected
	// as a non-mutating dry run.
	out, code := h.run("--dry-run", "selfupdate", "--json")
	if code != 2 || !strings.Contains(out, "not a release build") {
		t.Fatalf("dry-run selfupdate must project the check, not reject the flag: code=%d %s", code, out)
	}
}
