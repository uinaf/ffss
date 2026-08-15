package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/uinaf/slopshipper/internal/machine"
	"github.com/uinaf/slopshipper/internal/repo"
	"github.com/uinaf/slopshipper/internal/status"
	"github.com/uinaf/slopshipper/internal/store"
)

type cliHarness struct {
	t       *testing.T
	bin     string
	db      string
	repoDir string
	env     []string
}

func newCLIHarness(t *testing.T) *cliHarness {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "slopshipper")
	buildArgs := []string{"build", "-o", bin, "."}
	if os.Getenv("SLOPSHIPPER_COVERAGE_DIR") != "" {
		buildArgs = []string{"build", "-cover", "-covermode=atomic", "-coverpkg=github.com/uinaf/slopshipper/...", "-o", bin, "."}
	}
	build := exec.Command("go", buildArgs...)
	build.Dir = "."
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}

	db := filepath.Join(t.TempDir(), "t.sqlite")
	repoDir := t.TempDir()
	runGit(t, repoDir, "init")
	runGit(t, repoDir, "config", "user.email", "t@example.com")
	runGit(t, repoDir, "config", "user.name", "t")
	if err := os.WriteFile(filepath.Join(repoDir, "README"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, repoDir, "add", ".")
	runGit(t, repoDir, "commit", "-m", "init")

	return &cliHarness{
		t:       t,
		bin:     bin,
		db:      db,
		repoDir: repoDir,
		env:     append(os.Environ(), "SLOPSHIPPER_DB="+db),
	}
}

func (h *cliHarness) run(args ...string) (string, int) {
	return h.runInput("", args...)
}

func (h *cliHarness) runInput(input string, args ...string) (string, int) {
	h.t.Helper()
	cmd := exec.Command(h.bin, args...)
	cmd.Dir = h.repoDir
	cmd.Env = h.env
	if input != "" {
		cmd.Stdin = strings.NewReader(input)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	code := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else {
			h.t.Fatalf("%v: %s", err, stderr.String())
		}
	}
	if stdout.Len() > 0 {
		return stdout.String(), code
	}
	return stderr.String(), code
}

func (h *cliHarness) must(args ...string) string {
	h.t.Helper()
	out, code := h.run(args...)
	if code != 0 {
		h.t.Fatalf("%v: exit %d\n%s", args, code, out)
	}
	return out
}

func (h *cliHarness) mustInput(input string, args ...string) string {
	h.t.Helper()
	out, code := h.runInput(input, args...)
	if code != 0 {
		h.t.Fatalf("%v: exit %d\n%s", args, code, out)
	}
	return out
}

func TestCLINorthStarSmoke(t *testing.T) {
	h := newCLIHarness(t)
	h.must("init", "--run", "smoke")

	intake := filepath.Join(t.TempDir(), "intake.json")
	mustWrite(t, intake, `{
		"required_reviewers":["autoreview"],
		"series_bound":1,
		"units":[{"id":"u1","title":"one"}]
	}`)
	h.must("intake", "--file", intake, "--run", "smoke")

	out := h.must("status", "--json", "--run", "smoke")
	var st struct {
		IntakeRevision int64 `json:"intake_revision"`
	}
	if err := json.Unmarshal([]byte(out), &st); err != nil {
		t.Fatalf("json: %v\n%s", err, out)
	}
	h.must("release", "--revision", itoa(st.IntakeRevision), "--run", "smoke")
	h.must("build", "--run", "smoke")
	h.must("verify", "--cmd", "true", "--run", "smoke")

	rev := filepath.Join(t.TempDir(), "review.json")
	mustWrite(t, rev, `{"reviewer":"autoreview","verdict":"clean","artifact_ref":"test://1"}`)
	h.must("review", "--evidence", rev, "--run", "smoke")

	del := filepath.Join(t.TempDir(), "deliver.json")
	mustWrite(t, del, `{"delivery_mode":"pr-hold","pr_url":"https://example.com/pr/1"}`)
	h.must("deliver", "--evidence", del, "--run", "smoke")

	out = h.must("status", "--json", "--run", "smoke")
	var fin struct {
		State          string   `json:"state"`
		DeliveredUnits []string `json:"delivered_units"`
	}
	if err := json.Unmarshal([]byte(out), &fin); err != nil {
		t.Fatal(err)
	}
	if fin.State != "AWAITING_SIGNALS" || len(fin.DeliveredUnits) != 1 {
		t.Fatalf("want AWAITING_SIGNALS with one delivered unit got %s", out)
	}
	h.must("observe", "--signal", "merged", "--run", "smoke")
	out = h.must("status", "--json", "--run", "smoke")
	if err := json.Unmarshal([]byte(out), &fin); err != nil {
		t.Fatal(err)
	}
	if fin.State != "RUN_DONE" {
		t.Fatalf("want RUN_DONE got %s (%s)", fin.State, out)
	}

	// Terminal status remains readable without --run.
	out = h.must("status", "--json")
	if err := json.Unmarshal([]byte(out), &fin); err != nil || fin.State != "RUN_DONE" {
		t.Fatalf("status without --run: %v %s", err, out)
	}
}

func TestAgentDXStructuredProtocolAndSchema(t *testing.T) {
	h := newCLIHarness(t)
	out, code := h.run("--json", "unknown")
	if code != 2 {
		t.Fatalf("unknown exit=%d output=%s", code, out)
	}
	var failure errorDocument
	if err := json.Unmarshal([]byte(out), &failure); err != nil {
		t.Fatalf("structured error: %v\n%s", err, out)
	}
	if failure.OK || failure.Error.Kind != "invalid_input" || failure.Error.ExitCode != 2 {
		t.Fatalf("failure=%+v", failure)
	}

	out = h.must("schema", "--command", "intake")
	var schema introspectionDocument
	if err := json.Unmarshal([]byte(out), &schema); err != nil {
		t.Fatalf("schema JSON: %v\n%s", err, out)
	}
	if len(schema.Commands) != 1 || schema.Commands[0].Name != "intake" || schema.Commands[0].Input == nil {
		t.Fatalf("schema=%+v", schema)
	}
	if _, ok := schema.Commands[0].Input.Properties["units"]; !ok {
		t.Fatalf("intake schema lacks units: %+v", schema.Commands[0].Input)
	}

	help := h.must("intake", "--help", "--json")
	if err := json.Unmarshal([]byte(help), &schema); err != nil || len(schema.Commands) != 1 || schema.Commands[0].Name != "intake" {
		t.Fatalf("JSON help: %v %s", err, help)
	}
	version := h.must("--json", "version")
	var versionDoc map[string]any
	if err := json.Unmarshal([]byte(version), &versionDoc); err != nil || versionDoc["version"] == "" {
		t.Fatalf("JSON version: %v %s", err, version)
	}

	full, err := schemaDocument("")
	if err != nil {
		t.Fatal(err)
	}
	if len(full.Commands) != len(commandFlags) {
		t.Fatalf("schema commands=%d flags commands=%d", len(full.Commands), len(commandFlags))
	}
	for _, command := range full.Commands {
		parsed, ok := commandFlags[command.Name]
		if !ok {
			t.Fatalf("schema exposes unknown command %q", command.Name)
		}
		documented := map[string]bool{}
		for _, flag := range command.Flags {
			documented[strings.TrimPrefix(flag.Name, "--")] = true
		}
		for flag := range parsed {
			if !documented[flag] {
				t.Errorf("command %q parser flag --%s missing from schema", command.Name, flag)
			}
		}
	}
}

