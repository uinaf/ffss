package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/uinaf/ffss/cli/slopguard/internal/config"
	"github.com/uinaf/ffss/cli/slopguard/internal/protocol"
	"github.com/uinaf/ffss/cli/slopguard/internal/reviewpolicy"
	contractschema "github.com/uinaf/ffss/cli/slopguard/schema"
)

const defaultClaudeExecutable = "claude"

var claudeVersionPattern = regexp.MustCompile(`\b(\d+\.\d+\.\d+)\b`)

type ClaudeOptions struct {
	Repository  string
	Executable  string
	Environment []string
}

type Claude struct {
	repository  string
	executable  string
	environment []string
}

func NewClaude(options ClaudeOptions) *Claude {
	executable := options.Executable
	if executable == "" {
		executable = defaultClaudeExecutable
	}
	environment := options.Environment
	if environment == nil {
		environment = os.Environ()
	}
	return &Claude{
		repository:  options.Repository,
		executable:  executable,
		environment: append([]string(nil), environment...),
	}
}

func (claude *Claude) Review(ctx context.Context, request Request) (result Result, returnError error) {
	started := time.Now()
	if err := request.Config.Validate(); err != nil {
		return Result{}, newFailure(protocol.FailureConfig, fmt.Sprintf("invalid provider config: %v", err), nil, nil)
	}
	if request.Config.Engine.Value != protocol.ProviderClaude {
		return Result{}, newFailure(protocol.FailureConfig, fmt.Sprintf("Claude adapter cannot run engine %q", request.Config.Engine.Value), nil, nil)
	}
	switch request.Config.ReasoningEffort.Value {
	case config.ReasoningLow, config.ReasoningMedium, config.ReasoningHigh, config.ReasoningXHigh, config.ReasoningMax:
	default:
		return Result{}, newFailure(protocol.FailureConfig, fmt.Sprintf("Claude does not support reasoning effort %q", request.Config.ReasoningEffort.Value), nil, nil)
	}
	if !request.validPrompt() {
		return Result{}, newFailure(protocol.FailureConfig, "provider prompt must be non-empty valid UTF-8", nil, nil)
	}
	maximumPrompt := request.Config.MaxBytes.Value + providerPromptAllowance
	protocolBytes := int64(len(reviewpolicy.ClaudeReviewProtocol()))
	promptBytes, validLength := request.promptBytes()
	if !validLength || maximumPrompt < request.Config.MaxBytes.Value || protocolBytes > maximumPrompt || promptBytes > maximumPrompt-protocolBytes {
		return Result{}, newFailure(protocol.FailureConfig, fmt.Sprintf("Claude combined review input exceeds %d bytes (bundle plus trusted policy)", maximumPrompt), nil, nil)
	}
	reviewContext, cancelReview := context.WithTimeout(ctx, time.Duration(request.Config.Timeout.Value))
	defer cancelReview()
	repository, err := filepath.Abs(claude.repository)
	if err != nil {
		return Result{}, newFailure(protocol.FailureConfig, fmt.Sprintf("resolve reviewed repository: %v", err), nil, nil)
	}
	executable, err := discoverExecutable(claude.executable, repository, claude.environment)
	if err != nil {
		return Result{}, newFailure(protocol.FailureCapability, err.Error(), claude.environment, nil)
	}
	if failure := strictCredentialFailure(request.Config, protocol.ProviderClaude, claude.environment); failure != nil {
		return Result{}, failure
	}
	runtime, err := config.PrepareRuntime(request.Config, claude.environment)
	if err != nil {
		return Result{}, newFailure(protocol.FailureInternal, fmt.Sprintf("prepare provider runtime: %v", err), claude.environment, nil)
	}
	defer func() {
		if err := runtime.Close(); err != nil && returnError == nil {
			result = Result{}
			returnError = newFailure(protocol.FailureInternal, err.Error(), runtime.Environment(), nil)
		}
	}()
	environment := runtime.Environment()
	if request.Config.Isolation.Value == protocol.IsolationStrict {
		environment = setEnvironmentValue(environment, "CLAUDE_CODE_DISABLE_AUTO_MEMORY", "1")
	}
	version, err := claude.preflight(reviewContext, executable, runtime.Workspace, environment, request.Config)
	if err != nil {
		return Result{}, err
	}
	providerSchema, err := contractschema.ClaudeReviewV1()
	if err != nil {
		return Result{}, newFailure(protocol.FailureInternal, err.Error(), environment, nil)
	}
	model := request.Config.Model.Value
	if model == "" {
		model = DefaultClaudeModel
	}
	resolvedExecution := Execution{
		Provider:  protocol.Provider{Name: protocol.ProviderClaude, Model: model, Version: version},
		Isolation: request.Config.Isolation.Value,
		WebAccess: request.Config.WebAccess.Value,
	}
	process, processErr := runProcess(reviewContext, processSpec{
		Path:        executable,
		Arguments:   claudeArguments(request.Config, string(providerSchema), model),
		Directory:   runtime.Workspace,
		Environment: environment,
		Input:       request.promptReader(reviewpolicy.ClaudeReviewProtocol()),
		Timeout:     time.Duration(request.Config.Timeout.Value),
		StdoutLimit: providerStdoutLimit,
		StderrLimit: providerStderrLimit,
	})
	attempt := protocol.Attempt{Number: 1, DurationMS: process.Duration.Milliseconds()}
	if processErr != nil {
		if isOrdinaryProcessExit(processErr) {
			_, envelopeErr := decodeClaudeEnvelope(process.Stdout)
			var reported *reportedProviderError
			if errors.As(envelopeErr, &reported) {
				attempt.Outcome = protocol.AttemptFailed
				attempt.ErrorClass = &reported.Class
				return Result{}, newFailure(reported.Class, reported.Message, environment, &attempt).withExecution(resolvedExecution)
			}
		}
		class := classifyProcessFailure(processErr, process)
		attempt.Outcome = protocol.AttemptFailed
		attempt.ErrorClass = &class
		return Result{}, processFailure("Claude review", class, processErr, process, environment, &attempt, strictCredentialRecovery(request.Config, protocol.ProviderClaude)).withExecution(resolvedExecution)
	}
	reviewData, err := decodeClaudeEnvelope(process.Stdout)
	if err != nil {
		var reported *reportedProviderError
		if errors.As(err, &reported) {
			attempt.Outcome = protocol.AttemptFailed
			attempt.ErrorClass = &reported.Class
			return Result{}, newFailure(reported.Class, reported.Message, environment, &attempt).withExecution(resolvedExecution)
		}
		class := protocol.FailureProtocol
		attempt.Outcome = protocol.AttemptMalformed
		attempt.ErrorClass = &class
		return Result{}, invalidProviderOutput("Claude", "result envelope", protocol.ProtocolReasonInvalidEnvelope, environment, &attempt).withExecution(resolvedExecution)
	}
	review, err := protocol.DecodeReview(reviewData)
	if err != nil {
		class := protocol.FailureProtocol
		attempt.Outcome = protocol.AttemptMalformed
		attempt.ErrorClass = &class
		return Result{}, invalidProviderOutput("Claude", "review document", reviewDocumentReason(reviewData), environment, &attempt).withExecution(resolvedExecution)
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

func (claude *Claude) preflight(ctx context.Context, executable, workspace string, environment []string, effective config.Effective) (string, error) {
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
		return "", probeFailure("Claude --version", err, versionResult, environment, protocol.FailureCapability)
	}
	match := claudeVersionPattern.FindSubmatch(append(versionResult.Stdout, versionResult.Stderr...))
	if len(match) != 2 {
		return "", newFailure(protocol.FailureCapability, "Claude --version did not report a semantic version", environment, nil)
	}
	helpResult, err := run("--help")
	if err != nil {
		return "", probeFailure("Claude --help", err, helpResult, environment, protocol.FailureCapability)
	}
	required := []string{"--print", "--no-session-persistence", "--output-format", "--json-schema", "--model", "--effort", "--tools", "--permission-mode", "--no-chrome"}
	if effective.Isolation.Value == protocol.IsolationStrict {
		required = append(required, "--safe-mode", "--setting-sources", "--strict-mcp-config", "--disallowedTools")
	}
	if effective.WebAccess.Value {
		required = append(required, "--allowedTools")
	}
	if missing := missingCapabilities(string(helpResult.Stdout)+string(helpResult.Stderr), required); len(missing) != 0 {
		return "", newFailure(protocol.FailureCapability, "Claude is missing required flags: "+strings.Join(missing, ", "), environment, nil)
	}
	return string(match[1]), nil
}

func claudeArguments(effective config.Effective, schema, model string) []string {
	arguments := make([]string, 0, 32)
	if effective.Isolation.Value == protocol.IsolationStrict {
		arguments = append(arguments,
			"--safe-mode",
			"--setting-sources", "user",
			"--strict-mcp-config",
			"--disallowedTools", "mcp__*",
		)
	}
	arguments = append(arguments,
		"--print",
		"--no-session-persistence",
		"--output-format", "json",
		"--json-schema", schema,
		"--permission-mode", "dontAsk",
		"--no-chrome",
	)
	if effective.WebAccess.Value {
		arguments = append(arguments, "--tools", "WebSearch", "--allowedTools", "WebSearch")
	} else {
		arguments = append(arguments, "--tools", "")
	}
	arguments = append(arguments,
		"--model", model,
		"--effort", string(effective.ReasoningEffort.Value),
	)
	return arguments
}

func decodeClaudeEnvelope(output []byte) ([]byte, error) {
	output = bytes.TrimSpace(output)
	if len(output) == 0 {
		return nil, fmt.Errorf("Claude output is empty")
	}
	if err := protocol.RejectDuplicateKeys(output); err != nil {
		return nil, err
	}
	var envelope struct {
		Type             string          `json:"type"`
		Subtype          string          `json:"subtype"`
		IsError          *bool           `json:"is_error"`
		APIErrorStatus   *int            `json:"api_error_status"`
		Result           string          `json:"result"`
		StructuredOutput json.RawMessage `json:"structured_output"`
	}
	if err := json.Unmarshal(output, &envelope); err != nil {
		return nil, err
	}
	if envelope.Type == "result" && envelope.IsError != nil && *envelope.IsError {
		if !validClaudeFailureSubtype(envelope.Subtype) || strings.TrimSpace(envelope.Result) == "" {
			return nil, fmt.Errorf("Claude returned an invalid failure result")
		}
		class := protocol.FailureProvider
		message := "Claude reported a provider failure"
		if envelope.APIErrorStatus != nil {
			switch *envelope.APIErrorStatus {
			case 401, 403:
				class = protocol.FailureAuth
				message = "Claude reported an authentication failure"
			case 429:
				message = "Claude reported a rate limit"
			}
		}
		return nil, &reportedProviderError{Class: class, Message: message}
	}
	if envelope.Type != "result" || envelope.Subtype != "success" || envelope.IsError == nil {
		return nil, fmt.Errorf("Claude did not report a successful result")
	}
	structured := bytes.TrimSpace(envelope.StructuredOutput)
	if len(structured) == 0 || structured[0] != '{' {
		return nil, fmt.Errorf("Claude result is missing structured_output object")
	}
	return append([]byte(nil), structured...), nil
}

func validClaudeFailureSubtype(subtype string) bool {
	switch subtype {
	case "success",
		"error_during_execution",
		"error_max_turns",
		"error_max_budget_usd",
		"error_max_structured_output_retries":
		return true
	default:
		return false
	}
}

func setEnvironmentValue(environment []string, name, value string) []string {
	updated := make([]string, 0, len(environment)+1)
	for _, entry := range environment {
		key, _, found := strings.Cut(entry, "=")
		if found && strings.EqualFold(key, name) {
			continue
		}
		updated = append(updated, entry)
	}
	return append(updated, name+"="+value)
}
