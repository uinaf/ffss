package provider

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/uinaf/ffsstack/cli/slopguard/internal/config"
	"github.com/uinaf/ffsstack/cli/slopguard/internal/protocol"
	"github.com/uinaf/ffsstack/cli/slopguard/internal/reviewpolicy"
	contractschema "github.com/uinaf/ffsstack/cli/slopguard/schema"
)

func TestGrokReviewStrictUsesFrozenPromptAndBoundedPolicy(t *testing.T) {
	t.Parallel()

	fake := newFakeGrok(t, fakeGrokOptions{authError: "not logged in"})
	reviewer := NewGrok(GrokOptions{
		Repository: t.TempDir(), Executable: fake.path,
		Environment: []string{
			"PATH=/usr/bin:/bin",
			"XAI_API_KEY=test-provider-secret",
			"OPENAI_API_KEY=must-not-pass",
			"HOME=/private/home",
		},
	})
	result, err := reviewer.Review(context.Background(), Request{Prompt: "frozen review bundle", TrustedSuffix: "\nretry", Config: grokConfig(protocol.IsolationStrict, false, 5*time.Second)})
	if err != nil {
		t.Fatal(err)
	}
	if result.Provider.Name != protocol.ProviderGrok || result.Provider.Model != "test-model" || result.Provider.Version != "1.0.4" {
		t.Fatalf("provider = %+v", result.Provider)
	}
	if result.Attempt.Outcome != protocol.AttemptValid || result.ProtocolRecovery.Applied || len(result.Review.Findings) != 0 {
		t.Fatalf("result = %+v", result)
	}
	if prompt := readTestFile(t, fake.prompt); prompt != "frozen review bundle\nretry"+reviewpolicy.GrokReviewProtocol() {
		t.Fatalf("provider prompt = %q", prompt)
	}
	arguments := strings.Split(strings.TrimSpace(readTestFile(t, fake.arguments)), "\n")
	promptPath := argumentAfter(arguments, "--prompt-file")
	if _, err := os.Stat(promptPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("temporary Grok prompt survived review: %v", err)
	}
	for _, required := range []string{
		"--prompt-file", "--output-format", "json", "--json-schema", "--model", "test-model", "--reasoning-effort", "high",
		"--max-turns", "2", "--permission-mode", "dontAsk", "--no-plan", "--no-subagents", "--no-memory", "--verbatim",
		"--cwd", "--deny", "Bash", "Edit", "Write", "Read", "Grep", "MCPTool", "--tools", "--disallowed-tools",
		"web_search", "web_search,search_tool,use_tool,Agent", "--disable-web-search", "WebFetch", "WebSearch",
		"--sandbox", "workspace",
	} {
		if !contains(arguments, required) {
			t.Errorf("Grok arguments omitted %q: %v", required, arguments)
		}
	}
	if argumentAfter(arguments, "--tools") != "web_search" || argumentAfter(arguments, "--disallowed-tools") != "web_search,search_tool,use_tool,Agent" {
		t.Fatalf("Grok strict tool inventory = %v", arguments)
	}
	for _, tool := range []string{"WebFetch", "WebSearch"} {
		if !containsArgumentPair(arguments, "--deny", tool) || containsArgumentPair(arguments, "--allow", tool) {
			t.Fatalf("Grok strict web policy for %s = %v", tool, arguments)
		}
	}
	schema := argumentAfter(arguments, "--json-schema")
	if strings.Contains(schema, `"not"`) || !strings.Contains(schema, `"findings"`) || !strings.Contains(schema, `"minLength":`+strconv.Itoa(contractschema.GrokMinimumOverallExplanationCharacters)) {
		t.Fatalf("Grok schema projection = %s", schema)
	}
	environment := readTestFile(t, fake.environment)
	if !strings.Contains(environment, "XAI_API_KEY=test-provider-secret") || !strings.Contains(environment, "GROK_HOME=") || strings.Contains(environment, "OPENAI_API_KEY") || strings.Contains(environment, "HOME=/private/home") {
		t.Fatalf("strict environment = %s", environment)
	}
}

