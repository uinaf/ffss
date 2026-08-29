package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/uinaf/ffss/cli/slopguard/internal/buildinfo"
	"github.com/uinaf/ffss/cli/slopguard/internal/config"
	"github.com/uinaf/ffss/cli/slopguard/internal/orchestrator"
	"github.com/uinaf/ffss/cli/slopguard/internal/phase"
	"github.com/uinaf/ffss/cli/slopguard/internal/protocol"
	"github.com/uinaf/ffss/cli/slopguard/internal/provider"
	reportwriter "github.com/uinaf/ffss/cli/slopguard/internal/report"
	repositorypkg "github.com/uinaf/ffss/cli/slopguard/internal/repository"
	"github.com/uinaf/ffss/cli/slopguard/internal/target"
	telemetrypkg "github.com/uinaf/ffss/cli/slopguard/internal/telemetry"
)

type stringList []string

func (values *stringList) String() string { return fmt.Sprint([]string(*values)) }

func (values *stringList) Set(value string) error {
	*values = append(*values, value)
	return nil
}

func runReview(ctx context.Context, arguments []string, stdout, stderr io.Writer, dependencies dependencies) int {
	var metrics *telemetrypkg.Metrics
	var telemetryEnabled atomic.Bool
	enableTelemetry := func() {
		if telemetryEnabled.Load() {
			return
		}
		metrics = telemetrypkg.NewMetrics()
		telemetryEnabled.Store(true)
	}
	explicitTelemetryRequested := reviewTelemetryRequested(arguments)
	telemetryDecisionMade := explicitTelemetryRequested
	pendingMeasurements := make([]phase.Measurement, 0, 2)
	if explicitTelemetryRequested {
		enableTelemetry()
	}
	resolveTelemetry := func(enabled bool) {
		if telemetryDecisionMade {
			return
		}
		telemetryDecisionMade = true
		if enabled {
			enableTelemetry()
			for _, measurement := range pendingMeasurements {
				metrics.Observe(measurement)
			}
		}
		pendingMeasurements = nil
	}
	recorder := phase.New(dependencies.now, func(measurement phase.Measurement) {
		if dependencies.observePhase != nil {
			dependencies.observePhase(measurement)
		}
		if telemetryEnabled.Load() {
			metrics.Observe(measurement)
		} else if !telemetryDecisionMade {
			pendingMeasurements = append(pendingMeasurements, measurement)
		}
	})
	ctx = phase.WithRecorder(ctx, recorder)
	started := recorder.Now()
	configSpan := recorder.Start(phase.Config)
	defer configSpan.End()
	recordTelemetry := func(result protocol.Report) {
		if telemetryEnabled.Load() {
			recordReviewTelemetry(dependencies, metrics, result)
		}
	}
	finishArgumentFailure := func(jsonRequested bool, err error) int {
		result := failureWithElapsed(protocol.FailureConfig, err, started, recorder)
		exit := writeReviewArgumentFailureReport(ctx, stdout, stderr, jsonRequested, err, result)
		recordTelemetry(result)
		return exit
	}
	jsonRequested, outputErr := reviewJSONRequested(arguments)
	if outputErr != nil {
		configSpan.End()
		return finishArgumentFailure(jsonRequested, outputErr)
	}
	flags := flag.NewFlagSet("slopguard review", flag.ContinueOnError)
	repository := flags.String("repository", ".", "Git repository or path within it")
	mode := flags.String("mode", "", "target mode: local, branch, or commit")
	base := flags.String("base", "", "base revision for branch mode")
	commit := flags.String("commit", "", "revision for commit mode")
	prompt := flags.String("prompt", "", "trusted review instructions")
	promptFile := flags.String("prompt-file", "", "trusted review instructions file, or - for stdin")
	var contextFiles stringList
	flags.Var(&contextFiles, "context-file", "repository-relative context file (repeatable)")
	output := flags.String("output", "terminal", "output format: terminal or json")
	configFlags := bindConfigFlags(flags)
	flagStderr := stderr
	if jsonRequested {
		flagStderr = io.Discard
	}
	if err := parseFlags(flags, arguments, stdout, flagStderr); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		configSpan.End()
		return finishArgumentFailure(jsonRequested, err)
	}
	finish := func(result protocol.Report) int {
		exit := writeReviewResult(ctx, stdout, stderr, *output, result)
		recordTelemetry(result)
		return exit
	}
	promptSet := false
	promptFileSet := false
	flags.Visit(func(item *flag.Flag) {
		promptSet = promptSet || item.Name == "prompt"
		promptFileSet = promptFileSet || item.Name == "prompt-file"
	})
	if flags.NArg() != 0 {
		configSpan.End()
		return finishArgumentFailure(jsonRequested, errors.New("slopguard review does not accept positional arguments"))
	}
	if *output != "terminal" && *output != "json" {
		configSpan.End()
		return finishArgumentFailure(jsonRequested, errors.New("flag output must be terminal or json"))
	}
	if promptSet && promptFileSet {
		configSpan.End()
		return finishArgumentFailure(jsonRequested, errors.New("flags prompt and prompt-file are mutually exclusive"))
	}
	overrides, err := configFlags.overrides(flags)
	if err != nil {
		configSpan.End()
		return finishArgumentFailure(jsonRequested, err)
	}
	resolvedPrompt := *prompt
	if promptFileSet {
		resolvedPrompt, err = readReviewPrompt(*promptFile, dependencies.stdin, target.MaximumMaxBytes)
		if err != nil {
			configSpan.End()
			return finish(failureWithElapsed(protocol.FailureTarget, err, started, recorder))
		}
	} else if err := target.ValidatePrompt(resolvedPrompt); err != nil {
		configSpan.End()
		return finish(failureWithElapsed(protocol.FailureTarget, err, started, recorder))
	}
	if strings.TrimSpace(resolvedPrompt) == "" {
		configSpan.End()
		return finishArgumentFailure(jsonRequested, errors.New("prompt must not be blank"))
	}
	configSpan.End()
	repositoryProbeSpan := recorder.Start(phase.DependencyProbes)
	repositoryContext, err := repositorypkg.Resolve(ctx, repositorypkg.Options{Path: *repository})
	repositoryProbeSpan.End()
	if err != nil {
		return finish(failureWithElapsed(protocol.FailureConfig, err, started, recorder))
	}
	configSpan = recorder.Start(phase.Config)
	effective, err := config.Load(ctx, config.Options{
		Context:   repositoryContext,
		Overrides: overrides,
		LookupEnv: dependencies.lookupEnv,
		HomeDir:   dependencies.homeDir,
	})
	if err != nil {
		configSpan.End()
		return finish(failureWithElapsed(protocol.FailureConfig, fmt.Errorf("config: %w", err), started, recorder))
	}
	resolveTelemetry(effective.Telemetry.Value && (effective.Telemetry.Source != config.SourceFlag || explicitTelemetryRequested))
	if int64(len(resolvedPrompt)) > effective.MaxBytes.Value {
		configSpan.End()
		return finish(failureWithElapsed(protocol.FailureTarget, fmt.Errorf("prompt exceeds max_bytes limit of %d", effective.MaxBytes.Value), started, recorder))
	}
	configSpan.End()

	newCollector := dependencies.newCollector
	if newCollector == nil {
		newCollector = func() (*target.Collector, error) {
			return target.NewContext(ctx, target.Options{Context: repositoryContext})
		}
	}
	probeSpan := recorder.Start(phase.DependencyProbes)
	collector, err := newCollector()
	probeSpan.End()
	if err != nil {
		class := protocol.FailureCapability
		switch {
		case errors.Is(err, context.Canceled):
			class = protocol.FailureCancelled
		case errors.Is(err, context.DeadlineExceeded):
			class = protocol.FailureTimeout
		}
		return finish(failureWithElapsed(class, fmt.Errorf("initialize target collector: %w", err), started, recorder))
	}
	newReviewer := dependencies.newReviewer
	if newReviewer == nil {
		newReviewer = defaultReviewer
	}
	result := orchestrator.Run(ctx, orchestrator.Options{
		Collector:   collector,
		NewReviewer: newReviewer,
		Repository:  *repository,
		Target: target.Request{
			Mode:         protocol.TargetMode(*mode),
			Base:         *base,
			Commit:       *commit,
			Prompt:       resolvedPrompt,
			ContextFiles: append([]string(nil), contextFiles...),
			MaxBytes:     effective.MaxBytes.Value,
		},
		Config: effective,
		Progress: func(message string) {
			report(stderr, "%s\n", message)
		},
		Now:     recorder.Now,
		Started: started,
		ObserveBundleBytes: func(value int64) {
			if telemetryEnabled.Load() {
				metrics.SetBundleBytes(value)
			}
		},
	})
	return finish(result)
}

