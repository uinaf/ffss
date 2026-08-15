package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/uinaf/autoreview/internal/protocol"
	"github.com/uinaf/autoreview/internal/provider"
)

type liveProvider struct {
	name       protocol.ProviderName
	executable string
	model      string
	webAccess  bool
}

type liveControl struct {
	name        string
	repository  string
	commit      string
	contract    string
	wantStatus  protocol.Status
	wantExit    int
	defectFiles []string
}

func TestBinaryLiveProviderMatrix(t *testing.T) {
	if os.Getenv("AUTOREVIEW_TEST_LIVE") != "1" {
		t.Skip("run with mise run verify:live")
	}
	if _, err := exec.LookPath("trufflehog"); err != nil {
		t.Fatal("live verification requires trufflehog on PATH")
	}

	providers := selectedLiveProviders(t)
	for _, live := range providers {
		if _, err := exec.LookPath(live.executable); err != nil {
			t.Fatalf("live %s verification requires %s on PATH", live.name, live.executable)
		}
	}
	repeat := liveRepeat(t)
	binary := buildAutoreviewBinary(t)
	cleanRepository, cleanCommit := liveFixtureRepository(t, "after")
	defectiveRepository, defectiveCommit := liveFixtureRepository(t, "defective")
	controls := []liveControl{
		{
			name:       "defective",
			repository: defectiveRepository,
			commit:     defectiveCommit,
			contract: "Review this public synthetic commit for every concrete correctness and resource-management defect in the changed lines. " +
				"CountByOwner must count non-empty input without panicking. Batch must preserve every input in order for every positive size, including a final partial batch and sizes larger than the input. ReadConfig must close every opened file. Report every actionable finding, including test gaps.",
			wantStatus:  protocol.StatusFindings,
			wantExit:    1,
			defectFiles: []string{"batch.go", "config.go", "counts.go"},
		},
		{
			name:       "clean",
			repository: cleanRepository,
			commit:     cleanCommit,
			contract: "Review this public synthetic commit. Mean must return zero for empty input, support positive and negative integers without integer overflow, and leave its input unchanged. " +
				"Report every actionable defect in the changed lines.",
			wantStatus: protocol.StatusClean,
			wantExit:   0,
		},
	}

	for round := 1; round <= repeat; round++ {
		for _, live := range providers {
			for _, control := range controls {
				name := fmt.Sprintf("%s/round-%d/%s", live.name, round, control.name)
				t.Run(name, func(t *testing.T) {
					report := runLiveReview(t, binary, live, control)
					validateLiveReport(t, report, live, control)
				})
			}
		}
	}
}

func selectedLiveProviders(t *testing.T) []liveProvider {
	t.Helper()
	available := []liveProvider{
		{name: protocol.ProviderCodex, executable: "codex", model: provider.DefaultCodexModel},
		{name: protocol.ProviderClaude, executable: "claude", model: provider.DefaultClaudeModel},
		{name: protocol.ProviderCursor, executable: "cursor-agent", model: provider.DefaultCursorModel, webAccess: true},
		{name: protocol.ProviderGrok, executable: "grok", model: provider.DefaultGrokModel},
	}
	selection := strings.TrimSpace(os.Getenv("AUTOREVIEW_LIVE_PROVIDERS"))
	if selection == "" {
		return available
	}
	requested := make(map[protocol.ProviderName]struct{})
	for _, value := range strings.Split(selection, ",") {
		name := protocol.ProviderName(strings.TrimSpace(value))
		if name == "" {
			t.Fatalf("AUTOREVIEW_LIVE_PROVIDERS contains an empty provider")
		}
		requested[name] = struct{}{}
	}
	selected := make([]liveProvider, 0, len(requested))
	for _, live := range available {
		if _, ok := requested[live.name]; ok {
			selected = append(selected, live)
			delete(requested, live.name)
		}
	}
	if len(requested) != 0 {
		unknown := make([]string, 0, len(requested))
		for name := range requested {
			unknown = append(unknown, string(name))
		}
		t.Fatalf("AUTOREVIEW_LIVE_PROVIDERS contains unsupported providers: %s", strings.Join(unknown, ", "))
	}
	return selected
}

func liveRepeat(t *testing.T) int {
	t.Helper()
	value := strings.TrimSpace(os.Getenv("AUTOREVIEW_LIVE_REPEAT"))
	if value == "" {
		return 1
	}
	repeat, err := strconv.Atoi(value)
	if err != nil || repeat < 1 || repeat > 10 {
		t.Fatalf("AUTOREVIEW_LIVE_REPEAT must be an integer from 1 through 10, got %q", value)
	}
	return repeat
}

func liveFixtureRepository(t *testing.T, variant string) (string, string) {
	t.Helper()
	repository := cliRepository(t)
	gitCommand(t, repository, "config", "user.name", "Autoreview Live Test")
	gitCommand(t, repository, "config", "user.email", "autoreview-live@example.invalid")
	fixtureRoot := filepath.Join("..", "..", "testdata", "v0.1-fixture")
	copyLiveFixture(t, filepath.Join(fixtureRoot, "base"), repository)
	gitCommand(t, repository, "add", ".")
	gitCommand(t, repository, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "test: establish live review baseline")
	copyLiveFixture(t, filepath.Join(fixtureRoot, variant), repository)
	gitCommand(t, repository, "add", ".")
	gitCommand(t, repository, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "test: add "+variant+" live review control")
	runLiveFixtureTests(t, repository)
	return repository, gitOutput(t, repository, "rev-parse", "HEAD")
}