func TestAgentDXProtocolHelperBranches(t *testing.T) {
	cleaned, opts, err := parseRunOptions([]string{"--json", "build", "--dry-run", "--run=x"})
	if err != nil || !opts.json || !opts.dryRun || strings.Join(cleaned, " ") != "build --run=x" {
		t.Fatalf("parse options cleaned=%v opts=%+v err=%v", cleaned, opts, err)
	}
	for _, args := range [][]string{
		{"--json", "--json", "status"},
		{"--json=false", "status"},
		{"--dry-run", "--dry-run", "build"},
		{"--dry-run=false", "build"},
	} {
		if _, _, err := parseRunOptions(args); err == nil {
			t.Errorf("accepted global options %v", args)
		}
	}
	fields, err := parseFields("state, next_action")
	if err != nil || strings.Join(fields, ",") != "state,next_action" {
		t.Fatalf("fields=%v err=%v", fields, err)
	}
	for _, value := range []string{"", "state,,next_action", "state,state"} {
		if _, err := parseFields(value); err == nil {
			t.Errorf("accepted fields %q", value)
		}
	}

	kinds := map[error]string{
		errUnsafeStatePath:           "unsafe_state_path",
		errInvalidStateConfig:        "invalid_state_config",
		machine.ErrBadArgs:           "invalid_input",
		machine.ErrIllegalTransition: "illegal_transition",
		machine.ErrUnmetGuard:        "unmet_guard",
		machine.ErrRevisionConflict:  "revision_conflict",
		machine.ErrAmbiguousRun:      "ambiguous_run",
		machine.ErrRunExists:         "run_exists",
		machine.ErrNotFound:          "not_found",
		machine.ErrCorruptState:      "corrupt_state",
	}
	for err, want := range kinds {
		if got := errorKind(10, fmt.Errorf("wrapped: %w", err)); got != want {
			t.Errorf("kind %v=%q want %q", err, got, want)
		}
	}
	if got := errorKind(6, errors.New("failed")); got != "verification_failed" {
		t.Fatalf("verification kind=%q", got)
	}
	if got := errorKind(10, errors.New("failed")); got != "internal" {
		t.Fatalf("internal kind=%q", got)
	}

	if err := writeJSON(make(chan int)); err == nil {
		t.Fatal("unencodable JSON accepted")
	}
	var mirror strings.Builder
	mirroredDigest, expectedDigest := newOutputDigester(&mirror), newOutputDigester()
	for _, chunk := range []string{"out", "err"} {
		_, _ = mirroredDigest.Write([]byte(chunk))
		_, _ = expectedDigest.Write([]byte(chunk))
	}
	if digest := mirroredDigest.String(); mirror.String() != "outerr" || !strings.HasPrefix(digest, "sha256:") || digest != expectedDigest.String() {
		t.Fatalf("mirrored output=%q digest=%q expected=%q", mirror.String(), digest, expectedDigest.String())
	}
	streamed, shellCode, shellDigest := captureStderrResult(t, func() (int, string) {
		return runShell("printf out; printf err >&2", true)
	})
	stdoutExpected, stderrExpected := newOutputDigester(), newOutputDigester()
	_, _ = stdoutExpected.Write([]byte("out"))
	_, _ = stderrExpected.Write([]byte("err"))
	if shellCode != 0 || (streamed != "outerr" && streamed != "errout") || shellDigest != digestOutputs(stdoutExpected, stderrExpected) {
		t.Fatalf("runShell code=%d output=%q digest=%q", shellCode, streamed, shellDigest)
	}
	plainExpected := newOutputDigester()
	_, _ = plainExpected.Write([]byte("plain-error"))
	emptyExpected := newOutputDigester()
	plainStreamed, plainCode, plainDigest := captureStderrResult(t, func() (int, string) {
		return runShell("printf plain-error >&2", false)
	})
	if plainCode != 0 || plainStreamed != "plain-error" || plainDigest != digestOutputs(emptyExpected, plainExpected) {
		t.Fatalf("plain runShell code=%d output=%q digest=%q", plainCode, plainStreamed, plainDigest)
	}
	output, code := captureStdoutResult(t, func() int { return writeFailure(runOptions{json: true}, 2, errors.New("bad")) })
	var failure errorDocument
	if err := json.Unmarshal([]byte(output), &failure); err != nil || code != 2 || failure.Error.ExitCode != 2 {
		t.Fatalf("failure=%+v code=%d err=%v output=%s", failure, code, err, output)
	}
	output = captureStdout(t, func() int { return run([]string{"schema", "--command", "build"}) })
	var schema introspectionDocument
	if err := json.Unmarshal([]byte(output), &schema); err != nil || len(schema.Commands) != 1 || schema.Commands[0].Name != "build" {
		t.Fatalf("schema=%+v err=%v output=%s", schema, err, output)
	}
	if strings.Join(schema.RecommendedStatusFields, ",") != status.AgentFieldMask {
		t.Fatalf("recommended status fields=%v", schema.RecommendedStatusFields)
	}
	output = captureStdout(t, func() int { return run([]string{"--help", "--json"}) })
	if err := json.Unmarshal([]byte(output), &schema); err != nil || len(schema.Commands) != len(commandFlags) {
		t.Fatalf("JSON root help: %v %s", err, output)
	}
	output = captureStdout(t, func() int { return run([]string{"version", "--json"}) })
	if !strings.Contains(output, `"version"`) {
		t.Fatalf("JSON version=%s", output)
	}
	if code := captureCode(t, func() int { return run([]string{"schema", "--command", "missing", "--json"}) }); code != 2 {
		t.Fatalf("missing schema code=%d", code)
	}
	if _, err := schemaDocument("missing"); err == nil {
		t.Fatal("unknown schema command accepted")
	}

	validPath := filepath.Join(t.TempDir(), "input.json")
	mustWrite(t, validPath, `{"run":"raw"}`)
	var input initInput
	if raw, err := decodeMutationInput(map[string]string{}, &input); err != nil || raw {
		t.Fatalf("absent input raw=%v err=%v", raw, err)
	}
	if raw, err := decodeMutationInput(map[string]string{"input": validPath}, &input); err != nil || !raw || input.Run != "raw" {
		t.Fatalf("valid input raw=%v input=%+v err=%v", raw, input, err)
	}
	for _, flags := range []map[string]string{
		{"input": ""},
		{"input": validPath, "run": "raw"},
		{"input": filepath.Join(t.TempDir(), "missing.json")},
	} {
		if _, err := decodeMutationInput(flags, &initInput{}); err == nil {
			t.Errorf("accepted input flags=%v", flags)
		}
	}
}

func TestAgentDocsUseCanonicalStatusMask(t *testing.T) {
	for _, path := range []string{
		filepath.Join("..", "..", "docs", "AGENT_INTERFACE.md"),
		filepath.Join("..", "..", "skills", "slopship", "SKILL.md"),
		filepath.Join("..", "..", "skills", "slopship", "references", "status.md"),
	} {
		contents, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(contents), status.AgentFieldMask) {
			t.Errorf("%s does not use canonical status mask", path)
		}
	}
}

