package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/uinaf/ffss/cli/slopguard/internal/config"
	"github.com/uinaf/ffss/cli/slopguard/internal/protocol"
	"github.com/uinaf/ffss/cli/slopguard/internal/reviewpolicy"
	contractschema "github.com/uinaf/ffss/cli/slopguard/schema"
)

const (
	defaultGrokExecutable = "grok"
	grokMaxTurns          = "2"
)

var grokVersionPattern = regexp.MustCompile(`\b(\d+\.\d+\.\d+)\b`)

var (
	errGrokIncompleteTurn   = errors.New("Grok did not complete the bounded review turn")
	errGrokIncompleteReview = errors.New("Grok did not complete the review")
	errGrokUnauthenticated  = errors.New("Grok is not authenticated")
)

type GrokOptions struct {
	Repository  string
	Executable  string
	Environment []string
}

type Grok struct {
	repository  string
	executable  string
	environment []string
}

func NewGrok(options GrokOptions) *Grok {
	executable := options.Executable
	if executable == "" {
		executable = defaultGrokExecutable
	}
	environment := options.Environment
	if environment == nil {
		environment = os.Environ()
	}
	return &Grok{
		repository:  options.Repository,
		executable:  executable,
		environment: append([]string(nil), environment...),
	}
}

func (grok *Grok) Review(ctx context.Context, request Request) (result Result, returnError error) {
	started := time.Now()
	if err := request.Config.Validate(); err != nil {
		return Result{}, newFailure(protocol.FailureConfig, fmt.Sprintf("invalid provider config: %v", err), nil, nil)
	}
	if request.Config.Engine.Value != protocol.ProviderGrok {
		return Result{}, newFailure(protocol.FailureConfig, fmt.Sprintf("Grok adapter cannot run engine %q", request.Config.Engine.Value), nil, nil)
	}
	if !request.validPrompt() {
		return Result{}, newFailure(protocol.FailureConfig, "provider prompt must be non-empty valid UTF-8", nil, nil)
	}
	maximumPrompt := request.Config.MaxBytes.Value + providerPromptAllowance
	protocolBytes := int64(len(reviewpolicy.GrokReviewProtocol()))
	promptBytes, validLength := request.promptBytes()
	if !validLength || maximumPrompt < request.Config.MaxBytes.Value || protocolBytes > maximumPrompt || promptBytes > maximumPrompt-protocolBytes {
		return Result{}, newFailure(protocol.FailureConfig, fmt.Sprintf("Grok combined review input exceeds %d bytes (bundle plus trusted policy)", maximumPrompt), nil, nil)
	}
	reviewContext, cancelReview := context.WithTimeout(ctx, time.Duration(request.Config.Timeout.Value))
	defer cancelReview()
	repository, err := filepath.Abs(grok.repository)
	if err != nil {
		return Result{}, newFailure(protocol.FailureConfig, fmt.Sprintf("resolve reviewed repository: %v", err), nil, nil)
	}
	executables, err := discoverExecutableCandidates(grok.executable, repository, grok.environment)
	if err != nil {
		return Result{}, newFailure(protocol.FailureCapability, err.Error(), grok.environment, nil)
	}
	if failure := strictCredentialFailure(request.Config, protocol.ProviderGrok, grok.environment); failure != nil {
		return Result{}, failure
	}
	runtime, err := config.PrepareRuntime(request.Config, grok.environment)
	if err != nil {
		return Result{}, newFailure(protocol.FailureInternal, fmt.Sprintf("prepare provider runtime: %v", err), grok.environment, nil)
	}
	defer func() {
		if err := runtime.Close(); err != nil && returnError == nil {
			result = Result{}
			returnError = newFailure(protocol.FailureInternal, err.Error(), runtime.Environment(), nil)
		}
	}()
	executable, version, err := grok.selectExecutable(reviewContext, executables, runtime.Workspace, runtime.Environment(), request.Config)
	if err != nil {
		return Result{}, err
	}
	promptPath := filepath.Join(runtime.Workspace, "review.prompt")
	prompt, err := os.OpenFile(promptPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return Result{}, newFailure(protocol.FailureInternal, fmt.Sprintf("create Grok prompt: %v", err), runtime.Environment(), nil)
	}
	if _, err := io.Copy(prompt, request.promptReader(reviewpolicy.GrokReviewProtocol())); err != nil {
		_ = prompt.Close()
		return Result{}, newFailure(protocol.FailureInternal, fmt.Sprintf("write Grok prompt: %v", err), runtime.Environment(), nil)
	}
	if err := prompt.Close(); err != nil {
		return Result{}, newFailure(protocol.FailureInternal, fmt.Sprintf("close Grok prompt: %v", err), runtime.Environment(), nil)
	}
	filePaths := make([]string, 0, len(request.Target.Files))
	for _, file := range request.Target.Files {
		filePaths = append(filePaths, file.FilePath)
	}
	providerSchema, err := contractschema.GrokReviewV1(filePaths)
	if err != nil {
		return Result{}, newFailure(protocol.FailureInternal, err.Error(), runtime.Environment(), nil)
	}
	model := request.Config.Model.Value
	if model == "" {
		model = DefaultGrokModel
	}
	resolvedExecution := Execution{
		Provider:  protocol.Provider{Name: protocol.ProviderGrok, Model: model, Version: version},
		Isolation: request.Config.Isolation.Value,
		WebAccess: request.Config.WebAccess.Value,
	}
	process, processErr := runProcess(reviewContext, processSpec{
		Path:        executable,
		Arguments:   grokArguments(request.Config, runtime.Workspace, promptPath, string(providerSchema), model),
		Directory:   runtime.Workspace,
		Environment: runtime.Environment(),
		Timeout:     time.Duration(request.Config.Timeout.Value),
		StdoutLimit: providerStdoutLimit,
		StderrLimit: providerStderrLimit,
	})
	attempt := protocol.Attempt{Number: 1, DurationMS: process.Duration.Milliseconds()}
	if processErr != nil {
		class := classifyProcessFailure(processErr, process)
		attempt.Outcome = protocol.AttemptFailed
		attempt.ErrorClass = &class
		return Result{}, processFailure("Grok review", class, processErr, process, runtime.Environment(), &attempt, strictCredentialRecovery(request.Config, protocol.ProviderGrok)).withExecution(resolvedExecution)
	}
	review, err := decodeGrokEnvelope(process.Stdout, request.Target)
	if err != nil {
		class := protocol.FailureProtocol
		attempt.Outcome = protocol.AttemptMalformed
		if errors.Is(err, errGrokIncompleteTurn) {
			class = protocol.FailureProvider
			attempt.Outcome = protocol.AttemptFailed
		}
		attempt.ErrorClass = &class
		if class == protocol.FailureProvider {
			return Result{}, newFailure(class, err.Error(), runtime.Environment(), &attempt).withExecution(resolvedExecution)
		}
		reason := protocol.ProtocolReasonInvalidEnvelope
		if errors.Is(err, errGrokIncompleteReview) {
			reason = protocol.ProtocolReasonReviewValidation
		}
		return Result{}, invalidProviderOutput("Grok", "result envelope", reason, runtime.Environment(), &attempt).withExecution(resolvedExecution)
	}
	attempt.Outcome = protocol.AttemptValid
	return Result{
		Review:    review,
		Provider:  resolvedExecution.Provider,
		Attempt:   attempt,
		Duration:  time.Since(started),
		Isolation: resolvedExecution.Isolation,
		WebAccess: resolvedExecution.WebAccess,
		ProtocolRecovery: protocol.ProtocolRecovery{
			Applied: false,
		},
	}, nil
}

