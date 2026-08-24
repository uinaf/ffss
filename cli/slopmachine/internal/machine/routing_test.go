package machine_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/uinaf/ffss/cli/slopmachine/internal/machine"
)

func routingFixture() *machine.RoutingProfile {
	return &machine.RoutingProfile{
		Version: 1,
		Venues: map[string]machine.VenueDefinition{
			"local": {Kind: "local-worktree"},
		},
		Executors: map[string]machine.ExecutorDefinition{
			"codex-fast": {Harness: "codex", Models: map[string]string{"build": "gpt-5.6-terra"}},
			"codex-deep": {Harness: "codex", Models: map[string]string{"build": "gpt-5.6-sol", "review": "gpt-5.6-terra"}},
		},
		Rules: []machine.RouteRule{
			{RiskTier: machine.RiskMedium, Complexity: machine.ComplexityHigh, Rework: true, Venue: "local", Executor: "codex-deep", Parallelism: 1, ReviewDepth: 2, Budget: machine.Budget{Tokens: 400, Minutes: 40}},
			{RiskTier: machine.RiskMedium, Complexity: machine.ComplexityHigh, Venue: "local", Executor: "codex-fast", Parallelism: 1, ReviewDepth: 1, Budget: machine.Budget{Tokens: 200, Minutes: 20}},
		},
	}
}

func releasedRoutingRun() machine.Run {
	run := machine.NewRun("run", "repo")
	run.RiskTier = machine.RiskMedium
	run.Budget = machine.Budget{Tokens: 500, Minutes: 60}
	revision := run.IntakeRevision
	run.ReleasedRevision = &revision
	return run
}

func TestResolveRouteInitialAndRework(t *testing.T) {
	profile := routingFixture()
	run := releasedRoutingRun()
	initial, err := machine.ResolveRoute(profile, run, machine.Unit{ID: "u1", Complexity: machine.ComplexityHigh, Attempt: 1})
	if err != nil {
		t.Fatal(err)
	}
	if initial.Executor != "codex-fast" || initial.Harness != "codex" || initial.ReviewDepth != 1 || initial.Models["build"] != "gpt-5.6-terra" {
		t.Fatalf("initial route = %+v", initial)
	}
	rework, err := machine.ResolveRoute(profile, run, machine.Unit{ID: "u1", Complexity: machine.ComplexityHigh, Attempt: 2})
	if err != nil {
		t.Fatal(err)
	}
	if rework.Executor != "codex-deep" || rework.ReviewDepth != 2 || rework.Budget.Tokens != 400 {
		t.Fatalf("rework route = %+v", rework)
	}
	for _, tt := range []struct {
		name string
		run  machine.Run
		unit machine.Unit
	}{
		{name: "durable phase", run: run, unit: machine.Unit{ID: "u1", Complexity: machine.ComplexityHigh, Attempt: 1, Phase: machine.PhaseRework}},
		{name: "pending active rework", run: func() machine.Run {
			pending := run
			pending.State = machine.StateRework
			pending.CurrentUnitID = "u1"
			return pending
		}(), unit: machine.Unit{ID: "u1", Complexity: machine.ComplexityHigh, Attempt: 1, Phase: machine.PhaseActive}},
		{name: "parked pending rework", run: func() machine.Run {
			parked := run
			parked.State = machine.StateNeedsDecision
			parked.ReturnState = machine.StateRework
			parked.CurrentUnitID = "u1"
			return parked
		}(), unit: machine.Unit{ID: "u1", Complexity: machine.ComplexityHigh, Attempt: 1, Phase: machine.PhaseActive}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := machine.ResolveRoute(profile, tt.run, tt.unit)
			if err != nil || got.Executor != "codex-deep" {
				t.Fatalf("route = %+v err=%v", got, err)
			}
		})
	}
}

func TestResolveRouteAllRiskAndComplexityClasses(t *testing.T) {
	profile := &machine.RoutingProfile{
		Version:   1,
		Venues:    map[string]machine.VenueDefinition{"local": {Kind: "local-worktree"}},
		Executors: map[string]machine.ExecutorDefinition{"worker": {Harness: "codex", Models: map[string]string{"build": "gpt-5.6-terra"}}},
	}
	risks := []machine.RiskTier{machine.RiskLow, machine.RiskMedium, machine.RiskHigh}
	complexities := []machine.Complexity{machine.ComplexityLow, machine.ComplexityMedium, machine.ComplexityHigh}
	for _, risk := range risks {
		for _, complexity := range complexities {
			for _, rework := range []bool{false, true} {
				profile.Rules = append(profile.Rules, machine.RouteRule{
					RiskTier: risk, Complexity: complexity, Rework: rework, Venue: "local", Executor: "worker",
					Parallelism: 1, ReviewDepth: 1, Budget: machine.Budget{Tokens: 100, Minutes: 10},
				})
			}
		}
	}
	for _, risk := range risks {
		for _, complexity := range complexities {
			for _, attempt := range []int{1, 2} {
				run := releasedRoutingRun()
				run.RiskTier = risk
				got, err := machine.ResolveRoute(profile, run, machine.Unit{ID: "u", Complexity: complexity, Attempt: attempt})
				if err != nil || got.Executor != "worker" {
					t.Fatalf("risk=%s complexity=%s attempt=%d route=%+v err=%v", risk, complexity, attempt, got, err)
				}
			}
		}
	}
}

