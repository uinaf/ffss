package machine_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/uinaf/slopshipper/internal/machine"
)

func TestHappyPathMultiUnit(t *testing.T) {
	run := machine.NewRun("r1", "repo")
	units := []machine.Unit{
		{ID: "u1", Title: "first"},
		{ID: "u2", Title: "second", Blockers: []string{"u1"}},
	}

	res, err := machine.Apply(run, units, machine.CmdIntake, machine.ApplyInput{
		ExpectedRevision: run.Revision,
		Intake: &machine.IntakePatch{
			SeriesBound:       intPtr(2),
			RequiredReviewers: []machine.ReviewerIdentity{machine.ReviewerBugbot},
			Units:             units,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	run, units = res.Run, res.Units

	res, err = machine.Apply(run, units, machine.CmdRelease, machine.ApplyInput{
		ExpectedRevision: run.Revision,
		IntakeRevision:   run.IntakeRevision,
	})
	if err != nil {
		t.Fatal(err)
	}
	run, units = res.Run, res.Units
	if !run.Released() {
		t.Fatal("expected released")
	}

	res, err = machine.Apply(run, units, machine.CmdBuild, machine.ApplyInput{ExpectedRevision: run.Revision})
	if err != nil {
		t.Fatal(err)
	}
	run, units = res.Run, res.Units
	if run.State != machine.StateBuild || run.CurrentUnitID != "u1" {
		t.Fatalf("build state=%s unit=%s", run.State, run.CurrentUnitID)
	}

	res, err = machine.Apply(run, units, machine.CmdVerify, machine.ApplyInput{
		ExpectedRevision: run.Revision,
		Verify:           &machine.VerifyEvidence{Command: "go test ./...", ExitCode: 0},
	})
	if err != nil {
		t.Fatal(err)
	}
	run, units = res.Run, res.Units
	if run.State != machine.StateReview {
		t.Fatalf("want REVIEW got %s", run.State)
	}

	res, err = machine.Apply(run, units, machine.CmdReview, machine.ApplyInput{
		ExpectedRevision: run.Revision,
		Review: &machine.ReviewEvidence{
			Reviewer:    machine.ReviewerBugbot,
			Verdict:     "clean",
			ArtifactRef: "bugbot://1",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	run, units = res.Run, res.Units

	res, err = machine.Apply(run, units, machine.CmdDeliver, machine.ApplyInput{
		ExpectedRevision: run.Revision,
		Deliver: &machine.DeliverEvidence{
			DeliveryMode: machine.DeliveryPRHold,
			PRURL:        "https://example.com/pr/1",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	run, units = res.Run, res.Units
	if run.State != machine.StateIntake || run.CompletedUnits != 1 {
		t.Fatalf("after deliver state=%s completed=%d", run.State, run.CompletedUnits)
	}

	res, err = machine.Apply(run, units, machine.CmdBuild, machine.ApplyInput{ExpectedRevision: run.Revision})
	if err != nil {
		t.Fatal(err)
	}
	run, units = res.Run, res.Units
	if run.CurrentUnitID != "u2" {
		t.Fatalf("want u2 got %s", run.CurrentUnitID)
	}

	res, err = machine.Apply(run, units, machine.CmdVerify, machine.ApplyInput{
		ExpectedRevision: run.Revision,
		Verify:           &machine.VerifyEvidence{Command: "go test", ExitCode: 0},
	})
	if err != nil {
		t.Fatal(err)
	}
	run, units = res.Run, res.Units
	res, err = machine.Apply(run, units, machine.CmdReview, machine.ApplyInput{
		ExpectedRevision: run.Revision,
		Review: &machine.ReviewEvidence{
			Reviewer: machine.ReviewerBugbot, Verdict: machine.ReviewClean, ArtifactRef: "bugbot://2",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	run, units = res.Run, res.Units
	res, err = machine.Apply(run, units, machine.CmdDeliver, machine.ApplyInput{
		ExpectedRevision: run.Revision,
		Deliver: &machine.DeliverEvidence{
			DeliveryMode: machine.DeliveryPRHold, PRURL: "https://example.com/pr/2",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Run.State != machine.StateRunDone {
		t.Fatalf("want RUN_DONE got %s", res.Run.State)
	}
}

func TestAskThenDecide(t *testing.T) {
	run := machine.NewRun("r1", "repo")
	units := []machine.Unit{{ID: "u1"}}
	res, err := machine.Apply(run, units, machine.CmdIntake, machine.ApplyInput{
		ExpectedRevision: 1,
		Intake:           &machine.IntakePatch{Units: units},
	})
	if err != nil {
		t.Fatal(err)
	}
	run, units = res.Run, res.Units
	res, err = machine.Apply(run, units, machine.CmdAsk, machine.ApplyInput{
		ExpectedRevision: run.Revision,
		Question:         "ship direct-trunk?",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Run.State != machine.StateNeedsDecision || res.Run.DecisionQuestion == "" {
		t.Fatalf("ask: %+v", res.Run)
	}
	res, err = machine.Apply(res.Run, res.Units, machine.CmdDecide, machine.ApplyInput{
		ExpectedRevision: res.Run.Revision,
		Decision:         &machine.Decision{Answer: "no, pr-hold"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Run.State != machine.StateIntake {
		t.Fatalf("want INTAKE got %s", res.Run.State)
	}
}

func TestIntakeClearsSpoofedDone(t *testing.T) {
	run := machine.NewRun("r1", "repo")
	units := []machine.Unit{{ID: "u1", Done: true, Attempt: 9}}
	res, err := machine.Apply(run, nil, machine.CmdIntake, machine.ApplyInput{
		ExpectedRevision: 1,
		Intake:           &machine.IntakePatch{Units: units},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Units[0].Done || res.Units[0].Attempt != 0 {
		t.Fatalf("spoofed fields kept: %+v", res.Units[0])
	}
}

func TestValidateGraphRejectsCycle(t *testing.T) {
	run := machine.NewRun("r1", "repo")
	units := []machine.Unit{
		{ID: "a", Blockers: []string{"b"}},
		{ID: "b", Blockers: []string{"a"}},
	}
	_, err := machine.Apply(run, nil, machine.CmdIntake, machine.ApplyInput{
		ExpectedRevision: 1,
		Intake:           &machine.IntakePatch{Units: units},
	})
	if !errors.Is(err, machine.ErrBadArgs) {
		t.Fatalf("got %v", err)
	}
}

func TestBuildBeforeReleaseFails(t *testing.T) {
	run := machine.NewRun("r1", "repo")
	units := []machine.Unit{{ID: "u1", Title: "x"}}
	res, err := machine.Apply(run, units, machine.CmdIntake, machine.ApplyInput{
		ExpectedRevision: 1,
		Intake:           &machine.IntakePatch{Units: units},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = machine.Apply(res.Run, res.Units, machine.CmdBuild, machine.ApplyInput{ExpectedRevision: res.Run.Revision})
	if !errors.Is(err, machine.ErrIllegalTransition) {
		t.Fatalf("got %v", err)
	}
}

func TestIntakeGraphReplaceResetsUnitCounters(t *testing.T) {
	run := machine.NewRun("r1", "repo")
	units := []machine.Unit{{ID: "u1", Title: "old"}}
	res, err := machine.Apply(run, units, machine.CmdIntake, machine.ApplyInput{
		ExpectedRevision: 1,
		Intake: &machine.IntakePatch{
			SeriesBound: intPtr(2),
			Units:       units,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	run = res.Run
	run.CompletedUnits = 1
	run.CurrentUnitID = "u1"
	res, err = machine.Apply(run, res.Units, machine.CmdIntake, machine.ApplyInput{
		ExpectedRevision: run.Revision,
		Intake: &machine.IntakePatch{
			Units: []machine.Unit{{ID: "u2", Title: "new"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Run.CompletedUnits != 0 || res.Run.CurrentUnitID != "" {
		t.Fatalf("counters not reset: completed=%d current=%q", res.Run.CompletedUnits, res.Run.CurrentUnitID)
	}
	if len(res.Units) != 1 || res.Units[0].ID != "u2" {
		t.Fatalf("units=%+v", res.Units)
	}
}

func TestBuildFromDeliverFails(t *testing.T) {
	run := machine.NewRun("r1", "repo")
	units := []machine.Unit{{ID: "u1", Title: "x"}}
	res, err := machine.Apply(run, units, machine.CmdIntake, machine.ApplyInput{
		ExpectedRevision: 1,
		Intake: &machine.IntakePatch{
			SeriesBound:       intPtr(1),
			RequiredReviewers: []machine.ReviewerIdentity{machine.ReviewerBugbot},
			Units:             units,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	run, units = res.Run, res.Units
	res, err = machine.Apply(run, units, machine.CmdRelease, machine.ApplyInput{
		ExpectedRevision: run.Revision,
		IntakeRevision:   run.IntakeRevision,
	})
	if err != nil {
		t.Fatal(err)
	}
	run, units = res.Run, res.Units
	res, err = machine.Apply(run, units, machine.CmdBuild, machine.ApplyInput{ExpectedRevision: run.Revision})
	if err != nil {
		t.Fatal(err)
	}
	run, units = res.Run, res.Units
	res, err = machine.Apply(run, units, machine.CmdVerify, machine.ApplyInput{
		ExpectedRevision: run.Revision,
		Verify:           &machine.VerifyEvidence{Command: "true", ExitCode: 0},
	})
	if err != nil {
		t.Fatal(err)
	}
	run, units = res.Run, res.Units
	res, err = machine.Apply(run, units, machine.CmdReview, machine.ApplyInput{
		ExpectedRevision: run.Revision,
		Review: &machine.ReviewEvidence{
			Reviewer: machine.ReviewerBugbot, Verdict: "clean", ArtifactRef: "bugbot://1",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	run, units = res.Run, res.Units
	if run.State != machine.StateDeliver {
		t.Fatalf("want DELIVER got %s", run.State)
	}
	_, err = machine.Apply(run, units, machine.CmdBuild, machine.ApplyInput{ExpectedRevision: run.Revision})
	if !errors.Is(err, machine.ErrIllegalTransition) {
		t.Fatalf("got %v", err)
	}
}

func TestReleaseStaleRevisionFails(t *testing.T) {
	run := machine.NewRun("r1", "repo")
	units := []machine.Unit{{ID: "u1"}}
	res, err := machine.Apply(run, units, machine.CmdIntake, machine.ApplyInput{
		ExpectedRevision: 1,
		Intake:           &machine.IntakePatch{Units: units},
	})
	if err != nil {
		t.Fatal(err)
	}
	run = res.Run
	_, err = machine.Apply(run, res.Units, machine.CmdRelease, machine.ApplyInput{
		ExpectedRevision: run.Revision,
		IntakeRevision:   run.IntakeRevision - 1,
	})
	if !errors.Is(err, machine.ErrUnmetGuard) {
		t.Fatalf("got %v", err)
	}
}

func TestIntakeInvalidatesRelease(t *testing.T) {
	run := machine.NewRun("r1", "repo")
	units := []machine.Unit{{ID: "u1"}}
	res, err := machine.Apply(run, units, machine.CmdIntake, machine.ApplyInput{
		ExpectedRevision: 1,
		Intake:           &machine.IntakePatch{Units: units},
	})
	if err != nil {
		t.Fatal(err)
	}
	run, units = res.Run, res.Units
	res, err = machine.Apply(run, units, machine.CmdRelease, machine.ApplyInput{
		ExpectedRevision: run.Revision,
		IntakeRevision:   run.IntakeRevision,
	})
	if err != nil {
		t.Fatal(err)
	}
	run, units = res.Run, res.Units
	res, err = machine.Apply(run, units, machine.CmdIntake, machine.ApplyInput{
		ExpectedRevision: run.Revision,
		Intake:           &machine.IntakePatch{SeriesBound: intPtr(2)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Run.Released() {
		t.Fatal("intake should invalidate release")
	}
}

func TestEmptyReviewEvidenceFails(t *testing.T) {
	run, units := releasedAtReview(t)
	_, err := machine.Apply(run, units, machine.CmdReview, machine.ApplyInput{
		ExpectedRevision: run.Revision,
		Review:           &machine.ReviewEvidence{Reviewer: machine.ReviewerAutoreview},
	})
	if !errors.Is(err, machine.ErrUnmetGuard) {
		t.Fatalf("got %v", err)
	}
}

func TestReviewConsentMismatchFails(t *testing.T) {
	run, units := releasedAtReview(t)
	_, err := machine.Apply(run, units, machine.CmdReview, machine.ApplyInput{
		ExpectedRevision: run.Revision,
		Review: &machine.ReviewEvidence{
			Reviewer: machine.ReviewerBugbot, Verdict: "x", ArtifactRef: "y",
		},
	})
	if !errors.Is(err, machine.ErrUnmetGuard) {
		t.Fatalf("got %v want consent mismatch", err)
	}
}

func TestVerifyFailureBlocks(t *testing.T) {
	run, units := releasedAtBuild(t)
	res, err := machine.Apply(run, units, machine.CmdVerify, machine.ApplyInput{
		ExpectedRevision: run.Revision,
		Verify:           &machine.VerifyEvidence{Command: "go test", ExitCode: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Run.State != machine.StateBlocked {
		t.Fatalf("want BLOCKED got %s", res.Run.State)
	}
}

func TestIllegalTransition(t *testing.T) {
	run := machine.NewRun("r1", "repo")
	_, err := machine.Apply(run, nil, machine.CmdReview, machine.ApplyInput{ExpectedRevision: 1})
	if !errors.Is(err, machine.ErrIllegalTransition) {
		t.Fatalf("got %v", err)
	}
}

func TestReviewBothRequiresDistinctCleanReviews(t *testing.T) {
	run, units := reviewRun(machine.ReviewerAutoreview, machine.ReviewerBugbot)
	first, err := machine.Apply(run, units, machine.CmdReview, machine.ApplyInput{
		ExpectedRevision: run.Revision,
		Review: &machine.ReviewEvidence{
			Reviewer: machine.ReviewerAutoreview, Verdict: machine.ReviewClean, ArtifactRef: "autoreview://1",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if first.Run.State != machine.StateReview || len(first.Run.CompletedReviewers) != 1 {
		t.Fatalf("first review: %+v", first.Run)
	}
	if _, err := machine.Apply(first.Run, first.Units, machine.CmdReview, machine.ApplyInput{
		ExpectedRevision: first.Run.Revision,
		Review: &machine.ReviewEvidence{
			Reviewer: machine.ReviewerAutoreview, Verdict: machine.ReviewClean, ArtifactRef: "autoreview://2",
		},
	}); !errors.Is(err, machine.ErrUnmetGuard) {
		t.Fatalf("duplicate reviewer: %v", err)
	}
	second, err := machine.Apply(first.Run, first.Units, machine.CmdReview, machine.ApplyInput{
		ExpectedRevision: first.Run.Revision,
		Review: &machine.ReviewEvidence{
			Reviewer: machine.ReviewerBugbot, Verdict: machine.ReviewClean, ArtifactRef: "bugbot://1",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if second.Run.State != machine.StateDeliver || len(second.Run.CompletedReviewers) != 2 {
		t.Fatalf("second review: %+v", second.Run)
	}
}

func TestReviewOutcomesRouteWithoutUnlockingDelivery(t *testing.T) {
	for _, tt := range []struct {
		verdict machine.ReviewVerdict
		state   machine.State
	}{
		{machine.ReviewFindings, machine.StateRework},
		{machine.ReviewAmbiguous, machine.StateNeedsDecision},
	} {
		t.Run(string(tt.verdict), func(t *testing.T) {
			run, units := reviewRun(machine.ReviewerAutoreview)
			res, err := machine.Apply(run, units, machine.CmdReview, machine.ApplyInput{
				ExpectedRevision: run.Revision,
				Review: &machine.ReviewEvidence{
					Reviewer: machine.ReviewerAutoreview, Verdict: tt.verdict, ArtifactRef: "autoreview://1",
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			if res.Run.State != tt.state {
				t.Fatalf("got %s want %s", res.Run.State, tt.state)
			}
		})
	}
}

func TestNeedsDecisionOnlyAllowsDecide(t *testing.T) {
	run := machine.NewRun("r", "repo")
	run.State = machine.StateNeedsDecision
	run.ReturnState = machine.StateBuild
	units := []machine.Unit{{ID: "u1"}}
	allowed := machine.AllowedCommands(run, units)
	if len(allowed) != 1 || allowed[0] != machine.CmdDecide {
		t.Fatalf("allowed: %v", allowed)
	}
	for _, cmd := range []machine.Command{machine.CmdIntake, machine.CmdBuild} {
		if _, err := machine.Apply(run, units, cmd, machine.ApplyInput{ExpectedRevision: run.Revision}); !errors.Is(err, machine.ErrIllegalTransition) {
			t.Fatalf("%s: %v", cmd, err)
		}
	}
}

func TestBlockedRetryRequiresReason(t *testing.T) {
	run := machine.NewRun("r", "repo")
	run.State = machine.StateBlocked
	run.CurrentUnitID = "u1"
	run.BlockerReason = "verify failed"
	units := []machine.Unit{{ID: "u1", Attempt: 1}}
	if _, err := machine.Apply(run, units, machine.CmdRetry, machine.ApplyInput{ExpectedRevision: run.Revision}); !errors.Is(err, machine.ErrBadArgs) {
		t.Fatalf("missing reason: %v", err)
	}
	res, err := machine.Apply(run, units, machine.CmdRetry, machine.ApplyInput{ExpectedRevision: run.Revision, RetryReason: "fixed dependency"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Run.State != machine.StateBuild || res.Run.BlockerReason != "" {
		t.Fatalf("retry: %+v", res.Run)
	}
	if _, ok := res.Evidence.(machine.RetryEvidence); !ok {
		t.Fatalf("retry evidence: %T", res.Evidence)
	}
}

func TestBlockedRetryRejectsInvalidCurrentUnit(t *testing.T) {
	for _, units := range [][]machine.Unit{
		{{ID: "other"}},
		{{ID: "u1", Done: true}},
	} {
		run := machine.NewRun("r", "repo")
		run.State = machine.StateBlocked
		run.CurrentUnitID = "u1"
		if _, err := machine.Apply(run, units, machine.CmdRetry, machine.ApplyInput{
			ExpectedRevision: run.Revision, RetryReason: "dependency restored",
		}); !errors.Is(err, machine.ErrCorruptState) {
			t.Fatalf("units=%+v err=%v", units, err)
		}
	}
}

func TestRetryClearsReviewProgress(t *testing.T) {
	run, units := reviewRun(machine.ReviewerAutoreview, machine.ReviewerBugbot)
	res, err := machine.Apply(run, units, machine.CmdReview, machine.ApplyInput{
		ExpectedRevision: run.Revision,
		Review: &machine.ReviewEvidence{
			Reviewer: machine.ReviewerAutoreview, Verdict: machine.ReviewClean, ArtifactRef: "autoreview://1",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	res, err = machine.Apply(res.Run, res.Units, machine.CmdBlock, machine.ApplyInput{
		ExpectedRevision: res.Run.Revision, BlockReason: "dependency unavailable",
	})
	if err != nil {
		t.Fatal(err)
	}
	res, err = machine.Apply(res.Run, res.Units, machine.CmdRetry, machine.ApplyInput{
		ExpectedRevision: res.Run.Revision, RetryReason: "dependency restored",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Run.State != machine.StateBuild || len(res.Run.CompletedReviewers) != 0 {
		t.Fatalf("retry retained review progress: %+v", res.Run)
	}
}

func TestEmptyRunCannotRelease(t *testing.T) {
	run := machine.NewRun("r", "repo")
	if slices.Contains(machine.AllowedCommands(run, nil), machine.CmdRelease) {
		t.Fatal("empty run advertised release")
	}
	if _, err := machine.Apply(run, nil, machine.CmdRelease, machine.ApplyInput{
		ExpectedRevision: run.Revision, IntakeRevision: run.IntakeRevision,
	}); !errors.Is(err, machine.ErrIllegalTransition) {
		t.Fatalf("release: %v", err)
	}
}

func reviewRun(required ...machine.ReviewerIdentity) (machine.Run, []machine.Unit) {
	run := machine.NewRun("r", "repo")
	run.State = machine.StateReview
	run.RequiredReviewers = required
	run.CurrentUnitID = "u1"
	return run, []machine.Unit{{ID: "u1", Attempt: 1}}
}

func releasedAtBuild(t *testing.T) (machine.Run, []machine.Unit) {
	t.Helper()
	run := machine.NewRun("r1", "repo")
	units := []machine.Unit{{ID: "u1"}}
	res, err := machine.Apply(run, units, machine.CmdIntake, machine.ApplyInput{
		ExpectedRevision: 1,
		Intake: &machine.IntakePatch{
			RequiredReviewers: []machine.ReviewerIdentity{machine.ReviewerAutoreview},
			Units:             units,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	run, units = res.Run, res.Units
	res, err = machine.Apply(run, units, machine.CmdRelease, machine.ApplyInput{
		ExpectedRevision: run.Revision,
		IntakeRevision:   run.IntakeRevision,
	})
	if err != nil {
		t.Fatal(err)
	}
	run, units = res.Run, res.Units
	res, err = machine.Apply(run, units, machine.CmdBuild, machine.ApplyInput{ExpectedRevision: run.Revision})
	if err != nil {
		t.Fatal(err)
	}
	return res.Run, res.Units
}

func releasedAtReview(t *testing.T) (machine.Run, []machine.Unit) {
	t.Helper()
	run, units := releasedAtBuild(t)
	res, err := machine.Apply(run, units, machine.CmdVerify, machine.ApplyInput{
		ExpectedRevision: run.Revision,
		Verify:           &machine.VerifyEvidence{Command: "t", ExitCode: 0},
	})
	if err != nil {
		t.Fatal(err)
	}
	return res.Run, res.Units
}

func intPtr(v int) *int { return &v }
