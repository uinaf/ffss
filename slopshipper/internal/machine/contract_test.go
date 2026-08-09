package machine_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/uinaf/slopomatic/internal/machine"
)

func TestResourceIDHardening(t *testing.T) {
	for _, valid := range []string{"run-1", "U_2", "unit.v3"} {
		if err := machine.ValidateResourceID("test id", valid); err != nil {
			t.Errorf("valid %q: %v", valid, err)
		}
	}
	for _, invalid := range []string{"", "-flag", "../unit", `unit\\child`, "unit?query", "unit#fragment", "unit%2e", "unit\nnext", strings.Repeat("x", 65)} {
		if err := machine.ValidateResourceID("test id", invalid); !errors.Is(err, machine.ErrBadArgs) {
			t.Errorf("invalid %q: %v", invalid, err)
		}
	}
}

func TestIntakeRejectsInvalidContracts(t *testing.T) {
	badDelivery := machine.DeliveryMode("other")
	badConsent := machine.ReviewConsent("other")
	zero := 0
	tests := []struct {
		name  string
		patch *machine.IntakePatch
	}{
		{"missing patch", nil},
		{"bad delivery", &machine.IntakePatch{DeliveryMode: &badDelivery}},
		{"bad consent", &machine.IntakePatch{ReviewConsent: &badConsent}},
		{"zero series", &machine.IntakePatch{SeriesBound: &zero}},
		{"empty id", &machine.IntakePatch{Units: []machine.Unit{{Title: "missing"}}}},
		{"duplicate id", &machine.IntakePatch{Units: []machine.Unit{{ID: "u1"}, {ID: "u1"}}}},
		{"unknown blocker", &machine.IntakePatch{Units: []machine.Unit{{ID: "u1", Blockers: []string{"u2"}}}}},
		{"self blocker", &machine.IntakePatch{Units: []machine.Unit{{ID: "u1", Blockers: []string{"u1"}}}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			run := machine.NewRun("r", "repo")
			_, err := machine.Apply(run, nil, machine.CmdIntake, machine.ApplyInput{
				ExpectedRevision: run.Revision, Intake: tt.patch,
			})
			if err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestReviewConsentMatrix(t *testing.T) {
	tests := []struct {
		consent  machine.ReviewConsent
		reviewer machine.ReviewerIdentity
		state    machine.State
	}{
		{machine.ReviewAutoreview, machine.ReviewerAutoreview, machine.StateDeliver},
		{machine.ReviewBugbot, machine.ReviewerBugbot, machine.StateDeliver},
		{machine.ReviewHuman, machine.ReviewerHuman, machine.StateDeliver},
		{machine.ReviewBoth, machine.ReviewerAutoreview, machine.StateReview},
		{machine.ReviewBoth, machine.ReviewerBugbot, machine.StateReview},
	}
	for _, tt := range tests {
		t.Run(string(tt.consent)+"/"+string(tt.reviewer), func(t *testing.T) {
			run, units := reviewRun(tt.consent)
			res, err := machine.Apply(run, units, machine.CmdReview, machine.ApplyInput{
				ExpectedRevision: run.Revision,
				Review:           &machine.ReviewEvidence{Reviewer: tt.reviewer, Verdict: machine.ReviewClean, ArtifactRef: "test://1"},
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

func TestReviewEvidenceValidation(t *testing.T) {
	tests := []machine.ReviewEvidence{
		{},
		{Reviewer: machine.ReviewerAutoreview, Verdict: "other", ArtifactRef: "test://1"},
		{Reviewer: "other", Verdict: machine.ReviewClean, ArtifactRef: "test://1"},
		{Reviewer: machine.ReviewerAutoreview, Verdict: machine.ReviewClean},
		{Reviewer: machine.ReviewerBugbot, Verdict: machine.ReviewClean, ArtifactRef: "test://1"},
	}
	for i := range tests {
		run, units := reviewRun(machine.ReviewAutoreview)
		if _, err := machine.Apply(run, units, machine.CmdReview, machine.ApplyInput{
			ExpectedRevision: run.Revision, Review: &tests[i],
		}); !errors.Is(err, machine.ErrUnmetGuard) {
			t.Fatalf("case %d: %v", i, err)
		}
	}
}

func TestVerifyRejectsMissingCommandBeforeRecordingFailure(t *testing.T) {
	run := machine.NewRun("r", "repo")
	run.State = machine.StateBuild
	run.CurrentUnitID = "u1"
	units := []machine.Unit{{ID: "u1", Attempt: 1}}
	for _, evidence := range []*machine.VerifyEvidence{
		nil,
		{Command: "   ", ExitCode: 1},
	} {
		if _, err := machine.Apply(run, units, machine.CmdVerify, machine.ApplyInput{
			ExpectedRevision: run.Revision, Verify: evidence,
		}); !errors.Is(err, machine.ErrUnmetGuard) {
			t.Fatalf("evidence=%+v err=%v", evidence, err)
		}
	}
}

func TestDeliveryModesAndValidation(t *testing.T) {
	tests := []struct {
		name string
		mode machine.DeliveryMode
		ev   machine.DeliverEvidence
		ok   bool
	}{
		{"pr hold", machine.DeliveryPRHold, machine.DeliverEvidence{PRURL: "https://example.com/1"}, true},
		{"merge ready", machine.DeliveryPRMergeWhenReady, machine.DeliverEvidence{PRURL: "https://example.com/2"}, true},
		{"direct", machine.DeliveryDirectTrunk, machine.DeliverEvidence{CommitSHA: "abc123"}, true},
		{"missing pr", machine.DeliveryPRHold, machine.DeliverEvidence{}, false},
		{"missing commit", machine.DeliveryDirectTrunk, machine.DeliverEvidence{}, false},
		{"mismatch", machine.DeliveryPRHold, machine.DeliverEvidence{DeliveryMode: machine.DeliveryDirectTrunk, PRURL: "x"}, false},
		{"unknown", machine.DeliveryMode("other"), machine.DeliverEvidence{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			run := machine.NewRun("r", "repo")
			run.State = machine.StateDeliver
			run.DeliveryMode = tt.mode
			run.CurrentUnitID = "u1"
			units := []machine.Unit{{ID: "u1"}}
			_, err := machine.Apply(run, units, machine.CmdDeliver, machine.ApplyInput{
				ExpectedRevision: run.Revision, Deliver: &tt.ev,
			})
			if tt.ok && err != nil {
				t.Fatal(err)
			}
			if !tt.ok && !errors.Is(err, machine.ErrUnmetGuard) {
				t.Fatalf("got %v", err)
			}
		})
	}
}

func TestReworkBlockAndCorruptStateGuards(t *testing.T) {
	run, units := reviewRun(machine.ReviewAutoreview)
	released := run.IntakeRevision
	run.ReleasedRevision = &released
	rework, err := machine.Apply(run, units, machine.CmdRework, machine.ApplyInput{ExpectedRevision: run.Revision})
	if err != nil {
		t.Fatal(err)
	}
	build, err := machine.Apply(rework.Run, rework.Units, machine.CmdBuild, machine.ApplyInput{ExpectedRevision: rework.Run.Revision})
	if err != nil {
		t.Fatal(err)
	}
	if build.Units[0].Attempt != 2 {
		t.Fatalf("attempt=%d", build.Units[0].Attempt)
	}
	blocked, err := machine.Apply(build.Run, build.Units, machine.CmdBlock, machine.ApplyInput{
		ExpectedRevision: build.Run.Revision, BlockReason: "external dependency",
	})
	if err != nil || blocked.Run.State != machine.StateBlocked {
		t.Fatalf("block: %+v %v", blocked.Run, err)
	}

	corrupt := rework.Run
	corrupt.CurrentUnitID = "missing"
	if _, err := machine.Apply(corrupt, rework.Units, machine.CmdBuild, machine.ApplyInput{ExpectedRevision: corrupt.Revision}); !errors.Is(err, machine.ErrCorruptState) {
		t.Fatalf("missing rework unit: %v", err)
	}
	deliver := run
	deliver.State = machine.StateDeliver
	deliver.CurrentUnitID = "missing"
	if _, err := machine.Apply(deliver, units, machine.CmdDeliver, machine.ApplyInput{
		ExpectedRevision: deliver.Revision,
		Deliver:          &machine.DeliverEvidence{PRURL: "https://example.com/1"},
	}); !errors.Is(err, machine.ErrCorruptState) {
		t.Fatalf("missing delivery unit: %v", err)
	}
}