func TestDryRunStoreOpeningBranches(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nested", "state.sqlite")
	t.Setenv("SLOPSHIPPER_DB", missing)
	st, err := openStoreForCommand("init", runOptions{dryRun: true})
	if err != nil || st != nil {
		t.Fatalf("fresh dry-run store=%v err=%v", st, err)
	}
	if _, err := openStoreForCommand("build", runOptions{dryRun: true}); !errors.Is(err, machine.ErrNotFound) {
		t.Fatalf("non-init dry-run missing store error=%v", err)
	}
	st, err = openStoreForCommand("init", runOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
	st, err = openStoreForCommand("build", runOptions{dryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestAgentDXDirectRuntimeBranches(t *testing.T) {
	repoDir := t.TempDir()
	runGit(t, repoDir, "init")
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repoDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldDir) })
	database := filepath.Join(t.TempDir(), "agent.sqlite")
	t.Setenv("SLOPSHIPPER_DB", database)

	output, code := captureStdoutResult(t, func() int {
		return run([]string{"init", "--run", "preview", "--dry-run", "--json"})
	})
	if code != 0 {
		t.Fatalf("fresh init dry-run=%d %s", code, output)
	}
	assertJSONState(t, output, "INTAKE")
	if _, err := os.Stat(database); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("fresh direct dry-run created database: %v", err)
	}
	output, code = captureStdoutResult(t, func() int {
		return run([]string{"build", "--run", "preview", "--dry-run", "--json"})
	})
	if code != 5 || !strings.Contains(output, `"kind": "not_found"`) {
		t.Fatalf("missing-state dry-run=%d %s", code, output)
	}

	writeInput := func(name, body string) string {
		t.Helper()
		path := filepath.Join(t.TempDir(), name)
		mustWrite(t, path, body)
		return path
	}
	initPath := writeInput("init.json", `{"run":"direct-agent"}`)
	output = captureStdout(t, func() int { return run([]string{"init", "--input", initPath, "--json"}) })
	assertJSONState(t, output, "INTAKE")
	output, code = captureStdoutResult(t, func() int {
		return run([]string{"init", "--run", "direct-agent", "--dry-run", "--json"})
	})
	if code != 2 || !strings.Contains(output, `"kind": "run_exists"`) {
		t.Fatalf("duplicate init dry-run=%d %s", code, output)
	}
	otherRepo, err := store.Open(database)
	if err != nil {
		t.Fatal(err)
	}
	if err := otherRepo.CreateRun(machine.NewRun("global-run", "different-repository"), nil); err != nil {
		t.Fatal(err)
	}
	if err := otherRepo.Close(); err != nil {
		t.Fatal(err)
	}
	output, code = captureStdoutResult(t, func() int {
		return run([]string{"init", "--run", "global-run", "--dry-run", "--json"})
	})
	if code != 2 || !strings.Contains(output, `"kind": "run_exists"`) {
		t.Fatalf("cross-repository duplicate init dry-run=%d %s", code, output)
	}

	intakePath := writeInput("intake.json", `{"run":"direct-agent","series_bound":1,"units":[{"id":"u1","title":"direct","blockers":[]}]}`)
	output = captureStdout(t, func() int {
		return run([]string{"intake", "--input", intakePath, "--dry-run", "--json"})
	})
	if !strings.Contains(output, `"dry_run": true`) || !strings.Contains(output, `"validated_command": "intake"`) {
		t.Fatalf("intake projection=%s", output)
	}
	output = captureStdout(t, func() int { return run([]string{"intake", "--input", intakePath, "--json"}) })
	assertJSONState(t, output, "INTAKE")
	output = captureStdout(t, func() int {
		return run([]string{"status", "--run", "direct-agent", "--fields", "state,next_action", "--json"})
	})
	var masked map[string]any
	if err := json.Unmarshal([]byte(output), &masked); err != nil || len(masked) != 2 {
		t.Fatalf("direct mask=%+v err=%v %s", masked, err, output)
	}
	if _, code = captureStdoutResult(t, func() int {
		return run([]string{"status", "--run", "direct-agent", "--fields", "missing", "--json"})
	}); code != 2 {
		t.Fatalf("unknown field code=%d", code)
	}
	if code := run([]string{"status", "--run", "direct-agent", "--fields", "state"}); code != 2 {
		t.Fatalf("plain fields code=%d", code)
	}

	releasePath := writeInput("release.json", `{"run":"direct-agent","revision":2}`)
	captureStdout(t, func() int { return run([]string{"release", "--input", releasePath, "--json"}) })
	buildPath := writeInput("build.json", `{"run":"direct-agent"}`)
	captureStdout(t, func() int { return run([]string{"build", "--input", buildPath, "--json"}) })
	output = captureStdout(t, func() int {
		return run([]string{"verify", "--cmd", "exit 99", "--run", "direct-agent", "--dry-run", "--json"})
	})
	assertJSONState(t, output, "BUILD")
	if !strings.Contains(output, `"dry_run": true`) {
		t.Fatalf("verify direct projection=%s", output)
	}
}

func captureStdout(t *testing.T, fn func() int) string {
	t.Helper()
	output, code := captureStdoutResult(t, fn)
	if code != 0 {
		t.Fatalf("captured command exit=%d output=%s", code, output)
	}
	return output
}

func captureStdoutResult(t *testing.T, fn func() int) (string, int) {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	previous := os.Stdout
	os.Stdout = writer
	code := fn()
	os.Stdout = previous
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	output, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	return string(output), code
}

func captureCode(t *testing.T, fn func() int) int {
	t.Helper()
	_, code := captureStdoutResult(t, fn)
	return code
}

func captureStderrResult(t *testing.T, fn func() (int, string)) (string, int, string) {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	previous := os.Stderr
	os.Stderr = writer
	code, digest := fn()
	os.Stderr = previous
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	output, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	return string(output), code, digest
}

func TestAgentDXRawInputsDryRunAndFieldMask(t *testing.T) {
	h := newCLIHarness(t)
	out := h.mustInput(`{"run":"agent"}`, "init", "--input", "-", "--json")
	assertJSONState(t, out, "INTAKE")

	intake := `{"run":"agent","delivery_mode":"pr-hold","required_reviewers":["autoreview"],"series_bound":1,"units":[{"id":"u1","title":"Agent DX","blockers":[]}]}`
	dry := h.mustInput(intake, "intake", "--input", "-", "--dry-run", "--json")
	var projection struct {
		DryRun              bool   `json:"dry_run"`
		ValidatedCommand    string `json:"validated_command"`
		IntakeRevision      int64  `json:"intake_revision"`
		OutcomeUndetermined bool   `json:"outcome_undetermined"`
	}
	if err := json.Unmarshal([]byte(dry), &projection); err != nil || !projection.DryRun || projection.ValidatedCommand != "intake" || projection.IntakeRevision != 2 {
		t.Fatalf("dry-run projection: %v %+v\n%s", err, projection, dry)
	}
	masked := h.must("status", "--json", "--run", "agent", "--fields", "state,intake_revision,next_action")
	var fields map[string]any
	if err := json.Unmarshal([]byte(masked), &fields); err != nil || len(fields) != 3 || fields["intake_revision"] != float64(1) {
		t.Fatalf("field mask: %v %+v\n%s", err, fields, masked)
	}

	out = h.mustInput(intake, "intake", "--input", "-", "--json")
	var intakeStatus struct {
		IntakeRevision int64 `json:"intake_revision"`
	}
	if err := json.Unmarshal([]byte(out), &intakeStatus); err != nil || intakeStatus.IntakeRevision != 2 {
		t.Fatalf("intake: %v %+v\n%s", err, intakeStatus, out)
	}
	h.mustInput(`{"run":"agent","question":"continue?"}`, "ask", "--input", "-", "--json")
	h.mustInput(`{"run":"agent","answer":"continue"}`, "decide", "--input", "-", "--json")
	h.mustInput(`{"run":"agent","revision":2}`, "release", "--input", "-", "--json")
	h.mustInput(`{"run":"agent"}`, "build", "--input", "-", "--json")

	marker := filepath.Join(t.TempDir(), "must-not-exist")
	dry = h.must("verify", "--cmd", "touch "+marker, "--run", "agent", "--dry-run", "--json")
	if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("dry-run executed command: %v", err)
	}
	if err := json.Unmarshal([]byte(dry), &projection); err != nil || !projection.DryRun || projection.ValidatedCommand != "verify" || !projection.OutcomeUndetermined {
		t.Fatalf("verify dry-run: %v %+v\n%s", err, projection, dry)
	}
	h.mustInput(`{"run":"agent","reason":"dependency unavailable"}`, "block", "--input", "-", "--json")
	h.mustInput(`{"run":"agent","reason":"dependency restored"}`, "retry", "--input", "-", "--json")
	out = h.must("verify", "--cmd", "printf noisy; printf error >&2", "--run", "agent", "--json")
	assertJSONState(t, out, "REVIEW")
	h.mustInput(`{"run":"agent"}`, "rework", "--input", "-", "--json")
	h.mustInput(`{"run":"agent"}`, "build", "--input", "-", "--json")
	h.mustInput(`{"run":"agent","command":"go test ./...","exit_code":0}`, "verify", "--input", "-", "--json")
	h.mustInput(`{"run":"agent","reviewer":"autoreview","verdict":"clean","artifact_ref":"autoreview://local"}`, "review", "--input", "-", "--json")
	out = h.mustInput(`{"run":"agent","delivery_mode":"pr-hold","pr_url":"https://example.com/pr/1"}`, "deliver", "--input", "-", "--json")
	assertJSONState(t, out, "AWAITING_SIGNALS")
	out = h.mustInput(`{"run":"agent","signal":"merged","reference":"https://example.com/pr/1"}`, "observe", "--input", "-", "--json")
	assertJSONState(t, out, "RUN_DONE")
	if status := gitOutput(t, h.repoDir, "status", "--porcelain"); status != "" {
		t.Fatalf("stdin workflow left repository artifacts: %q", status)
	}

	out, code := h.runInput(`{"run":"agent"}`, "build", "--input", "-", "--run", "agent", "--json")
	if code != 2 || !strings.Contains(out, `"kind": "invalid_input"`) {
		t.Fatalf("mixed raw input exit=%d output=%s", code, out)
	}
}

