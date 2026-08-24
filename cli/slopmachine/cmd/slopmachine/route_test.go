package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/uinaf/ffss/cli/slopmachine/internal/machine"
)

const routingPolicyJSON = `{
  "version": 1,
  "venues": {"local": {"kind": "local-worktree"}},
  "executors": {
    "fast": {"harness": "codex", "models": {"build": "gpt-5.6-terra"}},
    "deep": {"harness": "codex", "models": {"review": "gpt-5.6-terra", "build": "gpt-5.6-sol"}}
  },
  "rules": [
    {"risk_tier": "medium", "complexity": "high", "rework": true, "venue": "local", "executor": "deep", "parallelism": 1, "review_depth": 2, "budget": {"tokens": 400, "minutes": 40}},
    {"risk_tier": "medium", "complexity": "high", "rework": false, "venue": "local", "executor": "fast", "parallelism": 1, "review_depth": 1, "budget": {"tokens": 200, "minutes": 20}}
  ]
}`

func TestRoutePolicyLifecycleAndResolution(t *testing.T) {
	h := newCLIHarness(t)
	h.must("repo", "register", "--bind", "review=slopguard")

	dry := h.mustInput(routingPolicyJSON, "repo", "update", "--routing", "-", "--dry-run", "--json")
	var projected repoProfileDocument
	if err := json.Unmarshal([]byte(dry), &projected); err != nil {
		t.Fatal(err)
	}
	if !projected.DryRun || projected.Routing == nil || projected.Routing.Version != 1 {
		t.Fatalf("routing dry run = %s", dry)
	}
	if persisted := decodeRepoDoc(t, h.must("repo", "--json")); persisted.Routing != nil {
		t.Fatalf("dry run persisted routing: %+v", persisted.Routing)
	}
	h.mustInput(routingPolicyJSON, "repo", "update", "--routing", "-", "--json")

	h.must("init", "--run", "routing")
	h.mustInput(`{
      "delivery_mode":"direct-trunk",
      "required_reviewers":["slopguard"],
      "risk_tier":"medium",
      "budget":{"tokens":500,"minutes":60},
      "series_bound":1,
      "units":[{"id":"u1","title":"one","complexity":"high"}]
    }`, "intake", "--file", "-", "--run", "routing")

	out, code := h.run("route", "--json", "--run", "routing")
	if code != 3 || !strings.Contains(out, "released intake") {
		t.Fatalf("unreleased route exit=%d output=%s", code, out)
	}
	h.must("release", "--revision", "2", "--run", "routing")
	var statusDoc struct {
		RouteReady           bool `json:"route_ready"`
		RoutingPolicyVersion int  `json:"routing_policy_version"`
	}
	if err := json.Unmarshal([]byte(h.must("status", "--json", "--run", "routing")), &statusDoc); err != nil {
		t.Fatal(err)
	}
	if !statusDoc.RouteReady || statusDoc.RoutingPolicyVersion != 1 {
		t.Fatalf("routing status = %+v", statusDoc)
	}
	initial := h.must("route", "--json", "--run", "routing")
	if repeated := h.must("route", "--json", "--run", "routing"); repeated != initial {
		t.Fatalf("route JSON not byte-stable:\n%s\n%s", initial, repeated)
	}
	var doc routeDocument
	if err := json.Unmarshal([]byte(initial), &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Route.Executor != "fast" || doc.Route.Models["build"] != "gpt-5.6-terra" || doc.Rework {
		t.Fatalf("initial route = %+v", doc)
	}

	h.must("build", "--run", "routing")
	h.must("verify", "--cmd", "true", "--run", "routing")
	h.mustInput(`{"reviewer":"slopguard","verdict":"findings","artifact_ref":"file:///tmp/review.json"}`, "review", "--evidence", "-", "--run", "routing")
	pending := h.must("route", "--json", "--run", "routing")
	if err := json.Unmarshal([]byte(pending), &doc); err != nil {
		t.Fatal(err)
	}
	if !doc.Rework || doc.Route.Executor != "deep" {
		t.Fatalf("pending rework route = %+v", doc)
	}
	h.must("build", "--run", "routing")
	rework := h.must("route", "--json", "--run", "routing")
	if err := json.Unmarshal([]byte(rework), &doc); err != nil {
		t.Fatal(err)
	}
	if !doc.Rework || doc.Route.Executor != "deep" || doc.Route.ReviewDepth != 2 {
		t.Fatalf("rework route = %+v", doc)
	}
}

func TestDirectRouteCommand(t *testing.T) {
	repoDir := t.TempDir()
	runGit(t, repoDir, "init")
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repoDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldDir) })
	t.Setenv("SLOPMACHINE_DB", filepath.Join(t.TempDir(), "route.sqlite"))

	routingPath := filepath.Join(t.TempDir(), "routing.json")
	mustWrite(t, routingPath, routingPolicyJSON)
	captureStdout(t, func() int {
		return run([]string{"repo", "register", "--bind", "review=slopguard", "--json"})
	})
	captureStdout(t, func() int {
		return run([]string{"repo", "update", "--routing", routingPath, "--json"})
	})
	captureStdout(t, func() int {
		return run([]string{"init", "--run", "direct-route", "--json"})
	})
	intakePath := filepath.Join(t.TempDir(), "intake.json")
	mustWrite(t, intakePath, `{
      "run":"direct-route",
      "delivery_mode":"direct-trunk",
      "required_reviewers":["slopguard"],
      "risk_tier":"medium",
      "budget":{"tokens":500,"minutes":60},
      "series_bound":1,
      "units":[{"id":"u1","title":"one","complexity":"high"}]
    }`)
	captureStdout(t, func() int {
		return run([]string{"intake", "--input", intakePath, "--json"})
	})
	captureStdout(t, func() int {
		return run([]string{"release", "--revision", "2", "--run", "direct-route", "--json"})
	})

	out := captureStdout(t, func() int {
		return run([]string{"route", "--run", "direct-route", "--json"})
	})
	var doc routeDocument
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatal(err)
	}
	if doc.UnitID != "u1" || doc.Route.Executor != "fast" {
		t.Fatalf("route = %+v", doc)
	}
	plain := captureStdout(t, func() int {
		return run([]string{"route", "--unit", "u1", "--run", "direct-route"})
	})
	if !strings.Contains(plain, "policy-version=1") || !strings.Contains(plain, "venue-kind=local-worktree") ||
		!strings.Contains(plain, "executor=fast") || !strings.Contains(plain, "models=build:gpt-5.6-terra") ||
		!strings.Contains(plain, "budget-tokens=200") {
		t.Fatalf("plain route = %s", plain)
	}
	missing, code := captureStdoutResult(t, func() int {
		return run([]string{"route", "--unit", "missing", "--run", "direct-route", "--json"})
	})
	if code != 5 || !strings.Contains(missing, `"kind": "not_found"`) {
		t.Fatalf("missing route exit=%d output=%s", code, missing)
	}
}

