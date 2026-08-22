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

	"github.com/uinaf/ffss/cli/slopguard/internal/protocol"
	"github.com/uinaf/ffss/cli/slopguard/internal/provider"
)

type liveProvider struct {
	name        protocol.ProviderName
	executable  string
	model       string
	webAccess   bool
	credentials []string
}

type liveAuthRoute string

const (
	liveAuthNativeConfig liveAuthRoute = "native-config"
	liveAuthStrictKey    liveAuthRoute = "strict-key"
)

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
	if os.Getenv("SLOPGUARD_TEST_LIVE") != "1" {
		t.Skip("run with mise run verify:live")
	}
	if _, err := exec.LookPath("trufflehog"); err != nil {
		t.Fatal("live verification requires trufflehog on PATH")
	}

	providers := selectedLiveProviders(t)
	routes := selectedLiveAuthRoutes(t)
	requireLiveStrictCredentials(t, providers, routes)
	for _, live := range providers {
		if _, err := exec.LookPath(live.executable); err != nil {
			t.Fatalf("live %s verification requires %s on PATH", live.name, live.executable)
		}
	}
	repeat := liveRepeat(t)
	if err := validateLiveMatrixSize(len(providers), len(routes), repeat); err != nil {
		t.Fatal(err)
	}
	binary := buildSlopguardBinary(t)
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
			for _, route := range routes {
				for _, control := range controls {
					name := fmt.Sprintf("%s/%s/round-%d/%s", live.name, route, round, control.name)
					t.Run(name, func(t *testing.T) {
						report := runLiveReview(t, binary, live, route, control)
						validateLiveReport(t, report, live, route, control)
					})
				}
			}
		}
	}
}

func TestLiveReviewEnvironmentIsolation(t *testing.T) {
	t.Setenv("SLOPGUARD_REASONING_EFFORT", "low")
	t.Setenv("CURSOR_CONFIG_DIR", "/provider/state")
	t.Setenv("CODEX_API_KEY", "direct-codex-key")
	t.Setenv("OPENAI_API_KEY", "direct-openai-key")
	t.Setenv("GIT_CONFIG_GLOBAL", "/host/global.gitconfig")
	t.Setenv("GIT_CONFIG_SYSTEM", "/host/system.gitconfig")
	t.Setenv("XDG_CONFIG_HOME", "/host/xdg")

	values := make(map[string]string)
	live := liveProvider{name: protocol.ProviderCodex, credentials: []string{"CODEX_API_KEY", "OPENAI_API_KEY"}}
	for _, entry := range liveReviewEnvironment(t, live, liveAuthNativeConfig) {
		name, value, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(name, "SLOPGUARD_") {
			t.Fatalf("live environment retained %s", name)
		}
		values[name] = value
	}
	if values["CURSOR_CONFIG_DIR"] != "/provider/state" {
		t.Fatalf("CURSOR_CONFIG_DIR = %q", values["CURSOR_CONFIG_DIR"])
	}
	if values["GIT_CONFIG_GLOBAL"] != "/dev/null" || values["GIT_CONFIG_SYSTEM"] != "/dev/null" {
		t.Fatalf("Git config environment = %q, %q", values["GIT_CONFIG_GLOBAL"], values["GIT_CONFIG_SYSTEM"])
	}
	if values["XDG_CONFIG_HOME"] != "/host/xdg" {
		t.Fatalf("XDG_CONFIG_HOME = %q", values["XDG_CONFIG_HOME"])
	}
	if values["CODEX_API_KEY"] != "" || values["OPENAI_API_KEY"] != "" {
		t.Fatalf("native config route retained a direct provider key")
	}

	strictValues := make(map[string]string)
	for _, entry := range liveReviewEnvironment(t, live, liveAuthStrictKey) {
		name, value, _ := strings.Cut(entry, "=")
		strictValues[name] = value
	}
	if strictValues["CODEX_API_KEY"] == "" || strictValues["OPENAI_API_KEY"] != "" {
		t.Fatalf("strict key route did not isolate one provider key")
	}
}

