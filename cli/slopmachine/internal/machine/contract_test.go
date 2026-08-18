package machine_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/uinaf/ffss/cli/slopmachine/internal/machine"
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
		{"duplicate reviewer", &machine.IntakePatch{RequiredReviewers: []machine.ReviewerIdentity{machine.ReviewerSlopguard, machine.ReviewerSlopguard}}},
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
		{"single slopguard", []machine.ReviewerIdentity{machine.ReviewerSlopguard}, machine.ReviewerSlopguard, machine.StateDeliver},
		{"single bugbot", []machine.ReviewerIdentity{machine.ReviewerBugbot}, machine.ReviewerBugbot, machine.StateDeliver},
		{"single human", []machine.ReviewerIdentity{machine.ReviewerHuman}, machine.ReviewerHuman, machine.StateDeliver},
		{"single custom", []machine.ReviewerIdentity{"slopzapper"}, "slopzapper", machine.StateDeliver},
		{"pair first", []machine.ReviewerIdentity{machine.ReviewerSlopguard, machine.ReviewerBugbot}, machine.ReviewerSlopguard, machine.StateReview},
		{"pair second", []machine.ReviewerIdentity{machine.ReviewerSlopguard, machine.ReviewerBugbot}, machine.ReviewerBugbot, machine.StateReview},
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

func mustApply(t *testing.T, run machine.Run, units []machine.Unit, cmd machine.Command, in machine.ApplyInput) (machine.Run, []machine.Unit) {
	t.Helper()
	in.ExpectedRevision = run.Revision
	res, err := machine.Apply(run, units, cmd, in)
	if err != nil {
		t.Fatalf("%s: %v", cmd, err)
	}
	return res.Run, res.Units
}

func phaseOf(t *testing.T, units []machine.Unit, id string) machine.Unit {
	t.Helper()
	for _, u := range units {
		if u.ID == id {
			return u
		}
	}
	t.Fatalf("unit %s missing", id)
	return machine.Unit{}
}