func TestDryRunInitDoesNotCreateState(t *testing.T) {
	h := newCLIHarness(t)
	out := h.must("init", "--run", "dry", "--dry-run", "--json")
	assertJSONState(t, out, "INTAKE")
	if _, err := os.Stat(h.db); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("dry-run created database: %v", err)
	}
}

func TestCLIRejectsAgentHallucinationIDs(t *testing.T) {
	h := newCLIHarness(t)
	for _, runID := range []string{"../escape", "run?query", "run#fragment", "run%2e%2e", "run\nnext", "-flag"} {
		out, code := h.run("init", "--run", runID, "--json")
		if code != 2 || !strings.Contains(out, `"kind": "invalid_input"`) {
			t.Fatalf("run id %q exit=%d output=%s", runID, code, out)
		}
	}
	h.must("init", "--run", "valid-run")
	out, code := h.runInput(`{"run":"valid-run","units":[{"id":"../unit","title":"bad","blockers":[]}]}`, "intake", "--input", "-", "--json")
	if code != 2 || !strings.Contains(out, "path traversal") {
		t.Fatalf("unit id exit=%d output=%s", code, out)
	}
}

func assertJSONState(t *testing.T, output, want string) {
	t.Helper()
	var doc struct {
		State string `json:"state"`
	}
	if err := json.Unmarshal([]byte(output), &doc); err != nil || doc.State != want {
		t.Fatalf("state=%q want=%q err=%v\n%s", doc.State, want, err, output)
	}
}

func TestCLIMultiUnitAskDecide(t *testing.T) {
	h := newCLIHarness(t)
	h.must("init", "--run", "multi")

	intake := filepath.Join(t.TempDir(), "intake.json")
	mustWrite(t, intake, `{
		"required_reviewers":["bugbot"],
		"series_bound":2,
		"units":[
			{"id":"u1","title":"first"},
			{"id":"u2","title":"second","blockers":["u1"]}
		]
	}`)
	h.must("intake", "--file", intake, "--run", "multi")

	out := h.must("status", "--json", "--run", "multi")
	var st struct {
		IntakeRevision int64 `json:"intake_revision"`
	}
	if err := json.Unmarshal([]byte(out), &st); err != nil {
		t.Fatal(err)
	}
	h.must("release", "--revision", itoa(st.IntakeRevision), "--run", "multi")

	// Interrupt before first build.
	h.must("ask", "--question", "keep pr-hold?", "--run", "multi")
	out = h.must("status", "--json", "--run", "multi")
	var parked struct {
		State            string `json:"state"`
		DecisionQuestion string `json:"decision_question"`
	}
	if err := json.Unmarshal([]byte(out), &parked); err != nil {
		t.Fatal(err)
	}
	if parked.State != "NEEDS_DECISION" || parked.DecisionQuestion == "" {
		t.Fatalf("ask park: %s", out)
	}
	h.must("decide", "--answer", "yes pr-hold", "--run", "multi")

	walkUnit := func(unit string, n int) {
		t.Helper()
		h.must("build", "--run", "multi")
		h.must("verify", "--cmd", "true", "--run", "multi")
		rev := filepath.Join(t.TempDir(), "review-"+unit+".json")
		mustWrite(t, rev, `{"reviewer":"bugbot","verdict":"clean","artifact_ref":"test://`+unit+`"}`)
		h.must("review", "--evidence", rev, "--run", "multi")
		del := filepath.Join(t.TempDir(), "deliver-"+unit+".json")
		mustWrite(t, del, `{"delivery_mode":"pr-hold","pr_url":"https://example.com/pr/`+itoa(int64(n))+`"}`)
		h.must("deliver", "--evidence", del, "--run", "multi")
	}
	walkUnit("u1", 1)
	// u1 is delivered, not settled: u2 builds while u1 awaits signals.
	walkUnit("u2", 2)
	// Two delivered units: an unqualified observe is ambiguous.
	if out, code := h.run("observe", "--signal", "merged", "--run", "multi"); code != 2 || !strings.Contains(out, "pass observe.unit") {
		t.Fatalf("ambiguous observe exit=%d output=%s", code, out)
	}
	h.must("observe", "--signal", "merged", "--unit", "u1", "--run", "multi")
	h.must("observe", "--signal", "merged", "--unit", "u2", "--run", "multi")

	out = h.must("status", "--json", "--run", "multi")
	var fin struct {
		State          string `json:"state"`
		CompletedUnits int    `json:"completed_units"`
	}
	if err := json.Unmarshal([]byte(out), &fin); err != nil {
		t.Fatal(err)
	}
	if fin.State != "RUN_DONE" || fin.CompletedUnits != 2 {
		t.Fatalf("want RUN_DONE completed=2 got %s", out)
	}
}

func TestCLIFailClosedGuards(t *testing.T) {
	h := newCLIHarness(t)
	h.must("init", "--run", "a")
	h.must("init", "--run", "b")

	_, code := h.run("status", "--json")
	if code != 5 {
		t.Fatalf("ambiguous status want exit 5 got %d", code)
	}

	intake := filepath.Join(t.TempDir(), "intake.json")
	mustWrite(t, intake, `{
		"required_reviewers":["autoreview"],
		"series_bound":1,
		"units":[{"id":"u1","title":"one"}]
	}`)
	h.must("intake", "--file", intake, "--run", "a")

	_, code = h.run("build", "--run", "a")
	if code != 3 {
		t.Fatalf("build before release want exit 3 got %d", code)
	}

	// Internal state fields are rejected at the intake boundary.
	var out string
	for _, spoofed := range []struct{ field, value string }{{"attempt", "9"}, {"done", "true"}} {
		field, value := spoofed.field, spoofed.value
		spoof := filepath.Join(t.TempDir(), "spoof-"+field+".json")
		mustWrite(t, spoof, fmt.Sprintf(`{
			"required_reviewers":["autoreview"],
			"series_bound":1,
			"units":[{"id":"u1","title":"one",%q:%s}]
		}`, field, value))
		out, code = h.run("intake", "--file", spoof, "--run", "a")
		if code != 2 || !strings.Contains(out, fmt.Sprintf(`unknown JSON field %q`, field)) {
			t.Fatalf("spoofed %s: want strict rejection, got exit %d %s", field, code, out)
		}
	}
	out = h.must("status", "--json", "--run", "a")
	var after struct {
		Released       bool     `json:"released"`
		Frontier       []string `json:"frontier"`
		IntakeRevision int64    `json:"intake_revision"`
	}
	if err := json.Unmarshal([]byte(out), &after); err != nil {
		t.Fatal(err)
	}
	if after.Released {
		t.Fatal("intake should leave run unreleased")
	}
	if len(after.Frontier) != 1 || after.Frontier[0] != "u1" {
		t.Fatalf("rejected intake changed frontier: %s", out)
	}

	h.must("release", "--revision", itoa(after.IntakeRevision), "--run", "a")
	h.must("build", "--run", "a")
	h.must("verify", "--cmd", "true", "--run", "a")

	emptyRev := filepath.Join(t.TempDir(), "empty-review.json")
	mustWrite(t, emptyRev, `{"reviewer":"autoreview","verdict":"clean"}`)
	out, code = h.run("review", "--evidence", emptyRev, "--run", "a")
	if code != 3 {
		t.Fatalf("empty artifact_ref want exit 3 got %d %s", code, out)
	}

	// Consent mismatch: bugbot evidence under autoreview consent.
	bad := filepath.Join(t.TempDir(), "bad-review.json")
	mustWrite(t, bad, `{"reviewer":"bugbot","verdict":"clean","artifact_ref":"x"}`)
	out, code = h.run("review", "--evidence", bad, "--run", "a")
	if code != 3 {
		t.Fatalf("consent mismatch want exit 3 got %d %s", code, out)
	}
}

