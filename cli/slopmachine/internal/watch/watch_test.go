package watch_test

import (
	"strings"
	"testing"

	"github.com/uinaf/ffss/cli/slopmachine/internal/forge"
	"github.com/uinaf/ffss/cli/slopmachine/internal/machine"
	"github.com/uinaf/ffss/cli/slopmachine/internal/watch"
)

func target() watch.Target {
	return watch.Target{UnitID: "u1", PRURL: "https://github.com/o/r/pull/1", CommitSHA: "aaaaaaaaaaaaaaaa"}
}

func TestDecideMergedAtDeliveredHeadSettles(t *testing.T) {
	out := watch.Decide(target(), forge.Observation{
		Mergeability:      forge.MergeableMerged,
		HeadSHA:           "aaaaaaaaaaaaaaaa",
		Checks:            forge.ChecksFailing,
		UnresolvedThreads: 3,
	})
	if out.Signal != machine.SignalMerged || out.Reference != "https://github.com/o/r/pull/1" {
		t.Fatalf("merged at the delivered head must settle the unit: %+v", out)
	}
}

func TestDecideMergedWithMovedHeadIsRework(t *testing.T) {
	// A change request merged with a different head than delivered merged
	// something the run never reviewed; that is rework, not settlement.
	out := watch.Decide(target(), forge.Observation{
		Mergeability: forge.MergeableMerged,
		HeadSHA:      "bbbbbbbbbbbbbbbb",
	})
	if out.Signal != machine.SignalHeadMoved {
		t.Fatalf("merged with a moved head must not settle: %+v", out)
	}
}

func TestDecideClosedRecordsNothingActionably(t *testing.T) {
	out := watch.Decide(target(), forge.Observation{Mergeability: forge.MergeableClosed})
	if out.Signal != "" || !strings.Contains(out.Note, "closed without merge") {
		t.Fatalf("closed change requests need a human decision: %+v", out)
	}
}

func TestDecideHeadMovedOutranksChecksAndReviews(t *testing.T) {
	out := watch.Decide(target(), forge.Observation{
		Mergeability:      forge.MergeableClean,
		HeadSHA:           "bbbbbbbbbbbbbbbb",
		Checks:            forge.ChecksFailing,
		UnresolvedThreads: 2,
	})
	if out.Signal != machine.SignalHeadMoved {
		t.Fatalf("a moved head invalidates delivery evidence first: %+v", out)
	}
	if !strings.Contains(out.Reference, "aaaaaaaaaaaa") || !strings.Contains(out.Reference, "bbbbbbbbbbbb") {
		t.Fatalf("reference must name both heads: %q", out.Reference)
	}
}

func TestDecideWithoutRecordedSHASkipsHeadComparison(t *testing.T) {
	moved := target()
	moved.CommitSHA = ""
	out := watch.Decide(moved, forge.Observation{
		Mergeability: forge.MergeableClean,
		HeadSHA:      "bbbbbbbbbbbbbbbb",
		Checks:       forge.ChecksFailing,
	})
	if out.Signal != machine.SignalChecksFailed {
		t.Fatalf("no recorded SHA means no movement judgment: %+v", out)
	}
}

func TestDecideChecksFailed(t *testing.T) {
	out := watch.Decide(target(), forge.Observation{
		Mergeability: forge.MergeableClean,
		HeadSHA:      "aaaaaaaaaaaaaaaa",
		Checks:       forge.ChecksFailing,
	})
	if out.Signal != machine.SignalChecksFailed || !strings.Contains(out.Reference, "checks failing") {
		t.Fatalf("%+v", out)
	}
}

func TestDecideReviewFeedbackNamesFirstThread(t *testing.T) {
	out := watch.Decide(target(), forge.Observation{
		Mergeability:      forge.MergeableClean,
		HeadSHA:           "aaaaaaaaaaaaaaaa",
		Checks:            forge.ChecksPassing,
		UnresolvedThreads: 2,
		Threads: []forge.ReviewThread{
			{Path: "a.go", Line: 12, Resolved: false},
			{Path: "b.go", Line: 3, Resolved: false},
		},
	})
	if out.Signal != machine.SignalReviewFeedback {
		t.Fatalf("%+v", out)
	}
	if !strings.Contains(out.Reference, "2 unresolved") || !strings.Contains(out.Reference, "a.go:12") {
		t.Fatalf("reference must count threads and locate the first: %q", out.Reference)
	}
}