func TestGrokReviewNativeUsesSessionAuthAndExplicitWebPolicy(t *testing.T) {
	t.Parallel()

	fake := newFakeGrok(t, fakeGrokOptions{})
	reviewer := NewGrok(GrokOptions{
		Repository: t.TempDir(), Executable: fake.path,
		Environment: []string{"PATH=/usr/bin:/bin", "HOME=/native/home", "GROK_HOME=/native/grok"},
	})
	result, err := reviewer.Review(context.Background(), Request{Prompt: "bundle", Config: grokConfig(protocol.IsolationNative, true, 5*time.Second)})
	if err != nil {
		t.Fatal(err)
	}
	if !result.WebAccess || result.Isolation != protocol.IsolationNative {
		t.Fatalf("result policy = %+v", result)
	}
	arguments := strings.Split(strings.TrimSpace(readTestFile(t, fake.arguments)), "\n")
	for _, required := range []string{"--allow", "WebFetch", "WebSearch"} {
		if !contains(arguments, required) {
			t.Errorf("Grok web arguments omitted %q: %v", required, arguments)
		}
	}
	if argumentAfter(arguments, "--tools") != "web_search,web_fetch" || argumentAfter(arguments, "--disallowed-tools") != "search_tool,use_tool,Agent" {
		t.Fatalf("Grok web tool inventory = %v", arguments)
	}
	for _, forbidden := range []string{"--disable-web-search", "--sandbox"} {
		if contains(arguments, forbidden) {
			t.Errorf("Grok native web arguments retained %q: %v", forbidden, arguments)
		}
	}
	environment := readTestFile(t, fake.environment)
	if !strings.Contains(environment, "HOME=/native/home") || !strings.Contains(environment, "GROK_HOME=/native/grok") {
		t.Fatalf("native environment = %s", environment)
	}
}

func TestGrokReviewFailsCapabilityProbeBeforeInvocation(t *testing.T) {
	t.Parallel()

	fake := newFakeGrok(t, fakeGrokOptions{help: "--prompt-file --output-format"})
	reviewer := NewGrok(GrokOptions{Repository: t.TempDir(), Executable: fake.path, Environment: []string{"PATH=/usr/bin:/bin", "XAI_API_KEY=secret"}})
	_, err := reviewer.Review(context.Background(), Request{Prompt: "bundle", Config: grokConfig(protocol.IsolationStrict, false, 5*time.Second)})
	_ = assertProviderError(t, err, protocol.FailureCapability)
	if _, err := os.Stat(fake.arguments); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("model was invoked after failed capability probe: %v", err)
	}
}

func TestGrokReviewSkipsIncompatibleToolManagerShim(t *testing.T) {
	t.Parallel()

	fake := newFakeGrok(t, fakeGrokOptions{})
	shimDirectory := t.TempDir()
	manager := filepath.Join(t.TempDir(), "mise")
	writeTestExecutableAt(t, manager, "#!/bin/sh\nif [ \"${1:-}\" = '--version' ]; then printf '%s\\n' 'mise 2026.8.6'; exit 0; fi\nif [ \"${1:-}\" = '--help' ]; then printf '%s\\n' 'mise command help'; exit 0; fi\nexit 1\n")
	if err := os.Symlink(manager, filepath.Join(shimDirectory, "grok")); err != nil {
		t.Fatal(err)
	}
	reviewer := NewGrok(GrokOptions{
		Repository: t.TempDir(),
		Environment: []string{
			"PATH=" + strings.Join([]string{shimDirectory, filepath.Dir(fake.path), "/usr/bin", "/bin"}, string(os.PathListSeparator)),
			"XAI_API_KEY=secret",
		},
	})
	result, err := reviewer.Review(context.Background(), Request{Prompt: "bundle", Config: grokConfig(protocol.IsolationStrict, false, 5*time.Second)})
	if err != nil {
		t.Fatal(err)
	}
	if result.Provider.Version != "1.0.4" {
		t.Fatalf("provider = %+v", result.Provider)
	}
	if _, err := os.Stat(fake.arguments); err != nil {
		t.Fatalf("compatible Grok candidate was not invoked: %v", err)
	}
}

func TestGrokReviewReportsAuthenticationFailure(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name    string
		options fakeGrokOptions
	}{
		{name: "not authenticated", options: fakeGrokOptions{loggedOut: true}},
		{name: "not logged in", options: fakeGrokOptions{authStatus: "You are not logged in.\nAvailable models:"}},
		{name: "logged out", options: fakeGrokOptions{authStatus: "You are logged out.\nAvailable models:"}},
		{name: "no active session", options: fakeGrokOptions{authStatus: "No active session.\nAvailable models:"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			fake := newFakeGrok(t, test.options)
			reviewer := NewGrok(GrokOptions{Repository: t.TempDir(), Executable: fake.path, Environment: []string{"PATH=/usr/bin:/bin", "HOME=/native/home"}})
			_, err := reviewer.Review(context.Background(), Request{Prompt: "bundle", Config: grokConfig(protocol.IsolationNative, false, 5*time.Second)})
			_ = assertProviderError(t, err, protocol.FailureAuth)
		})
	}
}

