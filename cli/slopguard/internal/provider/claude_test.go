package provider

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/uinaf/ffss/cli/slopguard/internal/config"
	"github.com/uinaf/ffss/cli/slopguard/internal/protocol"
	"github.com/uinaf/ffss/cli/slopguard/internal/reviewpolicy"
)

func TestClaudeReviewStrictUsesFrozenStdinAndSafeMode(t *testing.T) {
	t.Parallel()

	fake := newFakeClaude(t, fakeClaudeOptions{})
	reviewer := NewClaude(ClaudeOptions{
		Repository: t.TempDir(), Executable: fake.path,
		Environment: []string{
			"PATH=/usr/bin:/bin",
			"ANTHROPIC_API_KEY=test-provider-secret",
			"OPENAI_API_KEY=must-not-pass",
			"HOME=/private/home",
		},
	})
	result, err := reviewer.Review(context.Background(), Request{Prompt: "frozen review bundle", Config: claudeConfig(protocol.IsolationStrict, false, 5*time.Second)})
	if err != nil {
		t.Fatal(err)
	}
	if result.Provider.Name != protocol.ProviderClaude || result.Provider.Model != "test-model" || result.Provider.Version != "2.1.220" {
		t.Fatalf("provider = %+v", result.Provider)
	}
	if result.Attempt.Outcome != protocol.AttemptValid || result.ProtocolRecovery.Applied || len(result.Review.Findings) != 0 {
		t.Fatalf("result = %+v", result)
	}
	prompt := readTestFile(t, fake.prompt)
	policy := reviewpolicy.ClaudeReviewProtocol()
	if prompt != "frozen review bundle"+policy || !strings.Contains(policy, "must cite a reviewed file") || !strings.Contains(policy, "one individual line_ranges entry") || !strings.Contains(policy, "split it into findings") {
		t.Fatalf("provider stdin omitted trusted review policy: %q", prompt)
	}
	arguments := strings.Split(strings.TrimSpace(readTestFile(t, fake.arguments)), "\n")
	for _, required := range []string{"--safe-mode", "--setting-sources", "user", "--strict-mcp-config", "--disallowedTools", "mcp__*", "--print", "--no-session-persistence", "--output-format", "json", "--json-schema", "--permission-mode", "dontAsk", "--no-chrome", "--tools", "--model", "test-model", "--effort", "high"} {
		if !contains(arguments, required) {
			t.Errorf("Claude arguments omitted %q: %v", required, arguments)
		}
	}
	if contains(arguments, "--fallback-model") || argumentAfter(arguments, "--tools") != "" {
		t.Fatalf("Claude strict arguments = %v", arguments)
	}
	schema := argumentAfter(arguments, "--json-schema")
	if strings.Contains(schema, `"not"`) || !strings.Contains(schema, `"findings"`) {
		t.Fatalf("Claude schema projection = %s", schema)
	}
	environment := readTestFile(t, fake.environment)
	if !strings.Contains(environment, "ANTHROPIC_API_KEY=test-provider-secret") || !strings.Contains(environment, "CLAUDE_CODE_DISABLE_AUTO_MEMORY=1") || strings.Contains(environment, "OPENAI_API_KEY") || strings.Contains(environment, "HOME=/private/home") {
		t.Fatalf("strict environment = %s", environment)
	}
}

func TestClaudeReviewRejectsCombinedPromptBeforeDiscovery(t *testing.T) {
	t.Parallel()

	policyBytes := int64(len(reviewpolicy.ClaudeReviewProtocol()))
	effective := claudeConfig(protocol.IsolationStrict, false, 5*time.Second)
	effective.MaxBytes = config.Value[int64]{Value: policyBytes, Source: config.SourceFlag}
	maximumPrompt := effective.MaxBytes.Value + providerPromptAllowance
	prompt := strings.Repeat("x", int(maximumPrompt-policyBytes+1))
	reviewer := NewClaude(ClaudeOptions{Repository: t.TempDir(), Executable: "missing-claude"})
	_, err := reviewer.Review(context.Background(), Request{Prompt: prompt, Config: effective})
	failure := assertProviderError(t, err, protocol.FailureConfig)
	if !strings.Contains(failure.Message, "Claude combined review input exceeds") {
		t.Fatalf("failure = %q", failure.Message)
	}
}