func TestSelectedLiveAuthRoutes(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		t.Setenv("SLOPGUARD_LIVE_AUTH_ROUTES", "")
		routes := selectedLiveAuthRoutes(t)
		if len(routes) != 1 || routes[0] != liveAuthNativeConfig {
			t.Fatalf("default routes = %v", routes)
		}
	})
	t.Run("explicit matrix", func(t *testing.T) {
		t.Setenv("SLOPGUARD_LIVE_AUTH_ROUTES", "native-config,strict-key")
		routes := selectedLiveAuthRoutes(t)
		if len(routes) != 2 || routes[0] != liveAuthNativeConfig || routes[1] != liveAuthStrictKey {
			t.Fatalf("explicit routes = %v", routes)
		}
	})
}

func TestValidateLiveMatrixSize(t *testing.T) {
	for _, test := range []struct {
		name      string
		providers int
		routes    int
		repeat    int
		wantError bool
	}{
		{name: "one route maximum", providers: 4, routes: 1, repeat: 10},
		{name: "two route maximum", providers: 4, routes: 2, repeat: 5},
		{name: "selected provider matrix", providers: 1, routes: 2, repeat: 10},
		{name: "too many reviews", providers: 4, routes: 2, repeat: 6, wantError: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := validateLiveMatrixSize(test.providers, test.routes, test.repeat)
			if (err != nil) != test.wantError {
				t.Fatalf("validateLiveMatrixSize() error = %v, wantError = %t", err, test.wantError)
			}
		})
	}
}