func recordReviewTelemetry(dependencies dependencies, metrics *telemetrypkg.Metrics, result protocol.Report) {
	start := dependencies.startTelemetry
	if start == nil {
		start = startTelemetryRecorder
	}
	_ = start(metrics.Event(buildinfo.TelemetryVersion(), result))
}

func telemetryStorePath(dependencies dependencies) (string, error) {
	if dependencies.telemetryPath != nil {
		return dependencies.telemetryPath()
	}
	return telemetrypkg.DefaultPath(dependencies.homeDir)
}

func reviewTelemetryRequested(arguments []string) bool {
	selected := false
	seen := false
	selectedOutput := ""
	for index := 0; index < len(arguments); index++ {
		argument := arguments[index]
		if argument == "--" || !strings.HasPrefix(argument, "-") {
			break
		}
		name, raw, inline := strings.Cut(argument, "=")
		if name == "--telemetry" || name == "-telemetry" {
			if !inline {
				raw = "true"
			}
			value, err := strconv.ParseBool(raw)
			if err != nil {
				return seen && selected
			}
			seen = true
			selected = value
			continue
		}
		if inline {
			switch name {
			case "--web-access", "-web-access":
				if _, err := strconv.ParseBool(raw); err != nil {
					return seen && selected
				}
				continue
			}
		}
		if reviewFlagConsumesNext(name) {
			value := raw
			if !inline {
				if index+1 >= len(arguments) {
					return seen && selected
				}
				index++
				value = arguments[index]
			}
			if !validReviewFlagSyntax(name, value, &selectedOutput) {
				return seen && selected
			}
			continue
		}
		switch argument {
		case "--help", "-h":
			return seen && selected
		case "--web-access", "-web-access":
			continue
		default:
			return seen && selected
		}
	}
	return seen && selected
}

