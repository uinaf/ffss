package status

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/uinaf/slopshipper/internal/machine"
)

const (
	schemaVersion      = 3
	stateUninitialized = "UNINITIALIZED"
	AgentFieldMask     = "state,run_id,next_action,allowed_commands,required_evidence,intake_revision,required_reviewers,completed_reviewers,delivery_mode,blocker,decision_question"
)

// Document is the compact agent-facing status contract.
type Document struct {
	SchemaVersion       int      `json:"schema_version"`
	Revision            int64    `json:"revision"`
	RunID               string   `json:"run_id"`
	RepoKey             string   `json:"repo_key"`
	State               string   `json:"state"`
	IntakeRevision      int64    `json:"intake_revision"`
	Released            bool     `json:"released"`
	ReleasedRevision    *int64   `json:"released_revision,omitempty"`
	DeliveryMode        string   `json:"delivery_mode"`
	RiskTier            string   `json:"risk_tier,omitempty"`
	BudgetTokens        int      `json:"budget_tokens,omitempty"`
	BudgetMinutes       int      `json:"budget_minutes,omitempty"`
	SeriesBound         int      `json:"series_bound"`
	CompletedUnits      int      `json:"completed_units"`
	CurrentUnitID       string   `json:"current_unit_id,omitempty"`
	Frontier            []string `json:"frontier"`
	RequiredReviewers   []string `json:"required_reviewers"`
	CompletedReviewers  []string `json:"completed_reviewers"`
	AllowedCommands     []string `json:"allowed_commands"`
	RequiredEvidence    []string `json:"required_evidence"`
	NextAction          string   `json:"next_action"`
	Blocker             string   `json:"blocker,omitempty"`
	DecisionQuestion    string   `json:"decision_question,omitempty"`
	DryRun              bool     `json:"dry_run,omitempty"`
	ValidatedCommand    string   `json:"validated_command,omitempty"`
	OutcomeUndetermined bool     `json:"outcome_undetermined,omitempty"`
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
	requiredReviewers := make([]string, 0, len(run.RequiredReviewers))
	for _, reviewer := range run.RequiredReviewers {
		requiredReviewers = append(requiredReviewers, string(reviewer))
	}
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
		RiskTier:           string(run.RiskTier),
		BudgetTokens:       run.Budget.Tokens,
		BudgetMinutes:      run.Budget.Minutes,
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
		NextAction:         "slopshipper init",
		Blocker:            "no run exists for this repository",
	}
}

func (d Document) CompactLine() string {
	prefix := "slopshipper"
	if d.DryRun {
		prefix += " dry-run"
	}
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

func (d Document) JSONFields(fields []string) ([]byte, error) {
	if len(fields) == 0 {
		return d.JSON()
	}
	if err := ValidateFields(fields); err != nil {
		return nil, err
	}
	full, err := d.JSON()
	if err != nil {
		return nil, err
	}
	var values map[string]any
	if err := json.Unmarshal(full, &values); err != nil {
		return nil, err
	}
	selected := make(map[string]any, len(fields))
	for _, field := range fields {
		if value, present := values[field]; present {
			selected[field] = value
		}
	}
	return json.MarshalIndent(selected, "", "  ")
}

func ValidateFields(fields []string) error {
	known := FieldNames()
	allowed := make(map[string]struct{}, len(known))
	for _, field := range known {
		allowed[field] = struct{}{}
	}
	for _, field := range fields {
		if _, ok := allowed[field]; !ok {
			return fmt.Errorf("unknown status field %q", field)
		}
	}
	return nil
}

func FieldNames() []string {
	document := reflect.TypeOf(Document{})
	fields := make([]string, 0, document.NumField())
	for i := 0; i < document.NumField(); i++ {
		name := strings.Split(document.Field(i).Tag.Get("json"), ",")[0]
		if name != "" && name != "-" {
			fields = append(fields, name)
		}
	}
	sort.Strings(fields)
	return fields
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
					command = "slopshipper intake --file -"
				case machine.CmdRelease:
					command = fmt.Sprintf("slopshipper release --revision %d", run.IntakeRevision)
				case machine.CmdVerify:
					command = "slopshipper verify --cmd '<verification command>'"
				case machine.CmdReview:
					command = "slopshipper review --evidence -"
				case machine.CmdDeliver:
					command = "slopshipper deliver --evidence -"
				case machine.CmdDecide:
					command = "slopshipper decide --answer '<answer>'"
				case machine.CmdRetry:
					command = "slopshipper retry --reason '<reason>'"
				default:
					command = "slopshipper " + string(p)
				}
				return withRun(command, run.ID)
			}
		}
	}
	if len(allowed) == 0 {
		return ""
	}
	return withRun("slopshipper "+string(allowed[0]), run.ID)
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
