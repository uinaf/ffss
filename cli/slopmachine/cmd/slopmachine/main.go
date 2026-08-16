package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/mattn/go-isatty"
	"github.com/uinaf/ffsstack/cli/slopmachine/internal/buildinfo"
	"github.com/uinaf/ffsstack/cli/slopmachine/internal/machine"
	"github.com/uinaf/ffsstack/cli/slopmachine/internal/repo"
	"github.com/uinaf/ffsstack/cli/slopmachine/internal/serve"
	"github.com/uinaf/ffsstack/cli/slopmachine/internal/status"
	"github.com/uinaf/ffsstack/cli/slopmachine/internal/store"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	cleaned, opts, err := parseRunOptions(args)
	if err != nil {
		return writeFailure(opts, 2, err)
	}
	return runWithOptions(cleaned, opts)
}

func runWithOptions(args []string, opts runOptions) int {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		if opts.dryRun {
			return writeFailure(opts, 2, fmt.Errorf("--dry-run requires a mutating command"))
		}
		if opts.json {
			doc, err := schemaDocument("")
			if err != nil {
				return writeFailure(opts, 10, err)
			}
			if err := writeJSON(doc); err != nil {
				return writeFailure(opts, 10, err)
			}
			return 0
		}
		fmt.Fprint(os.Stdout, usage())
		return 0
	}
	if len(args) == 2 && (args[1] == "-h" || args[1] == "--help") {
		if opts.dryRun {
			return writeFailure(opts, 2, fmt.Errorf("--dry-run requires a mutating command"))
		}
		if opts.json {
			doc, err := schemaDocument(args[0])
			if err != nil {
				return writeFailure(opts, 2, err)
			}
			if err := writeJSON(doc); err != nil {
				return writeFailure(opts, 10, err)
			}
			return 0
		}
		help, ok := commandUsage(args[0])
		if !ok {
			return writeFailure(opts, 2, fmt.Errorf("unknown command %q", args[0]))
		}
		fmt.Fprint(os.Stdout, help)
		return 0
	}
	if args[0] == "version" || args[0] == "--version" {
		if opts.dryRun {
			return writeFailure(opts, 2, fmt.Errorf("--dry-run requires a mutating command"))
		}
		if _, code := requireFlags("version", args[1:], opts); code != 0 {
			return code
		}
		if opts.json {
			if err := writeJSON(map[string]any{"schema_version": 1, "version": buildinfo.Version()}); err != nil {
				return writeFailure(opts, 10, err)
			}
			return 0
		}
		fmt.Fprintln(os.Stdout, buildinfo.Version())
		return 0
	}
	if args[0] == "schema" {
		if opts.dryRun {
			return writeFailure(opts, 2, fmt.Errorf("--dry-run requires a mutating command"))
		}
		return cmdSchema(args[1:], opts)
	}
	if args[0] == "storage" {
		if opts.dryRun {
			return writeFailure(opts, 2, fmt.Errorf("--dry-run requires a mutating command"))
		}
		return cmdStorage(args[1:], opts)
	}
	if opts.dryRun && !isMutatingCommand(args[0]) {
		return writeFailure(opts, 2, fmt.Errorf("--dry-run requires a mutating command"))
	}

	st, err := openStoreForCommand(args[0], opts)
	if err != nil {
		if errors.Is(err, errUnsafeStatePath) || errors.Is(err, errInvalidStateConfig) {
			return writeFailure(opts, 2, err)
		}
		if errors.Is(err, store.ErrStateUnavailable) {
			return writeFailure(opts, 2, fmt.Errorf("%w; set SLOPMACHINE_DB to a writable path and inspect resolution with slopmachine storage --json", err))
		}
		return mapErr(err, opts)
	}
	if st != nil {
		defer st.Close()
	}

	cmd := args[0]
	rest := args[1:]
	switch cmd {
	case "init":
		return cmdInit(st, rest, opts)
	case "intake":
		return cmdIntake(st, rest, opts)
	case "release":
		return cmdRelease(st, rest, opts)
	case "build":
		return cmdBuild(st, rest, opts)
	case "verify":
		return cmdVerify(st, rest, opts)
	case "review":
		return cmdReview(st, rest, opts)
	case "rework":
		return cmdRework(st, rest, opts)
	case "deliver":
		return cmdDeliver(st, rest, opts)
	case "observe":
		return cmdObserve(st, rest, opts)
	case "ask":
		return cmdAsk(st, rest, opts)
	case "decide":
		return cmdDecide(st, rest, opts)
	case "retry":
		return cmdRetry(st, rest, opts)
	case "block":
		return cmdBlock(st, rest, opts)
	case "status":
		return cmdStatus(st, rest, opts)
	case "reviewers":
		return cmdReviewers(st, rest, opts)
	case "watch":
		return cmdWatch(st, rest, opts)
	case "repo":
		return cmdRepo(st, rest, opts)
	case "serve":
		return cmdServe(st, rest, opts)
	default:
		return writeFailure(opts, 2, fmt.Errorf("unknown command %q", cmd))
	}
}

func isMutatingCommand(command string) bool {
	for _, spec := range allCommandSchemas() {
		if spec.Name == command {
			return spec.Mutating
		}
	}
	return false
}

func usage() string {
	return `slopmachine — deterministic structured slop cannoning

Usage:
  slopmachine init [--run ID]
  slopmachine intake --file PATH|- [--run ID]
  slopmachine release --revision N [--run ID]
  slopmachine build [--run ID]
  slopmachine verify --cmd CMD | --evidence PATH|- [--run ID]
  slopmachine review --evidence PATH|- [--unverified --reason TEXT] [--run ID]
  slopmachine rework [--run ID]
  slopmachine deliver --evidence PATH|- [--unverified --reason TEXT] [--run ID]
  slopmachine observe --signal SIGNAL [--unit ID] [--reference URL] [--run ID]
  slopmachine ask --question TEXT [--run ID]
  slopmachine decide --answer TEXT [--run ID]
  slopmachine retry --reason TEXT [--run ID]
  slopmachine block --reason TEXT [--run ID]
  slopmachine status [--json] [--run ID]
  slopmachine reviewers [--add NAME | --remove NAME] [--json]
  slopmachine watch [--once | --interval SECONDS [--iterations N]] [--run ID]
  slopmachine repo [show|register|update|unregister] [flags] [--json]
  slopmachine schema [--command NAME] [--json]
  slopmachine storage [--json]
  slopmachine serve [--addr 127.0.0.1:7780]
  slopmachine version

All mutating commands accept --input PATH and --dry-run. Run transitions
also accept --telemetry PATH|- recording duration, tokens, cost, and route.
Use --json before or after a command for structured success and error output.
`
}