func TestClaudeReviewNativePreservesConfigurationAndEnablesWeb(t *testing.T) {
	t.Parallel()

	fake := newFakeClaude(t, fakeClaudeOptions{})
	reviewer := NewClaude(ClaudeOptions{
		Repository: t.TempDir(), Executable: fake.path,
		Environment: []string{
			"PATH=/usr/bin:/bin",
			"HOME=/native/home",
			"CLAUDE_CONFIG_DIR=/native/claude",
			"ANTHROPIC_AUTH_TOKEN=test-provider-helper",
			"ANTHROPIC_BASE_URL=https://gateway.invalid",
		},
	})
	result, err := reviewer.Review(context.Background(), Request{Prompt: "bundle", Config: claudeConfig(protocol.IsolationNative, true, 5*time.Second)})
	if err != nil {
		t.Fatal(err)
	}
	if !result.WebAccess || result.Isolation != protocol.IsolationNative {
		t.Fatalf("result policy = %+v", result)
	}
	arguments := strings.Split(strings.TrimSpace(readTestFile(t, fake.arguments)), "\n")
	for _, forbidden := range []string{"--safe-mode", "--strict-mcp-config", "--disallowedTools", "--fallback-model"} {
		if contains(arguments, forbidden) {
			t.Errorf("native arguments retained %q: %v", forbidden, arguments)
		}
	}
	if argumentAfter(arguments, "--tools") != "WebSearch" || argumentAfter(arguments, "--allowedTools") != "WebSearch" {
		t.Fatalf("native web arguments = %v", arguments)
	}
	environment := readTestFile(t, fake.environment)
	if !strings.Contains(environment, "HOME=/native/home") || !strings.Contains(environment, "CLAUDE_CONFIG_DIR=/native/claude") || !strings.Contains(environment, "ANTHROPIC_AUTH_TOKEN=test-provider-helper") || !strings.Contains(environment, "ANTHROPIC_BASE_URL=https://gateway.invalid") || strings.Contains(environment, "CLAUDE_CODE_DISABLE_AUTO_MEMORY") {
		t.Fatalf("native environment = %s", environment)
	}
}

func TestClaudeReviewFailsCapabilityProbeBeforeInvocation(t *testing.T) {
	t.Parallel()

	fake := newFakeClaude(t, fakeClaudeOptions{help: "--print --output-format"})
	reviewer := NewClaude(ClaudeOptions{Repository: t.TempDir(), Executable: fake.path, Environment: []string{"PATH=/usr/bin:/bin", "ANTHROPIC_API_KEY=secret"}})
	_, err := reviewer.Review(context.Background(), Request{Prompt: "bundle", Config: claudeConfig(protocol.IsolationStrict, false, 5*time.Second)})
	_ = assertProviderError(t, err, protocol.FailureCapability)
	if _, err := os.Stat(fake.arguments); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("model was invoked after failed capability probe: %v", err)
	}
}

func TestClaudeReviewNativeReportsProviderAuthenticationFailure(t *testing.T) {
	t.Parallel()

	fake := newFakeClaude(t, fakeClaudeOptions{reviewOutputError: `{"type":"result","subtype":"success","is_error":true,"api_error_status":401,"result":"Invalid API key"}`})
	reviewer := NewClaude(ClaudeOptions{Repository: t.TempDir(), Executable: fake.path, Environment: []string{"PATH=/usr/bin:/bin", "HOME=/native/home"}})
	_, err := reviewer.Review(context.Background(), Request{Prompt: "bundle", Config: claudeConfig(protocol.IsolationNative, false, 5*time.Second)})
	failure := assertProviderError(t, err, protocol.FailureAuth)
	if failure.Attempt == nil || failure.Attempt.Outcome != protocol.AttemptFailed {
		t.Fatalf("attempt = %+v", failure.Attempt)
	}
	assertExecutionMetadata(t, failure, protocol.ProviderClaude, "2.1.220", protocol.IsolationNative, false)
}