func (grok *Grok) selectExecutable(ctx context.Context, executables []string, workspace string, environment []string, effective config.Effective) (string, string, error) {
	var lastCapabilityError error
	for _, executable := range executables {
		version, err := grok.preflight(ctx, executable, workspace, environment, effective)
		if err == nil {
			return executable, version, nil
		}
		var failure *Error
		if !errors.As(err, &failure) || failure.Class != protocol.FailureCapability {
			return "", "", err
		}
		lastCapabilityError = err
	}
	if lastCapabilityError != nil {
		return "", "", lastCapabilityError
	}
	return "", "", newFailure(protocol.FailureCapability, "Grok has no usable executable candidate", environment, nil)
}

func (grok *Grok) preflight(ctx context.Context, executable, workspace string, environment []string, effective config.Effective) (string, error) {
	timeout := 10 * time.Second
	if time.Duration(effective.Timeout.Value) < timeout {
		timeout = time.Duration(effective.Timeout.Value)
	}
	run := func(arguments ...string) (processResult, error) {
		return runProcess(ctx, processSpec{
			Path:        executable,
			Arguments:   arguments,
			Directory:   workspace,
			Environment: environment,
			Timeout:     timeout,
			StdoutLimit: probeOutputLimit,
			StderrLimit: probeOutputLimit,
		})
	}
	versionResult, err := run("--version")
	if err != nil {
		return "", probeFailure("Grok --version", err, versionResult, environment, protocol.FailureCapability)
	}
	match := grokVersionPattern.FindSubmatch(append(versionResult.Stdout, versionResult.Stderr...))
	if len(match) != 2 {
		return "", newFailure(protocol.FailureCapability, "Grok --version did not report a semantic version", environment, nil)
	}
	helpResult, err := run("--help")
	if err != nil {
		return "", probeFailure("Grok --help", err, helpResult, environment, protocol.FailureCapability)
	}
	required := []string{
		"--prompt-file", "--output-format", "--json-schema", "--model", "--reasoning-effort", "--max-turns",
		"--permission-mode", "--tools", "--disallowed-tools", "--allow", "--deny", "--no-plan", "--no-subagents", "--no-memory", "--disable-web-search",
		"--verbatim", "--cwd", "models",
	}
	if effective.Isolation.Value == protocol.IsolationStrict {
		required = append(required, "--sandbox")
	}
	help := string(helpResult.Stdout) + string(helpResult.Stderr)
	if missing := missingCapabilities(help, required); len(missing) != 0 {
		return "", newFailure(protocol.FailureCapability, "Grok is missing required flags: "+strings.Join(missing, ", "), environment, nil)
	}
	if !optionSupports(help, "--output-format", "json") || !optionSupports(help, "--permission-mode", "dontAsk") {
		return "", newFailure(protocol.FailureCapability, "Grok is missing required option values: --output-format=json or --permission-mode=dontAsk", environment, nil)
	}
	if environmentValue(environment, "XAI_API_KEY") == "" {
		authResult, err := run("models")
		if err != nil {
			return "", probeFailure("Grok authentication", err, authResult, environment, protocol.FailureAuth)
		}
		if err := decodeGrokAuth(append(authResult.Stdout, authResult.Stderr...)); err != nil {
			if errors.Is(err, errGrokUnauthenticated) {
				return "", newFailure(protocol.FailureAuth, "Grok is not authenticated; run grok login or set XAI_API_KEY, then retry", environment, nil)
			}
			return "", newFailure(protocol.FailureCapability, "Grok models returned an unsupported authentication status format", environment, nil)
		}
	}
	return string(match[1]), nil
}