func commandUsage(command string) (string, bool) {
	usage := map[string]string{
		"init": `Usage: slopmachine init [--run ID]

Create a run for the current Git repository. The next action is intake.
`,
		"intake": `Usage: slopmachine intake --file PATH [--run ID]

Load the released-work contract from JSON. Use --file - to read stdin.
Fields: delivery_mode, required_reviewers, risk_tier, budget, series_bound,
units (id, title, blockers, acceptance_criteria, complexity).
`,
		"release": `Usage: slopmachine release --revision N [--run ID]

Human approval latch for the exact intake_revision shown by status.
`,
		"build": `Usage: slopmachine build [--run ID]

Claim the next ready unit, or restart the current unit after rework.
`,
		"verify": `Usage: slopmachine verify (--cmd CMD | --evidence PATH) [--run ID]

Run verification or load strict JSON evidence. Use --evidence - for stdin.
A failed command is recorded as BLOCKED and exits with code 6.
`,
		"review": `Usage: slopmachine review --evidence PATH [--unverified --reason TEXT] [--run ID]

Record strict JSON review evidence. Use --evidence - for stdin.
Verdicts: clean, findings, ambiguous. The reviewer must be one of the run's
required registered identities; inspect the registry with slopmachine reviewers.

When the repo profile binds a forge and maps this reviewer with
--forge-reviewer, artifact_ref must be a change-request URL and the forge
must show a submitted review by that login; otherwise the evidence fails
closed (exit 3; exit 7 with error_kind observation_* when the forge is
unreachable). --unverified --reason TEXT records an explicit bypass in the
evidence. Other reviewers keep recorded-input behavior.
`,
		"rework": `Usage: slopmachine rework [--run ID]

Return the current unit from REVIEW to the build loop.
`,
		"deliver": `Usage: slopmachine deliver --evidence PATH [--unverified --reason TEXT] [--run ID]

Record strict JSON delivery evidence. Use --evidence - for stdin.

When the repo profile binds a forge and the run delivers change requests,
the evidence is verified against the live change request: it must exist
(exit 3 when not found) and match the recorded commit_sha (exit 3 on a head
mismatch); a verified delivery without commit_sha adopts the observed head.
Exit 7 with error_kind observation_* means the forge was unreachable and the
evidence was not accepted. --unverified --reason TEXT records an explicit
bypass in the evidence. Direct-trunk deliveries stay recorded input.
`,
		"observe": `Usage: slopmachine observe --signal SIGNAL [--unit ID] [--reference URL] [--run ID]

Record one external signal about a delivered unit. Signals: merged closes
the unit; checks_failed, review_feedback, and head_moved return it to the
build loop with the cause recorded. Omit --unit when exactly one unit is
delivered. slopmachine watch records these signals from the forge itself.
`,
		"ask": `Usage: slopmachine ask --question TEXT [--run ID]

Park the run until a human decision is recorded.
`,
		"decide": `Usage: slopmachine decide --answer TEXT [--run ID]

Record the human answer and resume the parked state.
`,
		"retry": `Usage: slopmachine retry --reason TEXT [--run ID]

Record the recovery decision and return a blocked verification to BUILD.
`,
		"block": `Usage: slopmachine block --reason TEXT [--run ID]

Record why active work cannot continue. Resume with retry after recovery.
`,
		"status": `Usage: slopmachine status [--json] [--fields LIST] [--run ID]

Show the current state and an actionable next command.
`,
		"schema": `Usage: slopmachine schema [--command NAME] [--json]

Describe commands, flags, strict input schemas, enums, and outputs as JSON.
`,
		"reviewers": `Usage: slopmachine reviewers [--add NAME | --remove NAME] [--json]

List built-in and registered reviewer identities, or register/unregister a
custom identity. Registration is declarative and idempotent; built-ins
(slopguard, bugbot) cannot be changed. Humans hold release and recovery
latches; a human sign-off reviewer must be registered explicitly.
`,
		"watch": `Usage: slopmachine watch [--once | --interval SECONDS [--iterations N]] [--run ID]

Observe every delivered unit's change request on the forge and record the
signals as observe events: merged settles the unit; failed checks, new
review feedback, or a moved head return it to the build loop with the
cause recorded. Unchanged forge state records nothing, so passes are
idempotent. --once (the default) runs one pass; --interval polls with a
bounded iteration count (default 20) and stops early once nothing awaits
signals. Interrupting with Ctrl-C is safe. Exit 7 means the final pass
left something unobserved or unrecorded; the emitted watch document
still reports every observation already recorded plus error_kind:
auth/rate_limit (fix gh access and rerun), transient (rerun),
not_found (the change request is gone; ask or re-deliver), unobservable
(delivery evidence unusable; record manually with slopmachine observe),
conflict (concurrent writers or a fresh re-delivery; rerun watch).
`,
		"repo": `Usage: slopmachine repo [show|register|update|unregister] [flags] [--json]

Show or declare this repository's profile: role bindings plus policy.
Flags for register and update:
  --forge github            forge kind hosting change requests
  --trust low|medium|high   earned autonomy tier
  --verify-cmd CMD          canonical verification command
  --delivery MODE           default delivery mode for new runs
  --readiness ready|not_ready   recorded agent-readiness verdict
  --bind 'role=name,...'    replace role bindings (roles: review, qa,
                            venue, memory); review bindings must be
                            registered reviewer identities
  --forge-reviewer 'identity=login,...'   replace forge-resident reviewer
                            mappings; a mapped reviewer's evidence is
                            corroborated against the forge (requires --forge)
A registered repo fails closed: releases require every required reviewer
to hold a review binding; a forge-bound profile verifies deliver evidence
and corroborates mapped reviewers. unregister restores profile-less behavior.
`,
		"storage": `Usage: slopmachine storage [--json]

Show the resolved database path, source, scope, existence, and Git safety.
`,
		"serve": `Usage: slopmachine serve [--addr 127.0.0.1:7780]

Serve the read-only run projector on a loopback address.
`,
		"version": `Usage: slopmachine version

Print the CLI version and source revision.
`,
	}
	help, ok := usage[command]
	if ok && isMutatingCommand(command) {
		help += "\nAgent-safe input: --input PATH accepts strict JSON; --dry-run validates without applying; --json returns structured output.\n"
	}
	return help, ok
}

func openStore() (*store.Store, error) {
	path, err := databasePath()
	if err != nil {
		return nil, err
	}
	return store.Open(path)
}

func openStoreForCommand(command string, opts runOptions) (*store.Store, error) {
	if !opts.dryRun {
		return openStore()
	}
	path, err := databasePath()
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// init and repo do not require a run; their dry runs project
			// against empty state, matching what the real command creates.
			if command != "init" && command != "repo" {
				return nil, fmt.Errorf("%w: canonical state does not exist at %q; run slopmachine init first", machine.ErrNotFound, path)
			}
			return nil, nil
		}
		return nil, fmt.Errorf("inspect database %q: %w", path, err)
	}
	return store.OpenReadOnly(path)
}

func databasePath() (string, error) {
	doc, err := resolveStorage(true)
	if err != nil {
		if errors.Is(err, errUnsafeStatePath) || errors.Is(err, errInvalidStateConfig) {
			return "", err
		}
		// Resolution failures outside the named config kinds are environment
		// problems (unreadable ancestors, non-directory parents); classify
		// them as recoverable state unavailability, not internal defects.
		return "", fmt.Errorf("%w: %w", store.ErrStateUnavailable, err)
	}
	return doc.Path, nil
}

func cmdInit(st *store.Store, args []string, opts runOptions) int {
	fs, code := requireFlags("init", args, opts)
	if code != 0 {
		return code
	}
	var input initInput
	raw, err := decodeMutationInput(fs, &input)
	if err != nil {
		return writeFailure(opts, 2, err)
	}
	runID := fs["run"]
	if raw {
		runID = input.Run
	}
	if runID == "" {
		id, err := randomID()
		if err != nil {
			return writeFailure(opts, 10, fmt.Errorf("generate run id: %w", err))
		}
		runID = "run-" + id
	}
	if err := validateRunID(runID); err != nil {
		return writeFailure(opts, 2, err)
	}
	key, err := resolveRepoKeyForOptions(st, opts)
	if err != nil {
		return writeFailure(opts, 2, err)
	}
	tel, telErr := telemetryInput(fs, input.Telemetry)
	if telErr != nil {
		return writeFailure(opts, 2, telErr)
	}
	run := machine.NewRun(runID, key)
	if st != nil {
		// A registered repo's delivery policy is the default contract; intake
		// may still override it explicitly.
		profile, found, err := st.GetRepoProfile(key)
		if err != nil {
			return mapErr(err, opts)
		}
		if found && profile.DeliveryMode != "" {
			run.DeliveryMode = profile.DeliveryMode
		}
	}
	if opts.dryRun {
		if st == nil {
			// A fresh installation cannot hold a profile, so its evidence
			// mode is recorded — stated like every other active document.
			fresh := status.Context{EvidenceVerification: string(machine.VerificationRecorded)}
			doc := status.FromContext(run, nil, contextWithTelemetry(fresh, tel))
			doc.DryRun = true
			doc.ValidatedCommand = string(machine.CmdInit)
			return writeStatus(doc, opts)
		}
		if _, _, err := st.GetRun(runID); err == nil {
			return writeFailure(opts, 2, fmt.Errorf("%w: run id %q already exists", machine.ErrRunExists, runID))
		} else if !errors.Is(err, machine.ErrNotFound) {
			return mapErr(err, opts)
		}
		ctx, err := statusContext(st, key, "")
		if err != nil {
			return mapErr(err, opts)
		}
		doc := status.FromContext(run, nil, contextWithTelemetry(ctx, tel))
		doc.DryRun = true
		doc.ValidatedCommand = string(machine.CmdInit)
		return writeStatus(doc, opts)
	}
	if err := st.CreateRun(run, nil, tel); err != nil {
		return mapErr(err, opts)
	}
	return printStatus(st, key, runID, opts)
}