func TestParseFlags(t *testing.T) {
	_, err := parseFlagsWith([]string{"--revision", "--run", "smoke"}, commandFlags["release"])
	if err == nil {
		t.Fatal("expected error when --revision has no value")
	}
	got, err := parseFlagsWith([]string{"--run=smoke", "--revision=2"}, commandFlags["release"])
	if err != nil {
		t.Fatal(err)
	}
	if got["run"] != "smoke" || got["revision"] != "2" {
		t.Fatalf("got %#v", got)
	}
	got, err = parseFlagsWith([]string{"--run=--json"}, commandFlags["build"])
	if err != nil || got["run"] != "--json" {
		t.Fatalf("flag-like run value: %#v %v", got, err)
	}
	_, err = parseFlagsWith([]string{"--nope", "x"}, commandFlags["release"])
	if err == nil {
		t.Fatal("expected unknown flag error")
	}
	_, err = parseFlagsWith([]string{"--addr", "127.0.0.1:7780"}, commandFlags["status"])
	if err == nil {
		t.Fatal("expected --addr unknown outside serve")
	}
	got, err = parseFlagsWith([]string{"--addr", "127.0.0.1:7780"}, commandFlags["serve"])
	if err != nil || got["addr"] != "127.0.0.1:7780" {
		t.Fatalf("serve flags: %#v %v", got, err)
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func mustWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func itoa(v int64) string {
	return strconv.FormatInt(v, 10)
}

func TestUsageMentionsStorageAndServe(t *testing.T) {
	got := usage()
	for _, command := range []string{"slopshipper storage", "slopshipper serve"} {
		if !strings.Contains(got, command) {
			t.Fatalf("usage missing %s:\n%s", command, got)
		}
	}
}

func TestCommandHelpAndScopedFlags(t *testing.T) {
	help, ok := commandUsage("review")
	if !ok || !strings.Contains(help, "Verdicts: clean, findings, ambiguous") {
		t.Fatalf("review help: %q", help)
	}
	if _, err := parseFlagsWith([]string{"--evidence", "x"}, commandFlags["init"]); err == nil {
		t.Fatal("init accepted review evidence")
	}
	if _, err := parseFlagsWith([]string{"--json=false"}, commandFlags["status"]); err == nil {
		t.Fatal("boolean flag accepted a value")
	}
}

func TestVerifyPreflightAndRecovery(t *testing.T) {
	h := newCLIHarness(t)
	h.must("init", "--run", "verify")
	marker := filepath.Join(t.TempDir(), "should-not-exist")
	out, code := h.run("verify", "--cmd", "touch "+marker, "--run", "verify")
	if code != 3 || !strings.Contains(out, "not allowed from INTAKE") {
		t.Fatalf("preflight: exit %d %s", code, out)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("illegal verify executed command: %v", err)
	}

}

func TestVerifyFailureReturnsNonzeroAndCanRetry(t *testing.T) {
	h := newCLIHarness(t)
	h.must("init", "--run", "retry")
	intake := `{"series_bound":1,"units":[{"id":"u1","title":"one","blockers":[]}]}`
	out, code := h.runInput(intake, "intake", "--file", "-", "--run", "retry")
	if code != 0 {
		t.Fatalf("stdin intake: %d %s", code, out)
	}
	statusJSON := h.must("status", "--json", "--run", "retry")
	var statusDoc struct {
		IntakeRevision int64 `json:"intake_revision"`
	}
	if err := json.Unmarshal([]byte(statusJSON), &statusDoc); err != nil {
		t.Fatal(err)
	}
	h.must("release", "--revision", itoa(statusDoc.IntakeRevision), "--run", "retry")
	h.must("build", "--run", "retry")
	out, code = h.run("verify", "--cmd", "false", "--run", "retry")
	if code != 6 || !strings.Contains(out, "state=BLOCKED") || !strings.Contains(out, "slopshipper retry") {
		t.Fatalf("failed verify: exit %d %s", code, out)
	}
	out = h.must("retry", "--reason", "fixed the failure", "--run", "retry")
	if !strings.Contains(out, "state=BUILD") {
		t.Fatalf("retry: %s", out)
	}
}

func TestVerifyEvidenceRequiresExplicitExitCode(t *testing.T) {
	h := newCLIHarness(t)
	h.must("init", "--run", "evidence")
	intake := filepath.Join(t.TempDir(), "intake.json")
	mustWrite(t, intake, `{"units":[{"id":"u1","title":"one"}]}`)
	h.must("intake", "--file", intake, "--run", "evidence")
	out := h.must("status", "--json", "--run", "evidence")
	var statusDoc struct {
		IntakeRevision int64 `json:"intake_revision"`
	}
	if err := json.Unmarshal([]byte(out), &statusDoc); err != nil {
		t.Fatal(err)
	}
	h.must("release", "--revision", itoa(statusDoc.IntakeRevision), "--run", "evidence")
	h.must("build", "--run", "evidence")
	missing := filepath.Join(t.TempDir(), "verify.json")
	mustWrite(t, missing, `{"command":"true"}`)
	out, code := h.run("verify", "--evidence", missing, "--run", "evidence")
	if code != 2 || !strings.Contains(out, "verify.exit_code is required") {
		t.Fatalf("missing exit code: %d %s", code, out)
	}
}

func TestVerifyEvidenceRejectsBlankFailedCommandWithoutBlocking(t *testing.T) {
	h := newCLIHarness(t)
	h.must("init", "--run", "blank-failure")
	intake := filepath.Join(t.TempDir(), "intake.json")
	mustWrite(t, intake, `{"units":[{"id":"u1","title":"one"}]}`)
	h.must("intake", "--file", intake, "--run", "blank-failure")
	h.must("release", "--revision", "2", "--run", "blank-failure")
	h.must("build", "--run", "blank-failure")
	evidence := filepath.Join(t.TempDir(), "verify.json")
	mustWrite(t, evidence, `{"command":"   ","exit_code":1}`)
	out, code := h.run("verify", "--evidence", evidence, "--run", "blank-failure")
	if code != 3 || !strings.Contains(out, "verify.command required") {
		t.Fatalf("blank failure: exit %d %s", code, out)
	}
	out = h.must("status", "--json", "--run", "blank-failure")
	var statusDoc struct {
		State string `json:"state"`
	}
	if err := json.Unmarshal([]byte(out), &statusDoc); err != nil || statusDoc.State != "BUILD" {
		t.Fatalf("blank failure changed state: err=%v %s", err, out)
	}
}

func TestRunEntryPointsAndFullRecoveryFlow(t *testing.T) {
	for _, args := range [][]string{
		nil, {"--help"}, {"help"}, {"version"}, {"--version"},
		{"init", "--help"}, {"intake", "-h"}, {"release", "--help"},
		{"build", "--help"}, {"verify", "--help"}, {"review", "--help"},
		{"rework", "--help"}, {"deliver", "--help"}, {"ask", "--help"},
		{"decide", "--help"}, {"retry", "--help"}, {"block", "--help"},
		{"status", "--help"}, {"storage", "--help"}, {"serve", "--help"},
		{"version", "--help"},
	} {
		if code := run(args); code != 0 {
			t.Fatalf("run(%v)=%d", args, code)
		}
	}
	if code := run([]string{"unknown", "--help"}); code != 2 {
		t.Fatalf("unknown help=%d", code)
	}
	if code := run([]string{"storage", "--dry-run"}); code != 2 {
		t.Fatalf("storage dry-run=%d", code)
	}
	if code := run([]string{"version", "--run", "unexpected"}); code != 2 {
		t.Fatalf("version accepted scoped flag: %d", code)
	}
	if code := run([]string{"--version", "unexpected"}); code != 2 {
		t.Fatalf("--version accepted argument: %d", code)
	}

	repoDir := t.TempDir()
	runGit(t, repoDir, "init")
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repoDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldDir) })
	t.Setenv("SLOPSHIPPER_DB", filepath.Join(t.TempDir(), "direct.sqlite"))

	if code := run([]string{"unknown"}); code != 2 {
		t.Fatalf("unknown=%d", code)
	}
	if code := run([]string{"init", "--run", "direct"}); code != 0 {
		t.Fatalf("init=%d", code)
	}
	for _, args := range [][]string{
		{"intake", "--run", "direct"},
		{"release", "--run", "direct"},
		{"release", "--revision", "nope", "--run", "direct"},
		{"verify", "--run", "direct"},
		{"verify", "--cmd", "true", "--evidence", "x", "--run", "direct"},
		{"review", "--run", "direct"}, {"deliver", "--run", "direct"},
		{"ask", "--run", "direct"}, {"decide", "--run", "direct"},
		{"retry", "--run", "direct"}, {"block", "--run", "direct"},
	} {
		if code := run(args); code != 2 {
			t.Fatalf("missing args %v=%d", args, code)
		}
	}

	intake := filepath.Join(t.TempDir(), "intake.json")
	mustWrite(t, intake, `{"delivery_mode":"direct-trunk","required_reviewers":["autoreview","bugbot"],"series_bound":1,"units":[{"id":"u1","title":"one"}]}`)
	if code := run([]string{"intake", "--file", intake, "--run", "direct"}); code != 0 {
		t.Fatalf("intake=%d", code)
	}
	if code := run([]string{"status", "--json", "--run", "direct"}); code != 0 {
		t.Fatalf("status=%d", code)
	}
	if code := run([]string{"release", "--revision", "2", "--run", "direct"}); code != 0 {
		t.Fatalf("release=%d", code)
	}
	if code := run([]string{"build", "--run", "direct"}); code != 0 {
		t.Fatalf("build=%d", code)
	}
	if code := run([]string{"block", "--reason", "dependency unavailable", "--run", "direct"}); code != 0 {
		t.Fatalf("block=%d", code)
	}
	if code := run([]string{"retry", "--reason", "dependency restored", "--run", "direct"}); code != 0 {
		t.Fatalf("retry=%d", code)
	}
	verify := filepath.Join(t.TempDir(), "verify.json")
	mustWrite(t, verify, `{"command":"go test ./...","exit_code":0,"output_digest":"sha256:test"}`)
	if code := run([]string{"verify", "--evidence", verify, "--run", "direct"}); code != 0 {
		t.Fatalf("verify=%d", code)
	}
	if code := run([]string{"rework", "--run", "direct"}); code != 0 {
		t.Fatalf("rework=%d", code)
	}
	if code := run([]string{"build", "--run", "direct"}); code != 0 {
		t.Fatalf("rebuild=%d", code)
	}
	if code := run([]string{"verify", "--cmd", "true", "--run", "direct"}); code != 0 {
		t.Fatalf("reverify=%d", code)
	}
	if code := runReviewInput(t, "direct", `{"reviewer":"autoreview","verdict":"ambiguous","artifact_ref":"review://ambiguous"}`); code != 0 {
		t.Fatalf("ambiguous review=%d", code)
	}
	if code := run([]string{"decide", "--answer", "continue review", "--run", "direct"}); code != 0 {
		t.Fatalf("decide=%d", code)
	}
	if code := runReviewInput(t, "direct", `{"reviewer":"autoreview","verdict":"clean","artifact_ref":"review://autoreview"}`); code != 0 {
		t.Fatalf("autoreview=%d", code)
	}
	if code := runReviewInput(t, "direct", `{"reviewer":"bugbot","verdict":"clean","artifact_ref":"review://bugbot"}`); code != 0 {
		t.Fatalf("bugbot=%d", code)
	}
	delivery := filepath.Join(t.TempDir(), "delivery.json")
	mustWrite(t, delivery, `{"delivery_mode":"direct-trunk","commit_sha":"abc123"}`)
	if code := run([]string{"deliver", "--evidence", delivery, "--run", "direct"}); code != 0 {
		t.Fatalf("deliver=%d", code)
	}
}