// TestUnitLatchBabysitLoop is the #29 acceptance narrative: a later unit
// builds while an earlier one awaits signals, feedback pulls the delivered
// unit back through rework with priority, and merges settle the run.
func TestUnitLatchBabysitLoop(t *testing.T) {
	run := machine.NewRun("r", "repo")
	units := []machine.Unit{
		{ID: "u1", Title: "first"},
		{ID: "u2", Title: "second", Blockers: []string{"u1"}},
	}
	run, units = mustApply(t, run, units, machine.CmdIntake, machine.ApplyInput{
		Intake: &machine.IntakePatch{SeriesBound: intPtr(2), Units: units},
	})
	run, units = mustApply(t, run, units, machine.CmdRelease, machine.ApplyInput{IntakeRevision: run.IntakeRevision})

	walkToDeliver := func() {
		t.Helper()
		run, units = mustApply(t, run, units, machine.CmdVerify, machine.ApplyInput{Verify: &machine.VerifyEvidence{Command: "true", ExitCode: 0}})
		run, units = mustApply(t, run, units, machine.CmdReview, machine.ApplyInput{Review: &machine.ReviewEvidence{Reviewer: machine.ReviewerSlopguard, Verdict: machine.ReviewClean, ArtifactRef: "a://1"}})
		run, units = mustApply(t, run, units, machine.CmdDeliver, machine.ApplyInput{Deliver: &machine.DeliverEvidence{DeliveryMode: machine.DeliveryPRHold, PRURL: "https://example.com/pr"}})
	}

	run, units = mustApply(t, run, units, machine.CmdBuild, machine.ApplyInput{})
	walkToDeliver()
	if phaseOf(t, units, "u1").Phase != machine.PhaseDelivered || run.State != machine.StateIntake {
		t.Fatalf("after first delivery: run=%s units=%+v", run.State, units)
	}

	// The latch: u2 builds while u1 is delivered, not settled.
	run, units = mustApply(t, run, units, machine.CmdBuild, machine.ApplyInput{})
	if run.CurrentUnitID != "u2" || run.State != machine.StateBuild {
		t.Fatalf("u2 claim while u1 delivered: %+v", run)
	}

	// Review feedback lands on u1 mid-build of u2: phase moves, pipeline stays.
	run, units = mustApply(t, run, units, machine.CmdObserve, machine.ApplyInput{
		Observe: &machine.ObserveEvidence{Signal: machine.SignalReviewFeedback, Reference: "thread on cmd/main.go"},
	})
	u1 := phaseOf(t, units, "u1")
	if u1.Phase != machine.PhaseRework || u1.ReworkCause != "review_feedback: thread on cmd/main.go" {
		t.Fatalf("u1 after feedback: %+v", u1)
	}
	if run.State != machine.StateBuild || run.CurrentUnitID != "u2" {
		t.Fatalf("pipeline disturbed by observation: %+v", run)
	}

	// u2 delivers; the rework unit takes priority over fresh frontier.
	walkToDeliver()
	if run.State != machine.StateIntake {
		t.Fatalf("rework should park for build, got %s", run.State)
	}
	previousAttempt := phaseOf(t, units, "u1").Attempt
	run, units = mustApply(t, run, units, machine.CmdBuild, machine.ApplyInput{})
	u1 = phaseOf(t, units, "u1")
	if run.CurrentUnitID != "u1" || u1.Phase != machine.PhaseActive ||
		u1.Attempt != previousAttempt+1 || u1.ReworkCause != "" {
		t.Fatalf("rework reclaim: run=%+v u1=%+v", run, u1)
	}
	walkToDeliver()
	if run.State != machine.StateAwaitingSignals {
		t.Fatalf("want AWAITING_SIGNALS got %s", run.State)
	}

	// Merges settle both units and only then does the run finish.
	run, units = mustApply(t, run, units, machine.CmdObserve, machine.ApplyInput{Observe: &machine.ObserveEvidence{UnitID: "u2", Signal: machine.SignalMerged}})
	if run.State != machine.StateAwaitingSignals {
		t.Fatalf("one merge should not finish the run: %s", run.State)
	}
	run, units = mustApply(t, run, units, machine.CmdObserve, machine.ApplyInput{Observe: &machine.ObserveEvidence{UnitID: "u1", Signal: machine.SignalMerged}})
	if run.State != machine.StateRunDone || run.CompletedUnits != 2 {
		t.Fatalf("final: %+v", run)
	}
	for _, id := range []string{"u1", "u2"} {
		if phaseOf(t, units, id).Phase != machine.PhaseDone {
			t.Fatalf("unit %s not done: %+v", id, units)
		}
	}
}