type intakeUnitDTO struct {
	ID                 string   `json:"id"`
	Title              string   `json:"title"`
	Blockers           []string `json:"blockers"`
	AcceptanceCriteria []string `json:"acceptance_criteria,omitempty"`
	Complexity         *string  `json:"complexity,omitempty"`
}

// unitComplexity distinguishes an omitted complexity from an explicitly
// empty one; only omission is valid.
func (u intakeUnitDTO) unitComplexity() (machine.Complexity, error) {
	if u.Complexity == nil {
		return "", nil
	}
	if *u.Complexity == "" {
		return "", fmt.Errorf("unit %q complexity must be low|medium|high when set; omit the field instead of passing an empty value", u.ID)
	}
	return machine.Complexity(*u.Complexity), nil
}

type budgetDTO struct {
	Tokens  *int `json:"tokens,omitempty"`
	Minutes *int `json:"minutes,omitempty"`
}

// toBudget rejects explicit non-positive dimensions at the boundary; an
// omitted dimension stays unbounded, matching the advertised schema minimum.
func (b budgetDTO) toBudget() (machine.Budget, error) {
	var budget machine.Budget
	if b.Tokens != nil {
		if *b.Tokens < 1 {
			return machine.Budget{}, fmt.Errorf("budget.tokens must be >= 1 when set; omit the field to leave it unbounded")
		}
		budget.Tokens = *b.Tokens
	}
	if b.Minutes != nil {
		if *b.Minutes < 1 {
			return machine.Budget{}, fmt.Errorf("budget.minutes must be >= 1 when set; omit the field to leave it unbounded")
		}
		budget.Minutes = *b.Minutes
	}
	return budget, nil
}

func cmdIntake(st *store.Store, args []string, opts runOptions) int {
	fs, code := requireFlags("intake", args, opts)
	if code != 0 {
		return code
	}
	var patch intakeInput
	raw, err := decodeMutationInput(fs, &patch)
	if err != nil {
		return writeFailure(opts, 2, err)
	}
	if !raw {
		file := fs["file"]
		if file == "" {
			return writeFailure(opts, 2, fmt.Errorf("intake requires --file or --input"))
		}
		var filePatch struct {
			DeliveryMode      *string         `json:"delivery_mode"`
			RequiredReviewers []string        `json:"required_reviewers"`
			RiskTier          *string         `json:"risk_tier"`
			Budget            *budgetDTO      `json:"budget"`
			SeriesBound       *int            `json:"series_bound"`
			Units             []intakeUnitDTO `json:"units"`
		}
		if err := readJSON(file, &filePatch); err != nil {
			return writeFailure(opts, 2, fmt.Errorf("invalid intake json: %w", err))
		}
		patch = intakeInput{
			DeliveryMode: filePatch.DeliveryMode, RequiredReviewers: filePatch.RequiredReviewers,
			RiskTier: filePatch.RiskTier, Budget: filePatch.Budget,
			SeriesBound: filePatch.SeriesBound, Units: filePatch.Units,
		}
	}
	if err := validateRunID(patch.Run); err != nil {
		return writeFailure(opts, 2, err)
	}
	ip := &machine.IntakePatch{SeriesBound: patch.SeriesBound}
	if patch.Units != nil {
		units := make([]machine.Unit, len(patch.Units))
		for i, u := range patch.Units {
			complexity, err := u.unitComplexity()
			if err != nil {
				return writeFailure(opts, 2, err)
			}
			units[i] = machine.Unit{
				ID: u.ID, Title: u.Title, Blockers: u.Blockers,
				AcceptanceCriteria: u.AcceptanceCriteria, Complexity: complexity,
			}
		}
		ip.Units = units
	}
	if patch.RiskTier != nil {
		tier := machine.RiskTier(*patch.RiskTier)
		ip.RiskTier = &tier
	}
	if patch.Budget != nil {
		budget, err := patch.Budget.toBudget()
		if err != nil {
			return writeFailure(opts, 2, err)
		}
		ip.Budget = &budget
	}
	if patch.DeliveryMode != nil {
		m := machine.DeliveryMode(*patch.DeliveryMode)
		ip.DeliveryMode = &m
	}
	if patch.RequiredReviewers != nil {
		reviewers := make([]machine.ReviewerIdentity, len(patch.RequiredReviewers))
		for i, reviewer := range patch.RequiredReviewers {
			reviewers[i] = machine.ReviewerIdentity(reviewer)
		}
		ip.RequiredReviewers = reviewers
	}
	runID := fs["run"]
	if raw {
		runID = patch.Run
	}
	tel, telErr := telemetryInput(fs, patch.Telemetry)
	if telErr != nil {
		return writeFailure(opts, 2, telErr)
	}
	return applyCmd(st, runID, machine.CmdIntake, machine.ApplyInput{Intake: ip, Telemetry: tel}, opts)
}

func cmdRelease(st *store.Store, args []string, opts runOptions) int {
	fs, code := requireFlags("release", args, opts)
	if code != 0 {
		return code
	}
	var input releaseInput
	raw, err := decodeMutationInput(fs, &input)
	if err != nil {
		return writeFailure(opts, 2, err)
	}
	var revision int64
	runID := fs["run"]
	if raw {
		if input.Revision == nil {
			return writeFailure(opts, 2, fmt.Errorf("release input requires revision"))
		}
		revision = *input.Revision
		runID = input.Run
	} else {
		revS := fs["revision"]
		if revS == "" {
			return writeFailure(opts, 2, fmt.Errorf("release requires --revision or --input"))
		}
		revision, err = strconv.ParseInt(revS, 10, 64)
		if err != nil {
			return writeFailure(opts, 2, fmt.Errorf("bad --revision: %w", err))
		}
	}
	if err := validateRunID(runID); err != nil {
		return writeFailure(opts, 2, err)
	}
	tel, telErr := telemetryInput(fs, input.Telemetry)
	if telErr != nil {
		return writeFailure(opts, 2, telErr)
	}
	return applyCmd(st, runID, machine.CmdRelease, machine.ApplyInput{IntakeRevision: revision, Telemetry: tel}, opts)
}

func cmdBuild(st *store.Store, args []string, opts runOptions) int {
	fs, code := requireFlags("build", args, opts)
	if code != 0 {
		return code
	}
	var input runInput
	raw, err := decodeMutationInput(fs, &input)
	if err != nil {
		return writeFailure(opts, 2, err)
	}
	runID := fs["run"]
	if raw {
		runID = input.Run
	}
	if err := validateRunID(runID); err != nil {
		return writeFailure(opts, 2, err)
	}
	tel, telErr := telemetryInput(fs, input.Telemetry)
	if telErr != nil {
		return writeFailure(opts, 2, telErr)
	}
	return applyCmd(st, runID, machine.CmdBuild, machine.ApplyInput{Telemetry: tel}, opts)
}

