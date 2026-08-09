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

	"github.com/uinaf/slopomatic/internal/buildinfo"
	"github.com/uinaf/slopomatic/internal/machine"
	"github.com/uinaf/slopomatic/internal/repo"
	"github.com/uinaf/slopomatic/internal/serve"
	"github.com/uinaf/slopomatic/internal/status"
	"github.com/uinaf/slopomatic/internal/store"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fmt.Fprint(os.Stdout, usage())
		return 0
	}
	if len(args) == 2 && (args[1] == "-h" || args[1] == "--help") {
		help, ok := commandUsage(args[0])
		if !ok {
			fmt.Fprintf(os.Stderr, "slopomatic: unknown command %q\n", args[0])
			return 2
		}
		fmt.Fprint(os.Stdout, help)
		return 0
	}
	if args[0] == "version" || args[0] == "--version" {
		if _, code := requireFlags("version", args[1:]); code != 0 {
			return code
		}
		fmt.Fprintln(os.Stdout, buildinfo.Version())
		return 0
	}

	st, err := openStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "slopomatic: %v\n", err)
		return 10
	}
	defer st.Close()

	cmd := args[0]
	rest := args[1:]
	switch cmd {
	case "init":
		return cmdInit(st, rest)
	case "intake":
		return cmdIntake(st, rest)
	case "release":
		return cmdRelease(st, rest)
	case "build":
		return cmdBuild(st, rest)
	case "verify":
		return cmdVerify(st, rest)
	case "review":
		return cmdReview(st, rest)
	case "rework":
		return cmdRework(st, rest)
	case "deliver":
		return cmdDeliver(st, rest)
	case "ask":
		return cmdAsk(st, rest)
	case "decide":
		return cmdDecide(st, rest)
	case "retry":
		return cmdRetry(st, rest)
	case "block":
		return cmdBlock(st, rest)
	case "status":
		return cmdStatus(st, rest)
	case "serve":
		return cmdServe(st, rest)
	default:
		fmt.Fprintf(os.Stderr, "slopomatic: unknown command %q\n", cmd)
		return 2
	}
}

func usage() string {
	return `slopomatic — deterministic structured slop cannoning

Usage:
  slopomatic init [--run ID]
  slopomatic intake --file intake.json [--run ID]
  slopomatic release --revision N [--run ID]
  slopomatic build [--run ID]
  slopomatic verify --cmd CMD | --evidence file.json [--run ID]
  slopomatic review --evidence file.json [--run ID]
  slopomatic rework [--run ID]
  slopomatic deliver --evidence file.json [--run ID]
  slopomatic ask --question TEXT [--run ID]
  slopomatic decide --answer TEXT [--run ID]
  slopomatic retry --reason TEXT [--run ID]
  slopomatic block --reason TEXT [--run ID]
  slopomatic status [--json] [--run ID]
  slopomatic serve [--addr 127.0.0.1:7780]
  slopomatic version
`
}

func commandUsage(command string) (string, bool) {
	usage := map[string]string{
		"init": `Usage: slopomatic init [--run ID]

Create a run for the current Git repository. The next action is intake.
`,
		"intake": `Usage: slopomatic intake --file PATH [--run ID]

Load the released-work contract from JSON. Use --file - to read stdin.
Fields: delivery_mode, review_consent, series_bound, units.
`,
		"release": `Usage: slopomatic release --revision N [--run ID]

Human approval latch for the exact intake_revision shown by status.
`,
		"build": `Usage: slopomatic build [--run ID]

Claim the next ready unit, or restart the current unit after rework.
`,
		"verify": `Usage: slopomatic verify (--cmd CMD | --evidence PATH) [--run ID]

Run verification or load strict JSON evidence. Use --evidence - for stdin.
A failed command is recorded as BLOCKED and exits with code 6.
`,
		"review": `Usage: slopomatic review --evidence PATH [--run ID]

Record strict JSON review evidence. Use --evidence - for stdin.
Verdicts: clean, findings, ambiguous. Reviewers: autoreview, bugbot, human.
`,
		"rework": `Usage: slopomatic rework [--run ID]

Return the current unit from REVIEW to the build loop.
`,
		"deliver": `Usage: slopomatic deliver --evidence PATH [--run ID]

Record strict JSON delivery evidence. Use --evidence - for stdin.
`,
		"ask": `Usage: slopomatic ask --question TEXT [--run ID]

Park the run until a human decision is recorded.
`,
		"decide": `Usage: slopomatic decide --answer TEXT [--run ID]

Record the human answer and resume the parked state.
`,
		"retry": `Usage: slopomatic retry --reason TEXT [--run ID]

Record the recovery decision and return a blocked verification to BUILD.
`,
		"block": `Usage: slopomatic block --reason TEXT [--run ID]

Record why active work cannot continue. Resume with retry after recovery.
`,
		"status": `Usage: slopomatic status [--json] [--run ID]

Show the current state and an actionable next command.
`,
		"serve": `Usage: slopomatic serve [--addr 127.0.0.1:7780]

Serve the read-only run projector on a loopback address.
`,
		"version": `Usage: slopomatic version

Print the CLI version and source revision.
`,
	}
	help, ok := usage[command]
	return help, ok
}