func validReviewFlagSyntax(name, value string, selectedOutput *string) bool {
	switch name {
	case "--output", "-output":
		if *selectedOutput != "" && *selectedOutput != value {
			return false
		}
		*selectedOutput = value
	case "--retries", "-retries":
		if _, err := strconv.ParseInt(value, 0, strconv.IntSize); err != nil {
			return false
		}
	case "--max-bytes", "-max-bytes":
		if _, err := strconv.ParseInt(value, 0, 64); err != nil {
			return false
		}
	}
	return true
}

func reviewJSONRequested(arguments []string) (bool, error) {
	selected := ""
	for index := 0; index < len(arguments); index++ {
		argument := arguments[index]
		if argument == "--" {
			break
		}
		value := ""
		found := false
		switch {
		case argument == "--output" || argument == "-output":
			if index+1 >= len(arguments) {
				continue
			}
			index++
			value = arguments[index]
			found = true
		case strings.HasPrefix(argument, "--output="):
			value = strings.TrimPrefix(argument, "--output=")
			found = true
		case strings.HasPrefix(argument, "-output="):
			value = strings.TrimPrefix(argument, "-output=")
			found = true
		}
		if !found {
			if reviewFlagConsumesNext(argument) && index+1 < len(arguments) {
				index++
			}
			continue
		}
		if selected != "" && selected != value {
			return selected == "json" || value == "json", errors.New("flag output must select exactly one format")
		}
		selected = value
	}
	return selected == "json", nil
}