func TestClaudeReviewStrictExplainsCredentialRequirement(t *testing.T) {
	t.Parallel()

	fake := newFakeClaude(t, fakeClaudeOptions{})
	reviewer := NewClaude(ClaudeOptions{Repository: t.TempDir(), Executable: fake.path, Environment: []string{"PATH=/usr/bin:/bin"}})
	_, err := reviewer.Review(context.Background(), Request{Prompt: "bundle", Config: claudeConfig(protocol.IsolationStrict, false, 5*time.Second)})
	failure := assertProviderError(t, err, protocol.FailureAuth)
	if !strings.Contains(failure.Message, "strict isolation requires ANTHROPIC_API_KEY") || !strings.Contains(failure.Message, "--isolation native") {
		t.Fatalf("failure = %q", failure.Message)
	}
	if _, err := os.Stat(fake.arguments); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("provider was probed without a strict credential: %v", err)
	}
}

func TestClaudeReviewStrictReportsMissingExecutableBeforeCredential(t *testing.T) {
	t.Parallel()

	reviewer := NewClaude(ClaudeOptions{Repository: t.TempDir(), Executable: "missing-claude", Environment: []string{"PATH=/usr/bin:/bin"}})
	_, err := reviewer.Review(context.Background(), Request{Prompt: "bundle", Config: claudeConfig(protocol.IsolationStrict, false, 5*time.Second)})
	failure := assertProviderError(t, err, protocol.FailureCapability)
	if !strings.Contains(failure.Message, "was not found") || strings.Contains(failure.Message, "API_KEY") {
		t.Fatalf("failure = %q", failure.Message)
	}
}

func TestClaudeReviewUsesExplicitDefaultModel(t *testing.T) {
	t.Parallel()

	effective := claudeConfig(protocol.IsolationStrict, false, 5*time.Second)
	effective.Model = config.Value[string]{Source: config.SourceDefault}
	fake := newFakeClaude(t, fakeClaudeOptions{})
	reviewer := NewClaude(ClaudeOptions{Repository: t.TempDir(), Executable: fake.path, Environment: []string{"PATH=/usr/bin:/bin", "ANTHROPIC_API_KEY=secret"}})
	result, err := reviewer.Review(context.Background(), Request{Prompt: "bundle", Config: effective})
	if err != nil {
		t.Fatal(err)
	}
	if result.Provider.Model != DefaultClaudeModel {
		t.Fatalf("provider model = %q", result.Provider.Model)
	}
	arguments := strings.Split(strings.TrimSpace(readTestFile(t, fake.arguments)), "\n")
	if argumentAfter(arguments, "--model") != DefaultClaudeModel {
		t.Fatalf("Claude arguments = %v", arguments)
	}
}

func TestClaudeReviewOmitsProviderFailureOutput(t *testing.T) {
	t.Parallel()

	fake := newFakeClaude(t, fakeClaudeOptions{reviewError: "\033[31m" + providerOutputSentinel + " test-provider-secret\r"})
	reviewer := NewClaude(ClaudeOptions{Repository: t.TempDir(), Executable: fake.path, Environment: []string{"PATH=/usr/bin:/bin", "ANTHROPIC_API_KEY=test-provider-secret"}})
	_, err := reviewer.Review(context.Background(), Request{Prompt: providerOutputSentinel, Config: claudeConfig(protocol.IsolationStrict, false, 5*time.Second)})
	failure := assertProviderError(t, err, protocol.FailureProvider)
	if strings.Contains(failure.Message, providerOutputSentinel) || strings.Contains(failure.Message, "test-provider-secret") || strings.ContainsRune(failure.Message, '\x1b') {
		t.Fatalf("provider failure = %q", failure.Message)
	}
	if !strings.Contains(failure.Message, "exit code 7") || !strings.Contains(failure.Message, "private diagnostics") {
		t.Fatalf("provider failure lacks safe context: %q", failure.Message)
	}
}