func copyLiveFixture(t *testing.T, source, destination string) {
	t.Helper()
	entries, err := os.ReadDir(source)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		sourcePath := filepath.Join(source, entry.Name())
		destinationPath := filepath.Join(destination, entry.Name())
		if entry.IsDir() {
			if err := os.MkdirAll(destinationPath, 0o755); err != nil {
				t.Fatal(err)
			}
			copyLiveFixture(t, sourcePath, destinationPath)
			continue
		}
		info, err := entry.Info()
		if err != nil {
			t.Fatal(err)
		}
		if !info.Mode().IsRegular() {
			t.Fatalf("live fixture contains non-regular file %s", sourcePath)
		}
		content, err := os.ReadFile(sourcePath)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(destinationPath, content, info.Mode().Perm()); err != nil {
			t.Fatal(err)
		}
	}
}

func runLiveFixtureTests(t *testing.T, repository string) {
	t.Helper()
	command := exec.CommandContext(t.Context(), "go", "test", "./...")
	command.Dir = repository
	command.Env = append(os.Environ(), "GOWORK=off")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("live fixture builder verification: %v: %s", err, output)
	}
}

func gitOutput(t *testing.T, repository string, arguments ...string) string {
	t.Helper()
	command := exec.CommandContext(t.Context(), "git", arguments...)
	command.Dir = repository
	command.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", arguments, err, output)
	}
	return strings.TrimSpace(string(output))
}

func runLiveReview(t *testing.T, binary string, live liveProvider, control liveControl) protocol.Report {
	t.Helper()
	arguments := []string{
		"review",
		"--repository", control.repository,
		"--mode", "commit",
		"--commit", control.commit,
		"--engine", string(live.name),
		"--model", live.model,
		"--isolation", "native",
		"--retries", "1",
		"--timeout", "3m",
		"--output", "json",
		"--prompt-file", "-",
	}
	if live.name == protocol.ProviderCursor {
		arguments = append(arguments, "--web-access")
	} else {
		arguments = append(arguments, "--web-access=false", "--reasoning-effort", "high")
	}
	command := exec.CommandContext(t.Context(), binary, arguments...)
	command.Stdin = strings.NewReader(control.contract)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	runErr := command.Run()
	if exit := exitCode(runErr); exit != control.wantExit {
		t.Fatalf("live %s %s exit = %d, want %d: %v\nstdout: %s\nstderr: %s", live.name, control.name, exit, control.wantExit, runErr, stdout.String(), stderr.String())
	}
	report, err := protocol.DecodeReport(stdout.Bytes())
	if err != nil {
		t.Fatalf("decode live %s %s report: %v\nstdout: %s\nstderr: %s", live.name, control.name, err, stdout.String(), stderr.String())
	}
	return report
}

func validateLiveReport(t *testing.T, report protocol.Report, live liveProvider, control liveControl) {
	t.Helper()
	if report.Status != control.wantStatus || report.Failure != nil || report.Review == nil {
		t.Fatalf("live %s %s result = %+v", live.name, control.name, report)
	}
	if report.Metadata.Provider == nil || report.Metadata.Provider.Name != live.name || report.Metadata.Provider.Model != live.model || report.Metadata.Provider.Version == "" {
		t.Fatalf("live %s provider metadata = %+v", live.name, report.Metadata.Provider)
	}
	if report.Metadata.Target == nil || report.Metadata.Target.Mode != protocol.TargetCommit || report.Metadata.Target.CommitRevision != control.commit {
		t.Fatalf("live %s target metadata = %+v", live.name, report.Metadata.Target)
	}
	if report.Metadata.Isolation == nil || *report.Metadata.Isolation != protocol.IsolationNative || report.Metadata.WebAccess != live.webAccess {
		t.Fatalf("live %s execution policy: isolation=%v web_access=%v", live.name, report.Metadata.Isolation, report.Metadata.WebAccess)
	}
	if len(report.Metadata.Attempts) == 0 || len(report.Metadata.Attempts) > 2 || report.Metadata.Attempts[len(report.Metadata.Attempts)-1].Outcome != protocol.AttemptValid {
		t.Fatalf("live %s attempts = %+v", live.name, report.Metadata.Attempts)
	}
	t.Logf(
		"provider=%s version=%s status=%s findings=%d attempts=%d duration_ms=%d web_access=%t",
		live.name,
		report.Metadata.Provider.Version,
		report.Status,
		len(report.Review.Findings),
		len(report.Metadata.Attempts),
		report.Metadata.DurationMS,
		report.Metadata.WebAccess,
	)
	if control.wantStatus == protocol.StatusClean {
		if len(report.Review.Findings) != 0 {
			t.Fatalf("live %s clean control findings = %+v", live.name, report.Review.Findings)
		}
		return
	}
	covered := make(map[string]bool, len(control.defectFiles))
	for _, finding := range report.Review.Findings {
		covered[finding.Location.FilePath] = true
	}
	for _, filePath := range control.defectFiles {
		if !covered[filePath] {
			t.Errorf("live %s defective control missed planted defect in %s; findings = %+v", live.name, filePath, report.Review.Findings)
		}
	}
}
