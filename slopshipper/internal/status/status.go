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
	AgentFieldMask     = "state,run_id,next_action,allowed_commands,required_evidence,intake_revision,required_reviewers,completed_reviewers,delivered_units,delivery_mode,blocker,decision_question"
)

// UnitStatus is the compact per-unit projection inside status.
type UnitStatus struct {
	ID          string `json:"id"`
	Phase       string `json:"phase"`
	Attempt     int    `json:"attempt"`
	ReworkCause string `json:"rework_cause,omitempty"`
}

// Document is the compact agent-facing status contract.
type Document struct {
	SchemaVersion       int          `json:"schema_version"`
	Revision            int64        `json:"revision"`
	RunID               string       `json:"run_id"`
	RepoKey             string       `json:"repo_key"`
	State               string       `json:"state"`
	IntakeRevision      int64        `json:"intake_revision"`
	Released            bool         `json:"released"`
	ReleasedRevision    *int64       `json:"released_revision,omitempty"`
	DeliveryMode        string       `json:"delivery_mode"`
	RiskTier            string       `json:"risk_tier,omitempty"`
	BudgetTokens        int          `json:"budget_tokens,omitempty"`
	BudgetMinutes       int          `json:"budget_minutes,omitempty"`
	SeriesBound         int          `json:"series_bound"`
	CompletedUnits      int          `json:"completed_units"`
	CurrentUnitID       string       `json:"current_unit_id,omitempty"`
	Frontier            []string     `json:"frontier"`
	DeliveredUnits      []string     `json:"delivered_units"`
	Units               []UnitStatus `json:"units"`
	RequiredReviewers   []string     `json:"required_reviewers"`
	CompletedReviewers  []string     `json:"completed_reviewers"`
	AllowedCommands     []string     `json:"allowed_commands"`
	RequiredEvidence    []string     `json:"required_evidence"`
	NextAction          string       `json:"next_action"`
	Blocker             string       `json:"blocker,omitempty"`
	DecisionQuestion    string       `json:"decision_question,omitempty"`
	DryRun              bool         `json:"dry_run,omitempty"`
	ValidatedCommand    string       `json:"validated_command,omitempty"`
	OutcomeUndetermined bool         `json:"outcome_undetermined,omitempty"`
	VerifyCommand       string       `json:"verify_command,omitempty"`
	RepoRegistered      bool         `json:"repo_registered,omitempty"`
	TotalDurationMS     int64        `json:"total_duration_ms"`
	TotalTokens         int          `json:"total_tokens"`
	TotalCostCents      int          `json:"total_cost_cents"`
	TelemetryEvents     int          `json:"telemetry_events"`
}

// Context carries repo-profile defaults the run inherits at read time.
type Context struct {
	// RepoRegistered marks that a repo profile exists for this checkout.
	RepoRegistered bool
	// VerifyCommand is the profile's canonical verify command; when set,
	// verify's next_action names it instead of a placeholder.
	VerifyCommand string
	// Telemetry totals aggregated from the run's recorded events.
	TotalDurationMS int64
	TotalTokens     int
	TotalCostCents  int
	TelemetryEvents int
}

func From(run machine.Run, units []machine.Unit) Document {
	return FromContext(run, units, Context{})
}

// FromContext projects status with repo-profile defaults applied.
func FromContext(run machine.Run, units []machine.Unit, ctx Context) Document {
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
	selected := selectedCommand(allowed)
	requiredEvidence := requiredEvidence(selected, units)
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
		DeliveredUnits:     deliveredUnits(units),
		Units:              unitStatuses(units),
		RequiredReviewers:  requiredReviewers,
		CompletedReviewers: completedReviewers,
		AllowedCommands:    cmds,
		RequiredEvidence:   requiredEvidence,
		NextAction:         nextAction(run, units, selected, ctx),
		Blocker:            run.BlockerReason,
		DecisionQuestion:   run.DecisionQuestion,
		VerifyCommand:      ctx.VerifyCommand,
		RepoRegistered:     ctx.RepoRegistered,
		TotalDurationMS:    ctx.TotalDurationMS,
		TotalTokens:        ctx.TotalTokens,
		TotalCostCents:     ctx.TotalCostCents,
		TelemetryEvents:    ctx.TelemetryEvents,
	}
}

