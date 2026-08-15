package status_test

import (
	"strings"
	"testing"

	"github.com/uinaf/slopshipper/internal/machine"
	"github.com/uinaf/slopshipper/internal/status"
)

func TestFromContextNamesCanonicalVerifyCommand(t *testing.T) {
	run := machine.NewRun("run", "repo")
	run.State = machine.StateBuild
	rev := run.IntakeRevision
	run.ReleasedRevision = &rev
	run.CurrentUnitID = "u1"
	units := []machine.Unit{{ID: "u1", Phase: machine.PhaseActive, Attempt: 1}}

	plain := status.From(run, units)
	if !strings.Contains(plain.NextAction, "'<verification command>'") {
		t.Fatalf("profile-less runs keep the placeholder: %s", plain.NextAction)
	}
	if plain.RepoRegistered || plain.VerifyCommand != "" {
		t.Fatalf("profile-less status must not claim a profile: %+v", plain)
	}

	doc := status.FromContext(run, units, status.Context{RepoRegistered: true, VerifyCommand: "mise run verify"})
	if !strings.Contains(doc.NextAction, "slopshipper verify --cmd 'mise run verify'") {
		t.Fatalf("canonical command must be executable verbatim: %s", doc.NextAction)
	}
	if !doc.RepoRegistered || doc.VerifyCommand != "mise run verify" {
		t.Fatalf("context fields must surface: %+v", doc)
	}

	// Quoting stays safe for embedded single quotes.
	quoted := status.FromContext(run, units, status.Context{VerifyCommand: "echo 'x'"})
	if !strings.Contains(quoted.NextAction, `--cmd 'echo '"'"'x'"'"''`) {
		t.Fatalf("verify command must be shell-quoted: %s", quoted.NextAction)
	}
}
