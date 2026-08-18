package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/uinaf/ffss/cli/slopguard/internal/config"
	"github.com/uinaf/ffss/cli/slopguard/internal/protocol"
	"github.com/uinaf/ffss/cli/slopguard/internal/provider"
	"github.com/uinaf/ffss/cli/slopguard/internal/target"
	contractschema "github.com/uinaf/ffss/cli/slopguard/schema"
)

func TestReviewCommandCleanJSON(t *testing.T) {
	t.Parallel()

	repository := reviewRepository(t)
	reviewer := &scriptedReviewer{results: []reviewStep{{result: cleanResult()}}}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exit := run(t.Context(), []string{
		"review", "--repository", repository, "--mode", "local", "--engine", "codex",
		"--prompt", "Check the acceptance criteria.", "--context-file", "CONTEXT.md", "--output", "json",
	}, &stdout, &stderr, reviewDependencies(t, cleanScanner{}, reviewer))
	if exit != 0 {
		t.Fatalf("run() exit = %d, stderr = %s, stdout = %s", exit, stderr.String(), stdout.String())
	}
	var result protocol.Report
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("decode report: %v: %s", err, stdout.String())
	}
	if err := result.Validate(); err != nil {
		t.Fatalf("report validation: %v", err)
	}
	if result.Status != protocol.StatusClean || len(result.Metadata.Attempts) != 1 {
		t.Fatalf("result = %+v", result)
	}
	if !strings.Contains(reviewer.prompts[0], "Check the acceptance criteria.") || !strings.Contains(reviewer.prompts[0], "context evidence") {
		t.Fatalf("provider prompt omitted trusted prompt or context: %s", reviewer.prompts[0])
	}
	if strings.Contains(stdout.String(), "collecting frozen target") || !strings.Contains(stderr.String(), "reviewing with codex") {
		t.Fatalf("stdout/stderr separation failed: stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestReviewCommandPrintsEveryFindingAndExitsOne(t *testing.T) {
	t.Parallel()

	repository := reviewRepository(t)
	result := cleanResult()
	result.Review.Findings = []protocol.Finding{
		finding("high confidence", 0.99),
		finding("low confidence", 0.01),
	}
	reviewer := &scriptedReviewer{results: []reviewStep{{result: result}}}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exit := run(t.Context(), []string{"review", "--repository", repository, "--mode", "local", "--engine", "codex"}, &stdout, &stderr, reviewDependencies(t, cleanScanner{}, reviewer))
	if exit != 1 || !strings.Contains(stdout.String(), "high confidence") || !strings.Contains(stdout.String(), "low confidence") {
		t.Fatalf("run() exit = %d, stdout = %s, stderr = %s", exit, stdout.String(), stderr.String())
	}
}

func TestReviewCommandRetriesOnlyProtocolFailures(t *testing.T) {
	t.Parallel()

	repository := reviewRepository(t)
	reviewer := &scriptedReviewer{results: []reviewStep{
		{err: providerError(protocol.FailureProtocol, protocol.AttemptMalformed)},
		{result: cleanResult()},
	}}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exit := run(t.Context(), []string{"review", "--repository", repository, "--mode", "local", "--engine", "codex", "--retries", "1", "--output", "json"}, &stdout, &stderr, reviewDependencies(t, cleanScanner{}, reviewer))
	if exit != 0 {
		t.Fatalf("run() exit = %d, stderr = %s, stdout = %s", exit, stderr.String(), stdout.String())
	}
	result, err := protocol.DecodeReport(stdout.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if len(reviewer.prompts) != 2 || len(result.Metadata.Attempts) != 2 || result.Metadata.Attempts[0].Outcome != protocol.AttemptMalformed {
		t.Fatalf("prompts=%d attempts=%+v", len(reviewer.prompts), result.Metadata.Attempts)
	}
	if !strings.HasPrefix(reviewer.prompts[1], reviewer.prompts[0]) || !strings.Contains(stderr.String(), "retrying malformed provider response") {
		t.Fatalf("retry did not reuse frozen prompt: stderr=%q", stderr.String())
	}

	authReviewer := &scriptedReviewer{results: []reviewStep{{err: providerError(protocol.FailureAuth, protocol.AttemptFailed)}, {result: cleanResult()}}}
	stdout.Reset()
	stderr.Reset()
	exit = run(t.Context(), []string{"review", "--repository", repository, "--mode", "local", "--engine", "codex", "--retries", "1", "--output", "json"}, &stdout, &stderr, reviewDependencies(t, cleanScanner{}, authReviewer))
	if exit != 2 || len(authReviewer.prompts) != 1 {
		t.Fatalf("authentication failure retried: exit=%d prompts=%d output=%s", exit, len(authReviewer.prompts), stdout.String())
	}
}

func TestReviewCommandClassifiesPlainReviewerError(t *testing.T) {
	t.Parallel()

	repository := reviewRepository(t)
	reviewer := &scriptedReviewer{results: []reviewStep{{err: errors.New("unwrapped reviewer failure")}, {result: cleanResult()}}}
	var stdout bytes.Buffer
	exit := run(t.Context(), []string{"review", "--repository", repository, "--mode", "local", "--engine", "codex", "--retries", "1", "--output", "json"}, &stdout, io.Discard, reviewDependencies(t, cleanScanner{}, reviewer))
	if exit != 2 || len(reviewer.prompts) != 1 {
		t.Fatalf("exit=%d calls=%d output=%s", exit, len(reviewer.prompts), stdout.String())
	}
	var result protocol.Report
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Failure == nil || result.Failure.Class != protocol.FailureProvider || len(result.Metadata.Attempts) != 1 || result.Metadata.Attempts[0].Outcome != protocol.AttemptFailed {
		t.Fatalf("result=%+v", result)
	}
	if result.Metadata.Provider != nil || result.Metadata.Isolation != nil || result.Metadata.WebAccess || result.Metadata.ProtocolRecovery.Applied {
		t.Fatalf("unresolved execution metadata = %+v", result.Metadata)
	}
}

func TestReviewCommandRejectsInvalidFindingBoundaries(t *testing.T) {
	t.Parallel()

	repository := reviewRepository(t)
	invalid := cleanResult()
	invalid.Review.Findings = []protocol.Finding{finding("outside target", 0.5)}
	invalid.Review.Findings[0].Location.FilePath = "other.go"
	reviewer := &scriptedReviewer{results: []reviewStep{{result: invalid}, {result: invalid}}}
	var stdout bytes.Buffer
	exit := run(t.Context(), []string{"review", "--repository", repository, "--mode", "local", "--engine", "codex", "--retries", "1", "--output", "json"}, &stdout, io.Discard, reviewDependencies(t, cleanScanner{}, reviewer))
	if exit != 2 || len(reviewer.prompts) != 2 {
		t.Fatalf("run() exit = %d, prompts = %d, output = %s", exit, len(reviewer.prompts), stdout.String())
	}
	retry := strings.TrimPrefix(reviewer.prompts[1], reviewer.prompts[0])
	if !strings.Contains(retry, "Correction category: finding_location") || !strings.Contains(retry, "must cite a reviewed file") {
		t.Fatalf("retry guidance = %q", retry)
	}
	var result protocol.Report
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Failure == nil || result.Failure.Class != protocol.FailureProtocol || !strings.Contains(result.Failure.Message, string(protocol.ProtocolReasonFindingLocation)) || len(result.Metadata.Attempts) != 2 {
		t.Fatalf("result = %+v", result)
	}
}

func TestReviewCommandRecoversDiscontiguousFindingLocation(t *testing.T) {
	t.Parallel()

	repository := reviewRepositoryWithDiscontiguousRanges(t)
	invalid := cleanResult()
	invalid.Review.Findings = []protocol.Finding{finding("cross-hunk location", 0.9)}
	invalid.Review.Findings[0].Location.StartLine = 10
	invalid.Review.Findings[0].Location.EndLine = 50
	valid := cleanResult()
	valid.Review.Findings = []protocol.Finding{finding("narrowed location", 0.9)}
	valid.Review.Findings[0].Location.StartLine = 30
	valid.Review.Findings[0].Location.EndLine = 30
	reviewer := &scriptedReviewer{results: []reviewStep{{result: invalid}, {result: valid}}}
	var stdout bytes.Buffer
	exit := run(t.Context(), []string{"review", "--repository", repository, "--mode", "local", "--engine", "codex", "--retries", "1", "--output", "json"}, &stdout, io.Discard, reviewDependencies(t, cleanScanner{}, reviewer))
	if exit != 1 || len(reviewer.prompts) != 2 {
		t.Fatalf("run() exit = %d, prompts = %d, output = %s", exit, len(reviewer.prompts), stdout.String())
	}
	if !strings.HasPrefix(reviewer.prompts[1], reviewer.prompts[0]) {
		t.Fatalf("retry did not preserve frozen prompt")
	}
	retry := strings.TrimPrefix(reviewer.prompts[1], reviewer.prompts[0])
	if !strings.Contains(retry, "Correction category: finding_location") || !strings.Contains(retry, "one individual line_ranges entry") || !strings.Contains(retry, "split it into findings") {
		t.Fatalf("retry guidance = %q", retry)
	}
	var result protocol.Report
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Status != protocol.StatusFindings || len(result.Metadata.Attempts) != 2 || result.Metadata.Attempts[0].Outcome != protocol.AttemptMalformed || result.Metadata.Attempts[1].Outcome != protocol.AttemptValid {
		t.Fatalf("result = %+v", result)
	}
}

func TestReviewCommandRetriesMetadataMismatchWithGenericCorrection(t *testing.T) {
	t.Parallel()

	repository := reviewRepository(t)
	mismatched := cleanResult()
	mismatched.Provider.Name = protocol.ProviderClaude
	reviewer := &scriptedReviewer{results: []reviewStep{{result: mismatched}, {result: cleanResult()}}}
	var stdout bytes.Buffer
	exit := run(t.Context(), []string{"review", "--repository", repository, "--mode", "local", "--engine", "codex", "--retries", "1", "--output", "json"}, &stdout, io.Discard, reviewDependencies(t, cleanScanner{}, reviewer))
	if exit != 0 || len(reviewer.prompts) != 2 {
		t.Fatalf("run() exit = %d, prompts = %d, output = %s", exit, len(reviewer.prompts), stdout.String())
	}
	retry := strings.TrimPrefix(reviewer.prompts[1], reviewer.prompts[0])
	if !strings.Contains(retry, "Correction category: metadata_mismatch") || !strings.Contains(retry, "Return exactly one review object matching the supplied schema") || strings.Contains(retry, "provider execution metadata") {
		t.Fatalf("retry guidance = %q", retry)
	}
}

func TestReviewCommandPreservesExecutionMetadataAfterProtocolRetryExhausts(t *testing.T) {
	t.Parallel()

	execution := &provider.Execution{
		Provider:  protocol.Provider{Name: protocol.ProviderCursor, Model: "cursor-grok-4.5-high-fast", Version: "2026.08.04-aaa8809"},
		Isolation: protocol.IsolationNative,
		WebAccess: true,
	}
	reviewer := &scriptedReviewer{results: []reviewStep{{err: cursorProtocolError(execution, protocol.ProtocolReasonMultipleDocuments)}, {err: cursorProtocolError(execution, protocol.ProtocolReasonMultipleDocuments)}}}
	var stdout bytes.Buffer
	exit := run(t.Context(), []string{"review", "--repository", reviewRepository(t), "--mode", "local", "--engine", "cursor", "--retries", "1", "--output", "json"}, &stdout, io.Discard, reviewDependencies(t, cleanScanner{}, reviewer))
	if exit != 2 || len(reviewer.prompts) != 2 {
		t.Fatalf("run() exit = %d, prompts = %d, output = %s", exit, len(reviewer.prompts), stdout.String())
	}
	retry := strings.TrimPrefix(reviewer.prompts[1], reviewer.prompts[0])
	if !strings.Contains(retry, "Correction category: multiple_documents") || !strings.Contains(retry, "exactly one review object") {
		t.Fatalf("retry guidance = %q", retry)
	}
	result, err := protocol.DecodeReport(stdout.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if result.Failure == nil || !strings.Contains(result.Failure.Message, string(protocol.ProtocolReasonMultipleDocuments)) || result.Metadata.Provider == nil || *result.Metadata.Provider != execution.Provider || result.Metadata.Isolation == nil || *result.Metadata.Isolation != execution.Isolation || !result.Metadata.WebAccess {
		t.Fatalf("result = %+v", result)
	}
}

func TestReviewCommandPreservesRecoveryAcrossProtocolRetry(t *testing.T) {
	t.Parallel()

	strategy := protocol.RecoveryCursorTrailingObject
	recovered := cleanResult()
	recovered.Provider = protocol.Provider{Name: protocol.ProviderCursor, Model: "cursor-grok-4.5-high-fast", Version: "2026.08.04-aaa8809"}
	recovered.WebAccess = true
	recovered.ProtocolRecovery = protocol.ProtocolRecovery{Applied: true, Strategy: &strategy}
	recovered.Review.Findings = []protocol.Finding{finding("outside target", 0.9)}
	recovered.Review.Findings[0].Location.FilePath = "other.go"
	execution := &provider.Execution{
		Provider:  recovered.Provider,
		Isolation: recovered.Isolation,
		WebAccess: recovered.WebAccess,
	}
	unrecovered := recovered
	unrecovered.ProtocolRecovery = protocol.ProtocolRecovery{}
	for _, test := range []struct {
		name  string
		retry reviewStep
	}{
		{name: "provider error", retry: reviewStep{err: cursorProtocolError(execution, protocol.ProtocolReasonMultipleDocuments)}},
		{name: "invalid result", retry: reviewStep{result: unrecovered}},
	} {
		t.Run(test.name, func(t *testing.T) {
			reviewer := &scriptedReviewer{results: []reviewStep{{result: recovered}, test.retry}}
			var stdout bytes.Buffer
			exit := run(t.Context(), []string{"review", "--repository", reviewRepository(t), "--mode", "local", "--engine", "cursor", "--retries", "1", "--output", "json"}, &stdout, io.Discard, reviewDependencies(t, cleanScanner{}, reviewer))
			if exit != 2 {
				t.Fatalf("run() exit = %d, output = %s", exit, stdout.String())
			}
			result, err := protocol.DecodeReport(stdout.Bytes())
			if err != nil {
				t.Fatal(err)
			}
			if !result.Metadata.ProtocolRecovery.Applied || result.Metadata.ProtocolRecovery.Strategy == nil || *result.Metadata.ProtocolRecovery.Strategy != strategy {
				t.Fatalf("protocol recovery = %+v", result.Metadata.ProtocolRecovery)
			}
		})
	}
}

func TestReviewCommandRejectsIncompleteLowConfidenceCleanResult(t *testing.T) {
	t.Parallel()

	repository := reviewRepository(t)
	incomplete := cleanResult()
	incomplete.Review.OverallExplanation = "Review is still in progress."
	incomplete.Review.OverallConfidence = 0.01
	reviewer := &scriptedReviewer{results: []reviewStep{{result: incomplete}, {result: incomplete}}}
	var stdout bytes.Buffer
	exit := run(t.Context(), []string{"review", "--repository", repository, "--mode", "local", "--engine", "codex", "--retries", "1", "--output", "json"}, &stdout, io.Discard, reviewDependencies(t, cleanScanner{}, reviewer))
	if exit != 2 || len(reviewer.prompts) != 2 {
		t.Fatalf("run() exit = %d, prompts = %d, output = %s", exit, len(reviewer.prompts), stdout.String())
	}
	var result protocol.Report
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Failure == nil || result.Failure.Class != protocol.FailureProtocol || len(result.Metadata.Attempts) != 2 || result.Metadata.Attempts[0].Outcome != protocol.AttemptMalformed {
		t.Fatalf("result = %+v", result)
	}
}

func TestReviewCommandClassifiesSecretTimeoutAndCancellation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		scanner     target.Scanner
		reviewer    *scriptedReviewer
		wantClass   protocol.FailureClass
		wantMessage string
		wantCalls   int
	}{
		{name: "secret", scanner: scannerError{err: target.ErrSecretFound}, reviewer: &scriptedReviewer{}, wantClass: protocol.FailureSecretScan, wantMessage: target.ErrSecretFound.Error(), wantCalls: 0},
		{name: "timeout", scanner: cleanScanner{}, reviewer: &scriptedReviewer{results: []reviewStep{{err: providerError(protocol.FailureTimeout, protocol.AttemptFailed)}}}, wantClass: protocol.FailureTimeout, wantCalls: 1},
		{name: "cancelled", scanner: cleanScanner{}, reviewer: &scriptedReviewer{results: []reviewStep{{err: providerError(protocol.FailureCancelled, protocol.AttemptFailed)}}}, wantClass: protocol.FailureCancelled, wantCalls: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := reviewRepository(t)
			var stdout bytes.Buffer
			exit := run(t.Context(), []string{"review", "--repository", repository, "--mode", "local", "--engine", "codex", "--output", "json"}, &stdout, io.Discard, reviewDependencies(t, test.scanner, test.reviewer))
			if exit != 2 || len(test.reviewer.prompts) != test.wantCalls {
				t.Fatalf("exit=%d calls=%d output=%s", exit, len(test.reviewer.prompts), stdout.String())
			}
			var result protocol.Report
			if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if result.Failure == nil || result.Failure.Class != test.wantClass {
				t.Fatalf("failure = %+v", result.Failure)
			}
			if test.wantMessage != "" && result.Failure.Message != test.wantMessage {
				t.Fatalf("failure message = %q, want %q", result.Failure.Message, test.wantMessage)
			}
		})
	}
}

