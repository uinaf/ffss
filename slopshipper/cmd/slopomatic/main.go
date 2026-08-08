package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/uinaf/slopomatic/internal/machine"
	"github.com/uinaf/slopomatic/internal/repo"
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
	if args[0] == "version" || args[0] == "--version" {
		fmt.Fprintln(os.Stdout, "slopomatic 0.0.0-dev")
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
	case "status":
		return cmdStatus(st, rest)
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
  slopomatic status [--json] [--run ID]
  slopomatic version
`
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
	fs, code := requireFlags(args)
	if code != 0 {
		return code
	}
	runID := fs["run"]
	if runID == "" {
		runID = "run-" + randomID()
	}
	cwd, _ := os.Getwd()
	key, err := repo.Key(cwd)
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
	fs, code := requireFlags(args)
	if code != 0 {
		return code
	}
	file := fs["file"]
	if file == "" {
		fmt.Fprintln(os.Stderr, "slopomatic: intake requires --file")
		return 2
	}
	raw, err := os.ReadFile(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "slopomatic: %v\n", err)
		return 2
	}
	var patch struct {
		DeliveryMode  *string         `json:"delivery_mode"`
		ReviewConsent *string         `json:"review_consent"`
		SeriesBound   *int            `json:"series_bound"`
		Units         []intakeUnitDTO `json:"units"`
	}
	if err := json.Unmarshal(raw, &patch); err != nil {
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
	return applyCmd(st, fs["run"], machine.CmdIntake, machine.ApplyInput{Intake: ip}, nil)
}

func cmdRelease(st *store.Store, args []string) int {
	fs, code := requireFlags(args)
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
	return applyCmd(st, fs["run"], machine.CmdRelease, machine.ApplyInput{IntakeRevision: rev}, map[string]any{"intake_revision": rev})
}

func cmdBuild(st *store.Store, args []string) int {
	fs, code := requireFlags(args)
	if code != 0 {
		return code
	}
	return applyCmd(st, fs["run"], machine.CmdBuild, machine.ApplyInput{}, nil)
}

func cmdVerify(st *store.Store, args []string) int {
	fs, code := requireFlags(args)
	if code != 0 {
		return code
	}
	var ev machine.VerifyEvidence
	if f := fs["evidence"]; f != "" {
		if err := readJSON(f, &ev); err != nil {
			fmt.Fprintf(os.Stderr, "slopomatic: %v\n", err)
			return 2
		}
	} else if c := fs["cmd"]; c != "" {
		code := runShell(c)
		ev = machine.VerifyEvidence{Command: c, ExitCode: code}
	} else {
		fmt.Fprintln(os.Stderr, "slopomatic: verify requires --cmd or --evidence")
		return 2
	}
	return applyCmd(st, fs["run"], machine.CmdVerify, machine.ApplyInput{Verify: &ev}, ev)
}

func cmdReview(st *store.Store, args []string) int {
	fs, code := requireFlags(args)
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
	return applyCmd(st, fs["run"], machine.CmdReview, machine.ApplyInput{Review: &ev}, ev)
}

func cmdRework(st *store.Store, args []string) int {
	fs, code := requireFlags(args)
	if code != 0 {
		return code
	}
	return applyCmd(st, fs["run"], machine.CmdRework, machine.ApplyInput{}, nil)
}

func cmdDeliver(st *store.Store, args []string) int {
	fs, code := requireFlags(args)
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
	return applyCmd(st, fs["run"], machine.CmdDeliver, machine.ApplyInput{Deliver: &ev}, ev)
}

func cmdAsk(st *store.Store, args []string) int {
	fs, code := requireFlags(args)
	if code != 0 {
		return code
	}
	if fs["question"] == "" {
		fmt.Fprintln(os.Stderr, "slopomatic: ask requires --question")
		return 2
	}
	return applyCmd(st, fs["run"], machine.CmdAsk, machine.ApplyInput{
		Question: fs["question"],
	}, map[string]any{"question": fs["question"]})
}

func cmdDecide(st *store.Store, args []string) int {
	fs, code := requireFlags(args)
	if code != 0 {
		return code
	}
	if fs["answer"] == "" {
		fmt.Fprintln(os.Stderr, "slopomatic: decide requires --answer")
		return 2
	}
	return applyCmd(st, fs["run"], machine.CmdDecide, machine.ApplyInput{
		Decision: &machine.Decision{Answer: fs["answer"]},
	}, map[string]any{"answer": fs["answer"]})
}

func cmdStatus(st *store.Store, args []string) int {
	fs, code := requireFlags(args)
	if code != 0 {
		return code
	}
	jsonOut := fs["json"] == "1" || flagHas(args, "--json")
	cwd, _ := os.Getwd()
	key, err := repo.Key(cwd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "slopomatic: %v\n", err)
		return 2
	}
	return printStatus(st, key, fs["run"], jsonOut)
}

func applyCmd(st *store.Store, runID string, cmd machine.Command, in machine.ApplyInput, evidence any) int {
	cwd, _ := os.Getwd()
	key, err := repo.Key(cwd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "slopomatic: %v\n", err)
		return 2
	}
	run, units, err := st.ResolveActiveRun(key, runID)
	if err != nil {
		return mapErr(err)
	}
	in.ExpectedRevision = run.Revision
	res, err := machine.Apply(run, units, cmd, in)
	if err != nil {
		return mapErr(err)
	}
	if err := st.SaveApply(res, evidence); err != nil {
		return mapErr(err)
	}
	return printStatus(st, key, res.Run.ID, false)
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

var knownFlags = map[string]bool{
	"run": true, "file": true, "revision": true, "cmd": true, "evidence": true,
	"answer": true, "question": true, "json": true,
}

func requireFlags(args []string) (map[string]string, int) {
	fs, err := parseFlags(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "slopomatic: %v\n", err)
		return nil, 2
	}
	return fs, 0
}

func parseFlags(args []string) (map[string]string, error) {
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
		if !knownFlags[key] {
			return nil, fmt.Errorf("unknown flag --%s", key)
		}
		if key == "json" {
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
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, dest)
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

func randomID() string {
	var b [4]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