func TestResolveRouteFailsClosed(t *testing.T) {
	base := releasedRoutingRun()
	tests := []struct {
		name    string
		profile *machine.RoutingProfile
		run     machine.Run
		unit    machine.Unit
		want    string
	}{
		{name: "missing policy", run: base, unit: machine.Unit{ID: "u", Complexity: machine.ComplexityHigh, Attempt: 1}, want: "no routing policy"},
		{name: "missing rule", profile: routingFixture(), run: base, unit: machine.Unit{ID: "u", Complexity: machine.ComplexityLow, Attempt: 1}, want: "no routing rule"},
		{name: "missing complexity", profile: routingFixture(), run: base, unit: machine.Unit{ID: "u", Attempt: 1}, want: "requires complexity"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := machine.ResolveRoute(tt.profile, tt.run, tt.unit)
			if !errors.Is(err, machine.ErrUnmetGuard) || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v", err)
			}
		})
	}
	unreleased := base
	unreleased.ReleasedRevision = nil
	if _, err := machine.ResolveRoute(routingFixture(), unreleased, machine.Unit{ID: "u", Complexity: machine.ComplexityHigh}); !errors.Is(err, machine.ErrUnmetGuard) {
		t.Fatalf("unreleased error = %v", err)
	}
	bounded := base
	bounded.Budget = machine.Budget{Tokens: 100, Minutes: 60}
	if _, err := machine.ResolveRoute(routingFixture(), bounded, machine.Unit{ID: "u", Complexity: machine.ComplexityHigh, Attempt: 1}); !errors.Is(err, machine.ErrUnmetGuard) || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("budget error = %v", err)
	}
	bounded = base
	bounded.Budget = machine.Budget{Tokens: 500, Minutes: 10}
	if _, err := machine.ResolveRoute(routingFixture(), bounded, machine.Unit{ID: "u", Complexity: machine.ComplexityHigh, Attempt: 1}); !errors.Is(err, machine.ErrUnmetGuard) || !strings.Contains(err.Error(), "minute budget") {
		t.Fatalf("minute budget error = %v", err)
	}
}

func TestValidateRoutingProfileRejectsInvalidAndCanonicalizes(t *testing.T) {
	profile := routingFixture()
	if err := machine.ValidateRoutingProfile(profile); err != nil {
		t.Fatal(err)
	}
	if profile.Rules[0].Rework {
		t.Fatalf("rules not canonicalized: %+v", profile.Rules)
	}
	duplicate := routingFixture()
	duplicate.Rules = append(duplicate.Rules, duplicate.Rules[0])
	if err := machine.ValidateRoutingProfile(duplicate); !errors.Is(err, machine.ErrBadArgs) || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("duplicate error = %v", err)
	}
	unknown := routingFixture()
	unknown.Rules[0].Venue = "missing"
	if err := machine.ValidateRoutingProfile(unknown); !errors.Is(err, machine.ErrBadArgs) || !strings.Contains(err.Error(), "unknown venue") {
		t.Fatalf("unknown venue error = %v", err)
	}
	unknown = routingFixture()
	unknown.Rules[0].Executor = "missing"
	if err := machine.ValidateRoutingProfile(unknown); !errors.Is(err, machine.ErrBadArgs) || !strings.Contains(err.Error(), "unknown executor") {
		t.Fatalf("unknown executor error = %v", err)
	}
	zero := routingFixture()
	zero.Rules[0].Budget = machine.Budget{}
	if err := machine.ValidateRoutingProfile(zero); !errors.Is(err, machine.ErrBadArgs) || !strings.Contains(err.Error(), "budget") {
		t.Fatalf("zero budget error = %v", err)
	}
}

func TestResolvedRouteJSONIsStable(t *testing.T) {
	run := releasedRoutingRun()
	left := routingFixture()
	right := routingFixture()
	right.Executors["codex-fast"] = machine.ExecutorDefinition{Harness: "codex", Models: map[string]string{"review": "gpt-5.6-luna", "build": "gpt-5.6-terra"}}
	left.Executors["codex-fast"] = machine.ExecutorDefinition{Harness: "codex", Models: map[string]string{"build": "gpt-5.6-terra", "review": "gpt-5.6-luna"}}
	a, err := machine.ResolveRoute(left, run, machine.Unit{ID: "u", Complexity: machine.ComplexityHigh, Attempt: 1})
	if err != nil {
		t.Fatal(err)
	}
	b, err := machine.ResolveRoute(right, run, machine.Unit{ID: "u", Complexity: machine.ComplexityHigh, Attempt: 1})
	if err != nil {
		t.Fatal(err)
	}
	aJSON, _ := json.MarshalIndent(a, "", "  ")
	bJSON, _ := json.MarshalIndent(b, "", "  ")
	if string(aJSON) != string(bJSON) {
		t.Fatalf("route JSON differs:\n%s\n%s", aJSON, bJSON)
	}
}