func TestRunAutoIDAndDefaultDataPath(t *testing.T) {
	repoDir := t.TempDir()
	runGit(t, repoDir, "init")
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repoDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldDir) })
	t.Setenv("SLOPSHIPPER_DB", "")
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	if code := run([]string{"init"}); code != 0 {
		t.Fatalf("auto init=%d", code)
	}
}

func TestCLIFirstRunCreatesDefaultStateAndReturnsBootstrapStatus(t *testing.T) {
	h := newCLIHarness(t)
	stateRoot := filepath.Join(t.TempDir(), "new", "xdg")
	h.env = append(h.env,
		"SLOPSHIPPER_DB=",
		"XDG_DATA_HOME="+stateRoot,
		"HOME="+filepath.Join(t.TempDir(), "home"),
	)

	out, code := h.run("status", "--json")
	if code != 0 {
		t.Fatalf("first status exit=%d: %s", code, out)
	}
	var bootstrap struct {
		State           string   `json:"state"`
		AllowedCommands []string `json:"allowed_commands"`
		NextAction      string   `json:"next_action"`
		Blocker         string   `json:"blocker"`
	}
	if err := json.Unmarshal([]byte(out), &bootstrap); err != nil {
		t.Fatalf("bootstrap JSON: %v\n%s", err, out)
	}
	if bootstrap.State != "UNINITIALIZED" || bootstrap.NextAction != "slopshipper init" ||
		len(bootstrap.AllowedCommands) != 1 || bootstrap.AllowedCommands[0] != "init" || bootstrap.Blocker == "" {
		t.Fatalf("bootstrap status: %+v", bootstrap)
	}
	databasePath := filepath.Join(stateRoot, "slopshipper", "slopshipper.sqlite")
	if info, err := os.Stat(databasePath); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("database info=%v err=%v", info, err)
	}

	h.must("init", "--run", "first")
	out = h.must("status", "--json", "--run", "first")
	var initialized struct {
		State string `json:"state"`
	}
	if err := json.Unmarshal([]byte(out), &initialized); err != nil || initialized.State != "INTAKE" {
		t.Fatalf("initialized status: %v %s", err, out)
	}
}