func TestGrokReviewReportsAuthenticationFormatDriftAsCapabilityFailure(t *testing.T) {
	t.Parallel()

	fake := newFakeGrok(t, fakeGrokOptions{authStatus: "Account ready."})
	reviewer := NewGrok(GrokOptions{Repository: t.TempDir(), Executable: fake.path, Environment: []string{"PATH=/usr/bin:/bin", "HOME=/native/home"}})
	_, err := reviewer.Review(context.Background(), Request{Prompt: "bundle", Config: grokConfig(protocol.IsolationNative, false, 5*time.Second)})
	_ = assertProviderError(t, err, protocol.FailureCapability)
}

func TestGrokReviewStrictExplainsCredentialRequirement(t *testing.T) {
	t.Parallel()

	fake := newFakeGrok(t, fakeGrokOptions{})
	reviewer := NewGrok(GrokOptions{Repository: t.TempDir(), Executable: fake.path, Environment: []string{"PATH=/usr/bin:/bin"}})
	_, err := reviewer.Review(context.Background(), Request{Prompt: "bundle", Config: grokConfig(protocol.IsolationStrict, false, 5*time.Second)})
	failure := assertProviderError(t, err, protocol.FailureAuth)
	if !strings.Contains(failure.Message, "strict isolation requires XAI_API_KEY") || !strings.Contains(failure.Message, "--isolation native") {
		t.Fatalf("failure = %q", failure.Message)
	}
}

func TestGrokReviewStrictReportsMissingExecutableBeforeCredential(t *testing.T) {
	t.Parallel()

	reviewer := NewGrok(GrokOptions{Repository: t.TempDir(), Executable: "missing-grok", Environment: []string{"PATH=/usr/bin:/bin"}})
	_, err := reviewer.Review(context.Background(), Request{Prompt: "bundle", Config: grokConfig(protocol.IsolationStrict, false, 5*time.Second)})
	failure := assertProviderError(t, err, protocol.FailureCapability)
	if !strings.Contains(failure.Message, "was not found") || strings.Contains(failure.Message, "API_KEY") {
		t.Fatalf("failure = %q", failure.Message)
	}
}

func TestGrokReviewRejectsMalformedEnvelopeAndReview(t *testing.T) {
	t.Parallel()

	validReview := validGrokReview()
	for _, test := range []struct {
		name             string
		output           string
		incomplete       bool
		wantReason       protocol.ProtocolReason
		wantMessage      string
		forbiddenMessage string
	}{
		{name: "not JSON", output: "not-json"},
		{name: "cancelled zero exit", output: `{"text":"","stopReason":"cancelled","sessionId":"s","requestId":"r","structuredOutput":null,"structuredOutputError":"model did not produce structured output"}`, incomplete: true, wantMessage: "stop reason cancelled"},
		{name: "unknown stop reason", output: `{"text":"","stopReason":"private-provider-detail","sessionId":"s","requestId":"r","structuredOutput":null}`, incomplete: true, wantMessage: "stop reason was not end_turn", forbiddenMessage: "private-provider-detail"},
		{name: "missing request identifier", output: `{"text":"` + escapeJSONString(validReview) + `","stopReason":"end_turn","sessionId":"","requestId":"r","structuredOutput":` + validReview + `}`, incomplete: true, wantMessage: "missing request identifiers"},
		{name: "missing structured output", output: `{"text":"{\"findings\":[]}","stopReason":"end_turn","sessionId":"s","requestId":"r","structuredOutput":null}`},
		{name: "duplicate envelope key", output: `{"text":"x","text":"x","stopReason":"end_turn","sessionId":"s","requestId":"r","structuredOutput":{}}`},
		{name: "invalid canonical review", output: grokEnvelope(`{"findings":[]}`)},
		{name: "incomplete low-confidence clean review", output: grokEnvelope(`{"findings":[],"overall_explanation":"Review is still in progress.","overall_confidence":0.01}`)},
		{name: "short high-confidence clean review", output: grokEnvelope(`{"findings":[],"overall_explanation":"Reviewing the requested changes before reporting the final result.","overall_confidence":0.95}`), wantReason: protocol.ProtocolReasonReviewValidation},
		{name: "short placeholder finding", output: grokEnvelope(`{"findings":[{"title":"Placeholder while reviewing","body":"Reviewing the requested change before reporting a concrete defect.","priority":"P2","confidence":0.5,"category":"maintainability","location":{"file_path":"file.go","start_line":1,"end_line":1}}],"overall_explanation":"Reviewing the requested changes before reporting the final result.","overall_confidence":0.5}`), wantReason: protocol.ProtocolReasonReviewValidation},
		{name: "explicit progress despite valid shape and confidence", output: grokEnvelope(validGrokReviewWithExplanation("Initial pass only records the frozen file set and acceptance criteria so the next complete review can inspect the actual implementation. This response is structurally complete but does not yet contain the requested analysis of changed behavior or actionable defects.")), wantReason: protocol.ProtocolReasonReviewValidation},
		{name: "padded canonical placeholder without completion evidence", output: grokRawEnvelope(validGrokReviewWithExplanation(validGrokExplanation + " The remaining analysis is still being checked and this interim response will be replaced by the final review after inspection finishes."))},
		{name: "duplicate review key", output: `{"text":"` + escapeJSONString(`{"findings":[],"overall_explanation":"No defects.","overall_confidence":0.95}`) + `","stopReason":"end_turn","sessionId":"s","requestId":"r","structuredOutput":{"findings":[],"findings":[],"overall_explanation":"No defects.","overall_confidence":0.95}}`},
		{name: "text mismatch", output: grokMismatchedEnvelope(validReview, validGrokReviewWithExplanation(validGrokExplanation+" The alternate document intentionally differs from the canonical control."))},
		{name: "valid control", output: grokEnvelope(validReview)},
	} {
		t.Run(test.name, func(t *testing.T) {
			fake := newFakeGrok(t, fakeGrokOptions{output: test.output})
			reviewer := NewGrok(GrokOptions{Repository: t.TempDir(), Executable: fake.path, Environment: []string{"PATH=/usr/bin:/bin", "XAI_API_KEY=secret"}})
			_, err := reviewer.Review(context.Background(), Request{Prompt: "bundle", Config: grokConfig(protocol.IsolationStrict, false, 5*time.Second)})
			if test.name == "valid control" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			expectedClass := protocol.FailureProtocol
			expectedOutcome := protocol.AttemptMalformed
			if test.incomplete {
				expectedClass = protocol.FailureProvider
				expectedOutcome = protocol.AttemptFailed
			}
			failure := assertProviderError(t, err, expectedClass)
			if test.wantReason != "" && failure.Reason != test.wantReason {
				t.Fatalf("protocol reason = %q, want %q", failure.Reason, test.wantReason)
			}
			if failure.Attempt == nil || failure.Attempt.Outcome != expectedOutcome {
				t.Fatalf("attempt = %+v", failure.Attempt)
			}
			assertExecutionMetadata(t, failure, protocol.ProviderGrok, "1.0.4", protocol.IsolationStrict, false)
			if test.wantMessage != "" && !strings.Contains(failure.Message, test.wantMessage) {
				t.Fatalf("failure message = %q, want %q", failure.Message, test.wantMessage)
			}
			if test.forbiddenMessage != "" && strings.Contains(failure.Message, test.forbiddenMessage) {
				t.Fatalf("failure message exposed private provider detail: %q", failure.Message)
			}
		})
	}
}