func grokArguments(effective config.Effective, workspace, promptPath, schema, model string) []string {
	arguments := []string{
		"--prompt-file", promptPath,
		"--output-format", "json",
		"--json-schema", schema,
		"--model", model,
		"--reasoning-effort", string(effective.ReasoningEffort.Value),
		"--max-turns", grokMaxTurns,
		"--permission-mode", "dontAsk",
		"--no-plan",
		"--no-subagents",
		"--no-memory",
		"--verbatim",
		"--cwd", workspace,
		"--deny", "Bash",
		"--deny", "Edit",
		"--deny", "Write",
		"--deny", "Read",
		"--deny", "Grep",
		"--deny", "MCPTool",
	}
	if effective.WebAccess.Value {
		arguments = append(arguments,
			"--tools", "web_search,web_fetch",
			"--disallowed-tools", "search_tool,use_tool,Agent",
			"--allow", "WebFetch",
			"--allow", "WebSearch",
		)
	} else {
		arguments = append(arguments,
			"--tools", "web_search",
			"--disallowed-tools", "web_search,search_tool,use_tool,Agent",
			"--disable-web-search",
			"--deny", "WebFetch",
			"--deny", "WebSearch",
		)
	}
	if effective.Isolation.Value == protocol.IsolationStrict {
		arguments = append(arguments, "--sandbox", "workspace")
	}
	return arguments
}

func decodeGrokAuth(output []byte) error {
	text := string(bytes.TrimSpace(output))
	lower := strings.ToLower(text)
	if text == "" || containsAny(lower, "not authenticated", "not logged in", "logged out", "no active session") {
		return errGrokUnauthenticated
	}
	if !strings.Contains(lower, "you are logged in") || !strings.Contains(lower, "available models:") {
		return fmt.Errorf("unrecognized authentication status")
	}
	return nil
}