func TestDecideAbbreviatedSHAJudgment(t *testing.T) {
	fullHead := "abc1234def5678900000aaaabbbbccccddddeeee"
	cases := []struct {
		name     string
		recorded string
		head     string
		moved    bool
	}{
		{"abbreviated prefix is the same revision", "abc1234", fullHead, false},
		{"abbreviated mismatch moved", "abc9999", fullHead, true},
		{"full mismatch moved", strings.Repeat("1", 40), fullHead, true},
		{"too short to judge", "abc", "def4567", false},
		{"non-hex evidence never judges", "not-a-sha!", fullHead, false},
		{"recorded longer than head compares as prefix", fullHead, "abc1234", false},
	}
	for _, c := range cases {
		tgt := target()
		tgt.CommitSHA = c.recorded
		out := watch.Decide(tgt, forge.Observation{Mergeability: forge.MergeableClean, HeadSHA: c.head, Checks: forge.ChecksPassing})
		gotMoved := out.Signal == machine.SignalHeadMoved
		if gotMoved != c.moved {
			t.Fatalf("%s: moved=%v want %v (%+v)", c.name, gotMoved, c.moved, out)
		}
	}
}

func TestDecideReviewFeedbackSkipsResolvedAndPathlessThreads(t *testing.T) {
	out := watch.Decide(target(), forge.Observation{
		Mergeability:      forge.MergeableClean,
		HeadSHA:           "aaaaaaaaaaaaaaaa",
		Checks:            forge.ChecksPassing,
		UnresolvedThreads: 1,
		Threads: []forge.ReviewThread{
			{Path: "resolved.go", Line: 1, Resolved: true},
			{Path: "", Line: 0, Resolved: false},
		},
	})
	if out.Signal != machine.SignalReviewFeedback {
		t.Fatalf("%+v", out)
	}
	if strings.Contains(out.Reference, "resolved.go") || strings.Contains(out.Reference, "first at") {
		t.Fatalf("resolved and pathless threads must not be located: %q", out.Reference)
	}
}

func TestDecideSanitizesForgeControlledPaths(t *testing.T) {
	out := watch.Decide(target(), forge.Observation{
		Mergeability:      forge.MergeableClean,
		HeadSHA:           "aaaaaaaaaaaaaaaa",
		Checks:            forge.ChecksPassing,
		UnresolvedThreads: 1,
		Threads:           []forge.ReviewThread{{Path: "a\nfake: line\x1b[31m.go", Line: 3}},
	})
	if strings.ContainsAny(out.Reference, "\n\x1b") {
		t.Fatalf("control characters must not reach durable evidence: %q", out.Reference)
	}
	if !strings.Contains(out.Reference, "a?fake: line?[31m.go:3") {
		t.Fatalf("sanitized path must stay recognizable: %q", out.Reference)
	}
}

func TestThreadTokensIdentifyFeedback(t *testing.T) {
	obs := forge.Observation{Threads: []forge.ReviewThread{
		{ID: "t1", LastCommentID: "c1", Path: "a.go", Line: 3},
		{ID: "t2", LastCommentID: "c2", Path: "b.go", Line: 9, Resolved: true},
		{Path: "legacy.go", Line: 1, Snippet: "no id"},
	}}
	tokens := watch.ThreadTokens(obs)
	if len(tokens) != 2 {
		t.Fatalf("resolved threads carry no token: %v", tokens)
	}
	if tokens[0] != "t1@c1@" {
		t.Fatalf("stable ids win: %v", tokens)
	}
	if !strings.Contains(tokens[1], "legacy.go:1:no id") {
		t.Fatalf("threads without ids fall back to location and text: %v", tokens)
	}
	// Tokens are sanitized like every forge-controlled string.
	hostile := watch.ThreadTokens(forge.Observation{Threads: []forge.ReviewThread{{ID: "x\ny", LastCommentID: "c\x1b"}}})
	if strings.ContainsAny(hostile[0], "\n\x1b") {
		t.Fatalf("tokens must be sanitized: %q", hostile[0])
	}
	// Unicode C1 controls and bidi overrides are neutralized too.
	hostileC1 := watch.ThreadTokens(forge.Observation{Threads: []forge.ReviewThread{{ID: "a\u0085b\u202ec", LastCommentID: "d"}}})
	if strings.ContainsRune(hostileC1[0], '\u0085') || strings.ContainsRune(hostileC1[0], '\u202e') {
		t.Fatalf("C1 and format controls must be neutralized: %q", hostileC1[0])
	}
	long := watch.ThreadTokens(forge.Observation{Threads: []forge.ReviewThread{{ID: strings.Repeat("x", 500), LastCommentID: "c"}}})
	if len(long[0]) > 200 {
		t.Fatalf("tokens must stay bounded: %d bytes", len(long[0]))
	}
}

func TestDecideWaitingRecordsNothing(t *testing.T) {
	out := watch.Decide(target(), forge.Observation{
		Mergeability: forge.MergeableClean,
		HeadSHA:      "aaaaaaaaaaaaaaaa",
		Checks:       forge.ChecksPending,
	})
	if out.Signal != "" || !strings.Contains(out.Note, "waiting") {
		t.Fatalf("unchanged pending state must record nothing: %+v", out)
	}
}
