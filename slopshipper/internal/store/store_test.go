package store_test

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/uinaf/slopomatic/internal/machine"
	"github.com/uinaf/slopomatic/internal/store"
)

func TestCreateResolveAndCAS(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Open(filepath.Join(dir, "t.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	run := machine.NewRun("run-a", "repo-a")
	units := []machine.Unit{{ID: "u1", Title: "one"}}
	if err := s.CreateRun(run, units); err != nil {
		t.Fatal(err)
	}

	got, gotUnits, err := s.ResolveActiveRun("repo-a", "")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "run-a" || len(gotUnits) != 1 {
		t.Fatalf("resolve: %+v units=%d", got, len(gotUnits))
	}

	// second open run → ambiguous
	run2 := machine.NewRun("run-b", "repo-a")
	if err := s.CreateRun(run2, units); err != nil {
		t.Fatal(err)
	}
	_, _, err = s.ResolveActiveRun("repo-a", "")
	if !errors.Is(err, machine.ErrAmbiguousRun) {
		t.Fatalf("want ambiguous got %v", err)
	}
	got, _, err = s.ResolveActiveRun("repo-a", "run-b")
	if err != nil || got.ID != "run-b" {
		t.Fatalf("explicit run: %v %+v", err, got)
	}

	other := machine.NewRun("run-other", "repo-b")
	if err := s.CreateRun(other, units); err != nil {
		t.Fatal(err)
	}
	_, _, err = s.ResolveActiveRun("repo-a", "run-other")
	if !errors.Is(err, machine.ErrNotFound) {
		t.Fatalf("want cross-repo not found got %v", err)
	}

	res, err := machine.Apply(got, units, machine.CmdIntake, machine.ApplyInput{
		ExpectedRevision: got.Revision,
		Intake:           &machine.IntakePatch{SeriesBound: intPtr(1)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SaveApply(res, map[string]any{"series_bound": 1}); err != nil {
		t.Fatal(err)
	}

	// stale CAS: pretend we advanced further than the DB
	conflict := res
	conflict.Run.Revision = res.Run.Revision + 10
	if err := s.SaveApply(conflict, nil); !errors.Is(err, machine.ErrRevisionConflict) {
		t.Fatalf("want revision conflict got %v", err)
	}
}

func TestResolveStatusRunIncludesClosed(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Open(filepath.Join(dir, "t.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	run := machine.NewRun("run-done", "repo-a")
	units := []machine.Unit{{ID: "u1", Title: "one"}}
	if err := s.CreateRun(run, units); err != nil {
		t.Fatal(err)
	}
	run.State = machine.StateRunDone
	run.Revision++
	if err := s.SaveApply(machine.ApplyResult{
		Run: run, Units: units, Command: machine.CmdDeliver,
		EventFrom: machine.StateDeliver, EventTo: machine.StateRunDone,
	}, map[string]any{}); err != nil {
		t.Fatal(err)
	}

	_, _, err = s.ResolveActiveRun("repo-a", "")
	if !errors.Is(err, machine.ErrNotFound) {
		t.Fatalf("active should miss closed: %v", err)
	}
	got, _, err := s.ResolveStatusRun("repo-a", "")
	if err != nil || got.ID != "run-done" || got.State != machine.StateRunDone {
		t.Fatalf("status resolve: %v %+v", err, got)
	}
}

func TestListRunsAndEvents(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Open(filepath.Join(dir, "t.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	runA := machine.NewRun("run-a", "repo-a")
	units := []machine.Unit{{ID: "u1", Title: "one"}}
	if err := s.CreateRun(runA, units); err != nil {
		t.Fatal(err)
	}
	res, err := machine.Apply(runA, units, machine.CmdIntake, machine.ApplyInput{
		ExpectedRevision: runA.Revision,
		Intake:           &machine.IntakePatch{SeriesBound: intPtr(1)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SaveApply(res, map[string]any{"series_bound": 1}); err != nil {
		t.Fatal(err)
	}
	runA = res.Run
	units = res.Units
	res, err = machine.Apply(runA, units, machine.CmdRelease, machine.ApplyInput{
		ExpectedRevision: runA.Revision,
		IntakeRevision:   runA.IntakeRevision,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SaveApply(res, map[string]any{"intake_revision": runA.IntakeRevision}); err != nil {
		t.Fatal(err)
	}

	runB := machine.NewRun("run-b", "repo-a")
	if err := s.CreateRun(runB, units); err != nil {
		t.Fatal(err)
	}
	other := machine.NewRun("run-other", "repo-b")
	if err := s.CreateRun(other, units); err != nil {
		t.Fatal(err)
	}

	summaries, err := s.ListRuns("repo-a")
	if err != nil || len(summaries) != 2 {
		t.Fatalf("ListRuns: %v %+v", err, summaries)
	}
	if summaries[0].ID != "run-b" || summaries[1].ID != "run-a" {
		t.Fatalf("ListRuns order: %+v", summaries)
	}
	if !summaries[1].Released || !summaries[1].Open {
		t.Fatalf("run-a summary flags: %+v", summaries[1])
	}
	if summaries[0].Released {
		t.Fatalf("run-b should be unreleased: %+v", summaries[0])
	}

	events, err := s.ListEvents("repo-a", "run-a")
	if err != nil || len(events) != 2 {
		t.Fatalf("ListEvents: %v %+v", err, events)
	}
	if events[0].Command != string(machine.CmdIntake) || events[1].Command != string(machine.CmdRelease) {
		t.Fatalf("ListEvents order: %+v", events)
	}
	_, err = s.ListEvents("repo-b", "run-a")
	if !errors.Is(err, machine.ErrNotFound) {
		t.Fatalf("ListEvents cross-repo: %v", err)
	}
	_, _, _, err = s.GetRunProjection("repo-a", "run-a")
	if err != nil {
		t.Fatal(err)
	}
	_, _, _, err = s.GetRunProjection("repo-b", "run-a")
	if !errors.Is(err, machine.ErrNotFound) {
		t.Fatalf("cross-repo projection: %v", err)
	}
}

func TestRunIDsAreGloballyUnique(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "t.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.CreateRun(machine.NewRun("demo", "repo-a"), nil); err != nil {
		t.Fatal(err)
	}
	err = s.CreateRun(machine.NewRun("demo", "repo-b"), nil)
	if err == nil {
		t.Fatal("expected global primary-key conflict for duplicate run id")
	}
}

func intPtr(v int) *int { return &v }