func TestCLIRegisteredCustomReviewerFlow(t *testing.T) {
	h := newCLIHarness(t)
	h.must("init", "--run", "custom")

	var registry struct {
		Builtin    []string `json:"builtin"`
		Registered []string `json:"registered"`
		DryRun     bool     `json:"dry_run"`
	}
	out := h.must("reviewers", "--json")
	if err := json.Unmarshal([]byte(out), &registry); err != nil ||
		len(registry.Builtin) != 2 || len(registry.Registered) != 0 {
		t.Fatalf("initial registry: %v %s", err, out)
	}

	// Built-ins are immutable; unregistered requirements fail closed at intake.
	if out, code := h.run("reviewers", "--add", "autoreview", "--json"); code != 2 || !strings.Contains(out, "built-in") {
		t.Fatalf("builtin add exit=%d output=%s", code, out)
	}
	if out, code := h.run("reviewers", "--add", "slopzapper", "--remove", "slopzapper"); code != 2 {
		t.Fatalf("add+remove exit=%d output=%s", code, out)
	}
	intake := `{"run":"custom","delivery_mode":"pr-hold","required_reviewers":["slopzapper","autoreview"],"series_bound":1,"units":[{"id":"u1","title":"one","blockers":[]}]}`
	if out, code := h.runInput(intake, "intake", "--input", "-", "--json"); code != 2 || !strings.Contains(out, "not registered") {
		t.Fatalf("unregistered intake exit=%d output=%s", code, out)
	}

	// A dry run projects the registry without persisting it.
	out = h.must("reviewers", "--add", "slopzapper", "--dry-run", "--json")
	if err := json.Unmarshal([]byte(out), &registry); err != nil || !registry.DryRun ||
		len(registry.Registered) != 1 || registry.Registered[0] != "slopzapper" {
		t.Fatalf("dry-run projection: %v %s", err, out)
	}
	out = h.must("reviewers", "--json")
	if err := json.Unmarshal([]byte(out), &registry); err != nil || len(registry.Registered) != 0 {
		t.Fatalf("dry run persisted state: %v %s", err, out)
	}

	h.must("reviewers", "--add", "slopzapper")
	h.must("reviewers", "--add", "slopzapper") // idempotent
	if out := h.must("reviewers"); !strings.Contains(out, "registered=slopzapper") {
		t.Fatalf("plain registry output: %s", out)
	}

	h.mustInput(intake, "intake", "--input", "-", "--json")
	h.must("release", "--revision", "2", "--run", "custom")
	h.must("build", "--run", "custom")
	h.must("verify", "--cmd", "true", "--run", "custom")

	// A registered reviewer outside the required set is refused.
	if out, code := h.runInput(`{"run":"custom","reviewer":"bugbot","verdict":"clean","artifact_ref":"bugbot://x"}`,
		"review", "--input", "-", "--json"); code != 3 || !strings.Contains(out, "not a required reviewer") {
		t.Fatalf("non-required reviewer exit=%d output=%s", code, out)
	}

	out = h.mustInput(`{"run":"custom","reviewer":"slopzapper","verdict":"clean","artifact_ref":"slopzapper://run/1"}`,
		"review", "--input", "-", "--json")
	var afterFirst struct {
		State              string   `json:"state"`
		CompletedReviewers []string `json:"completed_reviewers"`
	}
	if err := json.Unmarshal([]byte(out), &afterFirst); err != nil ||
		afterFirst.State != "REVIEW" || len(afterFirst.CompletedReviewers) != 1 {
		t.Fatalf("first custom review: %v %s", err, out)
	}
	out = h.mustInput(`{"run":"custom","reviewer":"autoreview","verdict":"clean","artifact_ref":"autoreview://approval"}`,
		"review", "--input", "-", "--json")
	if err := json.Unmarshal([]byte(out), &afterFirst); err != nil || afterFirst.State != "DELIVER" {
		t.Fatalf("second review: %v %s", err, out)
	}
	h.mustInput(`{"run":"custom","delivery_mode":"pr-hold","pr_url":"https://example.com/pr/9"}`,
		"deliver", "--input", "-", "--json")

	// Flag and identity validation fail closed.
	if out, code := h.run("reviewers", "--dry-run"); code != 2 || !strings.Contains(out, "--add or --remove") {
		t.Fatalf("bare dry-run exit=%d output=%s", code, out)
	}
	if out, code := h.run("reviewers", "--add", "bad..name", "--json"); code != 2 || !strings.Contains(out, "reviewer identity") {
		t.Fatalf("invalid identity exit=%d output=%s", code, out)
	}

	// A remove dry run projects the shrunken registry without persisting.
	if out := h.must("reviewers", "--remove", "slopzapper", "--dry-run"); !strings.Contains(out, "dry-run") ||
		!strings.Contains(out, "registered=(none)") {
		t.Fatalf("remove dry-run plain output: %s", out)
	}
	out = h.must("reviewers", "--json")
	if err := json.Unmarshal([]byte(out), &registry); err != nil || len(registry.Registered) != 1 {
		t.Fatalf("remove dry run persisted: %v %s", err, out)
	}

	// Registry removal is idempotent and fails closed at the next release.
	h.must("reviewers", "--remove", "slopzapper")
	h.must("reviewers", "--remove", "slopzapper")
}

func TestRunReviewersRegistryAndCustomFlowInProcess(t *testing.T) {
	repoDir := t.TempDir()
	runGit(t, repoDir, "init")
	withWorkingDirectory(t, repoDir)
	t.Setenv("SLOPSHIPPER_DB", filepath.Join(t.TempDir(), "registry.sqlite"))

	if code := run([]string{"init", "--run", "reg"}); code != 0 {
		t.Fatalf("init=%d", code)
	}
	out, code := captureStdoutResult(t, func() int { return run([]string{"reviewers", "--json"}) })
	if code != 0 || !strings.Contains(out, `"builtin"`) || !strings.Contains(out, `"registered": []`) {
		t.Fatalf("registry list exit=%d output=%s", code, out)
	}
	if out, code = captureStdoutResult(t, func() int { return run([]string{"reviewers", "--add", "autoreview"}) }); code != 2 {
		t.Fatalf("builtin add exit=%d output=%s", code, out)
	}
	if out, code = captureStdoutResult(t, func() int {
		return run([]string{"reviewers", "--add", "slopzapper", "--remove", "slopzapper"})
	}); code != 2 {
		t.Fatalf("conflicting flags exit=%d output=%s", code, out)
	}
	if out, code = captureStdoutResult(t, func() int { return run([]string{"reviewers", "--dry-run", "--json"}) }); code != 2 {
		t.Fatalf("bare dry-run exit=%d output=%s", code, out)
	}
	if out, code = captureStdoutResult(t, func() int { return run([]string{"reviewers", "--add", "bad..name"}) }); code != 2 {
		t.Fatalf("invalid identity exit=%d output=%s", code, out)
	}
	if out, code = captureStdoutResult(t, func() int {
		return run([]string{"reviewers", "--add", "slopzapper", "--dry-run", "--json"})
	}); code != 0 || !strings.Contains(out, `"dry_run": true`) || !strings.Contains(out, "slopzapper") {
		t.Fatalf("dry-run add exit=%d output=%s", code, out)
	}
	if out, code = captureStdoutResult(t, func() int { return run([]string{"reviewers", "--add", "slopzapper"}) }); code != 0 ||
		!strings.Contains(out, "registered=slopzapper") {
		t.Fatalf("add exit=%d output=%s", code, out)
	}
	if out, code = captureStdoutResult(t, func() int {
		return run([]string{"reviewers", "--remove", "slopzapper", "--dry-run"})
	}); code != 0 || !strings.Contains(out, "dry-run") || !strings.Contains(out, "registered=(none)") {
		t.Fatalf("dry-run remove exit=%d output=%s", code, out)
	}

	intake := filepath.Join(t.TempDir(), "intake.json")
	mustWrite(t, intake, `{
		"required_reviewers":["slopzapper","autoreview"],
		"risk_tier":"high",
		"budget":{"tokens":250000,"minutes":45},
		"series_bound":1,
		"units":[{"id":"u1","title":"one","complexity":"medium","acceptance_criteria":["registry gate enforced"]}]
	}`)
	if code := run([]string{"intake", "--file", intake, "--run", "reg"}); code != 0 {
		t.Fatalf("intake=%d", code)
	}
	emptyComplexity := filepath.Join(t.TempDir(), "empty-complexity.json")
	mustWrite(t, emptyComplexity, `{"units":[{"id":"e1","title":"e","complexity":""}]}`)
	if out, code := captureStdoutResult(t, func() int {
		return run([]string{"intake", "--file", emptyComplexity, "--run", "reg"})
	}); code != 2 {
		t.Fatalf("explicit empty complexity exit=%d output=%s", code, out)
	}
	zeroBudget := filepath.Join(t.TempDir(), "zero-budget.json")
	mustWrite(t, zeroBudget, `{"budget":{"tokens":0},"units":[{"id":"z1","title":"z"}]}`)
	if out, code := captureStdoutResult(t, func() int {
		return run([]string{"intake", "--file", zeroBudget, "--run", "reg"})
	}); code != 2 {
		t.Fatalf("explicit zero budget exit=%d output=%s", code, out)
	}
	contract, code := captureStdoutResult(t, func() int {
		return run([]string{"status", "--json", "--fields", "risk_tier,budget_tokens,budget_minutes", "--run", "reg"})
	})
	if code != 0 || !strings.Contains(contract, `"risk_tier": "high"`) ||
		!strings.Contains(contract, `"budget_tokens": 250000`) || !strings.Contains(contract, `"budget_minutes": 45`) {
		t.Fatalf("contract status exit=%d output=%s", code, contract)
	}
	if code := run([]string{"release", "--revision", "2", "--run", "reg"}); code != 0 {
		t.Fatalf("release=%d", code)
	}
	if code := run([]string{"build", "--run", "reg"}); code != 0 {
		t.Fatalf("build=%d", code)
	}
	if code := run([]string{"verify", "--cmd", "true", "--run", "reg"}); code != 0 {
		t.Fatalf("verify=%d", code)
	}
	outside := filepath.Join(t.TempDir(), "outside.json")
	mustWrite(t, outside, `{"reviewer":"bugbot","verdict":"clean","artifact_ref":"bugbot://x"}`)
	if code := run([]string{"review", "--evidence", outside, "--run", "reg"}); code != 3 {
		t.Fatalf("non-required reviewer=%d", code)
	}
	first := filepath.Join(t.TempDir(), "first.json")
	mustWrite(t, first, `{"reviewer":"slopzapper","verdict":"clean","artifact_ref":"slopzapper://1"}`)
	if code := run([]string{"review", "--evidence", first, "--run", "reg"}); code != 0 {
		t.Fatalf("custom review=%d", code)
	}
	// The registry lost the custom reviewer: a fresh run requiring it fails
	// closed at intake with a caller-recoverable error.
	if code := run([]string{"reviewers", "--remove", "slopzapper"}); code != 0 {
		t.Fatal("remove failed")
	}
	if code := run([]string{"init", "--run", "reg2"}); code != 0 {
		t.Fatal("second init failed")
	}
	if code := run([]string{"intake", "--file", intake, "--run", "reg2"}); code != 2 {
		t.Fatalf("unregistered intake=%d", code)
	}
}

