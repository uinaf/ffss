package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/uinaf/autoreview/internal/config"
	"github.com/uinaf/autoreview/internal/orchestrator"
	"github.com/uinaf/autoreview/internal/protocol"
	"github.com/uinaf/autoreview/internal/provider"
	reportwriter "github.com/uinaf/autoreview/internal/report"
	"github.com/uinaf/autoreview/internal/target"
)

type stringList []string

func (values *stringList) String() string { return fmt.Sprint([]string(*values)) }

func (values *stringList) Set(value string) error {
	*values = append(*values, value)
	return nil
}

func runReview(ctx context.Context, arguments []string, stdout, stderr io.Writer, dependencies dependencies) int {
	jsonRequested, outputErr := reviewJSONRequested(arguments)
	if outputErr != nil {
		return writeReviewArgumentFailure(stdout, stderr, jsonRequested, outputErr)
	}
	flags := flag.NewFlagSet("autoreview review", flag.ContinueOnError)
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
		return writeReviewArgumentFailure(stdout, stderr, jsonRequested, err)
	}
	if flags.NArg() != 0 {
		return writeReviewArgumentFailure(stdout, stderr, jsonRequested, errors.New("autoreview review does not accept positional arguments"))
	}
	if *output != "terminal" && *output != "json" {
		return writeReviewArgumentFailure(stdout, stderr, jsonRequested, errors.New("flag output must be terminal or json"))
	}
	promptSet := false
	promptFileSet := false
	flags.Visit(func(item *flag.Flag) {
		promptSet = promptSet || item.Name == "prompt"
		promptFileSet = promptFileSet || item.Name == "prompt-file"
	})
	if promptSet && promptFileSet {
		return writeReviewArgumentFailure(stdout, stderr, jsonRequested, errors.New("flags prompt and prompt-file are mutually exclusive"))
	}
	overrides, err := configFlags.overrides(flags)
	if err != nil {
		return writeReviewArgumentFailure(stdout, stderr, jsonRequested, err)
	}
	effective, err := config.Load(ctx, config.Options{
		Repository: *repository,
		Overrides:  overrides,
		LookupEnv:  dependencies.lookupEnv,
		HomeDir:    dependencies.homeDir,
	})
	if err != nil {
		return writeReviewResult(stdout, stderr, *output, orchestrator.Failure(protocol.FailureConfig, fmt.Errorf("config: %w", err)))
	}
	resolvedPrompt := *prompt
	if promptFileSet {
		resolvedPrompt, err = readReviewPrompt(*promptFile, dependencies.stdin, effective.MaxBytes.Value)
		if err != nil {
			return writeReviewResult(stdout, stderr, *output, orchestrator.Failure(protocol.FailureTarget, err))
		}
	}

	newCollector := dependencies.newCollector
	if newCollector == nil {
		newCollector = func() (*target.Collector, error) {
			return target.NewContext(ctx, target.Options{Repository: *repository})
		}
	}
	collector, err := newCollector()
	if err != nil {
		return writeReviewResult(stdout, stderr, *output, orchestrator.Failure(protocol.FailureCapability, fmt.Errorf("initialize target collector: %w", err)))
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
	})
	return writeReviewResult(stdout, stderr, *output, result)
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

func writeReviewArgumentFailure(stdout, stderr io.Writer, jsonRequested bool, err error) int {
	if jsonRequested {
		return writeReviewResult(stdout, stderr, "json", orchestrator.Failure(protocol.FailureConfig, err))
	}
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

func writeReviewResult(stdout, stderr io.Writer, output string, result protocol.Report) int {
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
