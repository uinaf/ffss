package store_test

import (
	"path/filepath"
	"testing"

	"github.com/uinaf/slopshipper/internal/machine"
	"github.com/uinaf/slopshipper/internal/store"
)

func applyWithTelemetry(t *testing.T, s *store.Store, cmd machine.Command, in machine.ApplyInput) {
	t.Helper()
	run, units, err := s.GetRun("run")
	if err != nil {
		t.Fatal(err)
	}
	in.ExpectedRevision = run.Revision
	res, err := machine.Apply(run, units, cmd, in)
	if err != nil {
		t.Fatalf("%s: %v", cmd, err)
	}
	if err := s.SaveApply(res); err != nil {
		t.Fatalf("save %s: %v", cmd, err)
	}
}

func TestTelemetryPersistsAndTotals(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "t.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	if err := s.CreateRun(machine.NewRun("run", "repo"), nil, nil); err != nil {
		t.Fatal(err)
	}

	bound := 1
	applyWithTelemetry(t, s, machine.CmdIntake, machine.ApplyInput{
		Intake: &machine.IntakePatch{Units: []machine.Unit{{ID: "u1", Title: "one"}}, SeriesBound: &bound},
		Telemetry: &machine.Telemetry{
			DurationMS: 1500, Tokens: 4000, CostCents: 12,
			Route: &machine.Route{Venue: "local", Harness: "claude-code"},
		},
	})
	// A transition without telemetry stays valid and adds nothing.
	applyWithTelemetry(t, s, machine.CmdRelease, machine.ApplyInput{IntakeRevision: 2})
	applyWithTelemetry(t, s, machine.CmdBuild, machine.ApplyInput{
		Telemetry: &machine.Telemetry{DurationMS: 500, Tokens: 1000},
	})

	totals, err := s.TelemetryTotals("run")
	if err != nil {
		t.Fatal(err)
	}
	if totals.DurationMS != 2000 || totals.Tokens != 5000 || totals.CostCents != 12 || totals.RecordedEvents != 2 {
		t.Fatalf("totals: %+v", totals)
	}

	events, err := s.ListEvents("repo", "run")
	if err != nil {
		t.Fatal(err)
	}
	var sawRoute bool
	for _, event := range events {
		if event.Command == "intake" && event.TelemetryJSON != "{}" {
			sawRoute = true
		}
		if event.TelemetryJSON == "" {
			t.Fatalf("telemetry json must never be empty: %+v", event)
		}
	}
	if !sawRoute {
		t.Fatal("intake telemetry must persist on its event")
	}
}

func TestCreateRunRecordsInitTelemetry(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "t.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()

	err = s.CreateRun(machine.NewRun("bad", "repo"), nil, &machine.Telemetry{Tokens: -1})
	if err == nil {
		t.Fatal("invalid init telemetry must fail closed")
	}
	if err := s.CreateRun(machine.NewRun("run", "repo"), nil, &machine.Telemetry{DurationMS: 42}); err != nil {
		t.Fatal(err)
	}
	totals, err := s.TelemetryTotals("run")
	if err != nil || totals.DurationMS != 42 || totals.RecordedEvents != 1 {
		t.Fatalf("init telemetry must ride the init event: %+v err=%v", totals, err)
	}
}

func TestMigratesVersionSixAddsTelemetry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v6.sqlite")
	s, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.CreateRun(machine.NewRun("run", "repo"), nil, nil); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	db := openSQLite(t, path)
	if _, err := db.Exec(`ALTER TABLE events DROP COLUMN telemetry_json`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`ALTER TABLE repos DROP COLUMN forge_reviewers_json`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE meta SET value = '6' WHERE key = 'schema_version'`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	s, err = store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	totals, err := s.TelemetryTotals("run")
	if err != nil || totals.RecordedEvents != 0 {
		t.Fatalf("migrated events read as telemetry-free: %+v err=%v", totals, err)
	}
	if _, _, err := s.GetRun("run"); err != nil {
		t.Fatalf("existing runs must keep reading: %v", err)
	}
}
