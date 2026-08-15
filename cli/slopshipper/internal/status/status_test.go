package status

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/uinaf/slopshipper/internal/machine"
)

func TestJSONFieldsAndDryRunMetadata(t *testing.T) {
	if err := ValidateFields(strings.Split(AgentFieldMask, ",")); err != nil {
		t.Fatalf("agent field mask: %v", err)
	}
	doc := From(machine.Run{ID: "dry", State: machine.StateIntake, RequiredReviewers: []machine.ReviewerIdentity{machine.ReviewerAutoreview}}, nil)
	doc.DryRun = true
	doc.ValidatedCommand = "intake"
	b, err := doc.JSONFields([]string{"state", "dry_run", "validated_command", "blocker"})
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(b, &fields); err != nil {
		t.Fatal(err)
	}
	if len(fields) != 3 || fields["state"] != "INTAKE" || fields["dry_run"] != true || fields["validated_command"] != "intake" {
		t.Fatalf("fields=%+v", fields)
	}
	if _, present := fields["blocker"]; present {
		t.Fatalf("omitted blocker present in projection: %+v", fields)
	}
	if _, err := doc.JSONFields([]string{"missing"}); err == nil {
		t.Fatal("unknown field accepted")
	}
	if got := doc.CompactLine(); !strings.HasPrefix(got, "slopshipper dry-run dry ") {
		t.Fatalf("compact dry-run=%q", got)
	}
}

func TestFromContracts(t *testing.T) {
	released := int64(3)
	tests := []struct {
		name      string
		run       machine.Run
		units     []machine.Unit
		next      string
		allowed   []string
		required  []string
		frontier  []string
		completed []string
	}{
		{
			name: "empty intake requires intake",
			run:  machine.Run{State: machine.StateIntake, IntakeRevision: 3, RequiredReviewers: []machine.ReviewerIdentity{machine.ReviewerAutoreview}},
			next: "slopshipper intake --file -", allowed: []string{"intake", "ask"},
			required: []string{}, frontier: []string{}, completed: []string{},
		},
		{
			name:  "populated intake requires exact release",
			run:   machine.Run{State: machine.StateIntake, IntakeRevision: 3, RequiredReviewers: []machine.ReviewerIdentity{machine.ReviewerBugbot}},
			units: []machine.Unit{{ID: "u1"}},
			next:  "slopshipper release --revision 3", allowed: []string{"intake", "ask", "release"},
			required: []string{}, frontier: []string{"u1"}, completed: []string{},
		},
		{
			name:  "released intake builds",
			run:   machine.Run{State: machine.StateIntake, IntakeRevision: 3, ReleasedRevision: &released, RequiredReviewers: []machine.ReviewerIdentity{machine.ReviewerHuman}, SeriesBound: 1},
			units: []machine.Unit{{ID: "u1"}},
			next:  "slopshipper build", allowed: []string{"intake", "ask", "build"},
			required: []string{}, frontier: []string{"u1"}, completed: []string{},
		},
		{
			name: "build requires verification",
			run:  machine.Run{State: machine.StateBuild, RequiredReviewers: []machine.ReviewerIdentity{machine.ReviewerAutoreview}},
			next: "slopshipper verify --cmd '<verification command>'", allowed: []string{"verify", "ask", "block"},
			required: []string{"verify.command", "verify.exit_code"}, frontier: []string{}, completed: []string{},
		},
		{
			name: "both reviews shows progress",
			run:  machine.Run{State: machine.StateReview, RequiredReviewers: []machine.ReviewerIdentity{machine.ReviewerAutoreview, machine.ReviewerBugbot}, CompletedReviewers: []machine.ReviewerIdentity{machine.ReviewerAutoreview}},
			next: "slopshipper review --evidence -", allowed: []string{"review", "rework", "ask", "block"},
			required: []string{"review.reviewer", "review.verdict", "review.artifact_ref"}, frontier: []string{}, completed: []string{"autoreview"},
		},
		{
			name: "blocked requires explicit retry",
			run:  machine.Run{State: machine.StateBlocked, RequiredReviewers: []machine.ReviewerIdentity{machine.ReviewerAutoreview}},
			next: "slopshipper retry --reason '<reason>'", allowed: []string{"retry"},
			required: []string{"retry.reason"}, frontier: []string{}, completed: []string{},
		},
		{
			name: "decision uses an answer placeholder",
			run:  machine.Run{State: machine.StateNeedsDecision, RequiredReviewers: []machine.ReviewerIdentity{machine.ReviewerAutoreview}, DecisionQuestion: "continue?"},
			next: "slopshipper decide --answer '<answer>'", allowed: []string{"decide"},
			required: []string{}, frontier: []string{}, completed: []string{},
		},
		{
			name: "delivery names its evidence",
			run:  machine.Run{State: machine.StateDeliver, RequiredReviewers: []machine.ReviewerIdentity{machine.ReviewerAutoreview}},
			next: "slopshipper deliver --evidence -", allowed: []string{"deliver", "ask"},
			required: []string{"deliver.delivery_mode", "deliver.pr_url|deliver.commit_sha"}, frontier: []string{}, completed: []string{},
		},
		{
			name:    "done has no next action",
			run:     machine.Run{State: machine.StateRunDone, RequiredReviewers: []machine.ReviewerIdentity{machine.ReviewerAutoreview}},
			allowed: []string{}, required: []string{}, frontier: []string{}, completed: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := From(tt.run, tt.units)
			if doc.SchemaVersion != 3 || doc.NextAction != tt.next {
				t.Fatalf("schema=%d next=%q", doc.SchemaVersion, doc.NextAction)
			}
			assertStrings(t, "allowed", doc.AllowedCommands, tt.allowed)
			assertStrings(t, "required", doc.RequiredEvidence, tt.required)
			assertStrings(t, "frontier", doc.Frontier, tt.frontier)
			assertStrings(t, "completed reviewers", doc.CompletedReviewers, tt.completed)
		})
	}
}

