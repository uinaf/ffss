package machine

import (
	"fmt"
	"sort"
)

const (
	routingPolicyVersion = 1
	maxRoutingEntries    = 128
	maxRouteParallelism  = 16
	maxReviewDepth       = 8
)

// RoutingProfile is the repository-declared registry and deterministic policy
// table used to resolve work routes. It records identities and configuration;
// dispatch adapters remain outside this contract.
type RoutingProfile struct {
	Version   int                           `json:"version"`
	Venues    map[string]VenueDefinition    `json:"venues"`
	Executors map[string]ExecutorDefinition `json:"executors"`
	Rules     []RouteRule                   `json:"rules"`
}

// VenueDefinition declares one place work may run. Kind is adapter-neutral
// metadata such as local-worktree, remote-lease, cloud-agent, or sandbox.
type VenueDefinition struct {
	Kind string `json:"kind"`
}

// ExecutorDefinition declares the harness and role-to-model bindings selected
// together by a route. It never carries an executable path or credentials.
type ExecutorDefinition struct {
	Harness string            `json:"harness"`
	Models  map[string]string `json:"models"`
}

// RouteRule is one exact table entry. Rework distinguishes the first attempt
// from every later attempt, making escalation bounded to two declared levels.
type RouteRule struct {
	RiskTier    RiskTier   `json:"risk_tier"`
	Complexity  Complexity `json:"complexity"`
	Rework      bool       `json:"rework"`
	Venue       string     `json:"venue"`
	Executor    string     `json:"executor"`
	Parallelism int        `json:"parallelism"`
	ReviewDepth int        `json:"review_depth"`
	Budget      Budget     `json:"budget"`
}

// ResolvedRoute is the immutable route tuple produced from a released task
// contract and repository policy. It is output only; no adapter is launched.
type ResolvedRoute struct {
	PolicyVersion int               `json:"policy_version"`
	Venue         string            `json:"venue"`
	VenueKind     string            `json:"venue_kind"`
	Executor      string            `json:"executor"`
	Harness       string            `json:"harness"`
	Models        map[string]string `json:"models"`
	Parallelism   int               `json:"parallelism"`
	ReviewDepth   int               `json:"review_depth"`
	Budget        Budget            `json:"budget"`
}

// ValidateRoutingProfile validates and canonicalizes a declared policy. Rules
// are sorted by their selector so equivalent documents persist identically.
func ValidateRoutingProfile(profile *RoutingProfile) error {
	if profile == nil {
		return nil
	}
	if profile.Version != routingPolicyVersion {
		return fmt.Errorf("%w: routing policy version must be %d", ErrBadArgs, routingPolicyVersion)
	}
	if len(profile.Venues) == 0 || len(profile.Venues) > maxRoutingEntries {
		return fmt.Errorf("%w: routing venues must contain 1-%d entries", ErrBadArgs, maxRoutingEntries)
	}
	if len(profile.Executors) == 0 || len(profile.Executors) > maxRoutingEntries {
		return fmt.Errorf("%w: routing executors must contain 1-%d entries", ErrBadArgs, maxRoutingEntries)
	}
	if len(profile.Rules) == 0 || len(profile.Rules) > maxRoutingEntries {
		return fmt.Errorf("%w: routing rules must contain 1-%d entries", ErrBadArgs, maxRoutingEntries)
	}
	for id, venue := range profile.Venues {
		if err := ValidateResourceID("venue id", id); err != nil {
			return err
		}
		if err := ValidateResourceID("venue kind", venue.Kind); err != nil {
			return err
		}
	}
	for id, executor := range profile.Executors {
		if err := ValidateResourceID("executor id", id); err != nil {
			return err
		}
		if err := ValidateResourceID("executor harness", executor.Harness); err != nil {
			return err
		}
		if len(executor.Models) == 0 || len(executor.Models) > maxRoutingEntries {
			return fmt.Errorf("%w: executor %q models must contain 1-%d role bindings", ErrBadArgs, id, maxRoutingEntries)
		}
		for role, model := range executor.Models {
			if err := ValidateResourceID("model role", role); err != nil {
				return err
			}
			if err := ValidateResourceID("model id", model); err != nil {
				return err
			}
		}
	}
	seen := make(map[string]struct{}, len(profile.Rules))
	for i := range profile.Rules {
		rule := &profile.Rules[i]
		if err := validRiskTier(rule.RiskTier); err != nil {
			return err
		}
		if rule.RiskTier == "" {
			return fmt.Errorf("%w: routing rule risk_tier is required", ErrBadArgs)
		}
		if err := validComplexity(rule.Complexity); err != nil {
			return err
		}
		if rule.Complexity == "" {
			return fmt.Errorf("%w: routing rule complexity is required", ErrBadArgs)
		}
		if _, ok := profile.Venues[rule.Venue]; !ok {
			return fmt.Errorf("%w: routing rule references unknown venue %q", ErrBadArgs, rule.Venue)
		}
		if _, ok := profile.Executors[rule.Executor]; !ok {
			return fmt.Errorf("%w: routing rule references unknown executor %q", ErrBadArgs, rule.Executor)
		}
		if rule.Parallelism < 1 || rule.Parallelism > maxRouteParallelism {
			return fmt.Errorf("%w: routing rule parallelism must be between 1 and %d", ErrBadArgs, maxRouteParallelism)
		}
		if rule.ReviewDepth < 1 || rule.ReviewDepth > maxReviewDepth {
			return fmt.Errorf("%w: routing rule review_depth must be between 1 and %d", ErrBadArgs, maxReviewDepth)
		}
		if rule.Budget.Tokens <= 0 || rule.Budget.Minutes <= 0 {
			return fmt.Errorf("%w: routing rule budget requires positive tokens and minutes", ErrBadArgs)
		}
		key := routeRuleKey(rule.RiskTier, rule.Complexity, rule.Rework)
		if _, duplicate := seen[key]; duplicate {
			return fmt.Errorf("%w: duplicate routing rule for risk=%s complexity=%s rework=%t", ErrBadArgs, rule.RiskTier, rule.Complexity, rule.Rework)
		}
		seen[key] = struct{}{}
	}
	sort.Slice(profile.Rules, func(i, j int) bool {
		return routeRuleKey(profile.Rules[i].RiskTier, profile.Rules[i].Complexity, profile.Rules[i].Rework) <
			routeRuleKey(profile.Rules[j].RiskTier, profile.Rules[j].Complexity, profile.Rules[j].Rework)
	})
	return nil
}

