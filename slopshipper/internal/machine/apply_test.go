package machine_test

import (
	"errors"
	"testing"

	"github.com/uinaf/slopomatic/internal/machine"
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
			SeriesBound:   intPtr(2),
			ReviewConsent: consentPtr(machine.ReviewBugbot),
			Units:         units,
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
			Reviewer: machine.ReviewerBugbot, Verdict: "ok", ArtifactRef: "bugbot://2",
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
	if !errors.Is(err, machine.ErrUnmetGuard) {
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
			SeriesBound:   intPtr(1),
			ReviewConsent: consentPtr(machine.ReviewBugbot),
			Units:         units,
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

func releasedAtBuild(t *testing.T) (machine.Run, []machine.Unit) {
	t.Helper()
	run := machine.NewRun("r1", "repo")
	units := []machine.Unit{{ID: "u1"}}
	res, err := machine.Apply(run, units, machine.CmdIntake, machine.ApplyInput{
		ExpectedRevision: 1,
		Intake: &machine.IntakePatch{
			ReviewConsent: consentPtr(machine.ReviewAutoreview),
			Units:         units,
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

func intPtr(v int) *int                                         { return &v }
func consentPtr(v machine.ReviewConsent) *machine.ReviewConsent { return &v }