func TestEvidenceFollowsSelectedAction(t *testing.T) {
	released := int64(2)
	run := machine.Run{
		ID: "r", State: machine.StateIntake, IntakeRevision: 2, ReleasedRevision: &released,
		SeriesBound: 2, RequiredReviewers: []machine.ReviewerIdentity{machine.ReviewerAutoreview},
	}
	units := []machine.Unit{
		{ID: "d1", Phase: machine.PhaseDelivered},
		{ID: "p1", Phase: machine.PhasePending},
	}
	doc := From(run, units)
	if !strings.Contains(doc.NextAction, "slopshipper build") {
		t.Fatalf("next action: %s", doc.NextAction)
	}
	if len(doc.RequiredEvidence) != 0 {
		t.Fatalf("evidence must follow the selected action, got %v", doc.RequiredEvidence)
	}
	if len(doc.DeliveredUnits) != 1 || doc.DeliveredUnits[0] != "d1" {
		t.Fatalf("delivered units: %v", doc.DeliveredUnits)
	}

	// One delivered unit: observe arrives executable with the unit inlined.
	run.State = machine.StateAwaitingSignals
	single := []machine.Unit{
		{ID: "d1", Phase: machine.PhaseDelivered},
		{ID: "x1", Phase: machine.PhaseDone},
	}
	doc = From(run, single)
	if !strings.Contains(doc.NextAction, "observe --signal '<signal>' --unit='d1'") {
		t.Fatalf("single-delivered observe action: %s", doc.NextAction)
	}
	if strings.Join(doc.RequiredEvidence, ",") != "observe.signal" {
		t.Fatalf("single-delivered evidence: %v", doc.RequiredEvidence)
	}

	// Several delivered units: the unit placeholder is part of the contract.
	multi := []machine.Unit{
		{ID: "d1", Phase: machine.PhaseDelivered},
		{ID: "d2", Phase: machine.PhaseDelivered},
	}
	doc = From(run, multi)
	if !strings.Contains(doc.NextAction, "observe --signal '<signal>' --unit '<unit>'") {
		t.Fatalf("multi-delivered observe action: %s", doc.NextAction)
	}
	if strings.Join(doc.RequiredEvidence, ",") != "observe.signal,observe.unit" {
		t.Fatalf("multi-delivered evidence: %v", doc.RequiredEvidence)
	}
}

