package status

import (
	"strings"
	"testing"

	"github.com/uinaf/slopomatic/internal/machine"
)

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
			run:  machine.Run{State: machine.StateIntake, IntakeRevision: 3, ReviewConsent: machine.ReviewAutoreview},
			next: "slopomatic intake --file <intake.json>", allowed: []string{"intake", "ask"},
			required: []string{}, frontier: []string{}, completed: []string{},
		},
		{
			name:  "populated intake requires exact release",
			run:   machine.Run{State: machine.StateIntake, IntakeRevision: 3, ReviewConsent: machine.ReviewBugbot},
			units: []machine.Unit{{ID: "u1"}},
			next:  "slopomatic release --revision 3", allowed: []string{"intake", "ask", "release"},
			required: []string{}, frontier: []string{"u1"}, completed: []string{},
		},
		{
			name:  "released intake builds",
			run:   machine.Run{State: machine.StateIntake, IntakeRevision: 3, ReleasedRevision: &released, ReviewConsent: machine.ReviewHuman, SeriesBound: 1},
			units: []machine.Unit{{ID: "u1"}},
			next:  "slopomatic build", allowed: []string{"intake", "ask", "build"},
			required: []string{}, frontier: []string{"u1"}, completed: []string{},
		},
		{
			name: "build requires verification",
			run:  machine.Run{State: machine.StateBuild, ReviewConsent: machine.ReviewAutoreview},
			next: "slopomatic verify --cmd '<verification command>'", allowed: []string{"verify", "ask", "block"},
			required: []string{"verify.command", "verify.exit_code"}, frontier: []string{}, completed: []string{},
		},
		{
			name: "both reviews shows progress",
			run:  machine.Run{State: machine.StateReview, ReviewConsent: machine.ReviewBoth, CompletedReviewers: []machine.ReviewerIdentity{machine.ReviewerAutoreview}},
			next: "slopomatic review --evidence <review.json>", allowed: []string{"review", "rework", "ask", "block"},
			required: []string{"review.reviewer", "review.verdict", "review.artifact_ref"}, frontier: []string{}, completed: []string{"autoreview"},
		},
		{
			name: "blocked requires explicit retry",
			run:  machine.Run{State: machine.StateBlocked, ReviewConsent: machine.ReviewAutoreview},
			next: "slopomatic retry --reason '<reason>'", allowed: []string{"retry"},
			required: []string{"retry.reason"}, frontier: []string{}, completed: []string{},
		},
		{
			name: "decision uses an answer placeholder",
			run:  machine.Run{State: machine.StateNeedsDecision, ReviewConsent: machine.ReviewAutoreview, DecisionQuestion: "continue?"},
			next: "slopomatic decide --answer '<answer>'", allowed: []string{"decide"},
			required: []string{}, frontier: []string{}, completed: []string{},
		},
		{
			name: "delivery names its evidence",
			run:  machine.Run{State: machine.StateDeliver, ReviewConsent: machine.ReviewAutoreview},
			next: "slopomatic deliver --evidence <deliver.json>", allowed: []string{"deliver", "ask"},
			required: []string{"deliver.delivery_mode", "deliver.pr_url|deliver.commit_sha"}, frontier: []string{}, completed: []string{},
		},
		{
			name:    "done has no next action",
			run:     machine.Run{State: machine.StateRunDone, ReviewConsent: machine.ReviewAutoreview},
			allowed: []string{}, required: []string{}, frontier: []string{}, completed: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := From(tt.run, tt.units)
			if doc.SchemaVersion != 2 || doc.NextAction != tt.next {
				t.Fatalf("schema=%d next=%q", doc.SchemaVersion, doc.NextAction)
			}
			assertStrings(t, "allowed", doc.AllowedCommands, tt.allowed)
			assertStrings(t, "required", doc.RequiredEvidence, tt.required)
			assertStrings(t, "frontier", doc.Frontier, tt.frontier)
			assertStrings(t, "completed reviewers", doc.CompletedReviewers, tt.completed)
		})
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

func TestNextActionSelectsAndQuotesRun(t *testing.T) {
	doc := From(machine.Run{
		ID: "run with ' quote", State: machine.StateNeedsDecision,
		ReviewConsent: machine.ReviewAutoreview, DecisionQuestion: "continue?",
	}, nil)
	want := `slopomatic decide --answer '<answer>' --run='run with '"'"' quote'`
	if doc.NextAction != want {
		t.Fatalf("next_action=%q want %q", doc.NextAction, want)
	}
}

func TestNextActionKeepsFlagLikeRunIDAsValue(t *testing.T) {
	doc := From(machine.Run{
		ID: "--json", State: machine.StateNeedsDecision,
		ReviewConsent: machine.ReviewAutoreview, DecisionQuestion: "continue?",
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