func TestClaudeReviewDistinguishesTimeoutAndCancellation(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name    string
		timeout time.Duration
		cancel  bool
		class   protocol.FailureClass
	}{
		{name: "timeout", timeout: 100 * time.Millisecond, class: protocol.FailureTimeout},
		{name: "cancelled", timeout: 5 * time.Second, cancel: true, class: protocol.FailureCancelled},
	} {
		t.Run(test.name, func(t *testing.T) {
			fake := newFakeClaude(t, fakeClaudeOptions{delay: "0.5"})
			reviewer := NewClaude(ClaudeOptions{Repository: t.TempDir(), Executable: fake.path, Environment: []string{"PATH=/usr/bin:/bin", "ANTHROPIC_API_KEY=secret"}})
			ctx := context.Background()
			if test.cancel {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				t.Cleanup(cancel)
				go func() {
					time.Sleep(50 * time.Millisecond)
					cancel()
				}()
			}
			_, err := reviewer.Review(ctx, Request{Prompt: "bundle", Config: claudeConfig(protocol.IsolationStrict, false, test.timeout)})
			_ = assertProviderError(t, err, test.class)
		})
	}
}

func TestClaudeReviewRejectsMalformedEnvelopeAndReview(t *testing.T) {
	t.Parallel()

	validReview := `{"findings":[],"overall_explanation":"No defects.","overall_confidence":0.95}`
	for _, test := range []struct {
		name   string
		output string
	}{
		{name: "not JSON", output: "not-json"},
		{name: "unknown failure subtype", output: `{"type":"result","subtype":"unknown","is_error":true,"api_error_status":401,"result":"failed"}`},
		{name: "failure missing result", output: `{"type":"result","subtype":"error","is_error":true,"api_error_status":401}`},
		{name: "missing structured output", output: `{"type":"result","subtype":"success","is_error":false}`},
		{name: "duplicate envelope key", output: `{"type":"result","type":"result","subtype":"success","is_error":false,"structured_output":{}}`},
		{name: "sentinel envelope key", output: `{"` + providerOutputSentinel + `":1,"` + providerOutputSentinel + `":2}`},
		{name: "invalid canonical review", output: claudeEnvelope(`{"findings":[]}`)},
		{name: "duplicate review key", output: claudeEnvelope(`{"findings":[],"findings":[],"overall_explanation":"No defects.","overall_confidence":0.95}`)},
		{name: "valid control", output: claudeEnvelope(validReview)},
	} {
		t.Run(test.name, func(t *testing.T) {
			fake := newFakeClaude(t, fakeClaudeOptions{output: test.output})
			reviewer := NewClaude(ClaudeOptions{Repository: t.TempDir(), Executable: fake.path, Environment: []string{"PATH=/usr/bin:/bin", "ANTHROPIC_API_KEY=secret"}})
			_, err := reviewer.Review(context.Background(), Request{Prompt: "bundle", Config: claudeConfig(protocol.IsolationStrict, false, 5*time.Second)})
			if test.name == "valid control" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			failure := assertProviderError(t, err, protocol.FailureProtocol)
			if strings.Contains(failure.Message, providerOutputSentinel) {
				t.Fatalf("protocol failure disclosed provider output: %q", failure.Message)
			}
			if failure.Attempt == nil || failure.Attempt.Outcome != protocol.AttemptMalformed {
				t.Fatalf("attempt = %+v", failure.Attempt)
			}
			assertExecutionMetadata(t, failure, protocol.ProviderClaude, "2.1.220", protocol.IsolationStrict, false)
		})
	}
}