func selectedLiveProviders(t *testing.T) []liveProvider {
	t.Helper()
	available := []liveProvider{
		{name: protocol.ProviderCodex, executable: "codex", model: provider.DefaultCodexModel, credentials: []string{"CODEX_API_KEY", "OPENAI_API_KEY"}},
		{name: protocol.ProviderClaude, executable: "claude", model: provider.DefaultClaudeModel, credentials: []string{"ANTHROPIC_API_KEY"}},
		{name: protocol.ProviderCursor, executable: "cursor-agent", model: provider.DefaultCursorModel, webAccess: true, credentials: []string{"CURSOR_API_KEY"}},
		{name: protocol.ProviderGrok, executable: "grok", model: provider.DefaultGrokModel, credentials: []string{"XAI_API_KEY"}},
	}
	selection := strings.TrimSpace(os.Getenv("SLOPGUARD_LIVE_PROVIDERS"))
	if selection == "" {
		return available
	}
	requested := make(map[protocol.ProviderName]struct{})
	for _, value := range strings.Split(selection, ",") {
		name := protocol.ProviderName(strings.TrimSpace(value))
		if name == "" {
			t.Fatalf("SLOPGUARD_LIVE_PROVIDERS contains an empty provider")
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
		t.Fatalf("SLOPGUARD_LIVE_PROVIDERS contains unsupported providers: %s", strings.Join(unknown, ", "))
	}
	return selected
}

func selectedLiveAuthRoutes(t *testing.T) []liveAuthRoute {
	t.Helper()
	selection := strings.TrimSpace(os.Getenv("SLOPGUARD_LIVE_AUTH_ROUTES"))
	if selection == "" {
		return []liveAuthRoute{liveAuthNativeConfig}
	}
	seen := make(map[liveAuthRoute]struct{})
	routes := make([]liveAuthRoute, 0, 2)
	for _, value := range strings.Split(selection, ",") {
		route := liveAuthRoute(strings.TrimSpace(value))
		switch route {
		case liveAuthNativeConfig, liveAuthStrictKey:
		default:
			t.Fatalf("SLOPGUARD_LIVE_AUTH_ROUTES contains unsupported route %q", value)
		}
		if _, duplicate := seen[route]; duplicate {
			t.Fatalf("SLOPGUARD_LIVE_AUTH_ROUTES repeats route %q", route)
		}
		seen[route] = struct{}{}
		routes = append(routes, route)
	}
	return routes
}

func requireLiveStrictCredentials(t *testing.T, providers []liveProvider, routes []liveAuthRoute) {
	t.Helper()
	strictSelected := false
	for _, route := range routes {
		strictSelected = strictSelected || route == liveAuthStrictKey
	}
	if !strictSelected {
		return
	}
	for _, live := range providers {
		_, _ = liveStrictCredential(t, live)
	}
}

func validateLiveMatrixSize(providers, routes, repeat int) error {
	const controlsPerRoute = 2
	const maximumReviews = 80
	reviews := providers * routes * controlsPerRoute * repeat
	if reviews > maximumReviews {
		return fmt.Errorf("live provider matrix requests %d reviews; maximum is %d within the 8h30m test timeout", reviews, maximumReviews)
	}
	return nil
}

func liveRepeat(t *testing.T) int {
	t.Helper()
	value := strings.TrimSpace(os.Getenv("SLOPGUARD_LIVE_REPEAT"))
	if value == "" {
		return 1
	}
	repeat, err := strconv.Atoi(value)
	if err != nil || repeat < 1 || repeat > 10 {
		t.Fatalf("SLOPGUARD_LIVE_REPEAT must be an integer from 1 through 10, got %q", value)
	}
	return repeat
}

func liveFixtureRepository(t *testing.T, variant string) (string, string) {
	t.Helper()
	repository := cliRepository(t)
	gitCommand(t, repository, "config", "user.name", "Slopguard Live Test")
	gitCommand(t, repository, "config", "user.email", "slopguard-live@example.invalid")
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

func runLiveReview(t *testing.T, binary string, live liveProvider, route liveAuthRoute, control liveControl) protocol.Report {
	t.Helper()
	arguments := []string{
		"review",
		"--repository", control.repository,
		"--mode", "commit",
		"--commit", control.commit,
		"--engine", string(live.name),
		"--model", live.model,
		"--isolation", string(liveRouteIsolation(route)),
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
	command.Env = liveReviewEnvironment(t, live, route)
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

func liveReviewEnvironment(t *testing.T, live liveProvider, route liveAuthRoute) []string {
	t.Helper()
	environment := withoutEnvironmentPrefix(os.Environ(), "SLOPGUARD_")
	environment = withoutEnvironmentNames(environment, live.credentials)
	if route == liveAuthStrictKey {
		name, value := liveStrictCredential(t, live)
		environment = append(environment, name+"="+value)
	}
	return replaceEnvironment(environment, map[string]string{
		"GIT_CONFIG_GLOBAL": "/dev/null",
		"GIT_CONFIG_SYSTEM": "/dev/null",
	})
}

func liveStrictCredential(t *testing.T, live liveProvider) (string, string) {
	t.Helper()
	for _, name := range live.credentials {
		if value := strings.TrimSpace(os.Getenv(name)); value != "" {
			return name, value
		}
	}
	t.Fatalf("live %s strict-key route requires one of %s", live.name, strings.Join(live.credentials, ", "))
	return "", ""
}

func liveRouteIsolation(route liveAuthRoute) protocol.Isolation {
	if route == liveAuthStrictKey {
		return protocol.IsolationStrict
	}
	return protocol.IsolationNative
}

func withoutEnvironmentPrefix(environment []string, prefix string) []string {
	filtered := make([]string, 0, len(environment))
	for _, entry := range environment {
		name, _, _ := strings.Cut(entry, "=")
		if !strings.HasPrefix(name, prefix) {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

func withoutEnvironmentNames(environment, names []string) []string {
	removed := make(map[string]struct{}, len(names))
	for _, name := range names {
		removed[name] = struct{}{}
	}
	filtered := make([]string, 0, len(environment))
	for _, entry := range environment {
		name, _, _ := strings.Cut(entry, "=")
		if _, remove := removed[name]; !remove {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

func validateLiveReport(t *testing.T, report protocol.Report, live liveProvider, route liveAuthRoute, control liveControl) {
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
	if report.Metadata.Isolation == nil || *report.Metadata.Isolation != liveRouteIsolation(route) || report.Metadata.WebAccess != live.webAccess {
		t.Fatalf("live %s execution policy: isolation=%v web_access=%v", live.name, report.Metadata.Isolation, report.Metadata.WebAccess)
	}
	if len(report.Metadata.Attempts) == 0 || len(report.Metadata.Attempts) > 2 || report.Metadata.Attempts[len(report.Metadata.Attempts)-1].Outcome != protocol.AttemptValid {
		t.Fatalf("live %s attempts = %+v", live.name, report.Metadata.Attempts)
	}
	t.Logf(
		"provider=%s auth_route=%s version=%s status=%s findings=%d attempts=%d duration_ms=%d web_access=%t",
		live.name,
		route,
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