func TestSelectRouteUnit(t *testing.T) {
	run := machine.NewRun("run", "repo")
	units := []machine.Unit{{ID: "u1"}, {ID: "u2"}}
	if _, err := selectRouteUnit(run, units, ""); !errors.Is(err, machine.ErrUnmetGuard) {
		t.Fatalf("ambiguous error = %v", err)
	}
	got, err := selectRouteUnit(run, units, "u2")
	if err != nil || got.ID != "u2" {
		t.Fatalf("selected = %+v err=%v", got, err)
	}
	if _, err := selectRouteUnit(run, units, "missing"); !errors.Is(err, machine.ErrNotFound) {
		t.Fatalf("missing error = %v", err)
	}
	run = machine.NewRun("run", "repo")
	queued := []machine.Unit{
		{ID: "fresh", Phase: machine.PhasePending},
		{ID: "rework", Phase: machine.PhaseRework, Attempt: 1},
	}
	if got, err := selectRouteUnit(run, queued, ""); err != nil || got.ID != "rework" {
		t.Fatalf("queued rework selected = %+v err=%v", got, err)
	}
	run.CurrentUnitID = "ghost"
	if _, err := selectRouteUnit(run, units, ""); !errors.Is(err, machine.ErrCorruptState) {
		t.Fatalf("corrupt current error = %v", err)
	}
}

func TestRouteSchemaUsesPhaseNeutralUnitDescription(t *testing.T) {
	doc, err := schemaDocument("route")
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Commands) != 1 {
		t.Fatalf("route schema = %+v", doc.Commands)
	}
	for _, flag := range doc.Commands[0].Flags {
		if flag.Name == "--unit" {
			if strings.Contains(strings.ToLower(flag.Description), "delivered") {
				t.Fatalf("route unit description = %q", flag.Description)
			}
			return
		}
	}
	t.Fatal("route schema missing --unit")
}

func TestRouteFailsClosedOnPolicyInputAndBudget(t *testing.T) {
	h := newCLIHarness(t)
	h.must("repo", "register", "--bind", "review=slopguard")
	bad := strings.Replace(routingPolicyJSON, `"venue": "local"`, `"venue": "missing"`, 1)
	out, code := h.runInput(bad, "repo", "update", "--routing", "-")
	if code != 2 || !strings.Contains(out, "unknown venue") {
		t.Fatalf("bad policy exit=%d output=%s", code, out)
	}
	h.mustInput(routingPolicyJSON, "repo", "update", "--routing", "-")
	h.must("init", "--run", "budget")
	h.mustInput(`{
      "required_reviewers":["slopguard"],
      "risk_tier":"medium",
      "budget":{"tokens":100,"minutes":60},
      "units":[{"id":"u1","title":"one","complexity":"high"}]
    }`, "intake", "--file", "-", "--run", "budget")
	h.must("release", "--revision", "2", "--run", "budget")
	out, code = h.run("route", "--json", "--run", "budget")
	if code != 3 || !strings.Contains(out, "exceeds released budget") {
		t.Fatalf("budget route exit=%d output=%s", code, out)
	}
}