func TestGrokReviewRequiresExactPerFileCompletionEvidence(t *testing.T) {
	t.Parallel()

	target := protocol.Target{Files: []protocol.ReviewedFile{{FilePath: "a.go"}, {FilePath: "b.go"}}}
	assessment := "The changed logic in a.go was inspected against the complete task contract and its relevant edge cases, with no actionable defect identified."
	omittedFile := grokEnvelopeWithFiles(validGrokReview(), []grokCompletedFileReview{{
		FilePath:       "a.go",
		Assessment:     assessment,
		FindingIndexes: []int{},
	}})
	var review any
	if err := json.Unmarshal([]byte(validGrokReview()), &review); err != nil {
		t.Fatal(err)
	}
	missingFindingIndexes, err := json.Marshal(map[string]any{
		"review": review,
		"completion": map[string]any{
			"status": "complete",
			"files": []any{
				map[string]any{"file_path": "a.go", "assessment": assessment, "finding_indexes": []int{}},
				map[string]any{"file_path": "b.go", "assessment": assessment},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name   string
		output string
	}{
		{name: "omitted target file", output: omittedFile},
		{name: "missing required finding indexes", output: grokRawEnvelope(string(missingFindingIndexes))},
	} {
		t.Run(test.name, func(t *testing.T) {
			fake := newFakeGrok(t, fakeGrokOptions{output: test.output})
			reviewer := NewGrok(GrokOptions{Repository: t.TempDir(), Executable: fake.path, Environment: []string{"PATH=/usr/bin:/bin", "XAI_API_KEY=secret"}})
			_, err := reviewer.Review(context.Background(), Request{Prompt: "bundle", Config: grokConfig(protocol.IsolationStrict, false, 5*time.Second), Target: target})
			failure := assertProviderError(t, err, protocol.FailureProtocol)
			if failure.Reason != protocol.ProtocolReasonReviewValidation {
				t.Fatalf("protocol reason = %q, want %q", failure.Reason, protocol.ProtocolReasonReviewValidation)
			}
		})
	}
}

func TestValidateGrokCompletion(t *testing.T) {
	t.Parallel()

	target := protocol.Target{Files: []protocol.ReviewedFile{{FilePath: "a.go"}, {FilePath: "b.go"}}}
	review := protocol.Review{
		Findings: []protocol.Finding{{Location: protocol.Location{FilePath: "b.go"}}},
	}
	assessment := "The changed file was inspected against the complete task contract, relevant control flow, and edge cases before reaching this assessment."
	valid := grokCompletion{Status: "complete", Files: []grokCompletedFileReview{
		{FilePath: "a.go", Assessment: assessment, FindingIndexes: []int{}},
		{FilePath: "b.go", Assessment: assessment, FindingIndexes: []int{0}},
	}}
	if err := validateGrokCompletion(valid, review, target); err != nil {
		t.Fatalf("valid completion: %v", err)
	}

	for _, test := range []struct {
		name       string
		completion grokCompletion
	}{
		{name: "status", completion: grokCompletion{Files: valid.Files}},
		{name: "unexpected file", completion: grokCompletion{Status: "complete", Files: []grokCompletedFileReview{{FilePath: "a.go", Assessment: assessment}, {FilePath: "c.go", Assessment: assessment, FindingIndexes: []int{0}}}}},
		{name: "duplicate file", completion: grokCompletion{Status: "complete", Files: []grokCompletedFileReview{{FilePath: "a.go", Assessment: assessment}, {FilePath: "a.go", Assessment: assessment, FindingIndexes: []int{0}}}}},
		{name: "short assessment", completion: grokCompletion{Status: "complete", Files: []grokCompletedFileReview{{FilePath: "a.go", Assessment: "too short"}, {FilePath: "b.go", Assessment: assessment, FindingIndexes: []int{0}}}}},
		{name: "long assessment including whitespace", completion: grokCompletion{Status: "complete", Files: []grokCompletedFileReview{{FilePath: "a.go", Assessment: strings.Repeat("a", contractschema.GrokMaximumFileAssessmentCharacters) + " "}, {FilePath: "b.go", Assessment: assessment, FindingIndexes: []int{0}}}}},
		{name: "invalid finding index", completion: grokCompletion{Status: "complete", Files: []grokCompletedFileReview{{FilePath: "a.go", Assessment: assessment}, {FilePath: "b.go", Assessment: assessment, FindingIndexes: []int{1}}}}},
		{name: "wrong finding file", completion: grokCompletion{Status: "complete", Files: []grokCompletedFileReview{{FilePath: "a.go", Assessment: assessment, FindingIndexes: []int{0}}, {FilePath: "b.go", Assessment: assessment}}}},
		{name: "unlinked finding", completion: grokCompletion{Status: "complete", Files: []grokCompletedFileReview{{FilePath: "a.go", Assessment: assessment}, {FilePath: "b.go", Assessment: assessment}}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := validateGrokCompletion(test.completion, review, target); err == nil {
				t.Fatal("expected invalid completion")
			}
		})
	}
}

func TestGrokCompletionConfidenceDoesNotFilterIndividualFindings(t *testing.T) {
	t.Parallel()

	target := protocol.Target{Files: []protocol.ReviewedFile{{FilePath: "file.go"}}}
	review := protocol.Review{
		Findings: []protocol.Finding{{
			Title:      "Possible edge-case defect",
			Body:       "For the cited changed input, this branch may return the wrong result because the boundary condition skips the final element.",
			Priority:   protocol.PriorityP2,
			Confidence: 0.1,
			Category:   protocol.CategoryBug,
			Location:   protocol.Location{FilePath: "file.go", StartLine: 1, EndLine: 1},
		}},
		OverallExplanation: validGrokExplanation,
		OverallConfidence:  0.9,
	}
	encodedReview, err := json.Marshal(review)
	if err != nil {
		t.Fatal(err)
	}
	files := []grokCompletedFileReview{{
		FilePath:       "file.go",
		Assessment:     "The changed boundary branch was inspected against the supplied contract and the finding records the concrete uncertain edge case without suppressing it.",
		FindingIndexes: []int{0},
	}}
	document, err := decodeGrokCompletedReview([]byte(grokCompletedReviewJSON(string(encodedReview), files)), target)
	if err != nil {
		t.Fatal(err)
	}
	if len(document.Review.Findings) != 1 || document.Review.Findings[0].Confidence != 0.1 {
		t.Fatalf("review findings = %+v", document.Review.Findings)
	}

	review.OverallConfidence = 0.5
	encodedReview, err = json.Marshal(review)
	if err != nil {
		t.Fatal(err)
	}
	_, err = decodeGrokCompletedReview([]byte(grokCompletedReviewJSON(string(encodedReview), files)), target)
	if !errors.Is(err, errGrokIncompleteReview) {
		t.Fatalf("low completion confidence error = %v", err)
	}
}

func TestGrokReviewDistinguishesTimeoutAndCancellation(t *testing.T) {
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
			fake := newFakeGrok(t, fakeGrokOptions{delay: "0.5"})
			reviewer := NewGrok(GrokOptions{Repository: t.TempDir(), Executable: fake.path, Environment: []string{"PATH=/usr/bin:/bin", "XAI_API_KEY=secret"}})
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
			_, err := reviewer.Review(ctx, Request{Prompt: "bundle", Config: grokConfig(protocol.IsolationStrict, false, test.timeout)})
			_ = assertProviderError(t, err, test.class)
		})
	}
}