func TestCLIStoreFailureIncludesResolvedPathAndCause(t *testing.T) {
	h := newCLIHarness(t)
	blockingFile := filepath.Join(t.TempDir(), "not-a-directory")
	mustWrite(t, blockingFile, "blocked")
	databasePath := filepath.Join(blockingFile, "slopshipper.sqlite")
	h.env = append(h.env, "SLOPSHIPPER_DB="+databasePath)

	out, code := h.run("init", "--run", "blocked")
	if code != 2 || !strings.Contains(out, databasePath) || !strings.Contains(out, "not a directory") ||
		!strings.Contains(out, "SLOPSHIPPER_DB") || !strings.Contains(out, "slopshipper storage --json") {
		t.Fatalf("store failure exit=%d output=%s", code, out)
	}

	out, code = h.run("status", "--json")
	if code != 2 {
		t.Fatalf("json store failure exit=%d output=%s", code, out)
	}
	var failure struct {
		OK    bool `json:"ok"`
		Error struct {
			Kind     string `json:"kind"`
			Message  string `json:"message"`
			ExitCode int    `json:"exit_code"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(out), &failure); err != nil {
		t.Fatalf("failure JSON: %v\n%s", err, out)
	}
	if failure.OK || failure.Error.Kind != "state_unavailable" || failure.Error.ExitCode != 2 ||
		!strings.Contains(failure.Error.Message, databasePath) || !strings.Contains(failure.Error.Message, "SLOPSHIPPER_DB") {
		t.Fatalf("failure envelope: %+v", failure)
	}
}

func TestDirectErrorMappingJSONAndServeBindFailure(t *testing.T) {
	for err, want := range map[error]int{
		machine.ErrBadArgs: 2, machine.ErrIllegalTransition: 3, machine.ErrUnmetGuard: 3,
		machine.ErrRevisionConflict: 4, machine.ErrRunExists: 2, machine.ErrAmbiguousRun: 5, machine.ErrNotFound: 5,
		machine.ErrCorruptState: 5, errors.New("storage failed"): 10,
	} {
		if got := mapErr(fmt.Errorf("context: %w", err), runOptions{}); got != want {
			t.Errorf("mapErr(%v)=%d want %d", err, got, want)
		}
	}
	if !flagHas([]string{"--json"}, "--json") || !flagHas([]string{"--json=true"}, "--json") || flagHas(nil, "--json") {
		t.Fatal("flagHas contract")
	}

	repoDir := t.TempDir()
	runGit(t, repoDir, "init")
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repoDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldDir) })
	t.Setenv("SLOPSHIPPER_DB", filepath.Join(t.TempDir(), "errors.sqlite"))

	if code := run([]string{"status", "--run", "missing"}); code != 5 {
		t.Fatalf("missing status=%d", code)
	}
	if code := run([]string{"init", "--run", "errors"}); code != 0 {
		t.Fatalf("init=%d", code)
	}
	if code := run([]string{"status", "--run", "errors"}); code != 0 {
		t.Fatalf("plain status=%d", code)
	}
	if code := run([]string{"build", "--run", "errors"}); code != 3 {
		t.Fatalf("illegal build=%d", code)
	}
	if code := run([]string{"init", "--run", "errors"}); code != 2 {
		t.Fatalf("duplicate init=%d", code)
	}
	for _, args := range [][]string{
		{"intake", "--file", "missing.json", "--run", "errors"},
		{"review", "--evidence", "missing.json", "--run", "errors"},
		{"deliver", "--evidence", "missing.json", "--run", "errors"},
	} {
		if code := run(args); code != 2 {
			t.Fatalf("missing JSON %v=%d", args, code)
		}
	}
	badJSON := filepath.Join(t.TempDir(), "bad.json")
	mustWrite(t, badJSON, `{} {}`)
	if code := run([]string{"intake", "--file", badJSON, "--run", "errors"}); code != 2 {
		t.Fatalf("trailing JSON=%d", code)
	}
	nullJSON := filepath.Join(t.TempDir(), "null.json")
	mustWrite(t, nullJSON, `null`)
	if code := run([]string{"intake", "--file", nullJSON, "--run", "errors"}); code != 2 {
		t.Fatalf("null JSON=%d", code)
	}
	nestedNullJSON := filepath.Join(t.TempDir(), "nested-null.json")
	mustWrite(t, nestedNullJSON, `{"series_bound": null}`)
	if code := run([]string{"intake", "--file", nestedNullJSON, "--run", "errors"}); code != 2 {
		t.Fatalf("nested null JSON=%d", code)
	}
	duplicateJSON := filepath.Join(t.TempDir(), "duplicate.json")
	mustWrite(t, duplicateJSON, `{"series_bound": 1, "series_bound": 2}`)
	if code := run([]string{"intake", "--file", duplicateJSON, "--run", "errors"}); code != 2 {
		t.Fatalf("duplicate JSON=%d", code)
	}
	caseFoldedJSON := filepath.Join(t.TempDir(), "case-folded.json")
	mustWrite(t, caseFoldedJSON, `{"Units": []}`)
	if code := run([]string{"intake", "--file", caseFoldedJSON, "--run", "errors"}); code != 2 {
		t.Fatalf("case-folded JSON=%d", code)
	}
	caseAliasJSON := filepath.Join(t.TempDir(), "case-alias.json")
	mustWrite(t, caseAliasJSON, `{"units": [], "Units": []}`)
	if code := run([]string{"intake", "--file", caseAliasJSON, "--run", "errors"}); code != 2 {
		t.Fatalf("case-alias JSON=%d", code)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	if code := run([]string{"serve", "--addr", ln.Addr().String()}); code != 10 {
		t.Fatalf("serve bind failure=%d", code)
	}
	if code := run([]string{"serve", "--addr", "0.0.0.0:7780"}); code != 2 {
		t.Fatalf("serve unsafe addr=%d", code)
	}
}

func TestResolveRepoKeyMigratesMalformedVersionOneIdentity(t *testing.T) {
	repoDir := t.TempDir()
	runGit(t, repoDir, "init")
	runGit(t, repoDir, "remote", "add", "origin", "https://TOKEN@host:bad?keep=yes")
	key, root, err := repo.Keys(repoDir)
	if err != nil {
		t.Fatal(err)
	}
	legacyKey := "https://TOKEN@host:bad?keep=yes|" + root
	if strings.Contains(key, "TOKEN") {
		t.Fatalf("key=%q", key)
	}

	st, err := store.Open(filepath.Join(t.TempDir(), "legacy.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.CreateRun(machine.NewRun("legacy", legacyKey), nil); err != nil {
		t.Fatal(err)
	}
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repoDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldDir) })
	if got, err := resolveRepoKey(st); err != nil || got != key {
		t.Fatalf("resolveRepoKey=%q err=%v", got, err)
	}
	if got, _, err := st.ResolveStatusRun(key, "legacy"); err != nil || got.RepoKey != key {
		t.Fatalf("migrated run=%+v err=%v", got, err)
	}
	if oldRuns, err := st.ListRuns(legacyKey); err != nil || len(oldRuns) != 0 {
		t.Fatalf("legacy rows=%+v err=%v", oldRuns, err)
	}
}

func runReviewInput(t *testing.T, runID, body string) int {
	t.Helper()
	path := filepath.Join(t.TempDir(), "review.json")
	mustWrite(t, path, body)
	return run([]string{"review", "--evidence", path, "--run", runID})
}
