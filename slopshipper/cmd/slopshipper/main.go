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

	"github.com/mattn/go-isatty"
	"github.com/uinaf/slopshipper/internal/buildinfo"
	"github.com/uinaf/slopshipper/internal/machine"
	"github.com/uinaf/slopshipper/internal/repo"
	"github.com/uinaf/slopshipper/internal/serve"
	"github.com/uinaf/slopshipper/internal/status"
	"github.com/uinaf/slopshipper/internal/store"
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
			return writeFailure(opts, 2, fmt.Errorf("%w; set SLOPSHIPPER_DB to a writable path and inspect resolution with slopshipper storage --json", err))
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
	return `slopshipper — deterministic structured slop cannoning

Usage:
  slopshipper init [--run ID]
  slopshipper intake --file PATH|- [--run ID]
  slopshipper release --revision N [--run ID]
  slopshipper build [--run ID]
  slopshipper verify --cmd CMD | --evidence PATH|- [--run ID]
  slopshipper review --evidence PATH|- [--run ID]
  slopshipper rework [--run ID]
  slopshipper deliver --evidence PATH|- [--run ID]
  slopshipper ask --question TEXT [--run ID]
  slopshipper decide --answer TEXT [--run ID]
  slopshipper retry --reason TEXT [--run ID]
  slopshipper block --reason TEXT [--run ID]
  slopshipper status [--json] [--run ID]
  slopshipper reviewers [--add NAME | --remove NAME] [--json]
  slopshipper schema [--command NAME] [--json]
  slopshipper storage [--json]
  slopshipper serve [--addr 127.0.0.1:7780]
  slopshipper version

All mutating commands accept --input PATH and --dry-run.
Use --json before or after a command for structured success and error output.
`
}

