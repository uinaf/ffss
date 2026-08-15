package machine_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/uinaf/slopshipper/internal/machine"
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
	zero := 0
	tests := []struct {
		name  string
		patch *machine.IntakePatch
	}{
		{"missing patch", nil},
		{"bad delivery", &machine.IntakePatch{DeliveryMode: &badDelivery}},
		{"empty reviewers", &machine.IntakePatch{RequiredReviewers: []machine.ReviewerIdentity{}}},
		{"invalid reviewer id", &machine.IntakePatch{RequiredReviewers: []machine.ReviewerIdentity{"nope name"}}},
		{"duplicate reviewer", &machine.IntakePatch{RequiredReviewers: []machine.ReviewerIdentity{machine.ReviewerAutoreview, machine.ReviewerAutoreview}}},
		{"unregistered reviewer", &machine.IntakePatch{RequiredReviewers: []machine.ReviewerIdentity{"slopzapper"}}},
		{"zero series", &machine.IntakePatch{SeriesBound: &zero}},
		{"bad risk tier", &machine.IntakePatch{RiskTier: riskPtr("extreme")}},
		{"negative budget", &machine.IntakePatch{Budget: &machine.Budget{Tokens: -1}}},
		{"bad complexity", &machine.IntakePatch{Units: []machine.Unit{{ID: "u1", Complexity: "impossible"}}}},
		{"empty criterion", &machine.IntakePatch{Units: []machine.Unit{{ID: "u1", AcceptanceCriteria: []string{"  "}}}}},
		{"control criterion", &machine.IntakePatch{Units: []machine.Unit{{ID: "u1", AcceptanceCriteria: []string{"ok\x00bad"}}}}},
		{"multiline criterion", &machine.IntakePatch{Units: []machine.Unit{{ID: "u1", AcceptanceCriteria: []string{"line one\nline two"}}}}},
		{"tab criterion", &machine.IntakePatch{Units: []machine.Unit{{ID: "u1", AcceptanceCriteria: []string{"a\tb"}}}}},
		{"unicode NEL criterion", &machine.IntakePatch{Units: []machine.Unit{{ID: "u1", AcceptanceCriteria: []string{"a\u0085b"}}}}},
		{"line separator criterion", &machine.IntakePatch{Units: []machine.Unit{{ID: "u1", AcceptanceCriteria: []string{"a\u2028b"}}}}},
		{"oversize criterion", &machine.IntakePatch{Units: []machine.Unit{{ID: "u1", AcceptanceCriteria: []string{strings.Repeat("x", 501)}}}}},
		{"too many criteria", &machine.IntakePatch{Units: []machine.Unit{{ID: "u1", AcceptanceCriteria: manyCriteria(33)}}}},
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

func riskPtr(v machine.RiskTier) *machine.RiskTier { return &v }

func manyCriteria(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = "criterion"
	}
	return out
}