func cmdVerify(st *store.Store, args []string, opts runOptions) int {
	fs, code := requireFlags("verify", args, opts)
	if code != 0 {
		return code
	}
	var ev machine.VerifyEvidence
	var rawInput verifyInput
	raw, err := decodeMutationInput(fs, &rawInput)
	if err != nil {
		return writeFailure(opts, 2, err)
	}
	runID := fs["run"]
	if raw {
		if rawInput.ExitCode == nil {
			return writeFailure(opts, 2, fmt.Errorf("verify input requires exit_code"))
		}
		runID = rawInput.Run
		ev = machine.VerifyEvidence{Command: rawInput.Command, ExitCode: *rawInput.ExitCode, OutputDigest: rawInput.OutputDigest}
	} else {
		if fs["evidence"] != "" && fs["cmd"] != "" {
			return writeFailure(opts, 2, fmt.Errorf("verify accepts exactly one of --cmd, --evidence, or --input"))
		}
		if fs["evidence"] == "" && fs["cmd"] == "" {
			return writeFailure(opts, 2, fmt.Errorf("verify requires --cmd, --evidence, or --input"))
		}
	}
	if !raw && fs["evidence"] != "" {
		f := fs["evidence"]
		var dto struct {
			Command      string `json:"command"`
			ExitCode     *int   `json:"exit_code"`
			OutputDigest string `json:"output_digest,omitempty"`
		}
		if err := readJSON(f, &dto); err != nil {
			return writeFailure(opts, 2, err)
		}
		if dto.ExitCode == nil {
			return writeFailure(opts, 2, fmt.Errorf("verify.exit_code is required"))
		}
		ev = machine.VerifyEvidence{Command: dto.Command, ExitCode: *dto.ExitCode, OutputDigest: dto.OutputDigest}
	}
	if err := validateRunID(runID); err != nil {
		return writeFailure(opts, 2, err)
	}
	tel, telErr := telemetryInput(fs, rawInput.Telemetry)
	if telErr != nil {
		return writeFailure(opts, 2, telErr)
	}
	prepared, code := prepareApply(st, runID, machine.CmdVerify, opts)
	if code != 0 {
		return code
	}
	// Recorded verify evidence is a self-attestation; at low trust only the
	// machine-executed gate counts.
	if raw || fs["evidence"] != "" {
		profile, found, err := st.GetRepoProfile(prepared.repoKey)
		if err != nil {
			return mapErr(err, opts)
		}
		if found && profile.TrustTier == machine.TrustLow {
			return writeFailure(opts, 3, fmt.Errorf("%w: trust tier low accepts only machine-executed verification; run the gate through verify --cmd (recorded verify evidence requires medium or high trust)", machine.ErrUnmetGuard))
		}
	}
	if c := fs["cmd"]; !raw && c != "" {
		if opts.dryRun {
			ctx, err := statusContext(st, prepared.repoKey, prepared.run.ID)
			if err != nil {
				return mapErr(err, opts)
			}
			// Caller-supplied dimensions project like a real run; the
			// measured duration stays undetermined without execution, and a
			// caller-supplied duration never overrides it — clear it so the
			// projection matches the real command's merge. The real command
			// always records one event (measured duration), so the count
			// projects even when nothing else is known.
			projected := tel
			if projected != nil && projected.DurationMS != 0 {
				copied := *projected
				copied.DurationMS = 0
				projected = &copied
			}
			ctx = contextWithTelemetry(ctx, projected)
			if projected == nil || projected.IsZero() {
				ctx.TelemetryEvents++
			}
			doc := status.FromContext(prepared.run, prepared.units, ctx)
			doc.DryRun = true
			doc.ValidatedCommand = string(machine.CmdVerify)
			doc.OutcomeUndetermined = true
			return writeStatus(doc, opts)
		}
		started := time.Now()
		ctx, cancel := context.WithCancel(context.Background())
		signals := make(chan os.Signal, 1)
		caught := make(chan os.Signal, 1)
		signalDone := make(chan struct{})
		signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
		go func() {
			defer close(signalDone)
			select {
			case sig := <-signals:
				caught <- sig
				cancel()
			case <-ctx.Done():
			}
		}()
		code, outputDigest, err := runShell(ctx, c, opts.json)
		signal.Stop(signals)
		cancel()
		<-signalDone
		var received os.Signal
		select {
		case received = <-caught:
		default:
			select {
			case received = <-signals:
			default:
			}
		}
		if received != nil && err == nil {
			err = errVerificationCommandCancelled
		}
		if err != nil {
			cancelCode := 130
			if unixSignal, ok := received.(syscall.Signal); ok {
				cancelCode = 128 + int(unixSignal)
			}
			return writeFailure(opts, cancelCode, err)
		}
		ev = machine.VerifyEvidence{Command: c, ExitCode: code, OutputDigest: outputDigest}
		// verify measures its own wall clock; caller telemetry keeps its
		// other dimensions but never overrides the measured duration.
		if tel == nil {
			tel = &machine.Telemetry{}
		}
		tel.DurationMS = max(time.Since(started).Milliseconds(), 1)
	}
	_, code = applyPrepared(st, prepared, machine.CmdVerify, machine.ApplyInput{Verify: &ev, Telemetry: tel}, opts)
	if code != 0 {
		return code
	}
	if ev.ExitCode != 0 {
		return 6
	}
	return 0
}