func commandUsage(command string) (string, bool) {
	usage := map[string]string{
		"init": `Usage: slopshipper init [--run ID]

Create a run for the current Git repository. The next action is intake.
`,
		"intake": `Usage: slopshipper intake --file PATH [--run ID]

Load the released-work contract from JSON. Use --file - to read stdin.
Fields: delivery_mode, required_reviewers, series_bound, units.
`,
		"release": `Usage: slopshipper release --revision N [--run ID]

Human approval latch for the exact intake_revision shown by status.
`,
		"build": `Usage: slopshipper build [--run ID]

Claim the next ready unit, or restart the current unit after rework.
`,
		"verify": `Usage: slopshipper verify (--cmd CMD | --evidence PATH) [--run ID]

Run verification or load strict JSON evidence. Use --evidence - for stdin.
A failed command is recorded as BLOCKED and exits with code 6.
`,
		"review": `Usage: slopshipper review --evidence PATH [--run ID]

Record strict JSON review evidence. Use --evidence - for stdin.
Verdicts: clean, findings, ambiguous. The reviewer must be one of the run's
required registered identities; inspect the registry with slopshipper reviewers.
`,
		"rework": `Usage: slopshipper rework [--run ID]

Return the current unit from REVIEW to the build loop.
`,
		"deliver": `Usage: slopshipper deliver --evidence PATH [--run ID]

Record strict JSON delivery evidence. Use --evidence - for stdin.
`,
		"ask": `Usage: slopshipper ask --question TEXT [--run ID]

Park the run until a human decision is recorded.
`,
		"decide": `Usage: slopshipper decide --answer TEXT [--run ID]

Record the human answer and resume the parked state.
`,
		"retry": `Usage: slopshipper retry --reason TEXT [--run ID]

Record the recovery decision and return a blocked verification to BUILD.
`,
		"block": `Usage: slopshipper block --reason TEXT [--run ID]

Record why active work cannot continue. Resume with retry after recovery.
`,
		"status": `Usage: slopshipper status [--json] [--fields LIST] [--run ID]

Show the current state and an actionable next command.
`,
		"schema": `Usage: slopshipper schema [--command NAME] [--json]

Describe commands, flags, strict input schemas, enums, and outputs as JSON.
`,
		"reviewers": `Usage: slopshipper reviewers [--add NAME | --remove NAME] [--json]

List built-in and registered reviewer identities, or register/unregister a
custom identity. Registration is declarative and idempotent; built-ins
(autoreview, bugbot) cannot be changed. Humans hold release and recovery
latches; a human sign-off reviewer must be registered explicitly.
`,
		"storage": `Usage: slopshipper storage [--json]

Show the resolved database path, source, scope, existence, and Git safety.
`,
		"serve": `Usage: slopshipper serve [--addr 127.0.0.1:7780]

Serve the read-only run projector on a loopback address.
`,
		"version": `Usage: slopshipper version

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
			if command != "init" {
				return nil, fmt.Errorf("%w: canonical state does not exist at %q; run slopshipper init first", machine.ErrNotFound, path)
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
	run := machine.NewRun(runID, key)
	if opts.dryRun {
		if st == nil {
			doc := status.From(run, nil)
			doc.DryRun = true
			doc.ValidatedCommand = string(machine.CmdInit)
			return writeStatus(doc, opts)
		}
		if _, _, err := st.GetRun(runID); err == nil {
			return writeFailure(opts, 2, fmt.Errorf("%w: run id %q already exists", machine.ErrRunExists, runID))
		} else if !errors.Is(err, machine.ErrNotFound) {
			return mapErr(err, opts)
		}
		doc := status.From(run, nil)
		doc.DryRun = true
		doc.ValidatedCommand = string(machine.CmdInit)
		return writeStatus(doc, opts)
	}
	if err := st.CreateRun(run, nil); err != nil {
		return mapErr(err, opts)
	}
	return printStatus(st, key, runID, opts)
}

type intakeUnitDTO struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Blockers []string `json:"blockers"`
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
			SeriesBound       *int            `json:"series_bound"`
			Units             []intakeUnitDTO `json:"units"`
		}
		if err := readJSON(file, &filePatch); err != nil {
			return writeFailure(opts, 2, fmt.Errorf("invalid intake json: %w", err))
		}
		patch = intakeInput{
			DeliveryMode: filePatch.DeliveryMode, RequiredReviewers: filePatch.RequiredReviewers,
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
			units[i] = machine.Unit{ID: u.ID, Title: u.Title, Blockers: u.Blockers}
		}
		ip.Units = units
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
	return applyCmd(st, runID, machine.CmdIntake, machine.ApplyInput{Intake: ip}, opts)
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
	return applyCmd(st, runID, machine.CmdRelease, machine.ApplyInput{IntakeRevision: revision}, opts)
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
	return applyCmd(st, runID, machine.CmdBuild, machine.ApplyInput{}, opts)
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
	prepared, code := prepareApply(st, runID, machine.CmdVerify, opts)
	if code != 0 {
		return code
	}
	if c := fs["cmd"]; !raw && c != "" {
		if opts.dryRun {
			doc := status.From(prepared.run, prepared.units)
			doc.DryRun = true
			doc.ValidatedCommand = string(machine.CmdVerify)
			doc.OutcomeUndetermined = true
			return writeStatus(doc, opts)
		}
		code, outputDigest := runShell(c, opts.json)
		ev = machine.VerifyEvidence{Command: c, ExitCode: code, OutputDigest: outputDigest}
	}
	_, code = applyPrepared(st, prepared, machine.CmdVerify, machine.ApplyInput{Verify: &ev}, opts)
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
	} else {
		if fs["evidence"] == "" {
			return writeFailure(opts, 2, fmt.Errorf("review requires --evidence or --input"))
		}
		if err := readJSON(fs["evidence"], &ev); err != nil {
			return writeFailure(opts, 2, err)
		}
	}
	if err := validateRunID(runID); err != nil {
		return writeFailure(opts, 2, err)
	}
	return applyCmd(st, runID, machine.CmdReview, machine.ApplyInput{Review: &ev}, opts)
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
	return applyCmd(st, runID, machine.CmdRework, machine.ApplyInput{}, opts)
}

func cmdDeliver(st *store.Store, args []string, opts runOptions) int {
	fs, code := requireFlags("deliver", args, opts)
	if code != 0 {
		return code
	}
	var ev machine.DeliverEvidence
	var input deliverInput
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
	} else {
		if fs["evidence"] == "" {
			return writeFailure(opts, 2, fmt.Errorf("deliver requires --evidence or --input"))
		}
		if err := readJSON(fs["evidence"], &ev); err != nil {
			return writeFailure(opts, 2, err)
		}
	}
	if err := validateRunID(runID); err != nil {
		return writeFailure(opts, 2, err)
	}
	return applyCmd(st, runID, machine.CmdDeliver, machine.ApplyInput{Deliver: &ev}, opts)
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
	return applyCmd(st, runID, machine.CmdAsk, machine.ApplyInput{Question: question}, opts)
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
	return applyCmd(st, runID, machine.CmdDecide, machine.ApplyInput{Decision: &machine.Decision{Answer: answer}}, opts)
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
	return applyCmd(st, runID, machine.CmdRetry, machine.ApplyInput{RetryReason: reason}, opts)
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
	return applyCmd(st, runID, machine.CmdBlock, machine.ApplyInput{BlockReason: reason}, opts)
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
	}
	res, err := machine.Apply(run, prepared.units, cmd, in)
	if err != nil {
		return machine.ApplyResult{}, mapErr(err, opts)
	}
	if opts.dryRun {
		doc := status.From(res.Run, res.Units)
		doc.DryRun = true
		doc.ValidatedCommand = string(cmd)
		return res, writeStatus(doc, opts)
	}
	if err := st.SaveApply(res); err != nil {
		return machine.ApplyResult{}, mapErr(err, opts)
	}
	return res, printStatus(st, prepared.repoKey, res.Run.ID, opts)
}

func applyCmd(st *store.Store, runID string, cmd machine.Command, in machine.ApplyInput, opts runOptions) int {
	prepared, code := prepareApply(st, runID, cmd, opts)
	if code != 0 {
		return code
	}
	_, code = applyPrepared(st, prepared, cmd, in, opts)
	return code
}

func printStatus(st *store.Store, repoKey, runID string, opts runOptions) int {
	run, units, err := st.ResolveStatusRun(repoKey, runID)
	if err != nil {
		if runID == "" && errors.Is(err, machine.ErrNotFound) {
			return writeStatus(status.Bootstrap(repoKey), opts)
		}
		return mapErr(err, opts)
	}
	return writeStatus(status.From(run, units), opts)
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
	"init":      {"run": true, "input": true},
	"intake":    {"file": true, "run": true, "input": true},
	"release":   {"revision": true, "run": true, "input": true},
	"build":     {"run": true, "input": true},
	"verify":    {"cmd": true, "evidence": true, "run": true, "input": true},
	"review":    {"evidence": true, "run": true, "input": true},
	"rework":    {"run": true, "input": true},
	"deliver":   {"evidence": true, "run": true, "input": true},
	"ask":       {"question": true, "run": true, "input": true},
	"decide":    {"answer": true, "run": true, "input": true},
	"retry":     {"reason": true, "run": true, "input": true},
	"block":     {"reason": true, "run": true, "input": true},
	"status":    {"json": true, "run": true, "fields": true},
	"reviewers": {"add": true, "remove": true, "json": true},
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
	prefix := "slopshipper reviewers"
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
		if key == "json" {
			if hasVal {
				return nil, fmt.Errorf("flag --json does not accept a value")
			}
			out["json"] = "1"
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
			return fmt.Errorf("stdin is an interactive terminal; pipe or redirect JSON when using -, or inspect slopshipper schema for the command contract")
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

func runShell(command string, jsonOut bool) (int, string) {
	cmd := exec.Command("sh", "-c", command)
	stdoutDigest := newOutputDigester()
	stderrDigest := newOutputDigester()
	if jsonOut {
		cmd.Stdout = stdoutDigest.Mirror(os.Stderr)
		cmd.Stderr = stderrDigest.Mirror(os.Stderr)
	} else {
		cmd.Stdout = stdoutDigest.Mirror(os.Stdout)
		cmd.Stderr = stderrDigest.Mirror(os.Stderr)
	}
	code := 0
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			code = ee.ExitCode()
		} else {
			code = 1
		}
	}
	return code, digestOutputs(stdoutDigest, stderrDigest)
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
