package status

import (
	"encoding/json"

	"github.com/uinaf/slopomatic/internal/machine"
)

// Document is the compact agent-facing status contract.
type Document struct {
	SchemaVersion    int      `json:"schema_version"`
	Revision         int64    `json:"revision"`
	RunID            string   `json:"run_id"`
	RepoKey          string   `json:"repo_key"`
	State            string   `json:"state"`
	IntakeRevision   int64    `json:"intake_revision"`
	Released         bool     `json:"released"`
	ReleasedRevision *int64   `json:"released_revision,omitempty"`
	DeliveryMode     string   `json:"delivery_mode"`
	ReviewConsent    string   `json:"review_consent"`
	SeriesBound      int      `json:"series_bound"`
	CompletedUnits   int      `json:"completed_units"`
	CurrentUnitID    string   `json:"current_unit_id,omitempty"`
	Frontier         []string `json:"frontier"`
	AllowedCommands  []string `json:"allowed_commands"`
	RequiredEvidence []string `json:"required_evidence"`
	NextAction       string   `json:"next_action"`
	Blocker          string   `json:"blocker,omitempty"`
	DecisionQuestion string   `json:"decision_question,omitempty"`
}

func From(run machine.Run, units []machine.Unit) Document {
	allowed := filterCLICommands(machine.AllowedCommands(run, units))
	cmds := make([]string, 0, len(allowed))
	for _, c := range allowed {
		cmds = append(cmds, string(c))
	}
	frontier := make([]string, 0)
	done := map[string]bool{}
	for _, u := range units {
		if u.Done {
			done[u.ID] = true
		}
	}
	for _, u := range units {
		if u.Done {
			continue
		}
		ready := true
		for _, b := range u.Blockers {
			if !done[b] {
				ready = false
				break
			}
		}
		if ready {
			frontier = append(frontier, u.ID)
		}
	}
	return Document{
		SchemaVersion:    1,
		Revision:         run.Revision,
		RunID:            run.ID,
		RepoKey:          run.RepoKey,
		State:            string(run.State),
		IntakeRevision:   run.IntakeRevision,
		Released:         run.Released(),
		ReleasedRevision: run.ReleasedRevision,
		DeliveryMode:     string(run.DeliveryMode),
		ReviewConsent:    string(run.ReviewConsent),
		SeriesBound:      run.SeriesBound,
		CompletedUnits:   run.CompletedUnits,
		CurrentUnitID:    run.CurrentUnitID,
		Frontier:         frontier,
		AllowedCommands:  cmds,
		RequiredEvidence: requiredEvidence(allowed),
		NextAction:       nextAction(allowed),
		Blocker:          run.BlockerReason,
		DecisionQuestion: run.DecisionQuestion,
	}
}

// filterCLICommands drops machine commands the CLI does not expose yet.
func filterCLICommands(allowed []machine.Command) []machine.Command {
	out := make([]machine.Command, 0, len(allowed))
	for _, c := range allowed {
		if c == machine.CmdBlock {
			continue
		}
		out = append(out, c)
	}
	return out
}

func (d Document) CompactLine() string {
	rel := "unreleased"
	if d.Released {
		rel = "released"
	}
	next := d.NextAction
	if next == "" {
		next = "(none)"
	}
	return "slopomatic " + d.RunID + " state=" + d.State + " " + rel + " next=" + next
}

func (d Document) JSON() ([]byte, error) {
	return json.MarshalIndent(d, "", "  ")
}

func nextAction(allowed []machine.Command) string {
	priority := []machine.Command{
		machine.CmdRelease, machine.CmdBuild, machine.CmdVerify, machine.CmdReview,
		machine.CmdDeliver, machine.CmdRework, machine.CmdDecide, machine.CmdIntake,
	}
	for _, p := range priority {
		for _, a := range allowed {
			if a == p {
				return "slopomatic " + string(p)
			}
		}
	}
	if len(allowed) == 0 {
		return ""
	}
	return "slopomatic " + string(allowed[0])
}

func requiredEvidence(allowed []machine.Command) []string {
	for _, a := range allowed {
		switch a {
		case machine.CmdVerify:
			return []string{"verify.command", "verify.exit_code"}
		case machine.CmdReview:
			return []string{"review.reviewer", "review.verdict", "review.artifact_ref"}
		case machine.CmdDeliver:
			return []string{"deliver.delivery_mode", "deliver.pr_url|deliver.commit_sha"}
		}
	}
	return nil
}