// Bootstrap describes the supported state before a repository has a run.
func Bootstrap(repoKey string) Document {
	return Document{
		SchemaVersion:      schemaVersion,
		RepoKey:            repoKey,
		State:              stateUninitialized,
		Frontier:           []string{},
		DeliveredUnits:     []string{},
		Units:              []UnitStatus{},
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

// selectedCommand picks the one command next_action recommends; the
// required_evidence projection derives from the same choice so the two
// fields can never disagree.
func selectedCommand(allowed []machine.Command) machine.Command {
	// decide and retry outrank observe: resolving a human latch is always
	// actionable, while an external signal may not exist yet.
	priority := []machine.Command{
		machine.CmdRelease, machine.CmdBuild, machine.CmdVerify, machine.CmdReview,
		machine.CmdDeliver, machine.CmdRework, machine.CmdDecide, machine.CmdRetry, machine.CmdObserve, machine.CmdIntake,
	}
	for _, p := range priority {
		for _, a := range allowed {
			if a == p {
				return p
			}
		}
	}
	if len(allowed) == 0 {
		return ""
	}
	return allowed[0]
}

func nextAction(run machine.Run, units []machine.Unit, selected machine.Command, ctx Context) string {
	if selected == "" {
		return ""
	}
	var command string
	switch selected {
	case machine.CmdIntake:
		command = "slopshipper intake --file -"
	case machine.CmdRelease:
		command = fmt.Sprintf("slopshipper release --revision %d", run.IntakeRevision)
	case machine.CmdVerify:
		// A registered repo's canonical verify command keeps the action
		// executable verbatim; without one the placeholder stays documented.
		if ctx.VerifyCommand != "" {
			command = "slopshipper verify --cmd " + shellQuote(ctx.VerifyCommand)
		} else {
			command = "slopshipper verify --cmd '<verification command>'"
		}
	case machine.CmdReview:
		command = "slopshipper review --evidence -"
	case machine.CmdDeliver:
		command = "slopshipper deliver --evidence -"
	case machine.CmdObserve:
		command = observeAction(units)
	case machine.CmdDecide:
		command = "slopshipper decide --answer '<answer>'"
	case machine.CmdRetry:
		command = "slopshipper retry --reason '<reason>'"
	default:
		command = "slopshipper " + string(selected)
	}
	return withRun(command, run.ID)
}

// observeAction stays executable for any delivered-unit count: one delivered
// unit is named inline; several leave a documented unit placeholder.
func observeAction(units []machine.Unit) string {
	delivered := deliveredUnits(units)
	if len(delivered) == 1 {
		return "slopshipper observe --signal '<signal>' --unit=" + shellQuote(delivered[0])
	}
	return "slopshipper observe --signal '<signal>' --unit '<unit>'"
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

func deliveredUnits(units []machine.Unit) []string {
	out := make([]string, 0)
	for _, u := range units {
		if u.Phase == machine.PhaseDelivered {
			out = append(out, u.ID)
		}
	}
	return out
}

func unitStatuses(units []machine.Unit) []UnitStatus {
	out := make([]UnitStatus, 0, len(units))
	for _, u := range units {
		phase := u.Phase
		if phase == "" {
			phase = machine.PhasePending
		}
		out = append(out, UnitStatus{ID: u.ID, Phase: string(phase), Attempt: u.Attempt, ReworkCause: u.ReworkCause})
	}
	return out
}

// requiredEvidence lists evidence for the selected next command only.
func requiredEvidence(selected machine.Command, units []machine.Unit) []string {
	switch selected {
	case machine.CmdVerify:
		return []string{"verify.command", "verify.exit_code"}
	case machine.CmdReview:
		return []string{"review.reviewer", "review.verdict", "review.artifact_ref"}
	case machine.CmdDeliver:
		return []string{"deliver.delivery_mode", "deliver.pr_url|deliver.commit_sha"}
	case machine.CmdObserve:
		if len(deliveredUnits(units)) > 1 {
			return []string{"observe.signal", "observe.unit"}
		}
		return []string{"observe.signal"}
	case machine.CmdRetry:
		return []string{"retry.reason"}
	}
	return []string{}
}