type grokCompletion struct {
	Status string                    `json:"status"`
	Files  []grokCompletedFileReview `json:"files"`
}

type grokCompletedFileReview struct {
	FilePath       string `json:"file_path"`
	Assessment     string `json:"assessment"`
	FindingIndexes []int  `json:"finding_indexes"`
}

type grokCompletedReview struct {
	Review     protocol.Review
	Completion grokCompletion
}

func decodeGrokEnvelope(output []byte, target protocol.Target) (protocol.Review, error) {
	output = bytes.TrimSpace(output)
	if len(output) == 0 {
		return protocol.Review{}, fmt.Errorf("Grok output is empty")
	}
	if err := protocol.RejectDuplicateKeys(output); err != nil {
		return protocol.Review{}, err
	}
	var envelope struct {
		Text                  string          `json:"text"`
		StopReason            string          `json:"stopReason"`
		SessionID             string          `json:"sessionId"`
		RequestID             string          `json:"requestId"`
		StructuredOutput      json.RawMessage `json:"structuredOutput"`
		StructuredOutputError string          `json:"structuredOutputError"`
	}
	if err := json.Unmarshal(output, &envelope); err != nil {
		return protocol.Review{}, err
	}
	if envelope.StopReason != "end_turn" {
		detail := "stop reason was not end_turn"
		if envelope.StopReason == "cancelled" {
			detail = "stop reason cancelled"
		}
		return protocol.Review{}, fmt.Errorf("%w: %s", errGrokIncompleteTurn, detail)
	}
	if strings.TrimSpace(envelope.SessionID) == "" || strings.TrimSpace(envelope.RequestID) == "" {
		return protocol.Review{}, fmt.Errorf("%w: missing request identifiers", errGrokIncompleteTurn)
	}
	if strings.TrimSpace(envelope.StructuredOutputError) != "" {
		return protocol.Review{}, fmt.Errorf("Grok reported a structured output error")
	}
	structured := bytes.TrimSpace(envelope.StructuredOutput)
	if len(structured) == 0 || structured[0] != '{' {
		return protocol.Review{}, fmt.Errorf("Grok result is missing structuredOutput object")
	}
	structuredReview, err := decodeGrokCompletedReview(structured, target)
	if err != nil {
		return protocol.Review{}, err
	}
	textReview, err := decodeGrokCompletedReview([]byte(strings.TrimSpace(envelope.Text)), target)
	if err != nil {
		return protocol.Review{}, err
	}
	if !reflect.DeepEqual(structuredReview, textReview) {
		return protocol.Review{}, fmt.Errorf("Grok text and structuredOutput disagree")
	}
	return structuredReview.Review, nil
}

func decodeGrokCompletedReview(data []byte, target protocol.Target) (grokCompletedReview, error) {
	if err := protocol.RejectDuplicateKeys(data); err != nil {
		return grokCompletedReview{}, err
	}
	var document struct {
		Review     json.RawMessage `json:"review"`
		Completion json.RawMessage `json:"completion"`
	}
	if err := decodeGrokJSONDocument(data, &document); err != nil {
		return grokCompletedReview{}, err
	}
	review, err := protocol.DecodeReview(document.Review)
	if err != nil {
		return grokCompletedReview{}, err
	}
	completion, err := decodeGrokCompletion(document.Completion)
	if err != nil {
		return grokCompletedReview{}, fmt.Errorf("%w: %v", errGrokIncompleteReview, err)
	}
	if utf8.RuneCountInString(strings.TrimSpace(review.OverallExplanation)) < contractschema.GrokMinimumOverallExplanationCharacters {
		return grokCompletedReview{}, fmt.Errorf("%w: overall explanation is shorter than the required completion evidence", errGrokIncompleteReview)
	}
	if review.OverallConfidence < contractschema.GrokMinimumOverallConfidence {
		return grokCompletedReview{}, fmt.Errorf("%w: overall confidence is below the required completion threshold", errGrokIncompleteReview)
	}
	if reviewpolicy.GrokReviewIsIncomplete(review.OverallExplanation) {
		return grokCompletedReview{}, fmt.Errorf("%w: overall explanation explicitly describes unfinished review work", errGrokIncompleteReview)
	}
	if err := validateGrokCompletion(completion, review, target); err != nil {
		return grokCompletedReview{}, fmt.Errorf("%w: %v", errGrokIncompleteReview, err)
	}
	return grokCompletedReview{Review: review, Completion: completion}, nil
}

