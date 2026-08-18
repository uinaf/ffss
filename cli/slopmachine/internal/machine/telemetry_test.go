package machine_test

import (
	"errors"
	"testing"

	"github.com/uinaf/ffss/cli/slopmachine/internal/machine"
)

func TestValidateTelemetry(t *testing.T) {
	if err := machine.ValidateTelemetry(nil); err != nil {
		t.Fatalf("absent telemetry is always valid: %v", err)
	}
	valid := &machine.Telemetry{
		DurationMS: 1200, Tokens: 4500, CostCents: 123,
		Route: &machine.Route{Venue: "local", Harness: "claude-code", Models: map[string]string{"build": "claude-fable-5"}},
	}
	if err := machine.ValidateTelemetry(valid); err != nil {
		t.Fatal(err)
	}
	bad := map[string]*machine.Telemetry{
		"negative duration": {DurationMS: -1},
		"negative tokens":   {Tokens: -2},
		"negative cost":     {CostCents: -3},
		"empty route":       {Route: &machine.Route{}},
		"bad venue":         {Route: &machine.Route{Venue: "not a venue"}},
		"bad harness":       {Route: &machine.Route{Harness: "x y"}},
		"bad model role":    {Route: &machine.Route{Models: map[string]string{"bad role": "m"}}},
		"bad model":         {Route: &machine.Route{Models: map[string]string{"build": "bad model"}}},
	}
	for name, telemetry := range bad {
		if err := machine.ValidateTelemetry(telemetry); !errors.Is(err, machine.ErrBadArgs) {
			t.Fatalf("%s: want ErrBadArgs, got %v", name, err)
		}
	}
}

func TestApplyThreadsTelemetryThrough(t *testing.T) {
	run := machine.NewRun("run", "repo")
	telemetry := &machine.Telemetry{DurationMS: 10}
	res, err := machine.Apply(run, nil, machine.CmdIntake, machine.ApplyInput{
		Intake:    &machine.IntakePatch{Units: []machine.Unit{{ID: "u1", Title: "one"}}},
		Telemetry: telemetry,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Telemetry == nil || res.Telemetry.DurationMS != 10 {
		t.Fatalf("telemetry must ride the result: %+v", res.Telemetry)
	}
	_, err = machine.Apply(run, nil, machine.CmdIntake, machine.ApplyInput{
		Intake:    &machine.IntakePatch{Units: []machine.Unit{{ID: "u1", Title: "one"}}},
		Telemetry: &machine.Telemetry{Tokens: -1},
	})
	if !errors.Is(err, machine.ErrBadArgs) {
		t.Fatalf("invalid telemetry must fail the transition: %v", err)
	}
	// Absent telemetry never blocks.
	if _, err := machine.Apply(run, nil, machine.CmdIntake, machine.ApplyInput{
		Intake: &machine.IntakePatch{Units: []machine.Unit{{ID: "u1", Title: "one"}}},
	}); err != nil {
		t.Fatal(err)
	}
}