func TestParkedRecoveryOutranksObserve(t *testing.T) {
	released := int64(1)
	run := machine.Run{
		ID: "r", State: machine.StateBlocked, IntakeRevision: 1,
		ReleasedRevision: &released, SeriesBound: 2, CurrentUnitID: "u2",
		BlockerReason: "verify failed",
	}
	units := []machine.Unit{
		{ID: "u1", Phase: machine.PhaseDelivered},
		{ID: "u2", Phase: machine.PhaseActive},
	}
	doc := From(run, units)
	if !strings.Contains(doc.NextAction, "slopshipper retry") {
		t.Fatalf("retry must outrank observe: %s", doc.NextAction)
	}
	if !slices.Contains(doc.AllowedCommands, "observe") {
		t.Fatalf("observe must stay allowed: %v", doc.AllowedCommands)
	}
}

func TestJSONUsesStableEmptyArrays(t *testing.T) {
	doc := From(machine.Run{ID: "done", State: machine.StateRunDone}, nil)
	b, err := doc.JSON()
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{`"frontier": []`, `"allowed_commands": []`, `"required_evidence": []`, `"completed_reviewers": []`} {
		if !strings.Contains(string(b), field) {
			t.Fatalf("missing %s in %s", field, b)
		}
	}
	if got := doc.CompactLine(); !strings.Contains(got, "next=(none)") {
		t.Fatalf("compact line: %s", got)
	}
}

func TestBootstrapDirectsFirstRunToInit(t *testing.T) {
	doc := Bootstrap("repo-key")
	if doc.SchemaVersion != 3 || doc.State != "UNINITIALIZED" || doc.RepoKey != "repo-key" {
		t.Fatalf("bootstrap identity: %+v", doc)
	}
	assertStrings(t, "allowed", doc.AllowedCommands, []string{"init"})
	assertStrings(t, "frontier", doc.Frontier, []string{})
	assertStrings(t, "required reviewers", doc.RequiredReviewers, []string{})
	assertStrings(t, "completed reviewers", doc.CompletedReviewers, []string{})
	assertStrings(t, "required evidence", doc.RequiredEvidence, []string{})
	if doc.NextAction != "slopshipper init" || doc.Blocker == "" {
		t.Fatalf("bootstrap action: %+v", doc)
	}
	if got := doc.CompactLine(); got != "slopshipper state=UNINITIALIZED next=slopshipper init" {
		t.Fatalf("bootstrap compact line=%q", got)
	}
	b, err := doc.JSON()
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{`"frontier": []`, `"allowed_commands": [`, `"required_evidence": []`} {
		if !strings.Contains(string(b), field) {
			t.Fatalf("missing %s in %s", field, b)
		}
	}
}

func TestNextActionSelectsAndQuotesRun(t *testing.T) {
	doc := From(machine.Run{
		ID: "run with ' quote", State: machine.StateNeedsDecision,
		RequiredReviewers: []machine.ReviewerIdentity{machine.ReviewerAutoreview}, DecisionQuestion: "continue?",
	}, nil)
	want := `slopshipper decide --answer '<answer>' --run='run with '"'"' quote'`
	if doc.NextAction != want {
		t.Fatalf("next_action=%q want %q", doc.NextAction, want)
	}
}

func TestNextActionKeepsFlagLikeRunIDAsValue(t *testing.T) {
	doc := From(machine.Run{
		ID: "--json", State: machine.StateNeedsDecision,
		RequiredReviewers: []machine.ReviewerIdentity{machine.ReviewerAutoreview}, DecisionQuestion: "continue?",
	}, nil)
	if !strings.HasSuffix(doc.NextAction, `--run='--json'`) {
		t.Fatalf("next_action=%q", doc.NextAction)
	}
}

func assertStrings(t *testing.T, name string, got, want []string) {
	t.Helper()
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("%s: got %#v want %#v", name, got, want)
	}
}