func decodeGrokCompletion(data []byte) (grokCompletion, error) {
	var raw struct {
		Status *string            `json:"status"`
		Files  *[]json.RawMessage `json:"files"`
	}
	if err := decodeGrokJSONDocument(data, &raw); err != nil {
		return grokCompletion{}, err
	}
	if raw.Status == nil || raw.Files == nil {
		return grokCompletion{}, fmt.Errorf("completion is missing required fields")
	}
	completion := grokCompletion{Status: *raw.Status, Files: make([]grokCompletedFileReview, 0, len(*raw.Files))}
	for index, data := range *raw.Files {
		var file struct {
			FilePath       *string `json:"file_path"`
			Assessment     *string `json:"assessment"`
			FindingIndexes *[]int  `json:"finding_indexes"`
		}
		if err := decodeGrokJSONDocument(data, &file); err != nil {
			return grokCompletion{}, fmt.Errorf("decode completion file %d: %w", index, err)
		}
		if file.FilePath == nil || file.Assessment == nil || file.FindingIndexes == nil {
			return grokCompletion{}, fmt.Errorf("completion file %d is missing required fields", index)
		}
		completion.Files = append(completion.Files, grokCompletedFileReview{
			FilePath:       *file.FilePath,
			Assessment:     *file.Assessment,
			FindingIndexes: *file.FindingIndexes,
		})
	}
	return completion, nil
}

func decodeGrokJSONDocument(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return fmt.Errorf("multiple JSON values")
		}
		return err
	}
	return nil
}

func validateGrokCompletion(completion grokCompletion, review protocol.Review, target protocol.Target) error {
	if completion.Status != "complete" {
		return fmt.Errorf("completion status is not complete")
	}
	if len(completion.Files) != len(target.Files) {
		return fmt.Errorf("completion covers %d files, want %d", len(completion.Files), len(target.Files))
	}
	expectedFiles := make(map[string]struct{}, len(target.Files))
	for _, file := range target.Files {
		expectedFiles[file.FilePath] = struct{}{}
	}
	seenFiles := make(map[string]struct{}, len(completion.Files))
	seenFindings := make(map[int]struct{}, len(review.Findings))
	for _, file := range completion.Files {
		if _, expected := expectedFiles[file.FilePath]; !expected {
			return fmt.Errorf("completion includes unexpected file %q", file.FilePath)
		}
		if _, duplicate := seenFiles[file.FilePath]; duplicate {
			return fmt.Errorf("completion repeats file %q", file.FilePath)
		}
		seenFiles[file.FilePath] = struct{}{}
		trimmedAssessmentCharacters := utf8.RuneCountInString(strings.TrimSpace(file.Assessment))
		if trimmedAssessmentCharacters < contractschema.GrokMinimumFileAssessmentCharacters {
			return fmt.Errorf("completion assessment for %q is too short", file.FilePath)
		}
		if utf8.RuneCountInString(file.Assessment) > contractschema.GrokMaximumFileAssessmentCharacters {
			return fmt.Errorf("completion assessment for %q is too long", file.FilePath)
		}
		for _, findingIndex := range file.FindingIndexes {
			if findingIndex < 0 || findingIndex >= len(review.Findings) {
				return fmt.Errorf("completion for %q references invalid finding index %d", file.FilePath, findingIndex)
			}
			if _, duplicate := seenFindings[findingIndex]; duplicate {
				return fmt.Errorf("completion references finding index %d more than once", findingIndex)
			}
			if review.Findings[findingIndex].Location.FilePath != file.FilePath {
				return fmt.Errorf("completion links finding index %d to the wrong file", findingIndex)
			}
			seenFindings[findingIndex] = struct{}{}
		}
	}
	if len(seenFindings) != len(review.Findings) {
		return fmt.Errorf("completion links %d findings, want %d", len(seenFindings), len(review.Findings))
	}
	return nil
}

func containsAny(text string, values ...string) bool {
	for _, value := range values {
		if strings.Contains(text, value) {
			return true
		}
	}
	return false
}