func TestGrokReviewEnforcesOutputBounds(t *testing.T) {
	t.Parallel()

	fake := newFakeGrok(t, fakeGrokOptions{output: strings.Repeat("x", int(providerStdoutLimit)+1)})
	reviewer := NewGrok(GrokOptions{Repository: t.TempDir(), Executable: fake.path, Environment: []string{"PATH=/usr/bin:/bin", "XAI_API_KEY=secret"}})
	_, err := reviewer.Review(context.Background(), Request{Prompt: "bundle", Config: grokConfig(protocol.IsolationStrict, false, 5*time.Second)})
	failure := assertProviderError(t, err, protocol.FailureProvider)
	if failure.Attempt == nil || failure.Attempt.Outcome != protocol.AttemptFailed {
		t.Fatalf("attempt = %+v", failure.Attempt)
	}
	assertExecutionMetadata(t, failure, protocol.ProviderGrok, "1.0.4", protocol.IsolationStrict, false)
}

func TestGrokReviewUsesExplicitDefaultModel(t *testing.T) {
	t.Parallel()

	effective := grokConfig(protocol.IsolationStrict, false, 5*time.Second)
	effective.Model = config.Value[string]{Source: config.SourceDefault}
	fake := newFakeGrok(t, fakeGrokOptions{})
	reviewer := NewGrok(GrokOptions{Repository: t.TempDir(), Executable: fake.path, Environment: []string{"PATH=/usr/bin:/bin", "XAI_API_KEY=secret"}})
	result, err := reviewer.Review(context.Background(), Request{Prompt: "bundle", Config: effective})
	if err != nil {
		t.Fatal(err)
	}
	if DefaultGrokModel != "grok-4.6" {
		t.Fatalf("DefaultGrokModel = %q", DefaultGrokModel)
	}
	if result.Provider.Model != DefaultGrokModel {
		t.Fatalf("provider model = %q", result.Provider.Model)
	}
	arguments := strings.Split(strings.TrimSpace(readTestFile(t, fake.arguments)), "\n")
	if argumentAfter(arguments, "--model") != DefaultGrokModel {
		t.Fatalf("Grok arguments = %v", arguments)
	}
}