func TestClaudeReviewClassifiesReportedFailures(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name   string
		status string
		class  protocol.FailureClass
	}{
		{name: "authentication", status: `401`, class: protocol.FailureAuth},
		{name: "rate limit", status: `429`, class: protocol.FailureProvider},
		{name: "unavailable model stays generic", status: `400`, class: protocol.FailureProvider},
		{name: "missing status stays generic", status: `null`, class: protocol.FailureProvider},
	} {
		t.Run(test.name, func(t *testing.T) {
			output := `{"type":"result","subtype":"error","is_error":true,"api_error_status":` + test.status + `,"result":"` + providerOutputSentinel + `"}`
			fake := newFakeClaude(t, fakeClaudeOptions{output: output})
			reviewer := NewClaude(ClaudeOptions{Repository: t.TempDir(), Executable: fake.path, Environment: []string{"PATH=/usr/bin:/bin", "ANTHROPIC_API_KEY=secret"}})
			_, err := reviewer.Review(context.Background(), Request{Prompt: "bundle", Config: claudeConfig(protocol.IsolationStrict, false, 5*time.Second)})
			failure := assertProviderError(t, err, test.class)
			if strings.Contains(failure.Message, providerOutputSentinel) {
				t.Fatalf("provider failure disclosed provider output: %q", failure.Message)
			}
			if failure.Attempt == nil || failure.Attempt.Outcome != protocol.AttemptFailed {
				t.Fatalf("attempt = %+v", failure.Attempt)
			}
			assertExecutionMetadata(t, failure, protocol.ProviderClaude, "2.1.220", protocol.IsolationStrict, false)
		})
	}
}

func TestClaudeReviewPrefersReportedFailureOnNonZeroExit(t *testing.T) {
	t.Parallel()

	output := `{"type":"result","subtype":"error","is_error":true,"api_error_status":429,"result":"` + providerOutputSentinel + `"}`
	fake := newFakeClaude(t, fakeClaudeOptions{reviewOutputError: output, reviewError: "not authenticated"})
	reviewer := NewClaude(ClaudeOptions{Repository: t.TempDir(), Executable: fake.path, Environment: []string{"PATH=/usr/bin:/bin", "ANTHROPIC_API_KEY=secret"}})
	_, err := reviewer.Review(context.Background(), Request{Prompt: "bundle", Config: claudeConfig(protocol.IsolationStrict, false, 5*time.Second)})
	failure := assertProviderError(t, err, protocol.FailureProvider)
	if failure.Message != "Claude reported a rate limit" {
		t.Fatalf("failure = %q", failure.Message)
	}
	if failure.Attempt == nil || failure.Attempt.Outcome != protocol.AttemptFailed {
		t.Fatalf("attempt = %+v", failure.Attempt)
	}
}

func TestClaudeReviewRejectsMalformedTypedFailureOnNonZeroExit(t *testing.T) {
	t.Parallel()

	output := `{"type":"result","subtype":"unknown","is_error":true,"api_error_status":401,"result":"` + providerOutputSentinel + `"}`
	fake := newFakeClaude(t, fakeClaudeOptions{reviewOutputError: output})
	reviewer := NewClaude(ClaudeOptions{Repository: t.TempDir(), Executable: fake.path, Environment: []string{"PATH=/usr/bin:/bin", "ANTHROPIC_API_KEY=secret"}})
	_, err := reviewer.Review(context.Background(), Request{Prompt: "bundle", Config: claudeConfig(protocol.IsolationStrict, false, 5*time.Second)})
	failure := assertProviderError(t, err, protocol.FailureProvider)
	if strings.Contains(failure.Message, providerOutputSentinel) {
		t.Fatalf("provider failure disclosed provider output: %q", failure.Message)
	}
	if failure.Attempt == nil || failure.Attempt.Outcome != protocol.AttemptFailed {
		t.Fatalf("attempt = %+v", failure.Attempt)
	}
}

