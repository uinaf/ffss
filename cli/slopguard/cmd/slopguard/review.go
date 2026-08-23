package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/uinaf/ffss/cli/slopguard/internal/config"
	"github.com/uinaf/ffss/cli/slopguard/internal/orchestrator"
	"github.com/uinaf/ffss/cli/slopguard/internal/phase"
	"github.com/uinaf/ffss/cli/slopguard/internal/protocol"
	"github.com/uinaf/ffss/cli/slopguard/internal/provider"
	reportwriter "github.com/uinaf/ffss/cli/slopguard/internal/report"
	repositorypkg "github.com/uinaf/ffss/cli/slopguard/internal/repository"
	"github.com/uinaf/ffss/cli/slopguard/internal/target"
)

type stringList []string

func (values *stringList) String() string { return fmt.Sprint([]string(*values)) }

func (values *stringList) Set(value string) error {
	*values = append(*values, value)
	return nil
}

func runReview(ctx context.Context, arguments []string, stdout, stderr io.Writer, dependencies dependencies) int {
	recorder := phase.New(dependencies.now, dependencies.observePhase)
	ctx = phase.WithRecorder(ctx, recorder)
	started := recorder.Now()
	configSpan := recorder.Start(phase.Config)
	defer configSpan.End()
	jsonRequested, outputErr := reviewJSONRequested(arguments)
	if outputErr != nil {
		configSpan.End()
		return writeReviewArgumentFailure(ctx, stdout, stderr, jsonRequested, outputErr, started, recorder)
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
	skipSecretScan := flags.Bool("skip-secret-scan", false, "omit TruffleHog for this run")
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
		return writeReviewArgumentFailure(ctx, stdout, stderr, jsonRequested, err, started, recorder)
	}
	if flags.NArg() != 0 {
		configSpan.End()
		return writeReviewArgumentFailure(ctx, stdout, stderr, jsonRequested, errors.New("slopguard review does not accept positional arguments"), started, recorder)
	}
	if *output != "terminal" && *output != "json" {
		configSpan.End()
		return writeReviewArgumentFailure(ctx, stdout, stderr, jsonRequested, errors.New("flag output must be terminal or json"), started, recorder)
	}
	promptSet := false
	promptFileSet := false
	flags.Visit(func(item *flag.Flag) {
		promptSet = promptSet || item.Name == "prompt"
		promptFileSet = promptFileSet || item.Name == "prompt-file"
	})
	if promptSet && promptFileSet {
		configSpan.End()
		return writeReviewArgumentFailure(ctx, stdout, stderr, jsonRequested, errors.New("flags prompt and prompt-file are mutually exclusive"), started, recorder)
	}
	overrides, err := configFlags.overrides(flags)
	if err != nil {
		configSpan.End()
		return writeReviewArgumentFailure(ctx, stdout, stderr, jsonRequested, err, started, recorder)
	}
	resolvedPrompt := *prompt
	if promptFileSet {
		resolvedPrompt, err = readReviewPrompt(*promptFile, dependencies.stdin, target.MaximumMaxBytes)
		if err != nil {
			configSpan.End()
			return writeReviewResult(ctx, stdout, stderr, *output, failureWithElapsed(protocol.FailureTarget, err, started, recorder))
		}
	} else if err := target.ValidatePrompt(resolvedPrompt); err != nil {
		configSpan.End()
		return writeReviewResult(ctx, stdout, stderr, *output, failureWithElapsed(protocol.FailureTarget, err, started, recorder))
	}
	if strings.TrimSpace(resolvedPrompt) == "" {
		configSpan.End()
		return writeReviewArgumentFailure(ctx, stdout, stderr, jsonRequested, errors.New("prompt must not be blank"), started, recorder)
	}
	configSpan.End()
	repositoryProbeSpan := recorder.Start(phase.DependencyProbes)
	repositoryContext, err := repositorypkg.Resolve(ctx, repositorypkg.Options{Path: *repository})
	repositoryProbeSpan.End()
	if err != nil {
		return writeReviewResult(ctx, stdout, stderr, *output, failureWithElapsed(protocol.FailureConfig, err, started, recorder))
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
		return writeReviewResult(ctx, stdout, stderr, *output, failureWithElapsed(protocol.FailureConfig, fmt.Errorf("config: %w", err), started, recorder))
	}
	if int64(len(resolvedPrompt)) > effective.MaxBytes.Value {
		configSpan.End()
		return writeReviewResult(ctx, stdout, stderr, *output, failureWithElapsed(protocol.FailureTarget, fmt.Errorf("prompt exceeds max_bytes limit of %d", effective.MaxBytes.Value), started, recorder))
	}
	configSpan.End()

	newCollector := dependencies.newCollector
	if newCollector == nil {
		newCollector = func() (*target.Collector, error) {
			return target.NewContext(ctx, target.Options{Context: repositoryContext, SkipSecretScan: *skipSecretScan})
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
		case errors.Is(err, target.ErrSecretScan):
			class = protocol.FailureSecretScan
		}
		return writeReviewResult(ctx, stdout, stderr, *output, failureWithElapsed(class, fmt.Errorf("initialize target collector: %w", err), started, recorder))
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
			Mode:           protocol.TargetMode(*mode),
			Base:           *base,
			Commit:         *commit,
			Prompt:         resolvedPrompt,
			ContextFiles:   append([]string(nil), contextFiles...),
			MaxBytes:       effective.MaxBytes.Value,
			SkipSecretScan: *skipSecretScan,
		},
		Config: effective,
		Progress: func(message string) {
			report(stderr, "%s\n", message)
		},
		Now:     recorder.Now,
		Started: started,
	})
	return writeReviewResult(ctx, stdout, stderr, *output, result)
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
	if jsonRequested {
		return writeReviewResult(ctx, stdout, stderr, "json", failureWithElapsed(protocol.FailureConfig, err, started, recorder))
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