func cmdReview(st *store.Store, args []string, opts runOptions) int {
	fs, code := requireFlags("review", args, opts)
	if code != 0 {
		return code
	}
	var ev machine.ReviewEvidence
	var input reviewInput
	var override evidenceOverride
	raw, err := decodeMutationInput(fs, &input)
	if err != nil {
		return writeFailure(opts, 2, err)
	}
	runID := fs["run"]
	if raw {
		runID = input.Run
		ev = machine.ReviewEvidence{
			Reviewer: machine.ReviewerIdentity(input.Reviewer), Verdict: machine.ReviewVerdict(input.Verdict), ArtifactRef: input.ArtifactRef,
		}
		override, err = overrideFromInput(input.Unverified, input.UnverifiedReason)
		if err != nil {
			return writeFailure(opts, 2, err)
		}
	} else {
		if fs["evidence"] == "" {
			return writeFailure(opts, 2, fmt.Errorf("review requires --evidence or --input"))
		}
		// Verification is machine-stamped; field presence, not value,
		// decides rejection.
		var dto struct {
			Reviewer         string  `json:"reviewer,omitempty"`
			Verdict          string  `json:"verdict,omitempty"`
			ArtifactRef      string  `json:"artifact_ref,omitempty"`
			Verification     *string `json:"verification,omitempty"`
			UnverifiedReason *string `json:"unverified_reason,omitempty"`
		}
		if err := readJSON(fs["evidence"], &dto); err != nil {
			return writeFailure(opts, 2, err)
		}
		if dto.Verification != nil || dto.UnverifiedReason != nil {
			return writeFailure(opts, 2, fmt.Errorf("review evidence must not declare verification; the machine stamps it (use --unverified --reason to bypass)"))
		}
		ev = machine.ReviewEvidence{
			Reviewer: machine.ReviewerIdentity(dto.Reviewer), Verdict: machine.ReviewVerdict(dto.Verdict), ArtifactRef: dto.ArtifactRef,
		}
		override, err = overrideFromFlags(fs)
		if err != nil {
			return writeFailure(opts, 2, err)
		}
	}
	if err := validateRunID(runID); err != nil {
		return writeFailure(opts, 2, err)
	}
	tel, telErr := telemetryInput(fs, input.Telemetry)
	if telErr != nil {
		return writeFailure(opts, 2, telErr)
	}
	prepared, code := prepareApply(st, runID, machine.CmdReview, opts)
	if code != 0 {
		return code
	}
	in := machine.ApplyInput{Review: &ev, Telemetry: tel}
	if code := preflightApply(prepared, machine.CmdReview, in, opts); code != 0 {
		return code
	}
	profile, found, err := st.GetRepoProfile(prepared.repoKey)
	if err != nil {
		return mapErr(err, opts)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if code := resolveReviewVerification(ctx, &ev, profile, found, override, opts); code != 0 {
		return code
	}
	_, code = applyPrepared(st, prepared, machine.CmdReview, in, opts)
	return code
}

func cmdRework(st *store.Store, args []string, opts runOptions) int {
	fs, code := requireFlags("rework", args, opts)
	if code != 0 {
		return code
	}
	var input runInput
	raw, err := decodeMutationInput(fs, &input)
	if err != nil {
		return writeFailure(opts, 2, err)
	}
	runID := fs["run"]
	if raw {
		runID = input.Run
	}
	if err := validateRunID(runID); err != nil {
		return writeFailure(opts, 2, err)
	}
	tel, telErr := telemetryInput(fs, input.Telemetry)
	if telErr != nil {
		return writeFailure(opts, 2, telErr)
	}
	return applyCmd(st, runID, machine.CmdRework, machine.ApplyInput{Telemetry: tel}, opts)
}

func cmdDeliver(st *store.Store, args []string, opts runOptions) int {
	fs, code := requireFlags("deliver", args, opts)
	if code != 0 {
		return code
	}
	var ev machine.DeliverEvidence
	var input deliverInput
	var override evidenceOverride
	raw, err := decodeMutationInput(fs, &input)
	if err != nil {
		return writeFailure(opts, 2, err)
	}
	runID := fs["run"]
	if raw {
		runID = input.Run
		ev = machine.DeliverEvidence{
			DeliveryMode: machine.DeliveryMode(input.DeliveryMode), PRURL: input.PRURL, CommitSHA: input.CommitSHA,
		}
		override, err = overrideFromInput(input.Unverified, input.UnverifiedReason)
		if err != nil {
			return writeFailure(opts, 2, err)
		}
	} else {
		if fs["evidence"] == "" {
			return writeFailure(opts, 2, fmt.Errorf("deliver requires --evidence or --input"))
		}
		// Field presence, not value, decides rejection: "unit":"" is
		// caller-supplied too, and readJSON already rejects null values.
		var dto struct {
			Unit             *string `json:"unit,omitempty"`
			DeliveryMode     string  `json:"delivery_mode,omitempty"`
			PRURL            string  `json:"pr_url,omitempty"`
			CommitSHA        string  `json:"commit_sha,omitempty"`
			Verification     *string `json:"verification,omitempty"`
			UnverifiedReason *string `json:"unverified_reason,omitempty"`
		}
		if err := readJSON(fs["evidence"], &dto); err != nil {
			return writeFailure(opts, 2, err)
		}
		if dto.Unit != nil {
			return writeFailure(opts, 2, fmt.Errorf("deliver evidence must not name a unit; the machine stamps the delivered unit itself"))
		}
		if dto.Verification != nil || dto.UnverifiedReason != nil {
			return writeFailure(opts, 2, fmt.Errorf("deliver evidence must not declare verification; the machine stamps it (use --unverified --reason to bypass)"))
		}
		ev = machine.DeliverEvidence{
			DeliveryMode: machine.DeliveryMode(dto.DeliveryMode), PRURL: dto.PRURL, CommitSHA: dto.CommitSHA,
		}
		override, err = overrideFromFlags(fs)
		if err != nil {
			return writeFailure(opts, 2, err)
		}
	}
	if err := validateRunID(runID); err != nil {
		return writeFailure(opts, 2, err)
	}
	tel, telErr := telemetryInput(fs, input.Telemetry)
	if telErr != nil {
		return writeFailure(opts, 2, telErr)
	}
	prepared, code := prepareApply(st, runID, machine.CmdDeliver, opts)
	if code != 0 {
		return code
	}
	in := machine.ApplyInput{Deliver: &ev, Telemetry: tel}
	if code := preflightApply(prepared, machine.CmdDeliver, in, opts); code != 0 {
		return code
	}
	profile, found, err := st.GetRepoProfile(prepared.repoKey)
	if err != nil {
		return mapErr(err, opts)
	}
	// The evidence may omit delivery_mode; verification judges against the
	// run's effective mode, exactly as the machine will.
	mode := ev.DeliveryMode
	if mode == "" {
		mode = prepared.run.DeliveryMode
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if code := resolveDeliverVerification(ctx, &ev, profile, found, mode, override, opts); code != 0 {
		return code
	}
	_, code = applyPrepared(st, prepared, machine.CmdDeliver, in, opts)
	return code
}

func cmdObserve(st *store.Store, args []string, opts runOptions) int {
	fs, code := requireFlags("observe", args, opts)
	if code != 0 {
		return code
	}
	var input observeInput
	raw, err := decodeMutationInput(fs, &input)
	if err != nil {
		return writeFailure(opts, 2, err)
	}
	runID := fs["run"]
	if raw {
		runID = input.Run
	} else {
		input.Unit = fs["unit"]
		input.Signal = fs["signal"]
		input.Reference = fs["reference"]
	}
	if input.Signal == "" {
		return writeFailure(opts, 2, fmt.Errorf("observe requires --signal or --input"))
	}
	if err := validateRunID(runID); err != nil {
		return writeFailure(opts, 2, err)
	}
	if input.Unit != "" {
		if err := machine.ValidateResourceID("unit id", input.Unit); err != nil {
			return writeFailure(opts, 2, err)
		}
	}
	evidence := machine.ObserveEvidence{
		UnitID: input.Unit, Signal: machine.ObserveSignal(input.Signal), Reference: input.Reference,
	}
	tel, telErr := telemetryInput(fs, input.Telemetry)
	if telErr != nil {
		return writeFailure(opts, 2, telErr)
	}
	return applyCmd(st, runID, machine.CmdObserve, machine.ApplyInput{Observe: &evidence, Telemetry: tel}, opts)
}

func cmdAsk(st *store.Store, args []string, opts runOptions) int {
	fs, code := requireFlags("ask", args, opts)
	if code != 0 {
		return code
	}
	var input questionInput
	raw, err := decodeMutationInput(fs, &input)
	if err != nil {
		return writeFailure(opts, 2, err)
	}
	runID, question := fs["run"], fs["question"]
	if raw {
		runID, question = input.Run, input.Question
	}
	if question == "" {
		return writeFailure(opts, 2, fmt.Errorf("ask requires question"))
	}
	if err := validateRunID(runID); err != nil {
		return writeFailure(opts, 2, err)
	}
	tel, telErr := telemetryInput(fs, input.Telemetry)
	if telErr != nil {
		return writeFailure(opts, 2, telErr)
	}
	return applyCmd(st, runID, machine.CmdAsk, machine.ApplyInput{Question: question, Telemetry: tel}, opts)
}

func cmdDecide(st *store.Store, args []string, opts runOptions) int {
	fs, code := requireFlags("decide", args, opts)
	if code != 0 {
		return code
	}
	var input answerInput
	raw, err := decodeMutationInput(fs, &input)
	if err != nil {
		return writeFailure(opts, 2, err)
	}
	runID, answer := fs["run"], fs["answer"]
	if raw {
		runID, answer = input.Run, input.Answer
	}
	if answer == "" {
		return writeFailure(opts, 2, fmt.Errorf("decide requires answer"))
	}
	if err := validateRunID(runID); err != nil {
		return writeFailure(opts, 2, err)
	}
	tel, telErr := telemetryInput(fs, input.Telemetry)
	if telErr != nil {
		return writeFailure(opts, 2, telErr)
	}
	return applyCmd(st, runID, machine.CmdDecide, machine.ApplyInput{Decision: &machine.Decision{Answer: answer}, Telemetry: tel}, opts)
}

func cmdRetry(st *store.Store, args []string, opts runOptions) int {
	fs, code := requireFlags("retry", args, opts)
	if code != 0 {
		return code
	}
	var input reasonInput
	raw, err := decodeMutationInput(fs, &input)
	if err != nil {
		return writeFailure(opts, 2, err)
	}
	runID, reason := fs["run"], fs["reason"]
	if raw {
		runID, reason = input.Run, input.Reason
	}
	if strings.TrimSpace(reason) == "" {
		return writeFailure(opts, 2, fmt.Errorf("retry requires reason"))
	}
	if err := validateRunID(runID); err != nil {
		return writeFailure(opts, 2, err)
	}
	tel, telErr := telemetryInput(fs, input.Telemetry)
	if telErr != nil {
		return writeFailure(opts, 2, telErr)
	}
	return applyCmd(st, runID, machine.CmdRetry, machine.ApplyInput{RetryReason: reason, Telemetry: tel}, opts)
}

func cmdBlock(st *store.Store, args []string, opts runOptions) int {
	fs, code := requireFlags("block", args, opts)
	if code != 0 {
		return code
	}
	var input reasonInput
	raw, err := decodeMutationInput(fs, &input)
	if err != nil {
		return writeFailure(opts, 2, err)
	}
	runID, reason := fs["run"], fs["reason"]
	if raw {
		runID, reason = input.Run, input.Reason
	}
	if strings.TrimSpace(reason) == "" {
		return writeFailure(opts, 2, fmt.Errorf("block requires reason"))
	}
	if err := validateRunID(runID); err != nil {
		return writeFailure(opts, 2, err)
	}
	tel, telErr := telemetryInput(fs, input.Telemetry)
	if telErr != nil {
		return writeFailure(opts, 2, telErr)
	}
	return applyCmd(st, runID, machine.CmdBlock, machine.ApplyInput{BlockReason: reason, Telemetry: tel}, opts)
}

func cmdStatus(st *store.Store, args []string, opts runOptions) int {
	fs, code := requireFlags("status", args, opts)
	if code != 0 {
		return code
	}
	if fs["fields"] != "" {
		if !opts.json {
			return writeFailure(opts, 2, fmt.Errorf("--fields requires --json"))
		}
		fields, err := parseFields(fs["fields"])
		if err != nil {
			return writeFailure(opts, 2, err)
		}
		opts.fields = fields
	}
	if err := validateRunID(fs["run"]); err != nil {
		return writeFailure(opts, 2, err)
	}
	key, err := resolveRepoKeyForOptions(st, opts)
	if err != nil {
		return writeFailure(opts, 2, err)
	}
	return printStatus(st, key, fs["run"], opts)
}

type preparedApply struct {
	repoKey string
	run     machine.Run
	units   []machine.Unit
}

func prepareApply(st *store.Store, runID string, cmd machine.Command, opts runOptions) (preparedApply, int) {
	if err := validateRunID(runID); err != nil {
		return preparedApply{}, writeFailure(opts, 2, err)
	}
	key, err := resolveRepoKeyForOptions(st, opts)
	if err != nil {
		return preparedApply{}, writeFailure(opts, 2, err)
	}
	run, units, err := st.ResolveActiveRun(key, runID)
	if err != nil {
		return preparedApply{}, mapErr(err, opts)
	}
	if !slices.Contains(machine.AllowedCommands(run, units), cmd) {
		return preparedApply{}, mapErr(fmt.Errorf("%w: %s not allowed from %s", machine.ErrIllegalTransition, cmd, run.State), opts)
	}
	return preparedApply{repoKey: key, run: run, units: units}, 0
}

func applyPrepared(st *store.Store, prepared preparedApply, cmd machine.Command, in machine.ApplyInput, opts runOptions) (machine.ApplyResult, int) {
	run := prepared.run
	in.ExpectedRevision = run.Revision
	if cmd == machine.CmdIntake || cmd == machine.CmdRelease {
		registered, err := st.ListReviewers()
		if err != nil {
			return machine.ApplyResult{}, mapErr(err, opts)
		}
		in.RegisteredReviewers = registered
		profile, found, err := st.GetRepoProfile(prepared.repoKey)
		if err != nil {
			return machine.ApplyResult{}, mapErr(err, opts)
		}
		if found {
			in.Profile = &profile
		}
	}
	res, err := machine.Apply(run, prepared.units, cmd, in)
	if err != nil {
		return machine.ApplyResult{}, mapErr(err, opts)
	}
	if opts.dryRun {
		ctx, err := statusContext(st, prepared.repoKey, prepared.run.ID)
		if err != nil {
			return machine.ApplyResult{}, mapErr(err, opts)
		}
		// The projection reports what the real command would leave behind,
		// including the proposed (unpersisted) telemetry in the totals.
		ctx = contextWithTelemetry(ctx, res.Telemetry)
		doc := status.FromContext(res.Run, res.Units, ctx)
		doc.DryRun = true
		doc.ValidatedCommand = string(cmd)
		return res, writeStatus(doc, opts)
	}
	if err := st.SaveApply(res); err != nil {
		return machine.ApplyResult{}, mapErr(err, opts)
	}
	return res, printStatus(st, prepared.repoKey, res.Run.ID, opts)
}

// preflightApply runs the pure state machine once and discards the result,
// so locally invalid evidence fails deterministically before any forge
// contact. The machine stamps defaults into the evidence pointers exactly as
// the final apply will, so the rehearsal never diverges from the real run.
func preflightApply(prepared preparedApply, cmd machine.Command, in machine.ApplyInput, opts runOptions) int {
	in.ExpectedRevision = prepared.run.Revision
	if _, err := machine.Apply(prepared.run, prepared.units, cmd, in); err != nil {
		return mapErr(err, opts)
	}
	return 0
}

func applyCmd(st *store.Store, runID string, cmd machine.Command, in machine.ApplyInput, opts runOptions) int {
	prepared, code := prepareApply(st, runID, cmd, opts)
	if code != 0 {
		return code
	}
	_, code = applyPrepared(st, prepared, cmd, in, opts)
	return code
}

// telemetryInput resolves recorded telemetry for a transition: the raw
// --input payload's telemetry object, or a strict JSON document from
// --telemetry. All-zero telemetry is rejected: omit it instead.
func telemetryInput(fs map[string]string, raw *telemetryDTO) (*machine.Telemetry, error) {
	dto := raw
	if dto == nil {
		path, present := fs["telemetry"]
		if !present {
			return nil, nil
		}
		if path == "" {
			return nil, fmt.Errorf("--telemetry requires a path or - for stdin")
		}
		// Two JSON sources cannot share one stdin stream.
		if path == "-" && (fs["file"] == "-" || fs["evidence"] == "-") {
			return nil, fmt.Errorf("only one JSON source may read stdin; pass --telemetry or the evidence file by path")
		}
		dto = &telemetryDTO{}
		if err := readJSON(path, dto); err != nil {
			return nil, fmt.Errorf("invalid telemetry json: %w", err)
		}
	}
	telemetry, err := dto.toTelemetry()
	if err != nil {
		return nil, err
	}
	if telemetry.IsZero() {
		return nil, fmt.Errorf("telemetry must record at least one dimension; omit it instead")
	}
	if err := machine.ValidateTelemetry(telemetry); err != nil {
		return nil, err
	}
	return telemetry, nil
}

func printStatus(st *store.Store, repoKey, runID string, opts runOptions) int {
	run, units, err := st.ResolveStatusRun(repoKey, runID)
	if err != nil {
		if runID == "" && errors.Is(err, machine.ErrNotFound) {
			return writeStatus(status.Bootstrap(repoKey), opts)
		}
		return mapErr(err, opts)
	}
	ctx, err := statusContext(st, repoKey, run.ID)
	if err != nil {
		return mapErr(err, opts)
	}
	return writeStatus(status.FromContext(run, units, ctx), opts)
}

// contextWithTelemetry projects proposed (unpersisted) telemetry onto the
// totals a dry run reports; persisted aggregation saturates the same way.
func contextWithTelemetry(ctx status.Context, telemetry *machine.Telemetry) status.Context {
	if telemetry == nil || telemetry.IsZero() {
		return ctx
	}
	ctx.TotalDurationMS = store.SaturatingAdd64(ctx.TotalDurationMS, telemetry.DurationMS)
	ctx.TotalTokens = store.SaturatingAddInt(ctx.TotalTokens, telemetry.Tokens)
	ctx.TotalCostCents = store.SaturatingAddInt(ctx.TotalCostCents, telemetry.CostCents)
	ctx.TelemetryEvents++
	return ctx
}

// statusContext loads the repo profile's read-time defaults, if registered,
// and the run's aggregated telemetry totals.
func statusContext(st *store.Store, repoKey, runID string) (status.Context, error) {
	var ctx status.Context
	profile, found, err := st.GetRepoProfile(repoKey)
	if err != nil {
		return status.Context{}, err
	}
	if found {
		ctx.RepoRegistered = true
		ctx.VerifyCommand = profile.VerifyCommand
	}
	// Stated plainly either way: a forge-bound profile makes deliver/review
	// evidence observed; everything else is trusted recorded input.
	if found && profile.ForgeKind != "" {
		ctx.EvidenceVerification = string(machine.VerificationObserved)
	} else {
		ctx.EvidenceVerification = string(machine.VerificationRecorded)
	}
	if runID != "" {
		totals, err := st.TelemetryTotals(runID)
		if err != nil {
			return status.Context{}, err
		}
		ctx.TotalDurationMS = totals.DurationMS
		ctx.TotalTokens = totals.Tokens
		ctx.TotalCostCents = totals.CostCents
		ctx.TelemetryEvents = totals.RecordedEvents
	}
	return ctx, nil
}

func writeStatus(doc status.Document, opts runOptions) int {
	if opts.json {
		if err := status.ValidateFields(opts.fields); err != nil {
			return writeFailure(opts, 2, err)
		}
		b, err := doc.JSONFields(opts.fields)
		if err != nil {
			return writeFailure(opts, 10, err)
		}
		fmt.Fprintln(os.Stdout, string(b))
		return 0
	}
	fmt.Fprintln(os.Stdout, doc.CompactLine())
	return 0
}

func mapErr(err error, opts runOptions) int {
	code := 10
	switch {
	case errors.Is(err, machine.ErrBadArgs), errors.Is(err, machine.ErrRunExists):
		code = 2
	case errors.Is(err, machine.ErrIllegalTransition), errors.Is(err, machine.ErrUnmetGuard):
		code = 3
	case errors.Is(err, machine.ErrRevisionConflict):
		code = 4
	case errors.Is(err, machine.ErrAmbiguousRun), errors.Is(err, machine.ErrNotFound), errors.Is(err, machine.ErrCorruptState):
		code = 5
	}
	return writeFailure(opts, code, err)
}

var commandFlags = map[string]map[string]bool{
	"init":      {"run": true, "input": true, "telemetry": true},
	"intake":    {"file": true, "run": true, "input": true, "telemetry": true},
	"release":   {"revision": true, "run": true, "input": true, "telemetry": true},
	"build":     {"run": true, "input": true, "telemetry": true},
	"verify":    {"cmd": true, "evidence": true, "run": true, "input": true, "telemetry": true},
	"review":    {"evidence": true, "run": true, "input": true, "telemetry": true, "unverified": true, "reason": true},
	"rework":    {"run": true, "input": true, "telemetry": true},
	"deliver":   {"evidence": true, "run": true, "input": true, "telemetry": true, "unverified": true, "reason": true},
	"observe":   {"signal": true, "unit": true, "reference": true, "run": true, "input": true, "telemetry": true},
	"ask":       {"question": true, "run": true, "input": true, "telemetry": true},
	"decide":    {"answer": true, "run": true, "input": true, "telemetry": true},
	"retry":     {"reason": true, "run": true, "input": true, "telemetry": true},
	"block":     {"reason": true, "run": true, "input": true, "telemetry": true},
	"status":    {"json": true, "run": true, "fields": true},
	"reviewers": {"add": true, "remove": true, "json": true},
	"watch":     {"once": true, "interval": true, "iterations": true, "run": true, "json": true},
	"repo":      {"forge": true, "trust": true, "verify-cmd": true, "delivery": true, "readiness": true, "bind": true, "forge-reviewer": true, "json": true},
	"schema":    {"json": true, "command": true},
	"storage":   {"json": true},
	"serve":     {"addr": true},
	"version":   {},
}

type reviewersDocument struct {
	SchemaVersion    int      `json:"schema_version"`
	Builtin          []string `json:"builtin"`
	Registered       []string `json:"registered"`
	DryRun           bool     `json:"dry_run,omitempty"`
	ValidatedCommand string   `json:"validated_command,omitempty"`
}

func cmdReviewers(st *store.Store, args []string, opts runOptions) int {
	fs, code := requireFlags("reviewers", args, opts)
	if code != 0 {
		return code
	}
	add, hasAdd := fs["add"]
	remove, hasRemove := fs["remove"]
	if hasAdd && hasRemove {
		return writeFailure(opts, 2, fmt.Errorf("--add cannot be combined with --remove"))
	}
	if opts.dryRun && !hasAdd && !hasRemove {
		return writeFailure(opts, 2, fmt.Errorf("--dry-run requires --add or --remove"))
	}
	name := add
	if hasRemove {
		name = remove
	}
	if hasAdd || hasRemove {
		if err := machine.ValidateResourceID("reviewer identity", name); err != nil {
			return writeFailure(opts, 2, err)
		}
		if slices.Contains(machine.BuiltinReviewers(), machine.ReviewerIdentity(name)) {
			return writeFailure(opts, 2, fmt.Errorf("reviewer %q is built-in and always registered", name))
		}
		// Retired names stay reserved so a custom identity can never shadow
		// historical evidence recorded under them.
		if renamed := machine.LegacyReviewerRename(machine.ReviewerIdentity(name)); renamed != "" {
			return writeFailure(opts, 2, fmt.Errorf("reviewer identity %q was renamed to %q", name, renamed))
		}
	}
	if !opts.dryRun {
		var err error
		if hasAdd {
			err = st.RegisterReviewer(machine.ReviewerIdentity(name))
		} else if hasRemove {
			err = st.UnregisterReviewer(machine.ReviewerIdentity(name))
		}
		if err != nil {
			return mapErr(err, opts)
		}
	}
	registered, err := st.ListReviewers()
	if err != nil {
		return mapErr(err, opts)
	}
	names := make([]string, 0, len(registered)+1)
	for _, reviewer := range registered {
		names = append(names, string(reviewer))
	}
	// A dry run projects the registry the mutation would leave behind.
	if opts.dryRun && hasAdd && !slices.Contains(names, name) {
		names = append(names, name)
	}
	if opts.dryRun && hasRemove {
		names = slices.DeleteFunc(names, func(candidate string) bool { return candidate == name })
	}
	builtin := make([]string, 0, len(machine.BuiltinReviewers()))
	for _, reviewer := range machine.BuiltinReviewers() {
		builtin = append(builtin, string(reviewer))
	}
	doc := reviewersDocument{SchemaVersion: 1, Builtin: builtin, Registered: names}
	if opts.dryRun {
		doc.DryRun = true
		doc.ValidatedCommand = "reviewers"
	}
	if opts.json {
		if err := writeJSON(doc); err != nil {
			return writeFailure(opts, 10, err)
		}
		return 0
	}
	registeredText := "(none)"
	if len(names) > 0 {
		registeredText = strings.Join(names, ",")
	}
	prefix := "slopmachine reviewers"
	if doc.DryRun {
		prefix += " dry-run"
	}
	fmt.Fprintf(os.Stdout, "%s builtin=%s registered=%s\n", prefix, strings.Join(builtin, ","), registeredText)
	return 0
}

func cmdSchema(args []string, opts runOptions) int {
	fs, code := requireFlags("schema", args, opts)
	if code != 0 {
		return code
	}
	doc, err := schemaDocument(fs["command"])
	if err != nil {
		return writeFailure(opts, 2, err)
	}
	if err := writeJSON(doc); err != nil {
		return writeFailure(opts, 10, err)
	}
	return 0
}

func cmdServe(st *store.Store, args []string, opts runOptions) int {
	fs, err := parseFlagsWith(args, commandFlags["serve"])
	if err != nil {
		return writeFailure(opts, 2, err)
	}
	key, err := resolveRepoKey(st)
	if err != nil {
		return writeFailure(opts, 2, err)
	}
	addr := fs["addr"]
	if addr == "" {
		addr = "127.0.0.1:7780"
	}
	srv, err := serve.New(serve.Options{Store: st, RepoKey: key, Addr: addr})
	if err != nil {
		return writeFailure(opts, 2, err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := srv.ListenAndServe(ctx); err != nil {
		return writeFailure(opts, 10, err)
	}
	return 0
}

func requireFlags(command string, args []string, opts runOptions) (map[string]string, int) {
	fs, err := parseFlagsWith(args, commandFlags[command])
	if err != nil {
		return nil, writeFailure(opts, 2, err)
	}
	return fs, 0
}

func parseFlagsWith(args []string, allowed map[string]bool) (map[string]string, error) {
	out := map[string]string{}
	for i := 0; i < len(args); i++ {
		a := args[i]
		if !strings.HasPrefix(a, "--") {
			return nil, fmt.Errorf("unexpected argument %q", a)
		}
		key := strings.TrimPrefix(a, "--")
		val := ""
		hasVal := false
		if eq := strings.IndexByte(key, '='); eq >= 0 {
			val = key[eq+1:]
			key = key[:eq]
			hasVal = true
		}
		if !allowed[key] {
			return nil, fmt.Errorf("unknown flag --%s", key)
		}
		// Last-value-wins would let an earlier invalid declaration escape
		// validation; every flag is declared exactly once, fail-closed.
		if _, dup := out[key]; dup {
			return nil, fmt.Errorf("flag --%s may be specified only once", key)
		}
		if key == "json" || key == "once" || key == "unverified" {
			if hasVal {
				return nil, fmt.Errorf("flag --%s does not accept a value", key)
			}
			out[key] = "1"
			continue
		}
		if !hasVal {
			if i+1 >= len(args) || strings.HasPrefix(args[i+1], "--") {
				return nil, fmt.Errorf("flag --%s requires a value", key)
			}
			val = args[i+1]
			i++
		}
		out[key] = val
	}
	return out, nil
}

func flagHas(args []string, name string) bool {
	for _, a := range args {
		if a == name || strings.HasPrefix(a, name+"=") {
			return true
		}
	}
	return false
}

func readJSON(path string, dest any) error {
	interactive := isatty.IsTerminal(os.Stdin.Fd()) || isatty.IsCygwinTerminal(os.Stdin.Fd())
	return readJSONFrom(path, dest, os.Stdin, interactive)
}

func readJSONFrom(path string, dest any, stdin io.Reader, stdinInteractive bool) error {
	var r io.Reader
	if path == "-" {
		if stdinInteractive {
			return fmt.Errorf("stdin is an interactive terminal; pipe or redirect JSON when using -, or inspect slopmachine schema for the command contract")
		}
		r = stdin
	} else {
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		r = bytes.NewReader(b)
	}
	decoder := json.NewDecoder(r)
	var raw json.RawMessage
	if err := decoder.Decode(&raw); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return fmt.Errorf("unexpected trailing JSON value")
		}
		return err
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return fmt.Errorf("JSON object required, got null")
	}
	if err := rejectDuplicateJSONKeys(raw); err != nil {
		return err
	}
	if err := validateExactJSONFields(raw, reflect.TypeOf(dest)); err != nil {
		return err
	}
	strict := json.NewDecoder(bytes.NewReader(raw))
	strict.DisallowUnknownFields()
	if err := strict.Decode(dest); err != nil {
		return err
	}
	return nil
}

func validateExactJSONFields(raw []byte, typ reflect.Type) error {
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return fmt.Errorf("null JSON value not allowed")
	}
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	switch typ.Kind() {
	case reflect.Struct:
		var object map[string]json.RawMessage
		if err := json.Unmarshal(raw, &object); err != nil {
			return err
		}
		fields := make(map[string]reflect.Type)
		for i := 0; i < typ.NumField(); i++ {
			field := typ.Field(i)
			if !field.IsExported() {
				continue
			}
			name := strings.Split(field.Tag.Get("json"), ",")[0]
			if name == "-" {
				continue
			}
			if name == "" {
				name = field.Name
			}
			fields[name] = field.Type
		}
		for name, value := range object {
			fieldType, ok := fields[name]
			if !ok {
				return fmt.Errorf("unknown JSON field %q", name)
			}
			if err := validateExactJSONFields(value, fieldType); err != nil {
				return fmt.Errorf("JSON field %q: %w", name, err)
			}
		}
	case reflect.Slice, reflect.Array:
		var values []json.RawMessage
		if err := json.Unmarshal(raw, &values); err != nil {
			return err
		}
		for i, value := range values {
			if err := validateExactJSONFields(value, typ.Elem()); err != nil {
				return fmt.Errorf("JSON item %d: %w", i, err)
			}
		}
	case reflect.Map:
		var values map[string]json.RawMessage
		if err := json.Unmarshal(raw, &values); err != nil {
			return err
		}
		for key, value := range values {
			if err := validateExactJSONFields(value, typ.Elem()); err != nil {
				return fmt.Errorf("JSON map value %q: %w", key, err)
			}
		}
	}
	return nil
}

func rejectDuplicateJSONKeys(raw []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	var walk func() error
	walk = func() error {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		delim, ok := token.(json.Delim)
		if !ok {
			return nil
		}
		switch delim {
		case '{':
			seen := make(map[string]struct{})
			for decoder.More() {
				keyToken, err := decoder.Token()
				if err != nil {
					return err
				}
				key, ok := keyToken.(string)
				if !ok {
					return fmt.Errorf("JSON object key must be a string")
				}
				if _, exists := seen[key]; exists {
					return fmt.Errorf("duplicate JSON key %q", key)
				}
				seen[key] = struct{}{}
				if err := walk(); err != nil {
					return err
				}
			}
			_, err = decoder.Token()
			return err
		case '[':
			for decoder.More() {
				if err := walk(); err != nil {
					return err
				}
			}
			_, err = decoder.Token()
			return err
		default:
			return fmt.Errorf("unexpected JSON delimiter %q", delim)
		}
	}
	return walk()
}

const verificationTerminationGrace = 500 * time.Millisecond

var errVerificationCommandCancelled = errors.New("verification command cancelled")

func runShell(ctx context.Context, command string, jsonOut bool) (int, string, error) {
	cmd := exec.Command("sh", "-c", command)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	stdoutDigest := newOutputDigester()
	stderrDigest := newOutputDigester()
	if jsonOut {
		cmd.Stdout = stdoutDigest.Mirror(os.Stderr)
		cmd.Stderr = stderrDigest.Mirror(os.Stderr)
	} else {
		cmd.Stdout = stdoutDigest.Mirror(os.Stdout)
		cmd.Stderr = stderrDigest.Mirror(os.Stderr)
	}
	if err := cmd.Start(); err != nil {
		return 1, digestOutputs(stdoutDigest, stderrDigest), nil
	}
	waited := make(chan error, 1)
	go func() { waited <- cmd.Wait() }()

	select {
	case err := <-waited:
		return shellExitCode(err), digestOutputs(stdoutDigest, stderrDigest), nil
	case <-ctx.Done():
		select {
		case err := <-waited:
			return shellExitCode(err), digestOutputs(stdoutDigest, stderrDigest), nil
		default:
		}
	}

	pid := cmd.Process.Pid
	_ = signalShellGroup(pid, syscall.SIGTERM)
	timer := time.NewTimer(verificationTerminationGrace)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer timer.Stop()
	defer ticker.Stop()
	waitComplete := false
	for {
		if waitComplete && !shellGroupAlive(pid) {
			return 130, digestOutputs(stdoutDigest, stderrDigest), errVerificationCommandCancelled
		}
		select {
		case <-waited:
			waitComplete = true
		case <-timer.C:
			if shellGroupAlive(pid) {
				_ = signalShellGroup(pid, syscall.SIGKILL)
			}
		case <-ticker.C:
		}
	}
}

func shellExitCode(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return 1
}

func signalShellGroup(pid int, sig syscall.Signal) error {
	err := syscall.Kill(-pid, sig)
	if errors.Is(err, syscall.ESRCH) {
		return nil
	}
	return err
}

func shellGroupAlive(pid int) bool {
	err := syscall.Kill(-pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

func randomID() (string, error) {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

func resolveRepoKey(st *store.Store) (string, error) {
	return resolveRepoKeyForOptions(st, runOptions{})
}

func resolveRepoKeyForOptions(st *store.Store, opts runOptions) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	key, root, err := repo.Keys(cwd)
	if err != nil {
		return "", err
	}
	if opts.dryRun {
		return key, nil
	}
	if err := st.RekeyRepo(key, root); err != nil {
		return "", fmt.Errorf("sanitize repo identity: %w", err)
	}
	return key, nil
}