func TestClaudeReviewEnforcesOutputBounds(t *testing.T) {
	t.Parallel()

	fake := newFakeClaude(t, fakeClaudeOptions{output: strings.Repeat("x", int(providerStdoutLimit)+1)})
	reviewer := NewClaude(ClaudeOptions{Repository: t.TempDir(), Executable: fake.path, Environment: []string{"PATH=/usr/bin:/bin", "ANTHROPIC_API_KEY=secret"}})
	_, err := reviewer.Review(context.Background(), Request{Prompt: "bundle", Config: claudeConfig(protocol.IsolationStrict, false, 5*time.Second)})
	failure := assertProviderError(t, err, protocol.FailureProvider)
	if failure.Attempt == nil || failure.Attempt.Outcome != protocol.AttemptFailed {
		t.Fatalf("attempt = %+v", failure.Attempt)
	}
	assertExecutionMetadata(t, failure, protocol.ProviderClaude, "2.1.220", protocol.IsolationStrict, false)
}

func TestClaudeReviewRejectsUnsupportedEffort(t *testing.T) {
	t.Parallel()

	effective := claudeConfig(protocol.IsolationStrict, false, 5*time.Second)
	effective.ReasoningEffort.Value = config.ReasoningUltra
	reviewer := NewClaude(ClaudeOptions{Repository: t.TempDir()})
	_, err := reviewer.Review(context.Background(), Request{Prompt: "bundle", Config: effective})
	_ = assertProviderError(t, err, protocol.FailureConfig)
}

type fakeClaudeOptions struct {
	help              string
	output            string
	reviewError       string
	reviewOutputError string
	delay             string
}

type fakeClaude struct {
	path        string
	arguments   string
	prompt      string
	environment string
}

