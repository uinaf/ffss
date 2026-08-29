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

	"golang.org/x/mod/semver"

	"github.com/uinaf/ffss/cli/slopguard/internal/config"
	"github.com/uinaf/ffss/cli/slopguard/internal/phase"
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
	preparation preparationCache
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
	preparationSpan := phase.Start(ctx, phase.ProviderPreparation)
	defer func() { preparationSpan.End() }()
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
	key := effectivePreparationKey(request.Config)
	prepared, cached := grok.preparation.get(key)
	runtime, err := config.PrepareRuntime(request.Config, grok.environment)
	if err != nil {
		return Result{}, newFailure(protocol.FailureInternal, fmt.Sprintf("prepare provider runtime: %v", err), grok.environment, nil)
	}
	defer func() {
		preparationSpan.End()
		cleanupSpan := phase.Start(ctx, phase.ProviderPreparation)
		defer cleanupSpan.End()
		if err := runtime.Close(); err != nil && returnError == nil {
			result = Result{}
			returnError = newFailure(protocol.FailureInternal, err.Error(), runtime.Environment(), nil)
		}
	}()
	environment := runtime.Environment()
	environment = setEnvironmentValue(environment, "GROK_MEMORY", "0")
	environment = setEnvironmentValue(environment, "GROK_SUBAGENTS", "0")
	if !cached {
		preparationSpan.End()
		probeSpan := phase.Start(reviewContext, phase.DependencyProbes)
		prepared, err = grok.preparation.resolve(reviewContext, key, func() (preparedExecutable, error) {
			candidates, discoverErr := discoverExecutableCandidates(grok.executable, repository, grok.environment)
			if discoverErr != nil {
				return preparedExecutable{}, newFailure(protocol.FailureCapability, discoverErr.Error(), grok.environment, nil)
			}
			return selectCompatibleExecutable(candidates, func(candidate string) (string, error) {
				return grok.preflight(reviewContext, candidate, runtime.Workspace, environment, request.Config)
			})
		})
		probeSpan.End()
		if err != nil {
			return Result{}, err
		}
		preparationSpan = phase.Start(reviewContext, phase.ProviderPreparation)
	}
	executable, version := prepared.Path, prepared.Version
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
	providerSchema, err := contractschema.GrokReviewV1(len(request.Target.Files))
	if err != nil {
		return Result{}, newFailure(protocol.FailureInternal, err.Error(), runtime.Environment(), nil)
	}
	model := request.Config.Model.Value
	if model == "" {
		model = DefaultGrokModel
	}
	resolvedExecution := Execution{
		Provider:  protocol.Provider{Name: protocol.ProviderGrok, Model: model, Version: version},
		WebAccess: request.Config.WebAccess.Value,
	}
	preparationSpan.End()
	processSpan := phase.Start(reviewContext, phase.ProviderProcess)
	process, processErr := runProcess(reviewContext, processSpec{
		Path:        executable,
		Arguments:   grokArguments(request.Config, runtime.Workspace, promptPath, string(providerSchema), model, version),
		Directory:   runtime.Workspace,
		Environment: environment,
		Timeout:     time.Duration(request.Config.Timeout.Value),
		StdoutLimit: providerStdoutLimit,
		StderrLimit: providerStderrLimit,
	})
	processSpan.End()
	decodeSpan := phase.Start(reviewContext, phase.ProtocolDecode)
	defer decodeSpan.End()
	attempt := protocol.Attempt{Number: 1, DurationMS: process.Duration.Milliseconds()}
	if processErr != nil {
		class := classifyProcessFailure(processErr, process)
		attempt.Outcome = protocol.AttemptFailed
		attempt.ErrorClass = &class
		return Result{}, processFailure("Grok review", class, processErr, process, environment, &attempt).withExecution(resolvedExecution)
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
			return Result{}, newFailure(class, err.Error(), environment, &attempt).withExecution(resolvedExecution)
		}
		reason := protocol.ProtocolReasonInvalidEnvelope
		if errors.Is(err, errGrokIncompleteReview) {
			reason = protocol.ProtocolReasonReviewValidation
		}
		return Result{}, invalidProviderOutput("Grok", "result envelope", reason, environment, &attempt).withExecution(resolvedExecution)
	}
	attempt.Outcome = protocol.AttemptValid
	return Result{
		Review:    review,
		Provider:  resolvedExecution.Provider,
		Attempt:   attempt,
		Duration:  time.Since(started),
		WebAccess: resolvedExecution.WebAccess,
		ProtocolRecovery: protocol.ProtocolRecovery{
			Applied: false,
		},
	}, nil
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
		"--permission-mode", "--tools", "--disallowed-tools", "--allow", "--deny", "--no-plan", "--no-subagents", "--disable-web-search",
		"--verbatim", "--cwd",
	}
	if grokNeedsNoMemoryFlag(string(match[1])) {
		required = append(required, "--no-memory")
	}
	help := string(helpResult.Stdout) + string(helpResult.Stderr)
	if missing := missingCapabilities(help, required); len(missing) != 0 {
		return "", newFailure(protocol.FailureCapability, "Grok is missing required flags: "+strings.Join(missing, ", "), environment, nil)
	}
	if !optionSupports(help, "--output-format", "json") || !optionSupports(help, "--permission-mode", "dontAsk") {
		return "", newFailure(protocol.FailureCapability, "Grok is missing required option values: --output-format=json or --permission-mode=dontAsk", environment, nil)
	}
	return string(match[1]), nil
}