func TestIntakeCarriesTaskContractFields(t *testing.T) {
	run := machine.NewRun("r", "repo")
	res, err := machine.Apply(run, nil, machine.CmdIntake, machine.ApplyInput{
		ExpectedRevision: run.Revision,
		Intake: &machine.IntakePatch{
			RiskTier: riskPtr(machine.RiskHigh),
			Budget:   &machine.Budget{Tokens: 500000, Minutes: 90},
			Units: []machine.Unit{{
				ID: "u1", Title: "one", Complexity: machine.ComplexityMedium,
				AcceptanceCriteria: []string{"status exposes tier", "budget survives a round trip"},
			}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Run.RiskTier != machine.RiskHigh || res.Run.Budget.Tokens != 500000 || res.Run.Budget.Minutes != 90 {
		t.Fatalf("run contract fields: %+v", res.Run)
	}
	if res.Units[0].Complexity != machine.ComplexityMedium || len(res.Units[0].AcceptanceCriteria) != 2 {
		t.Fatalf("unit contract fields: %+v", res.Units[0])
	}
	evidence, ok := res.Evidence.(machine.IntakeEvidence)
	if !ok || evidence.RiskTier != machine.RiskHigh || evidence.Budget.Minutes != 90 ||
		len(evidence.Units[0].AcceptanceCriteria) != 2 {
		t.Fatalf("intake evidence snapshot: %+v", res.Evidence)
	}
}

func TestRequiredReviewerMatrix(t *testing.T) {
	tests := []struct {
		name     string
		required []machine.ReviewerIdentity
		reviewer machine.ReviewerIdentity
		state    machine.State
	}{
		{"single autoreview", []machine.ReviewerIdentity{machine.ReviewerAutoreview}, machine.ReviewerAutoreview, machine.StateDeliver},
		{"single bugbot", []machine.ReviewerIdentity{machine.ReviewerBugbot}, machine.ReviewerBugbot, machine.StateDeliver},
		{"single human", []machine.ReviewerIdentity{machine.ReviewerHuman}, machine.ReviewerHuman, machine.StateDeliver},
		{"single custom", []machine.ReviewerIdentity{"slopzapper"}, "slopzapper", machine.StateDeliver},
		{"pair first", []machine.ReviewerIdentity{machine.ReviewerAutoreview, machine.ReviewerBugbot}, machine.ReviewerAutoreview, machine.StateReview},
		{"pair second", []machine.ReviewerIdentity{machine.ReviewerAutoreview, machine.ReviewerBugbot}, machine.ReviewerBugbot, machine.StateReview},
		{"custom pair partial", []machine.ReviewerIdentity{"slopzapper", machine.ReviewerHuman}, "slopzapper", machine.StateReview},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			run, units := reviewRun(tt.required...)
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

func TestIntakeAcceptsRegisteredCustomReviewer(t *testing.T) {
	run := machine.NewRun("r", "repo")
	res, err := machine.Apply(run, nil, machine.CmdIntake, machine.ApplyInput{
		ExpectedRevision:    run.Revision,
		RegisteredReviewers: []machine.ReviewerIdentity{"slopzapper"},
		Intake: &machine.IntakePatch{
			RequiredReviewers: []machine.ReviewerIdentity{"slopzapper", machine.ReviewerBugbot},
			Units:             []machine.Unit{{ID: "u1", Title: "one"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Run.RequiredReviewers) != 2 || res.Run.RequiredReviewers[0] != "slopzapper" {
		t.Fatalf("required reviewers: %+v", res.Run.RequiredReviewers)
	}
}

func TestReleaseFailsClosedWhenRequiredReviewerUnregistered(t *testing.T) {
	run := machine.NewRun("r", "repo")
	res, err := machine.Apply(run, nil, machine.CmdIntake, machine.ApplyInput{
		ExpectedRevision:    run.Revision,
		RegisteredReviewers: []machine.ReviewerIdentity{"slopzapper"},
		Intake: &machine.IntakePatch{
			RequiredReviewers: []machine.ReviewerIdentity{"slopzapper"},
			Units:             []machine.Unit{{ID: "u1", Title: "one"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	// The registry lost slopzapper before the human latch: release must refuse.
	if _, err := machine.Apply(res.Run, res.Units, machine.CmdRelease, machine.ApplyInput{
		ExpectedRevision: res.Run.Revision,
		IntakeRevision:   res.Run.IntakeRevision,
	}); !errors.Is(err, machine.ErrUnmetGuard) {
		t.Fatalf("release with unregistered reviewer: %v", err)
	}
	// With the registry intact, the same release succeeds.
	if _, err := machine.Apply(res.Run, res.Units, machine.CmdRelease, machine.ApplyInput{
		ExpectedRevision:    res.Run.Revision,
		IntakeRevision:      res.Run.IntakeRevision,
		RegisteredReviewers: []machine.ReviewerIdentity{"slopzapper"},
	}); err != nil {
		t.Fatal(err)
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
		run, units := reviewRun(machine.ReviewerAutoreview)
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
	run, units := reviewRun(machine.ReviewerAutoreview)
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