func newFakeClaude(t *testing.T, options fakeClaudeOptions) fakeClaude {
	t.Helper()
	root := t.TempDir()
	fake := fakeClaude{
		path: filepath.Join(root, "claude"), arguments: filepath.Join(root, "arguments.txt"),
		prompt: filepath.Join(root, "prompt.txt"), environment: filepath.Join(root, "environment.txt"),
	}
	if options.help == "" {
		options.help = "--safe-mode --setting-sources --strict-mcp-config --disallowedTools --print --no-session-persistence --output-format --json-schema --model --effort --tools --allowedTools --permission-mode --no-chrome"
	}
	if options.output == "" {
		options.output = claudeEnvelope(`{"findings":[],"overall_explanation":"No defects.","overall_confidence":0.95}`)
	}
	outputPath := filepath.Join(root, "output.json")
	if err := os.WriteFile(outputPath, []byte(options.output), 0o600); err != nil {
		t.Fatal(err)
	}
	reviewFailure := ""
	if options.reviewOutputError != "" {
		reviewFailure += "printf '%s' " + shellQuote(options.reviewOutputError) + "\n"
	}
	if options.reviewError != "" {
		reviewFailure += "printf '%b' " + shellQuote(options.reviewError) + " >&2\n"
	}
	if reviewFailure != "" {
		reviewFailure += "exit 7\n"
	}
	delay := ""
	if options.delay != "" {
		delay = "sleep " + options.delay + "\n"
	}
	script := "#!/bin/sh\n" +
		"set -eu\n" +
		"fail_contract() { printf '%s\\n' 'unexpected Claude CLI arguments' >&2; exit 64; }\n" +
		"validate_review() {\n" +
		"  if [ \"${1:-}\" = '--safe-mode' ]; then\n" +
		"    shift\n" +
		"    [ \"${1:-}\" = '--setting-sources' ] || return 1; shift\n" +
		"    [ \"${1:-}\" = 'user' ] || return 1; shift\n" +
		"    [ \"${1:-}\" = '--strict-mcp-config' ] || return 1; shift\n" +
		"    [ \"${1:-}\" = '--disallowedTools' ] || return 1; shift\n" +
		"    [ \"${1:-}\" = 'mcp__*' ] || return 1; shift\n" +
		"  fi\n" +
		"  [ \"${1:-}\" = '--print' ] || return 1; shift\n" +
		"  [ \"${1:-}\" = '--no-session-persistence' ] || return 1; shift\n" +
		"  [ \"${1:-}\" = '--output-format' ] || return 1; shift\n" +
		"  [ \"${1:-}\" = 'json' ] || return 1; shift\n" +
		"  [ \"${1:-}\" = '--json-schema' ] || return 1; shift\n" +
		"  case \"${1:-}\" in '{'*'}') ;; *) return 1 ;; esac; shift\n" +
		"  [ \"${1:-}\" = '--permission-mode' ] || return 1; shift\n" +
		"  [ \"${1:-}\" = 'dontAsk' ] || return 1; shift\n" +
		"  [ \"${1:-}\" = '--no-chrome' ] || return 1; shift\n" +
		"  [ \"${1:-}\" = '--tools' ] || return 1; shift\n" +
		"  if [ \"${1:-}\" = 'WebSearch' ]; then shift; [ \"${1:-}\" = '--allowedTools' ] || return 1; shift; [ \"${1:-}\" = 'WebSearch' ] || return 1; shift; else [ \"$#\" -ge 1 ] || return 1; [ -z \"$1\" ] || return 1; shift; fi\n" +
		"  [ \"${1:-}\" = '--model' ] || return 1; shift\n" +
		"  [ -n \"${1:-}\" ] || return 1; case \"$1\" in -*) return 1 ;; esac; shift\n" +
		"  [ \"${1:-}\" = '--effort' ] || return 1; shift\n" +
		"  case \"${1:-}\" in low|medium|high|xhigh|max) ;; *) return 1 ;; esac; shift\n" +
		"  [ \"$#\" -eq 0 ]\n" +
		"}\n" +
		"if [ \"$#\" -eq 1 ] && [ \"$1\" = \"--version\" ]; then printf '%s\\n' '2.1.220 (Claude Code)'; exit 0; fi\n" +
		"if [ \"$#\" -eq 1 ] && [ \"$1\" = \"--help\" ]; then printf '%s\\n' " + shellQuote(options.help) + "; exit 0; fi\n" +
		"validate_review \"$@\" || fail_contract\n" +
		"printf '%s\\n' \"$@\" > " + shellQuote(fake.arguments) + "\n" +
		"cat > " + shellQuote(fake.prompt) + "\n" +
		"[ -s " + shellQuote(fake.prompt) + " ] || fail_contract\n" +
		"env > " + shellQuote(fake.environment) + "\n" +
		reviewFailure + delay +
		"cat " + shellQuote(outputPath) + "\n"
	writeTestExecutableAt(t, fake.path, script)
	return fake
}

func claudeEnvelope(review string) string {
	return `{"type":"result","subtype":"success","is_error":false,"structured_output":` + review + `}`
}

func claudeConfig(isolation protocol.Isolation, web bool, timeout time.Duration) config.Effective {
	return config.Effective{
		Engine:          config.Value[protocol.ProviderName]{Value: protocol.ProviderClaude, Source: config.SourceFlag},
		Model:           config.Value[string]{Value: "test-model", Source: config.SourceFlag},
		ReasoningEffort: config.Value[config.ReasoningEffort]{Value: config.ReasoningHigh, Source: config.SourceDefault},
		Timeout:         config.Value[config.Duration]{Value: config.Duration(timeout), Source: config.SourceFlag},
		Retries:         config.Value[int]{Value: 1, Source: config.SourceDefault},
		MaxBytes:        config.Value[int64]{Value: 1 << 20, Source: config.SourceDefault},
		Isolation:       config.Value[protocol.Isolation]{Value: isolation, Source: config.SourceFlag},
		WebAccess:       config.Value[bool]{Value: web, Source: config.SourceFlag},
	}
}

func argumentAfter(arguments []string, flag string) string {
	index := indexOf(arguments, flag)
	if index < 0 || index+1 >= len(arguments) {
		return ""
	}
	return arguments[index+1]
}