func grokArguments(effective config.Effective, workspace, promptPath, schema, model, version string) []string {
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
	}
	if grokNeedsNoMemoryFlag(version) {
		arguments = append(arguments, "--no-memory")
	}
	arguments = append(arguments,
		"--verbatim",
		"--cwd", workspace,
		"--deny", "Bash",
		"--deny", "Edit",
		"--deny", "Write",
		"--deny", "Read",
		"--deny", "Grep",
		"--deny", "MCPTool",
	)
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
	return arguments
}

func grokNeedsNoMemoryFlag(version string) bool {
	value := "v" + version
	return semver.IsValid(value) && semver.Compare(value, "v1.0.5") < 0
}

type grokCompletion struct {
	Status string                   `json:"status"`
	Files  []grokCompletedFileRange `json:"files"`
}

type grokCompletedFileRange struct {
	StartIndex int `json:"start_index"`
	EndIndex   int `json:"end_index"`
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
	completion := grokCompletion{Status: *raw.Status, Files: make([]grokCompletedFileRange, 0, len(*raw.Files))}
	for index, data := range *raw.Files {
		var file struct {
			StartIndex *int `json:"start_index"`
			EndIndex   *int `json:"end_index"`
		}
		if err := decodeGrokJSONDocument(data, &file); err != nil {
			return grokCompletion{}, fmt.Errorf("decode completion file %d: %w", index, err)
		}
		if file.StartIndex == nil || file.EndIndex == nil {
			return grokCompletion{}, fmt.Errorf("completion file %d is missing required fields", index)
		}
		completion.Files = append(completion.Files, grokCompletedFileRange{
			StartIndex: *file.StartIndex,
			EndIndex:   *file.EndIndex,
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
	expectedRanges := 0
	if len(target.Files) > 0 {
		expectedRanges = 1
	}
	if len(completion.Files) != expectedRanges {
		return fmt.Errorf("completion has %d file ranges, want %d", len(completion.Files), expectedRanges)
	}
	expectedFiles := make(map[string]struct{}, len(target.Files))
	for _, file := range target.Files {
		expectedFiles[file.FilePath] = struct{}{}
	}
	if expectedRanges == 1 {
		fileRange := completion.Files[0]
		if fileRange.StartIndex != 0 || fileRange.EndIndex != len(target.Files)-1 {
			return fmt.Errorf("completion file range is %d-%d, want 0-%d", fileRange.StartIndex, fileRange.EndIndex, len(target.Files)-1)
		}
	}
	for index, finding := range review.Findings {
		if _, expected := expectedFiles[finding.Location.FilePath]; !expected {
			return fmt.Errorf("finding index %d references unexpected file %q", index, finding.Location.FilePath)
		}
	}
	return nil
}