func reviewFlagConsumesNext(argument string) bool {
	switch argument {
	case "--repository", "-repository",
		"--mode", "-mode",
		"--base", "-base",
		"--commit", "-commit",
		"--prompt", "-prompt",
		"--prompt-file", "-prompt-file",
		"--context-file", "-context-file",
		"--output", "-output",
		"--engine", "-engine",
		"--model", "-model",
		"--reasoning-effort", "-reasoning-effort",
		"--timeout", "-timeout",
		"--retries", "-retries",
		"--max-bytes", "-max-bytes",
		"--isolation", "-isolation":
		return true
	default:
		return false
	}
}

func readReviewPrompt(path string, stdin io.Reader, maxBytes int64) (string, error) {
	var reader io.Reader
	if path == "-" {
		if stdin == nil {
			return "", errors.New("prompt stdin is unavailable")
		}
		reader = stdin
	} else {
		file, err := os.Open(path)
		if err != nil {
			return "", safePromptFileError(err)
		}
		defer func() { _ = file.Close() }()
		info, err := file.Stat()
		if err != nil {
			return "", errors.New("inspect prompt file: operation failed")
		}
		if !info.Mode().IsRegular() {
			return "", errors.New("prompt file must be a regular file")
		}
		if info.Size() > maxBytes {
			return "", fmt.Errorf("prompt exceeds max_bytes limit of %d", maxBytes)
		}
		reader = file
	}
	content, err := io.ReadAll(io.LimitReader(reader, maxBytes+1))
	if err != nil {
		return "", errors.New("read prompt input: operation failed")
	}
	if int64(len(content)) > maxBytes {
		return "", fmt.Errorf("prompt exceeds max_bytes limit of %d", maxBytes)
	}
	prompt := string(content)
	if err := target.ValidatePrompt(prompt); err != nil {
		return "", err
	}
	return prompt, nil
}

func safePromptFileError(err error) error {
	switch {
	case errors.Is(err, os.ErrNotExist):
		return errors.New("prompt file does not exist")
	case errors.Is(err, os.ErrPermission):
		return errors.New("prompt file is not readable")
	default:
		return errors.New("open prompt file: operation failed")
	}
}

func writeReviewArgumentFailure(ctx context.Context, stdout, stderr io.Writer, jsonRequested bool, err error, started time.Time, recorder *phase.Recorder) int {
	return writeReviewArgumentFailureReport(ctx, stdout, stderr, jsonRequested, err, failureWithElapsed(protocol.FailureConfig, err, started, recorder))
}

func writeReviewArgumentFailureReport(ctx context.Context, stdout, stderr io.Writer, jsonRequested bool, err error, result protocol.Report) int {
	if jsonRequested {
		return writeReviewResult(ctx, stdout, stderr, "json", result)
	}
	span := phase.Start(ctx, phase.ReportWrite)
	defer span.End()
	report(stderr, "%v\n", err)
	return 2
}

func defaultReviewer(name protocol.ProviderName, repository string) provider.Reviewer {
	switch name {
	case protocol.ProviderCodex:
		return provider.NewCodex(provider.CodexOptions{Repository: repository})
	case protocol.ProviderClaude:
		return provider.NewClaude(provider.ClaudeOptions{Repository: repository})
	case protocol.ProviderCursor:
		return provider.NewCursor(provider.CursorOptions{Repository: repository})
	case protocol.ProviderGrok:
		return provider.NewGrok(provider.GrokOptions{Repository: repository})
	default:
		return nil
	}
}

func writeReviewResult(ctx context.Context, stdout, stderr io.Writer, output string, result protocol.Report) int {
	span := phase.Start(ctx, phase.ReportWrite)
	defer span.End()
	var err error
	if output == "json" {
		err = reportwriter.WriteJSON(stdout, result)
	} else {
		err = reportwriter.WriteTerminal(stdout, result)
	}
	if err != nil {
		report(stderr, "write review result: %v\n", err)
		return 2
	}
	switch result.Status {
	case protocol.StatusClean:
		return 0
	case protocol.StatusFindings:
		return 1
	default:
		return 2
	}
}

func failureWithElapsed(class protocol.FailureClass, err error, started time.Time, recorder *phase.Recorder) protocol.Report {
	result := orchestrator.Failure(class, err)
	result.Metadata.DurationMS = phase.ElapsedMilliseconds(started, recorder.Now())
	return result
}