func TestGrokReviewOmitsProviderFailureOutput(t *testing.T) {
	t.Parallel()

	fake := newFakeGrok(t, fakeGrokOptions{reviewError: "\033[31m" + providerOutputSentinel + " test-provider-secret\r"})
	reviewer := NewGrok(GrokOptions{Repository: t.TempDir(), Executable: fake.path, Environment: []string{"PATH=/usr/bin:/bin", "XAI_API_KEY=test-provider-secret"}})
	_, err := reviewer.Review(context.Background(), Request{Prompt: providerOutputSentinel, Config: grokConfig(protocol.IsolationStrict, false, 5*time.Second)})
	failure := assertProviderError(t, err, protocol.FailureProvider)
	if strings.Contains(failure.Message, providerOutputSentinel) || strings.Contains(failure.Message, "test-provider-secret") || strings.ContainsRune(failure.Message, '\x1b') {
		t.Fatalf("provider failure = %q", failure.Message)
	}
	if !strings.Contains(failure.Message, "exit code 7") || !strings.Contains(failure.Message, "private diagnostics") {
		t.Fatalf("provider failure lacks safe context: %q", failure.Message)
	}
}

type fakeGrokOptions struct {
	help        string
	loggedOut   bool
	authError   string
	authStatus  string
	output      string
	reviewError string
	delay       string
}

type fakeGrok struct {
	path        string
	arguments   string
	prompt      string
	environment string
}

