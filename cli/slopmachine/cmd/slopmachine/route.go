package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/uinaf/ffss/cli/slopmachine/internal/machine"
	"github.com/uinaf/ffss/cli/slopmachine/internal/store"
)

type routeDocument struct {
	SchemaVersion int                   `json:"schema_version"`
	RunID         string                `json:"run_id"`
	UnitID        string                `json:"unit_id"`
	RiskTier      string                `json:"risk_tier"`
	Complexity    string                `json:"complexity"`
	Rework        bool                  `json:"rework"`
	Route         machine.ResolvedRoute `json:"route"`
}

func cmdRoute(st *store.Store, args []string, opts runOptions) int {
	fs, code := requireFlags("route", args, opts)
	if code != 0 {
		return code
	}
	key, err := resolveRepoKeyForOptions(st, opts)
	if err != nil {
		return writeFailure(opts, 2, err)
	}
	run, units, err := st.ResolveStatusRun(key, fs["run"])
	if err != nil {
		return mapErr(err, opts)
	}
	unit, err := selectRouteUnit(run, units, fs["unit"])
	if err != nil {
		return mapErr(err, opts)
	}
	profile, found, err := st.GetRepoProfile(key)
	if err != nil {
		return mapErr(err, opts)
	}
	if !found {
		return mapErr(fmt.Errorf("%w: repository has no profile; create one with slopmachine repo register", machine.ErrUnmetGuard), opts)
	}
	resolved, err := machine.ResolveRoute(profile.Routing, run, unit)
	if err != nil {
		return mapErr(err, opts)
	}
	doc := routeDocument{
		SchemaVersion: 1,
		RunID:         run.ID,
		UnitID:        unit.ID,
		RiskTier:      string(run.RiskTier),
		Complexity:    string(unit.Complexity),
		Rework:        machine.RouteIsRework(run, unit),
		Route:         resolved,
	}
	if opts.json {
		if err := writeJSON(doc); err != nil {
			return writeFailure(opts, 10, err)
		}
		return 0
	}
	fmt.Fprintf(os.Stdout, "slopmachine route run=%s unit=%s policy-version=%d venue=%s venue-kind=%s executor=%s harness=%s models=%s parallelism=%d review-depth=%d budget-tokens=%d budget-minutes=%d\n",
		doc.RunID, doc.UnitID, resolved.PolicyVersion, resolved.Venue, resolved.VenueKind, resolved.Executor, resolved.Harness, formatRouteModels(resolved.Models),
		resolved.Parallelism, resolved.ReviewDepth, resolved.Budget.Tokens, resolved.Budget.Minutes)
	return 0
}

func formatRouteModels(models map[string]string) string {
	roles := make([]string, 0, len(models))
	for role := range models {
		roles = append(roles, role)
	}
	sort.Strings(roles)
	bindings := make([]string, 0, len(roles))
	for _, role := range roles {
		bindings = append(bindings, role+":"+models[role])
	}
	return strings.Join(bindings, ",")
}

func selectRouteUnit(run machine.Run, units []machine.Unit, requested string) (machine.Unit, error) {
	if requested != "" {
		if err := machine.ValidateResourceID("unit id", requested); err != nil {
			return machine.Unit{}, err
		}
		for _, unit := range units {
			if unit.ID == requested {
				return unit, nil
			}
		}
		return machine.Unit{}, fmt.Errorf("%w: unit %q", machine.ErrNotFound, requested)
	}
	if run.CurrentUnitID != "" {
		for _, unit := range units {
			if unit.ID == run.CurrentUnitID {
				return unit, nil
			}
		}
		return machine.Unit{}, fmt.Errorf("%w: current unit %q missing", machine.ErrCorruptState, run.CurrentUnitID)
	}
	if unit := machine.NextReworkUnit(units); unit != nil {
		return *unit, nil
	}
	frontier := machine.Frontier(units)
	if len(frontier) != 1 {
		return machine.Unit{}, fmt.Errorf("%w: --unit is required when the route target is ambiguous", machine.ErrUnmetGuard)
	}
	for _, unit := range units {
		if unit.ID == frontier[0] {
			return unit, nil
		}
	}
	return machine.Unit{}, fmt.Errorf("%w: frontier unit %q missing", machine.ErrCorruptState, frontier[0])
}