func TestReviewCommandRefusesSourceMutation(t *testing.T) {
	t.Parallel()

	repository := reviewRepository(t)
	reviewer := &scriptedReviewer{results: []reviewStep{{before: func() {
		if err := os.WriteFile(filepath.Join(repository, "app.go"), []byte("mutated\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}, result: cleanResult()}}}
	var stdout bytes.Buffer
	exit := run(t.Context(), []string{"review", "--repository", repository, "--mode", "local", "--engine", "codex", "--output", "json"}, &stdout, io.Discard, reviewDependencies(t, cleanScanner{}, reviewer))
	if exit != 2 {
		t.Fatalf("run() exit = %d, output = %s", exit, stdout.String())
	}
	var result protocol.Report
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Failure == nil || result.Failure.Class != protocol.FailureSourceChanged {
		t.Fatalf("failure = %+v", result.Failure)
	}
	if len(result.Metadata.Attempts) != 1 || result.Metadata.Attempts[0].Outcome != protocol.AttemptValid {
		t.Fatalf("completed provider attempt was lost: %+v", result.Metadata.Attempts)
	}
}

func TestReviewCommandReportsOutputFailure(t *testing.T) {
	t.Parallel()

	repository := reviewRepository(t)
	var stderr bytes.Buffer
	exit := run(t.Context(), []string{"review", "--repository", repository, "--mode", "local", "--engine", "codex"}, failingWriter{}, &stderr, reviewDependencies(t, cleanScanner{}, &scriptedReviewer{results: []reviewStep{{result: cleanResult()}}}))
	if exit != 2 || !strings.Contains(stderr.String(), "write review result") {
		t.Fatalf("run() exit = %d, stderr = %s", exit, stderr.String())
	}
}

func TestTopLevelAndCommandHelp(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	if exit := run(t.Context(), []string{"--help"}, &stdout, io.Discard, dependencies{}); exit != 0 || !strings.Contains(stdout.String(), "slopguard <review|config|schema|selfupdate|version>") || !strings.Contains(stdout.String(), "canonical review or result JSON Schema") {
		t.Fatalf("top-level help exit=%d output=%q", exit, stdout.String())
	}
	stdout.Reset()
	if exit := run(t.Context(), []string{"review", "--help"}, &stdout, io.Discard, dependencies{}); exit != 0 || !strings.Contains(stdout.String(), "Usage of slopguard review") {
		t.Fatalf("review help exit=%d output=%q", exit, stdout.String())
	}
	stdout.Reset()
	if exit := run(t.Context(), []string{"schema", "--help"}, &stdout, io.Discard, dependencies{}); exit != 0 || !strings.Contains(stdout.String(), "schema <review|result>") {
		t.Fatalf("schema help exit=%d output=%q", exit, stdout.String())
	}
}

func TestSchemaCommandWritesCanonicalDocuments(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		want []byte
	}{
		{name: "review", want: contractschema.ReviewV1()},
		{name: "result", want: contractschema.ResultV1()},
	} {
		t.Run(test.name, func(t *testing.T) {
			var stdout bytes.Buffer
			if exit := run(t.Context(), []string{"schema", test.name}, &stdout, io.Discard, dependencies{}); exit != 0 {
				t.Fatalf("run() exit = %d", exit)
			}
			if !bytes.Equal(stdout.Bytes(), test.want) {
				t.Fatalf("schema output does not match embedded %s schema", test.name)
			}
			var document map[string]any
			if err := json.Unmarshal(stdout.Bytes(), &document); err != nil {
				t.Fatalf("schema output is not JSON: %v", err)
			}
		})
	}
}

func TestSchemaCommandRejectsInvalidArguments(t *testing.T) {
	t.Parallel()

	for _, arguments := range [][]string{{"schema"}, {"schema", "unknown"}, {"schema", "review", "extra"}} {
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		if exit := run(t.Context(), arguments, &stdout, &stderr, dependencies{}); exit != 2 || stdout.Len() != 0 || stderr.Len() == 0 {
			t.Fatalf("arguments=%q exit/output = %d/%q/%q", arguments, exit, stdout.String(), stderr.String())
		}
	}
}

func TestSchemaCommandReportsOutputFailure(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer
	if exit := run(t.Context(), []string{"schema", "review"}, failingWriter{}, &stderr, dependencies{}); exit != 2 || !strings.Contains(stderr.String(), "write review schema") {
		t.Fatalf("run() exit = %d, stderr = %q", exit, stderr.String())
	}
}

func TestInvalidFlagBeforeHelpStaysOnStderr(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exit := run(t.Context(), []string{"review", "--unknown", "--help"}, &stdout, &stderr, dependencies{})
	if exit != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "flag provided but not defined") {
		t.Fatalf("exit=%d stdout=%q stderr=%q", exit, stdout.String(), stderr.String())
	}
}

func TestReviewCommandKeepsRequestedJSONAcrossEarlyFailures(t *testing.T) {
	t.Parallel()

	repository := reviewRepository(t)
	tests := []struct {
		name      string
		arguments []string
	}{
		{name: "unknown flag before output", arguments: []string{"--unknown", "--output", "json"}},
		{name: "unknown flag after output", arguments: []string{"--output=json", "--unknown"}},
		{name: "positional argument", arguments: []string{"extra", "--output", "json"}},
		{name: "invalid typed override", arguments: []string{"--output", "json", "--timeout", "not-a-duration"}},
		{name: "configuration failure", arguments: []string{"--output", "json", "--repository", repository, "--engine", "invalid"}},
		{name: "conflicting output selectors", arguments: []string{"--output", "terminal", "--output=json"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			exit := run(t.Context(), append([]string{"review"}, test.arguments...), &stdout, &stderr, dependencies{})
			if exit != 2 {
				t.Fatalf("exit=%d stdout=%q stderr=%q", exit, stdout.String(), stderr.String())
			}
			var result protocol.Report
			if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
				t.Fatalf("decode result: %v: %s", err, stdout.String())
			}
			if err := result.Validate(); err != nil {
				t.Fatalf("validate result: %v", err)
			}
			if result.Status != protocol.StatusFailure || result.Failure == nil || result.Failure.Class != protocol.FailureConfig {
				t.Fatalf("result=%+v", result)
			}
			if stderr.Len() != 0 || strings.Contains(stdout.String(), "Usage of slopguard") {
				t.Fatalf("stdout/stderr contract failed: stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
		})
	}
}

func TestReviewCommandWithoutUnambiguousJSONStaysTerminal(t *testing.T) {
	t.Parallel()

	for _, arguments := range [][]string{
		{"review", "--output", "yaml"},
		{"review", "--prompt", "--output", "json"},
		{"review", "--context-file", "--output", "json"},
	} {
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		exit := run(t.Context(), arguments, &stdout, &stderr, dependencies{})
		if exit != 2 || stdout.Len() != 0 || stderr.Len() == 0 {
			t.Fatalf("arguments=%v exit=%d stdout=%q stderr=%q", arguments, exit, stdout.String(), stderr.String())
		}
	}
}

func TestReviewCommandLoadsEquivalentPromptSources(t *testing.T) {
	t.Parallel()

	repository := reviewRepository(t)
	prompt := "Review the first line.\nThen review the second line.\n"
	promptPath := filepath.Join(t.TempDir(), "task contract.md")
	if err := os.WriteFile(promptPath, []byte(prompt), 0o600); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name      string
		arguments []string
		stdin     io.Reader
	}{
		{name: "flag", arguments: []string{"--prompt", prompt}},
		{name: "file", arguments: []string{"--prompt-file", promptPath}},
		{name: "stdin", arguments: []string{"--prompt-file", "-"}, stdin: strings.NewReader(prompt)},
	}
	var providerPrompts []string
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			reviewer := &scriptedReviewer{results: []reviewStep{{result: cleanResult()}}}
			dependencies := reviewDependencies(t, cleanScanner{}, reviewer)
			dependencies.stdin = test.stdin
			arguments := []string{"review", "--repository", repository, "--mode", "local", "--engine", "codex", "--output", "json"}
			arguments = append(arguments, test.arguments...)
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			if exit := run(t.Context(), arguments, &stdout, &stderr, dependencies); exit != 0 {
				t.Fatalf("exit=%d stdout=%q stderr=%q", exit, stdout.String(), stderr.String())
			}
			if len(reviewer.prompts) != 1 {
				t.Fatalf("provider calls=%d", len(reviewer.prompts))
			}
			providerPrompts = append(providerPrompts, reviewer.prompts[0])
		})
	}
	for index := 1; index < len(providerPrompts); index++ {
		if providerPrompts[index] != providerPrompts[0] {
			t.Fatalf("prompt source changed provider payload")
		}
	}
}

func TestReviewCommandKeepsEmptyPromptBehaviorAcrossSources(t *testing.T) {
	t.Parallel()

	repository := reviewRepository(t)
	promptPath := filepath.Join(t.TempDir(), "empty task contract.md")
	if err := os.WriteFile(promptPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	sources := [][]string{
		nil,
		{"--prompt", ""},
		{"--prompt-file", promptPath},
	}
	var providerPrompts []string
	for _, source := range sources {
		reviewer := &scriptedReviewer{results: []reviewStep{{result: cleanResult()}}}
		arguments := []string{"review", "--repository", repository, "--mode", "local", "--engine", "codex", "--output", "json"}
		arguments = append(arguments, source...)
		var stdout bytes.Buffer
		if exit := run(t.Context(), arguments, &stdout, io.Discard, reviewDependencies(t, cleanScanner{}, reviewer)); exit != 0 {
			t.Fatalf("source=%v exit=%d stdout=%q", source, exit, stdout.String())
		}
		providerPrompts = append(providerPrompts, reviewer.prompts[0])
	}
	for index := 1; index < len(providerPrompts); index++ {
		if providerPrompts[index] != providerPrompts[0] {
			t.Fatalf("empty prompt source changed provider payload")
		}
	}
}

func TestReviewCommandRejectsInvalidPromptInputBeforeProvider(t *testing.T) {
	t.Parallel()

	repository := reviewRepository(t)
	temporary := t.TempDir()
	missing := filepath.Join(temporary, "missing prompt.md")
	oversized := filepath.Join(temporary, "oversized prompt.md")
	invalidUTF8 := filepath.Join(temporary, "invalid utf8.md")
	nulPrompt := filepath.Join(temporary, "nul prompt.md")
	if err := os.WriteFile(oversized, bytes.Repeat([]byte("x"), 33), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(invalidUTF8, []byte{0xff}, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(nulPrompt, []byte("before\x00after"), 0o600); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name        string
		arguments   []string
		wantClass   protocol.FailureClass
		privatePath string
	}{
		{name: "mutually exclusive", arguments: []string{"--prompt", "trusted", "--prompt-file", "-"}, wantClass: protocol.FailureConfig},
		{name: "missing file", arguments: []string{"--prompt-file", missing}, wantClass: protocol.FailureTarget, privatePath: missing},
		{name: "oversized file", arguments: []string{"--prompt-file", oversized, "--max-bytes", "32"}, wantClass: protocol.FailureTarget, privatePath: oversized},
		{name: "invalid UTF-8", arguments: []string{"--prompt-file", invalidUTF8}, wantClass: protocol.FailureTarget, privatePath: invalidUTF8},
		{name: "NUL", arguments: []string{"--prompt-file", nulPrompt}, wantClass: protocol.FailureTarget, privatePath: nulPrompt},
		{name: "stdin unavailable", arguments: []string{"--prompt-file", "-"}, wantClass: protocol.FailureTarget},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			reviewer := &scriptedReviewer{}
			dependencies := reviewDependencies(t, cleanScanner{}, reviewer)
			arguments := []string{"review", "--repository", repository, "--mode", "local", "--engine", "codex", "--output", "json"}
			arguments = append(arguments, test.arguments...)
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			if exit := run(t.Context(), arguments, &stdout, &stderr, dependencies); exit != 2 {
				t.Fatalf("exit=%d stdout=%q stderr=%q", exit, stdout.String(), stderr.String())
			}
			var result protocol.Report
			if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
				t.Fatalf("decode result: %v: %s", err, stdout.String())
			}
			if result.Failure == nil || result.Failure.Class != test.wantClass {
				t.Fatalf("failure=%+v", result.Failure)
			}
			if len(reviewer.prompts) != 0 {
				t.Fatalf("provider was invoked %d times", len(reviewer.prompts))
			}
			if test.privatePath != "" && (strings.Contains(stdout.String(), test.privatePath) || strings.Contains(stderr.String(), test.privatePath)) {
				t.Fatalf("diagnostic leaked prompt path: stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
		})
	}
}

func TestConfigCommandPrintsSourceAwareJSON(t *testing.T) {
	t.Parallel()

	repository := cliRepository(t)
	xdg := t.TempDir()
	lookup := func(name string) (string, bool) {
		if name == "XDG_CONFIG_HOME" {
			return xdg, true
		}
		return "", false
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exit := run(context.Background(), []string{"config", "--repository", repository, "--engine", "codex", "--web-access", "--json"}, &stdout, &stderr, dependencies{
		lookupEnv: lookup,
		homeDir:   func() (string, error) { return t.TempDir(), nil },
	})
	if exit != 0 {
		t.Fatalf("run() exit = %d, stderr = %s", exit, stderr.String())
	}
	var effective config.Effective
	if err := json.Unmarshal(stdout.Bytes(), &effective); err != nil {
		t.Fatalf("decode JSON: %v: %s", err, stdout.String())
	}
	if effective.Engine.Source != config.SourceFlag || effective.WebAccess.Source != config.SourceFlag || !effective.WebAccess.Value {
		t.Fatalf("effective config = %+v", effective)
	}
	if strings.Contains(stdout.String(), "XDG_CONFIG_HOME") {
		t.Fatalf("diagnostic leaked environment details: %s", stdout.String())
	}
}

func TestConfigCommandReportsNativeAndCursorWebDefaults(t *testing.T) {
	t.Parallel()

	repository := cliRepository(t)
	lookup := func(name string) (string, bool) {
		if name == "XDG_CONFIG_HOME" {
			return t.TempDir(), true
		}
		return "", false
	}
	for _, test := range []struct {
		name            string
		arguments       []string
		isolation       protocol.Isolation
		isolationSource config.Source
		web             bool
		webSource       config.Source
	}{
		{name: "implicit", arguments: []string{"config", "--repository", repository, "--engine", "cursor", "--json"}, isolation: protocol.IsolationNative, isolationSource: config.SourceDefault, web: true, webSource: config.SourceFlag},
		{name: "explicit false", arguments: []string{"config", "--repository", repository, "--engine", "cursor", "--web-access=false", "--json"}, isolation: protocol.IsolationNative, isolationSource: config.SourceDefault, webSource: config.SourceFlag},
		{name: "strict still implicit", arguments: []string{"config", "--repository", repository, "--engine", "cursor", "--isolation", "strict", "--json"}, isolation: protocol.IsolationStrict, isolationSource: config.SourceFlag, web: true, webSource: config.SourceFlag},
		{name: "Grok native web off", arguments: []string{"config", "--repository", repository, "--engine", "grok", "--json"}, isolation: protocol.IsolationNative, isolationSource: config.SourceDefault, webSource: config.SourceDefault},
	} {
		t.Run(test.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			exit := run(context.Background(), test.arguments, &stdout, &stderr, dependencies{
				lookupEnv: lookup,
				homeDir:   func() (string, error) { return t.TempDir(), nil },
			})
			if exit != 0 {
				t.Fatalf("run() exit = %d, stderr = %s", exit, stderr.String())
			}
			var effective config.Effective
			if err := json.Unmarshal(stdout.Bytes(), &effective); err != nil {
				t.Fatal(err)
			}
			if effective.Isolation.Value != test.isolation || effective.Isolation.Source != test.isolationSource {
				t.Fatalf("isolation = %+v", effective.Isolation)
			}
			if effective.WebAccess.Value != test.web || effective.WebAccess.Source != test.webSource {
				t.Fatalf("web_access = %+v", effective.WebAccess)
			}
		})
	}

	t.Run("repository engine does not grant web", func(t *testing.T) {
		repository := cliRepository(t)
		if err := os.WriteFile(filepath.Join(repository, ".slopguard.yaml"), []byte("engine: cursor\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		exit := run(context.Background(), []string{"config", "--repository", repository, "--json"}, &stdout, &stderr, dependencies{
			lookupEnv: lookup,
			homeDir:   func() (string, error) { return t.TempDir(), nil },
		})
		if exit != 0 {
			t.Fatalf("run() exit = %d, stderr = %s", exit, stderr.String())
		}
		var effective config.Effective
		if err := json.Unmarshal(stdout.Bytes(), &effective); err != nil {
			t.Fatal(err)
		}
		if effective.Engine.Value != protocol.ProviderCursor || effective.Engine.Source != config.SourceRepository {
			t.Fatalf("engine = %+v", effective.Engine)
		}
		if effective.WebAccess.Value || effective.WebAccess.Source != config.SourceDefault {
			t.Fatalf("web_access = %+v", effective.WebAccess)
		}
	})
}

func TestConfigCommandRejectsMissingEngine(t *testing.T) {
	t.Parallel()

	repository := cliRepository(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exit := run(context.Background(), []string{"config", "--repository", repository}, &stdout, &stderr, dependencies{
		lookupEnv: func(name string) (string, bool) {
			if name == "XDG_CONFIG_HOME" {
				return t.TempDir(), true
			}
			return "", false
		},
		homeDir: func() (string, error) { return t.TempDir(), nil },
	})
	if exit != 2 || !strings.Contains(stderr.String(), "engine is required") {
		t.Fatalf("run() exit = %d, stderr = %s", exit, stderr.String())
	}
}

func TestConfigCommandReportsPlainOutputFailure(t *testing.T) {
	t.Parallel()

	repository := cliRepository(t)
	var stderr bytes.Buffer
	exit := run(context.Background(), []string{"config", "--repository", repository, "--engine", "codex"}, failingWriter{}, &stderr, dependencies{
		lookupEnv: func(name string) (string, bool) {
			if name == "XDG_CONFIG_HOME" {
				return t.TempDir(), true
			}
			return "", false
		},
		homeDir: func() (string, error) { return t.TempDir(), nil },
	})
	if exit != 2 || !strings.Contains(stderr.String(), "write effective config") {
		t.Fatalf("run() exit = %d, stderr = %s", exit, stderr.String())
	}
}

func TestVersionCommandReportsOutputFailure(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer
	exit := run(context.Background(), []string{"--version"}, failingWriter{}, &stderr, dependencies{})
	if exit != 2 || !strings.Contains(stderr.String(), "write version") {
		t.Fatalf("run() exit = %d, stderr = %s", exit, stderr.String())
	}
}

func TestConfigHelpIsSuccessful(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	exit := run(context.Background(), []string{"config", "--help"}, &stdout, io.Discard, dependencies{})
	if exit != 0 || !strings.Contains(stdout.String(), "Usage of slopguard config") {
		t.Fatalf("run() exit = %d, stdout = %s", exit, stdout.String())
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errors.New("injected write failure")
}

func cliRepository(t *testing.T) string {
	t.Helper()
	repository := t.TempDir()
	command := exec.CommandContext(t.Context(), "git", "init", "-q", "-b", "main")
	command.Dir = repository
	command.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, output)
	}
	if err := os.MkdirAll(filepath.Join(repository, "nested"), 0o700); err != nil {
		t.Fatal(err)
	}
	return repository
}

type reviewStep struct {
	before func()
	result provider.Result
	err    error
}

type scriptedReviewer struct {
	results []reviewStep
	prompts []string
}

func (reviewer *scriptedReviewer) Review(_ context.Context, request provider.Request) (provider.Result, error) {
	reviewer.prompts = append(reviewer.prompts, request.Prompt+request.TrustedSuffix)
	index := len(reviewer.prompts) - 1
	if index >= len(reviewer.results) {
		return provider.Result{}, errors.New("unexpected review call")
	}
	step := reviewer.results[index]
	if step.before != nil {
		step.before()
	}
	return step.result, step.err
}

type cleanScanner struct{}

func (cleanScanner) Scan(context.Context, string) error { return nil }

type scannerError struct{ err error }

func (scanner scannerError) Scan(context.Context, string) error { return scanner.err }

func cleanResult() provider.Result {
	return provider.Result{
		Review: protocol.Review{
			Findings:           []protocol.Finding{},
			OverallExplanation: "No actionable defects found.",
			OverallConfidence:  0.9,
		},
		Provider: protocol.Provider{Name: protocol.ProviderCodex, Model: "fake-model", Version: "1.2.3"},
		Attempt: protocol.Attempt{
			Number:     1,
			Outcome:    protocol.AttemptValid,
			DurationMS: 1,
		},
		Duration:  time.Millisecond,
		Isolation: protocol.IsolationNative,
	}
}

func finding(title string, confidence float64) protocol.Finding {
	return protocol.Finding{
		Title:      title,
		Body:       "This behavior is incorrect.",
		Priority:   protocol.PriorityP1,
		Confidence: confidence,
		Category:   protocol.CategoryBug,
		Location:   protocol.Location{FilePath: "app.go", StartLine: 1, EndLine: 1},
	}
}

func providerError(class protocol.FailureClass, outcome protocol.AttemptOutcome) error {
	return &provider.Error{
		Class:   class,
		Message: "injected provider failure",
		Attempt: &protocol.Attempt{Number: 1, Outcome: outcome, DurationMS: 1, ErrorClass: &class},
	}
}

func cursorProtocolError(execution *provider.Execution, reason protocol.ProtocolReason) error {
	class := protocol.FailureProtocol
	return &provider.Error{
		Class:     class,
		Message:   "Cursor returned an invalid review document (" + string(reason) + ")",
		Attempt:   &protocol.Attempt{Number: 1, Outcome: protocol.AttemptMalformed, DurationMS: 1, ErrorClass: &class},
		Reason:    reason,
		Execution: execution,
	}
}

func reviewDependencies(t *testing.T, scanner target.Scanner, reviewer provider.Reviewer) dependencies {
	t.Helper()
	xdg := t.TempDir()
	return dependencies{
		lookupEnv: func(name string) (string, bool) {
			if name == "XDG_CONFIG_HOME" {
				return xdg, true
			}
			return "", false
		},
		homeDir: func() (string, error) { return t.TempDir(), nil },
		newCollector: func() (*target.Collector, error) {
			return target.New(target.Options{Scanner: scanner})
		},
		newReviewer: func(protocol.ProviderName, string) provider.Reviewer { return reviewer },
	}
}

func reviewRepository(t *testing.T) string {
	t.Helper()
	repository := cliRepository(t)
	if err := os.WriteFile(filepath.Join(repository, "app.go"), []byte("old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repository, "CONTEXT.md"), []byte("context evidence\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitCommand(t, repository, "add", "app.go", "CONTEXT.md")
	gitCommand(t, repository, "-c", "user.name=Slopguard Test", "-c", "user.email=test@example.invalid", "commit", "-q", "-m", "initial")
	if err := os.WriteFile(filepath.Join(repository, "app.go"), []byte("new application content\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return repository
}

func reviewRepositoryWithDiscontiguousRanges(t *testing.T) string {
	t.Helper()
	repository := cliRepository(t)
	lines := make([]string, 60)
	for index := range lines {
		lines[index] = "unchanged line"
	}
	path := filepath.Join(repository, "app.go")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitCommand(t, repository, "add", "app.go")
	gitCommand(t, repository, "-c", "user.name=Slopguard Test", "-c", "user.email=test@example.invalid", "commit", "-q", "-m", "initial")
	for _, line := range []int{10, 30, 50} {
		lines[line-1] = "changed line"
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return repository
}

func gitCommand(t *testing.T, repository string, arguments ...string) {
	t.Helper()
	command := exec.CommandContext(t.Context(), "git", arguments...)
	command.Dir = repository
	command.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", arguments, err, output)
	}
}