func newFakeGrok(t *testing.T, options fakeGrokOptions) fakeGrok {
	t.Helper()
	root := t.TempDir()
	fake := fakeGrok{
		path: filepath.Join(root, "grok"), arguments: filepath.Join(root, "arguments.txt"),
		prompt: filepath.Join(root, "prompt.txt"), environment: filepath.Join(root, "environment.txt"),
	}
	if options.help == "" {
		options.help = grokHelp()
	}
	if options.output == "" {
		options.output = grokEnvelope(validGrokReview())
	}
	outputPath := filepath.Join(root, "output.json")
	if err := os.WriteFile(outputPath, []byte(options.output), 0o600); err != nil {
		t.Fatal(err)
	}
	authBlock := "printf '%s\\n' 'You are logged in with grok.com.\n\nDefault model: grok-4.6\n\nAvailable models:\n  * grok-4.6 (default)'\nexit 0"
	if options.loggedOut {
		authBlock = "printf '%s\\n' 'You are not authenticated.\n\nDefault model: grok-4.6\n\nAvailable models:\n  * grok-4.6 (default)'\nexit 0"
	}
	if options.authStatus != "" {
		authBlock = "printf '%s\\n' " + shellQuote(options.authStatus) + "\nexit 0"
	}
	if options.authError != "" {
		authBlock = "printf '%s' " + shellQuote(options.authError) + " >&2\nexit 1"
	}
	reviewFailure := ""
	if options.reviewError != "" {
		reviewFailure = "printf '%b' " + shellQuote(options.reviewError) + " >&2\nexit 7\n"
	}
	delay := ""
	if options.delay != "" {
		delay = "sleep " + options.delay + "\n"
	}
	script := "#!/bin/sh\n" +
		"set -eu\n" +
		"fail_contract() { printf '%s\\n' 'unexpected Grok CLI arguments' >&2; exit 64; }\n" +
		"if [ \"$#\" -eq 1 ] && [ \"$1\" = '--version' ]; then printf '%s\\n' 'grok 1.0.4 (fake)'; exit 0; fi\n" +
		"if [ \"$#\" -eq 1 ] && [ \"$1\" = '--help' ]; then printf '%s\\n' " + shellQuote(options.help) + "; exit 0; fi\n" +
		"if [ \"$#\" -eq 1 ] && [ \"$1\" = 'models' ]; then " + authBlock + "; fi\n" +
		"printf '%s\\n' \"$@\" > " + shellQuote(fake.arguments) + "\n" +
		"[ \"${1:-}\" = '--prompt-file' ] || fail_contract; shift\n" +
		"prompt_path=${1:-}; [ -f \"$prompt_path\" ] || fail_contract; shift\n" +
		"[ \"${1:-}\" = '--output-format' ] || fail_contract; shift; [ \"${1:-}\" = 'json' ] || fail_contract; shift\n" +
		"[ \"${1:-}\" = '--json-schema' ] || fail_contract; shift; case \"${1:-}\" in '{'*'}') ;; *) fail_contract ;; esac; shift\n" +
		"[ \"${1:-}\" = '--model' ] || fail_contract; shift; [ -n \"${1:-}\" ] || fail_contract; case \"$1\" in -*) fail_contract ;; esac; shift\n" +
		"[ \"${1:-}\" = '--reasoning-effort' ] || fail_contract; shift; [ -n \"${1:-}\" ] || fail_contract; shift\n" +
		"[ \"${1:-}\" = '--max-turns' ] || fail_contract; shift; [ \"${1:-}\" = '2' ] || fail_contract; shift\n" +
		"[ \"${1:-}\" = '--permission-mode' ] || fail_contract; shift; [ \"${1:-}\" = 'dontAsk' ] || fail_contract; shift\n" +
		"for flag in --no-plan --no-subagents --no-memory --verbatim; do [ \"${1:-}\" = \"$flag\" ] || fail_contract; shift; done\n" +
		"[ \"${1:-}\" = '--cwd' ] || fail_contract; shift; [ -d \"${1:-}\" ] || fail_contract; shift\n" +
		"for tool in Bash Edit Write Read Grep MCPTool; do [ \"${1:-}\" = '--deny' ] || fail_contract; shift; [ \"${1:-}\" = \"$tool\" ] || fail_contract; shift; done\n" +
		"[ \"${1:-}\" = '--tools' ] || fail_contract; shift; if [ \"${1:-}\" = 'web_search,web_fetch' ]; then shift; [ \"${1:-}\" = '--disallowed-tools' ] || fail_contract; shift; [ \"${1:-}\" = 'search_tool,use_tool,Agent' ] || fail_contract; shift; [ \"${1:-}\" = '--allow' ] || fail_contract; shift; [ \"${1:-}\" = 'WebFetch' ] || fail_contract; shift; [ \"${1:-}\" = '--allow' ] || fail_contract; shift; [ \"${1:-}\" = 'WebSearch' ] || fail_contract; shift; else [ \"${1:-}\" = 'web_search' ] || fail_contract; shift; [ \"${1:-}\" = '--disallowed-tools' ] || fail_contract; shift; [ \"${1:-}\" = 'web_search,search_tool,use_tool,Agent' ] || fail_contract; shift; [ \"${1:-}\" = '--disable-web-search' ] || fail_contract; shift; for tool in WebFetch WebSearch; do [ \"${1:-}\" = '--deny' ] || fail_contract; shift; [ \"${1:-}\" = \"$tool\" ] || fail_contract; shift; done; fi\n" +
		"if [ \"${1:-}\" = '--sandbox' ]; then shift; [ \"${1:-}\" = 'workspace' ] || fail_contract; shift; fi\n" +
		"[ \"$#\" -eq 0 ] || fail_contract\n" +
		"cat \"$prompt_path\" > " + shellQuote(fake.prompt) + "\n" +
		"env > " + shellQuote(fake.environment) + "\n" +
		reviewFailure + delay +
		"cat " + shellQuote(outputPath) + "\n"
	writeTestExecutableAt(t, fake.path, script)
	return fake
}

