package status

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/uinaf/slopomatic/internal/machine"
)

const (
	schemaVersion      = 2
	stateUninitialized = "UNINITIALIZED"
)

// Document is the compact agent-facing status contract.
type Document struct {
	SchemaVersion      int      `json:"schema_version"`
	Revision           int64    `json:"revision"`
	RunID              string   `json:"run_id"`
	RepoKey            string   `json:"repo_key"`
	State              string   `json:"state"`
	IntakeRevision     int64    `json:"intake_revision"`
	Released           bool     `json:"released"`
	ReleasedRevision   *int64   `json:"released_revision,omitempty"`
	DeliveryMode       string   `json:"delivery_mode"`
	ReviewConsent      string   `json:"review_consent"`
	SeriesBound        int      `json:"series_bound"`
	CompletedUnits     int      `json:"completed_units"`
	CurrentUnitID      string   `json:"current_unit_id,omitempty"`
	Frontier           []string `json:"frontier"`
	RequiredReviewers  []string `json:"required_reviewers"`
	CompletedReviewers []string `json:"completed_reviewers"`
	AllowedCommands    []string `json:"allowed_commands"`
	RequiredEvidence   []string `json:"required_evidence"`
	NextAction         string   `json:"next_action"`
	Blocker            string   `json:"blocker,omitempty"`
	DecisionQuestion   string   `json:"decision_question,omitempty"`
}

func From(run machine.Run, units []machine.Unit) Document {
	allowed := machine.AllowedCommands(run, units)
	cmds := make([]string, 0, len(allowed))
	for _, c := range allowed {
		cmds = append(cmds, string(c))
	}
	completedReviewers := make([]string, 0, len(run.CompletedReviewers))
	for _, reviewer := range run.CompletedReviewers {
		completedReviewers = append(completedReviewers, string(reviewer))
	}
	requiredReviewers := requiredReviewers(run.ReviewConsent)
	requiredEvidence := requiredEvidence(allowed)
	return Document{
		SchemaVersion:      schemaVersion,
		Revision:           run.Revision,
		RunID:              run.ID,
		RepoKey:            run.RepoKey,
		State:              string(run.State),
		IntakeRevision:     run.IntakeRevision,
		Released:           run.Released(),
		ReleasedRevision:   run.ReleasedRevision,
		DeliveryMode:       string(run.DeliveryMode),
		ReviewConsent:      string(run.ReviewConsent),
		SeriesBound:        run.SeriesBound,
		CompletedUnits:     run.CompletedUnits,
		CurrentUnitID:      run.CurrentUnitID,
		Frontier:           machine.Frontier(units),
		RequiredReviewers:  requiredReviewers,
		CompletedReviewers: completedReviewers,
		AllowedCommands:    cmds,
		RequiredEvidence:   requiredEvidence,
		NextAction:         nextAction(run, allowed),
		Blocker:            run.BlockerReason,
		DecisionQuestion:   run.DecisionQuestion,
	}
}

// Bootstrap describes the supported state before a repository has a run.
func Bootstrap(repoKey string) Document {
	return Document{
		SchemaVersion:      schemaVersion,
		RepoKey:            repoKey,
		State:              stateUninitialized,
		Frontier:           []string{},
		RequiredReviewers:  []string{},
		CompletedReviewers: []string{},
		AllowedCommands:    []string{string(machine.CmdInit)},
		RequiredEvidence:   []string{},
		NextAction:         "slopomatic init",
		Blocker:            "no run exists for this repository",
	}
}

func (d Document) CompactLine() string {
	prefix := "slopomatic"
	if d.RunID != "" {
		prefix += " " + d.RunID
	}
	if d.State == stateUninitialized {
		return prefix + " state=" + d.State + " next=" + d.NextAction
	}
	rel := "unreleased"
	if d.Released {
		rel = "released"
	}
	next := d.NextAction
	if next == "" {
		next = "(none)"
	}
	return prefix + " state=" + d.State + " " + rel + " next=" + next
}

func (d Document) JSON() ([]byte, error) {
	return json.MarshalIndent(d, "", "  ")
}

func nextAction(run machine.Run, allowed []machine.Command) string {
	priority := []machine.Command{
		machine.CmdRelease, machine.CmdBuild, machine.CmdVerify, machine.CmdReview,
		machine.CmdDeliver, machine.CmdRework, machine.CmdDecide, machine.CmdRetry, machine.CmdIntake,
	}
	for _, p := range priority {
		for _, a := range allowed {
			if a == p {
				var command string
				switch p {
				case machine.CmdIntake:
					command = "slopomatic intake --file <intake.json>"
				case machine.CmdRelease:
					command = fmt.Sprintf("slopomatic release --revision %d", run.IntakeRevision)
				case machine.CmdVerify:
					command = "slopomatic verify --cmd '<verification command>'"
				case machine.CmdReview:
					command = "slopomatic review --evidence <review.json>"
				case machine.CmdDeliver:
					command = "slopomatic deliver --evidence <deliver.json>"
				case machine.CmdDecide:
					command = "slopomatic decide --answer '<answer>'"
				case machine.CmdRetry:
					command = "slopomatic retry --reason '<reason>'"
				default:
					command = "slopomatic " + string(p)
				}
				return withRun(command, run.ID)
			}
		}
	}
	if len(allowed) == 0 {
		return ""
	}
	return withRun("slopomatic "+string(allowed[0]), run.ID)
}

func withRun(command, runID string) string {
	if runID == "" {
		return command
	}
	return command + " --run=" + shellQuote(runID)
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
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
		case machine.CmdRetry:
			return []string{"retry.reason"}
		}
	}
	return []string{}
}

func requiredReviewers(consent machine.ReviewConsent) []string {
	switch consent {
	case machine.ReviewAutoreview:
		return []string{string(machine.ReviewerAutoreview)}
	case machine.ReviewBugbot:
		return []string{string(machine.ReviewerBugbot)}
	case machine.ReviewBoth:
		return []string{string(machine.ReviewerAutoreview), string(machine.ReviewerBugbot)}
	case machine.ReviewHuman:
		return []string{string(machine.ReviewerHuman)}
	default:
		return []string{}
	}
}