func openStore() (*store.Store, error) {
	if p := os.Getenv("SLOPOMATIC_DB"); p != "" {
		return store.Open(p)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	return store.Open(store.DefaultPath(os.Getenv("XDG_DATA_HOME"), home))
}

func cmdInit(st *store.Store, args []string) int {
	fs, code := requireFlags("init", args)
	if code != 0 {
		return code
	}
	runID := fs["run"]
	if runID == "" {
		id, err := randomID()
		if err != nil {
			fmt.Fprintf(os.Stderr, "slopomatic: generate run id: %v\n", err)
			return 10
		}
		runID = "run-" + id
	}
	key, err := resolveRepoKey(st)
	if err != nil {
		fmt.Fprintf(os.Stderr, "slopomatic: %v\n", err)
		return 2
	}
	run := machine.NewRun(runID, key)
	if err := st.CreateRun(run, nil); err != nil {
		fmt.Fprintf(os.Stderr, "slopomatic: %v\n", err)
		return 10
	}
	return printStatus(st, key, runID, false)
}

type intakeUnitDTO struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Blockers []string `json:"blockers"`
}

func cmdIntake(st *store.Store, args []string) int {
	fs, code := requireFlags("intake", args)
	if code != 0 {
		return code
	}
	file := fs["file"]
	if file == "" {
		fmt.Fprintln(os.Stderr, "slopomatic: intake requires --file")
		return 2
	}
	var patch struct {
		DeliveryMode  *string         `json:"delivery_mode"`
		ReviewConsent *string         `json:"review_consent"`
		SeriesBound   *int            `json:"series_bound"`
		Units         []intakeUnitDTO `json:"units"`
	}
	if err := readJSON(file, &patch); err != nil {
		fmt.Fprintf(os.Stderr, "slopomatic: invalid intake json: %v\n", err)
		return 2
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
	if patch.ReviewConsent != nil {
		c := machine.ReviewConsent(*patch.ReviewConsent)
		ip.ReviewConsent = &c
	}
	return applyCmd(st, fs["run"], machine.CmdIntake, machine.ApplyInput{Intake: ip})
}

func cmdRelease(st *store.Store, args []string) int {
	fs, code := requireFlags("release", args)
	if code != 0 {
		return code
	}
	revS := fs["revision"]
	if revS == "" {
		fmt.Fprintln(os.Stderr, "slopomatic: release requires --revision")
		return 2
	}
	rev, err := strconv.ParseInt(revS, 10, 64)
	if err != nil {
		fmt.Fprintf(os.Stderr, "slopomatic: bad --revision: %v\n", err)
		return 2
	}
	return applyCmd(st, fs["run"], machine.CmdRelease, machine.ApplyInput{IntakeRevision: rev})
}

func cmdBuild(st *store.Store, args []string) int {
	fs, code := requireFlags("build", args)
	if code != 0 {
		return code
	}
	return applyCmd(st, fs["run"], machine.CmdBuild, machine.ApplyInput{})
}

func cmdVerify(st *store.Store, args []string) int {
	fs, code := requireFlags("verify", args)
	if code != 0 {
		return code
	}
	if fs["evidence"] != "" && fs["cmd"] != "" {
		fmt.Fprintln(os.Stderr, "slopomatic: verify accepts exactly one of --cmd or --evidence")
		return 2
	}
	if fs["evidence"] == "" && fs["cmd"] == "" {
		fmt.Fprintln(os.Stderr, "slopomatic: verify requires --cmd or --evidence")
		return 2
	}
	var ev machine.VerifyEvidence
	if f := fs["evidence"]; f != "" {
		var dto struct {
			Command      string `json:"command"`
			ExitCode     *int   `json:"exit_code"`
			OutputDigest string `json:"output_digest,omitempty"`
		}
		if err := readJSON(f, &dto); err != nil {
			fmt.Fprintf(os.Stderr, "slopomatic: %v\n", err)
			return 2
		}
		if dto.ExitCode == nil {
			fmt.Fprintln(os.Stderr, "slopomatic: verify.exit_code is required")
			return 2
		}
		ev = machine.VerifyEvidence{Command: dto.Command, ExitCode: *dto.ExitCode, OutputDigest: dto.OutputDigest}
	}
	prepared, code := prepareApply(st, fs["run"], machine.CmdVerify)
	if code != 0 {
		return code
	}
	if c := fs["cmd"]; c != "" {
		code := runShell(c)
		ev = machine.VerifyEvidence{Command: c, ExitCode: code}
	}
	_, code = applyPrepared(st, prepared, machine.CmdVerify, machine.ApplyInput{Verify: &ev})
	if code != 0 {
		return code
	}
	if ev.ExitCode != 0 {
		return 6
	}
	return 0
}

func cmdReview(st *store.Store, args []string) int {
	fs, code := requireFlags("review", args)
	if code != 0 {
		return code
	}
	if fs["evidence"] == "" {
		fmt.Fprintln(os.Stderr, "slopomatic: review requires --evidence")
		return 2
	}
	var ev machine.ReviewEvidence
	if err := readJSON(fs["evidence"], &ev); err != nil {
		fmt.Fprintf(os.Stderr, "slopomatic: %v\n", err)
		return 2
	}
	return applyCmd(st, fs["run"], machine.CmdReview, machine.ApplyInput{Review: &ev})
}

func cmdRework(st *store.Store, args []string) int {
	fs, code := requireFlags("rework", args)
	if code != 0 {
		return code
	}
	return applyCmd(st, fs["run"], machine.CmdRework, machine.ApplyInput{})
}

func cmdDeliver(st *store.Store, args []string) int {
	fs, code := requireFlags("deliver", args)
	if code != 0 {
		return code
	}
	if fs["evidence"] == "" {
		fmt.Fprintln(os.Stderr, "slopomatic: deliver requires --evidence")
		return 2
	}
	var ev machine.DeliverEvidence
	if err := readJSON(fs["evidence"], &ev); err != nil {
		fmt.Fprintf(os.Stderr, "slopomatic: %v\n", err)
		return 2
	}
	return applyCmd(st, fs["run"], machine.CmdDeliver, machine.ApplyInput{Deliver: &ev})
}

func cmdAsk(st *store.Store, args []string) int {
	fs, code := requireFlags("ask", args)
	if code != 0 {
		return code
	}
	if fs["question"] == "" {
		fmt.Fprintln(os.Stderr, "slopomatic: ask requires --question")
		return 2
	}
	return applyCmd(st, fs["run"], machine.CmdAsk, machine.ApplyInput{
		Question: fs["question"],
	})
}

func cmdDecide(st *store.Store, args []string) int {
	fs, code := requireFlags("decide", args)
	if code != 0 {
		return code
	}
	if fs["answer"] == "" {
		fmt.Fprintln(os.Stderr, "slopomatic: decide requires --answer")
		return 2
	}
	return applyCmd(st, fs["run"], machine.CmdDecide, machine.ApplyInput{
		Decision: &machine.Decision{Answer: fs["answer"]},
	})
}

func cmdRetry(st *store.Store, args []string) int {
	fs, code := requireFlags("retry", args)
	if code != 0 {
		return code
	}
	if strings.TrimSpace(fs["reason"]) == "" {
		fmt.Fprintln(os.Stderr, "slopomatic: retry requires --reason")
		return 2
	}
	return applyCmd(st, fs["run"], machine.CmdRetry, machine.ApplyInput{RetryReason: fs["reason"]})
}

func cmdBlock(st *store.Store, args []string) int {
	fs, code := requireFlags("block", args)
	if code != 0 {
		return code
	}
	if strings.TrimSpace(fs["reason"]) == "" {
		fmt.Fprintln(os.Stderr, "slopomatic: block requires --reason")
		return 2
	}
	return applyCmd(st, fs["run"], machine.CmdBlock, machine.ApplyInput{BlockReason: fs["reason"]})
}

func cmdStatus(st *store.Store, args []string) int {
	fs, code := requireFlags("status", args)
	if code != 0 {
		return code
	}
	jsonOut := fs["json"] == "1" || flagHas(args, "--json")
	key, err := resolveRepoKey(st)
	if err != nil {
		fmt.Fprintf(os.Stderr, "slopomatic: %v\n", err)
		return 2
	}
	return printStatus(st, key, fs["run"], jsonOut)
}

type preparedApply struct {
	repoKey string
	run     machine.Run
	units   []machine.Unit
}

func prepareApply(st *store.Store, runID string, cmd machine.Command) (preparedApply, int) {
	key, err := resolveRepoKey(st)
	if err != nil {
		fmt.Fprintf(os.Stderr, "slopomatic: %v\n", err)
		return preparedApply{}, 2
	}
	run, units, err := st.ResolveActiveRun(key, runID)
	if err != nil {
		return preparedApply{}, mapErr(err)
	}
	if !slices.Contains(machine.AllowedCommands(run, units), cmd) {
		return preparedApply{}, mapErr(fmt.Errorf("%w: %s not allowed from %s", machine.ErrIllegalTransition, cmd, run.State))
	}
	return preparedApply{repoKey: key, run: run, units: units}, 0
}

func applyPrepared(st *store.Store, prepared preparedApply, cmd machine.Command, in machine.ApplyInput) (machine.ApplyResult, int) {
	run := prepared.run
	in.ExpectedRevision = run.Revision
	res, err := machine.Apply(run, prepared.units, cmd, in)
	if err != nil {
		return machine.ApplyResult{}, mapErr(err)
	}
	if err := st.SaveApply(res); err != nil {
		return machine.ApplyResult{}, mapErr(err)
	}
	return res, printStatus(st, prepared.repoKey, res.Run.ID, false)
}

func applyCmd(st *store.Store, runID string, cmd machine.Command, in machine.ApplyInput) int {
	prepared, code := prepareApply(st, runID, cmd)
	if code != 0 {
		return code
	}
	_, code = applyPrepared(st, prepared, cmd, in)
	return code
}

func printStatus(st *store.Store, repoKey, runID string, asJSON bool) int {
	run, units, err := st.ResolveStatusRun(repoKey, runID)
	if err != nil {
		return mapErr(err)
	}
	doc := status.From(run, units)
	if asJSON {
		b, err := doc.JSON()
		if err != nil {
			fmt.Fprintf(os.Stderr, "slopomatic: %v\n", err)
			return 10
		}
		fmt.Fprintln(os.Stdout, string(b))
		return 0
	}
	fmt.Fprintln(os.Stdout, doc.CompactLine())
	return 0
}

func mapErr(err error) int {
	fmt.Fprintf(os.Stderr, "slopomatic: %v\n", err)
	switch {
	case errors.Is(err, machine.ErrBadArgs):
		return 2
	case errors.Is(err, machine.ErrIllegalTransition), errors.Is(err, machine.ErrUnmetGuard):
		return 3
	case errors.Is(err, machine.ErrRevisionConflict):
		return 4
	case errors.Is(err, machine.ErrAmbiguousRun), errors.Is(err, machine.ErrNotFound), errors.Is(err, machine.ErrCorruptState):
		return 5
	default:
		return 10
	}
}

var commandFlags = map[string]map[string]bool{
	"init":    {"run": true},
	"intake":  {"file": true, "run": true},
	"release": {"revision": true, "run": true},
	"build":   {"run": true},
	"verify":  {"cmd": true, "evidence": true, "run": true},
	"review":  {"evidence": true, "run": true},
	"rework":  {"run": true},
	"deliver": {"evidence": true, "run": true},
	"ask":     {"question": true, "run": true},
	"decide":  {"answer": true, "run": true},
	"retry":   {"reason": true, "run": true},
	"block":   {"reason": true, "run": true},
	"status":  {"json": true, "run": true},
	"serve":   {"addr": true},
	"version": {},
}

func cmdServe(st *store.Store, args []string) int {
	fs, err := parseFlagsWith(args, commandFlags["serve"])
	if err != nil {
		fmt.Fprintf(os.Stderr, "slopomatic: %v\n", err)
		return 2
	}
	key, err := resolveRepoKey(st)
	if err != nil {
		fmt.Fprintf(os.Stderr, "slopomatic: %v\n", err)
		return 2
	}
	addr := fs["addr"]
	if addr == "" {
		addr = "127.0.0.1:7780"
	}
	srv, err := serve.New(serve.Options{Store: st, RepoKey: key, Addr: addr})
	if err != nil {
		fmt.Fprintf(os.Stderr, "slopomatic: %v\n", err)
		return 2
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := srv.ListenAndServe(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "slopomatic: %v\n", err)
		return 10
	}
	return 0
}

func requireFlags(command string, args []string) (map[string]string, int) {
	fs, err := parseFlagsWith(args, commandFlags[command])
	if err != nil {
		fmt.Fprintf(os.Stderr, "slopomatic: %v\n", err)
		return nil, 2
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
	var r io.Reader
	if path == "-" {
		r = os.Stdin
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

func runShell(command string) int {
	cmd := exec.Command("sh", "-c", command)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return ee.ExitCode()
		}
		return 1
	}
	return 0
}

func randomID() (string, error) {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

func resolveRepoKey(st *store.Store) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	key, root, err := repo.Keys(cwd)
	if err != nil {
		return "", err
	}
	if err := st.RekeyRepo(key, root); err != nil {
		return "", fmt.Errorf("sanitize repo identity: %w", err)
	}
	return key, nil
}