func TestObserveWorksInParkedStatesWithoutDisturbingThem(t *testing.T) {
	released := int64(1)
	base := machine.Run{
		ID: "r", RepoKey: "repo", IntakeRevision: 1, Revision: 5,
		ReleasedRevision: &released, SeriesBound: 2,
		RequiredReviewers: []machine.ReviewerIdentity{machine.ReviewerSlopguard},
	}
	units := []machine.Unit{
		{ID: "u1", Phase: machine.PhaseDelivered},
		{ID: "u2", Phase: machine.PhaseActive, Attempt: 1},
	}

	blocked := base
	blocked.State = machine.StateBlocked
	blocked.CurrentUnitID = "u2"
	blocked.BlockerReason = "verify failed: make test exit 1"
	res, err := machine.Apply(blocked, units, machine.CmdObserve, machine.ApplyInput{
		ExpectedRevision: blocked.Revision,
		Observe:          &machine.ObserveEvidence{UnitID: "u1", Signal: machine.SignalMerged},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Run.State != machine.StateBlocked || res.Run.BlockerReason == "" {
		t.Fatalf("observe disturbed a blocked run: %+v", res.Run)
	}
	if phaseOf(t, res.Units, "u1").Phase != machine.PhaseDone {
		t.Fatalf("signal not recorded: %+v", res.Units)
	}

	parked := base
	parked.State = machine.StateNeedsDecision
	parked.CurrentUnitID = "u2"
	parked.DecisionQuestion = "keep going?"
	parked.ReturnState = machine.StateBuild
	res, err = machine.Apply(parked, units, machine.CmdObserve, machine.ApplyInput{
		ExpectedRevision: parked.Revision,
		Observe:          &machine.ObserveEvidence{UnitID: "u1", Signal: machine.SignalReviewFeedback},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Run.State != machine.StateNeedsDecision || res.Run.DecisionQuestion != "keep going?" ||
		res.Run.ReturnState != machine.StateBuild {
		t.Fatalf("observe disturbed a parked decision: %+v", res.Run)
	}
	if phaseOf(t, res.Units, "u1").Phase != machine.PhaseRework {
		t.Fatalf("feedback not recorded: %+v", res.Units)
	}
}

func TestDecideReprojectsStaleRestingStates(t *testing.T) {
	released := int64(1)
	base := machine.Run{
		ID: "r", RepoKey: "repo", IntakeRevision: 1, Revision: 7,
		ReleasedRevision: &released, SeriesBound: 1,
		RequiredReviewers: []machine.ReviewerIdentity{machine.ReviewerSlopguard},
		State:             machine.StateNeedsDecision,
		DecisionQuestion:  "hold?",
		ReturnState:       machine.StateAwaitingSignals,
	}
	units := []machine.Unit{{ID: "u1", Phase: machine.PhaseDelivered, Attempt: 1}}

	// While parked, the last delivered unit merges.
	res, err := machine.Apply(base, units, machine.CmdObserve, machine.ApplyInput{
		ExpectedRevision: base.Revision,
		Observe:          &machine.ObserveEvidence{UnitID: "u1", Signal: machine.SignalMerged},
	})
	if err != nil {
		t.Fatal(err)
	}
	res, err = machine.Apply(res.Run, res.Units, machine.CmdDecide, machine.ApplyInput{
		ExpectedRevision: res.Run.Revision,
		Decision:         &machine.Decision{Answer: "proceed"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Run.State != machine.StateRunDone {
		t.Fatalf("stale AWAITING_SIGNALS survived decide: %+v", res.Run)
	}

	// Feedback while parked must resume into a buildable INTAKE, not a
	// signal-less AWAITING_SIGNALS.
	res, err = machine.Apply(base, units, machine.CmdObserve, machine.ApplyInput{
		ExpectedRevision: base.Revision,
		Observe:          &machine.ObserveEvidence{UnitID: "u1", Signal: machine.SignalChecksFailed},
	})
	if err != nil {
		t.Fatal(err)
	}
	res, err = machine.Apply(res.Run, res.Units, machine.CmdDecide, machine.ApplyInput{
		ExpectedRevision: res.Run.Revision,
		Decision:         &machine.Decision{Answer: "rework it"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Run.State != machine.StateIntake {
		t.Fatalf("rework not resumable after decide: %+v", res.Run)
	}
	if !slices.Contains(machine.AllowedCommands(res.Run, res.Units), machine.CmdBuild) {
		t.Fatalf("build not offered for rework unit: %+v", res.Run)
	}

	// An unreleased intake park still resumes literally.
	plain := machine.NewRun("p", "repo")
	plainUnits := []machine.Unit{{ID: "u1", Title: "t"}}
	parked, err := machine.Apply(plain, plainUnits, machine.CmdAsk, machine.ApplyInput{
		ExpectedRevision: plain.Revision, Question: "scope?",
	})
	if err != nil {
		t.Fatal(err)
	}
	resumed, err := machine.Apply(parked.Run, parked.Units, machine.CmdDecide, machine.ApplyInput{
		ExpectedRevision: parked.Run.Revision, Decision: &machine.Decision{Answer: "yes"},
	})
	if err != nil || resumed.Run.State != machine.StateIntake {
		t.Fatalf("unreleased resume: %+v %v", resumed.Run, err)
	}
}

func TestUnreleasedRunsNeverSettleThroughObserve(t *testing.T) {
	released := int64(1)
	run := machine.Run{
		ID: "r", RepoKey: "repo", IntakeRevision: 1, Revision: 4,
		ReleasedRevision: &released, SeriesBound: 1,
		RequiredReviewers: []machine.ReviewerIdentity{machine.ReviewerSlopguard},
		State:             machine.StateAwaitingSignals,
	}
	units := []machine.Unit{{ID: "u1", Phase: machine.PhaseDelivered, Attempt: 1}}

	// Amending the contract clears the release latch.
	one := 1
	res, err := machine.Apply(run, units, machine.CmdIntake, machine.ApplyInput{
		ExpectedRevision: run.Revision,
		Intake:           &machine.IntakePatch{SeriesBound: &one},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Run.Released() {
		t.Fatal("amendment kept the release latch")
	}
	// A merge signal still records, but cannot settle the amended run.
	res, err = machine.Apply(res.Run, res.Units, machine.CmdObserve, machine.ApplyInput{
		ExpectedRevision: res.Run.Revision,
		Observe:          &machine.ObserveEvidence{UnitID: "u1", Signal: machine.SignalMerged},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Run.State != machine.StateIntake {
		t.Fatalf("unreleased run moved to %s", res.Run.State)
	}
	if phaseOf(t, res.Units, "u1").Phase != machine.PhaseDone {
		t.Fatalf("signal not recorded: %+v", res.Units)
	}
	// Releasing the amended contract projects the settled graph.
	res, err = machine.Apply(res.Run, res.Units, machine.CmdRelease, machine.ApplyInput{
		ExpectedRevision: res.Run.Revision,
		IntakeRevision:   res.Run.IntakeRevision,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Run.State != machine.StateRunDone {
		t.Fatalf("release did not project settlement: %+v", res.Run)
	}
}

func TestReworkClaimsRespectBlockers(t *testing.T) {
	released := int64(1)
	run := machine.Run{
		ID: "r", RepoKey: "repo", IntakeRevision: 1, Revision: 9,
		ReleasedRevision: &released, SeriesBound: 2, State: machine.StateIntake,
		RequiredReviewers: []machine.ReviewerIdentity{machine.ReviewerSlopguard},
	}
	// The dependent is listed first and both took feedback.
	units := []machine.Unit{
		{ID: "u2", Blockers: []string{"u1"}, Phase: machine.PhaseRework, ReworkCause: "review_feedback", Attempt: 1},
		{ID: "u1", Phase: machine.PhaseRework, ReworkCause: "checks_failed", Attempt: 1},
	}
	res, err := machine.Apply(run, units, machine.CmdBuild, machine.ApplyInput{ExpectedRevision: run.Revision})
	if err != nil {
		t.Fatal(err)
	}
	if res.Run.CurrentUnitID != "u1" {
		t.Fatalf("dependent rebuilt before its blocker: %+v", res.Run)
	}
}

func TestIntakeRevalidatesRetainedReviewers(t *testing.T) {
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
	// The registry lost slopzapper; an amendment touching only the bound
	// must fail closed even though it retains the reviewer set.
	two := 2
	if _, err := machine.Apply(res.Run, res.Units, machine.CmdIntake, machine.ApplyInput{
		ExpectedRevision: res.Run.Revision,
		Intake:           &machine.IntakePatch{SeriesBound: &two},
	}); !errors.Is(err, machine.ErrBadArgs) {
		t.Fatalf("retained unregistered reviewer accepted: %v", err)
	}
}

func TestObserveGuards(t *testing.T) {
	base := func() (machine.Run, []machine.Unit) {
		run := machine.NewRun("r", "repo")
		released := run.IntakeRevision
		run.ReleasedRevision = &released
		run.State = machine.StateAwaitingSignals
		return run, []machine.Unit{
			{ID: "u1", Phase: machine.PhaseDelivered},
			{ID: "u2", Phase: machine.PhaseDelivered},
			{ID: "u3", Phase: machine.PhasePending},
		}
	}
	run, units := base()
	if _, err := machine.Apply(run, units, machine.CmdObserve, machine.ApplyInput{
		ExpectedRevision: run.Revision,
		Observe:          &machine.ObserveEvidence{Signal: machine.SignalMerged},
	}); !errors.Is(err, machine.ErrBadArgs) {
		t.Fatalf("ambiguous observe: %v", err)
	}
	if _, err := machine.Apply(run, units, machine.CmdObserve, machine.ApplyInput{
		ExpectedRevision: run.Revision,
		Observe:          &machine.ObserveEvidence{UnitID: "u3", Signal: machine.SignalMerged},
	}); !errors.Is(err, machine.ErrUnmetGuard) {
		t.Fatalf("pending unit observed: %v", err)
	}
	if _, err := machine.Apply(run, units, machine.CmdObserve, machine.ApplyInput{
		ExpectedRevision: run.Revision,
		Observe:          &machine.ObserveEvidence{UnitID: "nope", Signal: machine.SignalMerged},
	}); !errors.Is(err, machine.ErrBadArgs) {
		t.Fatalf("unknown unit observed: %v", err)
	}
	if _, err := machine.Apply(run, units, machine.CmdObserve, machine.ApplyInput{
		ExpectedRevision: run.Revision,
		Observe:          &machine.ObserveEvidence{UnitID: "u1", Signal: "vibes"},
	}); !errors.Is(err, machine.ErrBadArgs) {
		t.Fatalf("invalid signal: %v", err)
	}
	// RUN_DONE accepts no observations.
	run.State = machine.StateRunDone
	if _, err := machine.Apply(run, units, machine.CmdObserve, machine.ApplyInput{
		ExpectedRevision: run.Revision,
		Observe:          &machine.ObserveEvidence{UnitID: "u1", Signal: machine.SignalMerged},
	}); !errors.Is(err, machine.ErrIllegalTransition) {
		t.Fatalf("observe after RUN_DONE: %v", err)
	}
}

func TestSeriesBoundBlocksFreshClaimsNotRework(t *testing.T) {
	run := machine.NewRun("r", "repo")
	released := run.IntakeRevision
	run.ReleasedRevision = &released
	run.State = machine.StateIntake
	run.SeriesBound = 1
	run.CompletedUnits = 1
	units := []machine.Unit{
		{ID: "u1", Phase: machine.PhaseRework, ReworkCause: "checks_failed", Attempt: 1},
		{ID: "u2", Phase: machine.PhasePending},
	}
	res, err := machine.Apply(run, units, machine.CmdBuild, machine.ApplyInput{ExpectedRevision: run.Revision})
	if err != nil || res.Run.CurrentUnitID != "u1" {
		t.Fatalf("rework reclaim under bound: %+v %v", res.Run, err)
	}
	// With no rework and the bound spent, build is not even advertised.
	units = []machine.Unit{
		{ID: "u1", Phase: machine.PhaseDone, Attempt: 1},
		{ID: "u2", Phase: machine.PhasePending},
	}
	if slices.Contains(machine.AllowedCommands(run, units), machine.CmdBuild) {
		t.Fatal("bound-exceeded run advertised build")
	}
	if _, err := machine.Apply(run, units, machine.CmdBuild, machine.ApplyInput{ExpectedRevision: run.Revision}); !errors.Is(err, machine.ErrIllegalTransition) {
		t.Fatalf("bound-exceeding claim: %v", err)
	}
}

func TestReviewEvidenceValidation(t *testing.T) {
	tests := []machine.ReviewEvidence{
		{},
		{Reviewer: machine.ReviewerSlopguard, Verdict: "other", ArtifactRef: "test://1"},
		{Reviewer: "other", Verdict: machine.ReviewClean, ArtifactRef: "test://1"},
		{Reviewer: machine.ReviewerSlopguard, Verdict: machine.ReviewClean},
		{Reviewer: machine.ReviewerBugbot, Verdict: machine.ReviewClean, ArtifactRef: "test://1"},
	}
	for i := range tests {
		run, units := reviewRun(machine.ReviewerSlopguard)
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
		{"direct", machine.DeliveryDirectTrunk, machine.DeliverEvidence{CommitSHA: "abc1234"}, true},
		{"missing pr", machine.DeliveryPRHold, machine.DeliverEvidence{}, false},
		{"non-hex sha", machine.DeliveryDirectTrunk, machine.DeliverEvidence{CommitSHA: "zzz9999"}, false},
		{"short sha", machine.DeliveryDirectTrunk, machine.DeliverEvidence{CommitSHA: "abc12"}, false},
		{"pr with bad sha", machine.DeliveryPRHold, machine.DeliverEvidence{PRURL: "https://example.com/3", CommitSHA: "not-a-sha"}, false},
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
	run, units := reviewRun(machine.ReviewerSlopguard)
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