func grokHelp() string {
	return strings.Join([]string{
		"--prompt-file <path>",
		"--output-format <format> possible values: plain, json, streaming-json",
		"--json-schema <schema>",
		"--model <model>",
		"--reasoning-effort <effort>",
		"--max-turns <n>",
		"--permission-mode <mode> possible values: default, dontAsk",
		"--tools <tools>",
		"--disallowed-tools <tools>",
		"--allow <rule>",
		"--deny <rule>",
		"--no-plan", "--no-subagents", "--no-memory", "--disable-web-search", "--verbatim", "--cwd <path>", "--sandbox <profile>", "models",
	}, "\n")
}

func grokEnvelope(review string) string {
	return grokMismatchedEnvelope(review, review)
}

func grokEnvelopeWithFiles(review string, files []grokCompletedFileReview) string {
	document := grokCompletedReviewJSON(review, files)
	return grokRawMismatchedEnvelope(document, document)
}

func grokRawEnvelope(document string) string {
	return grokRawMismatchedEnvelope(document, document)
}

const validGrokExplanation = "The complete frozen target was reviewed against its trusted task contract, including every changed line and required edge case. The implementation preserves the requested behavior, introduces no actionable defect, and the cited tests cover the relevant success and failure paths."

func validGrokReview() string {
	return validGrokReviewWithExplanation(validGrokExplanation)
}

func validGrokReviewWithExplanation(explanation string) string {
	review, err := json.Marshal(protocol.Review{
		Findings:           []protocol.Finding{},
		OverallExplanation: explanation,
		OverallConfidence:  0.95,
	})
	if err != nil {
		panic(err)
	}
	return string(review)
}

func grokMismatchedEnvelope(textReview, structuredReview string) string {
	return grokRawMismatchedEnvelope(grokCompletedReviewJSON(textReview, nil), grokCompletedReviewJSON(structuredReview, nil))
}

func grokCompletedReviewJSON(review string, files []grokCompletedFileReview) string {
	var decodedReview any
	if err := json.Unmarshal([]byte(review), &decodedReview); err != nil {
		panic(err)
	}
	if files == nil {
		files = []grokCompletedFileReview{}
	}
	encoded, err := json.Marshal(map[string]any{
		"review": decodedReview,
		"completion": grokCompletion{
			Status: "complete",
			Files:  files,
		},
	})
	if err != nil {
		panic(err)
	}
	return string(encoded)
}

func grokRawMismatchedEnvelope(textReview, structuredReview string) string {
	var structured any
	if err := json.Unmarshal([]byte(structuredReview), &structured); err != nil {
		panic(err)
	}
	encoded, err := json.Marshal(map[string]any{
		"text": textReview, "stopReason": "end_turn", "sessionId": "fake-session", "requestId": "fake-request", "structuredOutput": structured,
	})
	if err != nil {
		panic(err)
	}
	return string(encoded)
}

func escapeJSONString(value string) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return string(encoded[1 : len(encoded)-1])
}

func containsArgumentPair(arguments []string, flag, value string) bool {
	for index := 0; index+1 < len(arguments); index++ {
		if arguments[index] == flag && arguments[index+1] == value {
			return true
		}
	}
	return false
}

func grokConfig(isolation protocol.Isolation, web bool, timeout time.Duration) config.Effective {
	return config.Effective{
		Engine:          config.Value[protocol.ProviderName]{Value: protocol.ProviderGrok, Source: config.SourceFlag},
		Model:           config.Value[string]{Value: "test-model", Source: config.SourceFlag},
		ReasoningEffort: config.Value[config.ReasoningEffort]{Value: config.ReasoningHigh, Source: config.SourceDefault},
		Timeout:         config.Value[config.Duration]{Value: config.Duration(timeout), Source: config.SourceFlag},
		Retries:         config.Value[int]{Value: 1, Source: config.SourceDefault},
		MaxBytes:        config.Value[int64]{Value: 1 << 20, Source: config.SourceDefault},
		Isolation:       config.Value[protocol.Isolation]{Value: isolation, Source: config.SourceFlag},
		WebAccess:       config.Value[bool]{Value: web, Source: config.SourceFlag},
	}
}