// ResolveRoute selects one exact policy row. Durable rework phase, a pending
// rework transition for the active unit, and second or later attempts all use
// the rework row.
func ResolveRoute(profile *RoutingProfile, run Run, unit Unit) (ResolvedRoute, error) {
	if profile == nil {
		return ResolvedRoute{}, fmt.Errorf("%w: repository has no routing policy; declare one with slopmachine repo update --routing PATH", ErrUnmetGuard)
	}
	if err := ValidateRoutingProfile(profile); err != nil {
		return ResolvedRoute{}, err
	}
	if !run.Released() {
		return ResolvedRoute{}, fmt.Errorf("%w: route resolution requires a released intake", ErrUnmetGuard)
	}
	if run.RiskTier == "" {
		return ResolvedRoute{}, fmt.Errorf("%w: route resolution requires risk_tier", ErrUnmetGuard)
	}
	if unit.Complexity == "" {
		return ResolvedRoute{}, fmt.Errorf("%w: route resolution requires complexity for unit %q", ErrUnmetGuard, unit.ID)
	}
	rework := RouteIsRework(run, unit)
	var selected *RouteRule
	for i := range profile.Rules {
		rule := &profile.Rules[i]
		if rule.RiskTier == run.RiskTier && rule.Complexity == unit.Complexity && rule.Rework == rework {
			selected = rule
			break
		}
	}
	if selected == nil {
		return ResolvedRoute{}, fmt.Errorf("%w: no routing rule for risk=%s complexity=%s rework=%t", ErrUnmetGuard, run.RiskTier, unit.Complexity, rework)
	}
	if err := routeWithinBudget(selected.Budget, run.Budget); err != nil {
		return ResolvedRoute{}, err
	}
	venue := profile.Venues[selected.Venue]
	executor := profile.Executors[selected.Executor]
	models := make(map[string]string, len(executor.Models))
	for role, model := range executor.Models {
		models[role] = model
	}
	return ResolvedRoute{
		PolicyVersion: profile.Version,
		Venue:         selected.Venue,
		VenueKind:     venue.Kind,
		Executor:      selected.Executor,
		Harness:       executor.Harness,
		Models:        models,
		Parallelism:   selected.Parallelism,
		ReviewDepth:   selected.ReviewDepth,
		Budget:        selected.Budget,
	}, nil
}

// RouteIsRework reports whether current evidence selects a rework rule before
// the next build claim increments the attempt or clears the durable phase.
func RouteIsRework(run Run, unit Unit) bool {
	if unit.Attempt > 1 || unit.Phase == PhaseRework {
		return true
	}
	if run.CurrentUnitID != unit.ID {
		return false
	}
	return run.State == StateRework ||
		run.State == StateNeedsDecision && run.ReturnState == StateRework
}

func routeRuleKey(risk RiskTier, complexity Complexity, rework bool) string {
	return fmt.Sprintf("%s\x00%s\x00%t", risk, complexity, rework)
}

func routeWithinBudget(route, released Budget) error {
	if released.Tokens > 0 && route.Tokens > released.Tokens {
		return fmt.Errorf("%w: route token budget %d exceeds released budget %d", ErrUnmetGuard, route.Tokens, released.Tokens)
	}
	if released.Minutes > 0 && route.Minutes > released.Minutes {
		return fmt.Errorf("%w: route minute budget %d exceeds released budget %d", ErrUnmetGuard, route.Minutes, released.Minutes)
	}
	return nil
}
