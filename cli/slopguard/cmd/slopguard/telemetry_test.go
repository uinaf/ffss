package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/uinaf/ffss/cli/slopguard/internal/phase"
	"github.com/uinaf/ffss/cli/slopguard/internal/protocol"
	telemetrypkg "github.com/uinaf/ffss/cli/slopguard/internal/telemetry"
)

func TestReviewTelemetryDisabledDoesNoFilesystemWork(t *testing.T) {
	repository := reviewRepository(t)
	reviewer := &scriptedReviewer{results: []reviewStep{{result: cleanResult()}}}
	dependencies := reviewDependencies(t, cleanScanner{}, reviewer)
	startCalls := 0
	dependencies.startTelemetry = func(telemetrypkg.Event) error {
		startCalls++
		return nil
	}
	var stdout bytes.Buffer
	exit := run(t.Context(), []string{
		"review", "--repository", repository, "--mode", "local", "--engine", "codex",
		"--prompt", "Review the target.", "--output", "json",
	}, &stdout, io.Discard, dependencies)
	if exit != 0 || startCalls != 0 {
		t.Fatalf("exit=%d start_calls=%d output=%s", exit, startCalls, stdout.String())
	}
}

func TestLastTelemetryFlagControlsOptIn(t *testing.T) {
	for _, test := range []struct {
		name       string
		flags      []string
		wantStarts int
	}{
		{name: "last true", flags: []string{"--telemetry=false", "--telemetry=true"}, wantStarts: 1},
		{name: "last false", flags: []string{"--telemetry=true", "--telemetry=false"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			repository := reviewRepository(t)
			reviewer := &scriptedReviewer{results: []reviewStep{{result: cleanResult()}}}
			dependencies := reviewDependencies(t, cleanScanner{}, reviewer)
			startCalls := 0
			dependencies.startTelemetry = func(telemetrypkg.Event) error {
				startCalls++
				return nil
			}
			arguments := []string{
				"review", "--repository", repository, "--mode", "local", "--engine", "codex", "--prompt", "Review the target.",
			}
			arguments = append(arguments, test.flags...)
			arguments = append(arguments, "--output", "json")
			var stdout bytes.Buffer
			exit := run(t.Context(), arguments, &stdout, io.Discard, dependencies)
			if exit != 0 || startCalls != test.wantStarts {
				t.Fatalf("exit=%d start_calls=%d output=%s", exit, startCalls, stdout.String())
			}
		})
	}
}

func TestTelemetryHandoffFailureCannotChangeReviewResult(t *testing.T) {
	repository := reviewRepository(t)
	runReview := func(enabled bool) (protocol.Report, int, int, telemetrypkg.Event) {
		t.Helper()
		reviewer := &scriptedReviewer{results: []reviewStep{{result: cleanResult()}}}
		clock := &steppingClock{current: time.Unix(0, 0), step: 5 * time.Millisecond}
		dependencies := reviewDependencies(t, cleanScanner{}, reviewer)
		dependencies.now = clock.Now
		startCalls := 0
		var event telemetrypkg.Event
		dependencies.startTelemetry = func(value telemetrypkg.Event) error {
			startCalls++
			event = value
			return errors.New("injected handoff failure")
		}
		arguments := []string{
			"review", "--repository", repository, "--mode", "local", "--engine", "codex",
			"--prompt", "Review the target.", "--output", "json",
		}
		if enabled {
			arguments = append(arguments, "--telemetry")
		}
		var stdout bytes.Buffer
		exit := run(t.Context(), arguments, &stdout, io.Discard, dependencies)
		var report protocol.Report
		if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
			t.Fatal(err)
		}
		return report, exit, startCalls, event
	}
	baseline, baselineExit, baselineStarts, _ := runReview(false)
	enabled, enabledExit, enabledStarts, event := runReview(true)
	if baselineExit != 0 || enabledExit != baselineExit || baselineStarts != 0 || enabledStarts != 1 {
		t.Fatalf("baseline_exit=%d enabled_exit=%d baseline_starts=%d enabled_starts=%d", baselineExit, enabledExit, baselineStarts, enabledStarts)
	}
	if !reflect.DeepEqual(baseline, enabled) {
		t.Fatalf("telemetry changed report:\nbaseline=%+v\nenabled=%+v", baseline, enabled)
	}
	if err := event.Validate(); err != nil || event.Outcome != string(protocol.StatusClean) {
		t.Fatalf("event=%+v error=%v", event, err)
	}
}

func TestTelemetryEnablementPathsCaptureSamePhases(t *testing.T) {
	repository := reviewRepository(t)
	runReview := func(accountConfig bool) telemetrypkg.Event {
		t.Helper()
		home := t.TempDir()
		if accountConfig {
			configDirectory := filepath.Join(home, ".config", "slopguard")
			if err := os.MkdirAll(configDirectory, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(configDirectory, "config.yaml"), []byte("telemetry: true\n"), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		reviewer := &scriptedReviewer{results: []reviewStep{{result: cleanResult()}}}
		clock := &steppingClock{current: time.Unix(0, 0), step: 5 * time.Millisecond}
		dependencies := reviewDependencies(t, cleanScanner{}, reviewer)
		dependencies.now = clock.Now
		dependencies.lookupEnv = func(string) (string, bool) { return "", false }
		dependencies.homeDir = func() (string, error) { return home, nil }
		var event telemetrypkg.Event
		dependencies.startTelemetry = func(value telemetrypkg.Event) error {
			event = value
			return nil
		}
		arguments := []string{
			"review", "--repository", repository, "--mode", "local", "--engine", "codex",
			"--prompt", "Review the target.", "--output", "json",
		}
		if !accountConfig {
			arguments = append(arguments, "--telemetry")
		}
		var stdout bytes.Buffer
		if exit := run(t.Context(), arguments, &stdout, io.Discard, dependencies); exit != 0 {
			t.Fatalf("exit=%d output=%s", exit, stdout.String())
		}
		return event
	}
	flagEvent := runReview(false)
	accountEvent := runReview(true)
	if !reflect.DeepEqual(accountEvent.PhaseDurationBuckets, flagEvent.PhaseDurationBuckets) {
		t.Fatalf("account phases=%v flag phases=%v", accountEvent.PhaseDurationBuckets, flagEvent.PhaseDurationBuckets)
	}
	if accountEvent.PhaseDurationBuckets[string(phase.Config)] == "" || accountEvent.PhaseDurationBuckets[string(phase.DependencyProbes)] == "" {
		t.Fatalf("early phases missing: %v", accountEvent.PhaseDurationBuckets)
	}
}

func TestTelemetryRecorderStartsWithoutWaitingForStorage(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "recorded")
	executable := filepath.Join(t.TempDir(), "recorder")
	script := "#!/bin/sh\n/bin/sleep 1\nprintf '%s' \"${HOME-unset}\" > " + shellLiteral(marker) + "\n"
	if err := os.WriteFile(executable, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", filepath.Join(t.TempDir(), "private-home"))
	event := telemetrypkg.NewMetrics().Event("development", protocol.Report{
		SchemaVersion: protocol.SchemaVersion,
		Status:        protocol.StatusClean,
		Review:        &protocol.Review{Findings: []protocol.Finding{}, OverallExplanation: "not handed off", OverallConfidence: 1},
		Metadata:      protocol.Metadata{Attempts: []protocol.Attempt{}},
	})
	started := time.Now()
	if err := startTelemetryRecorderWithExecutable(executable, event); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(started); elapsed > 500*time.Millisecond {
		t.Fatalf("telemetry handoff took %s", elapsed)
	}
	if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("recorder completed synchronously: %v", err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		if content, err := os.ReadFile(marker); err == nil {
			if string(content) != "unset" {
				t.Fatalf("recorder inherited HOME: %q", content)
			}
			break
		} else if !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
		if time.Now().After(deadline) {
			t.Fatal("detached recorder did not finish")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestExplicitTelemetryRecordsConfigurationFailure(t *testing.T) {
	startCalls := 0
	var event telemetrypkg.Event
	dependencies := dependencies{
		startTelemetry: func(value telemetrypkg.Event) error {
			startCalls++
			event = value
			return nil
		},
	}
	var stdout bytes.Buffer
	exit := run(t.Context(), []string{
		"review", "--repository", filepath.Join(t.TempDir(), "missing"), "--mode", "local", "--engine", "codex",
		"--prompt", "Review the target.", "--telemetry", "--output", "json",
	}, &stdout, io.Discard, dependencies)
	if exit != 2 || startCalls != 1 || event.Outcome != string(protocol.StatusFailure) || event.FailureClass != string(protocol.FailureConfig) {
		t.Fatalf("exit=%d start_calls=%d event=%+v output=%s", exit, startCalls, event, stdout.String())
	}
}

func TestExplicitTelemetryRecordsEarlyOutputFailure(t *testing.T) {
	startCalls := 0
	var event telemetrypkg.Event
	var stdout bytes.Buffer
	exit := run(t.Context(), []string{
		"review", "--telemetry", "--output=json", "--output=terminal",
	}, &stdout, io.Discard, dependencies{
		startTelemetry: func(value telemetrypkg.Event) error {
			startCalls++
			event = value
			return nil
		},
	})
	if exit != 2 || startCalls != 1 || event.FailureClass != string(protocol.FailureConfig) {
		t.Fatalf("exit=%d start_calls=%d event=%+v output=%s", exit, startCalls, event, stdout.String())
	}
	var report protocol.Report
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil || report.Failure == nil || report.Failure.Class != protocol.FailureConfig {
		t.Fatalf("report=%+v error=%v output=%s", report, err, stdout.String())
	}
}

func TestReviewTelemetryPreScan(t *testing.T) {
	tests := []struct {
		name      string
		arguments []string
		want      bool
	}{
		{name: "explicit", arguments: []string{"--telemetry"}, want: true},
		{name: "explicit true", arguments: []string{"--telemetry=true"}, want: true},
		{name: "full valid prefix", arguments: []string{"--repository", "/tmp/repository", "--mode", "local", "--engine", "codex", "--prompt", "task", "--output", "json", "--telemetry"}, want: true},
		{name: "explicit false", arguments: []string{"--telemetry=false"}},
		{name: "prompt value", arguments: []string{"--prompt", "--telemetry"}},
		{name: "later parse error", arguments: []string{"--telemetry", "--unknown"}, want: true},
		{name: "inline boolean before telemetry", arguments: []string{"--web-access=false", "--telemetry", "--unknown"}, want: true},
		{name: "inline skip before telemetry", arguments: []string{"--skip-secret-scan=false", "--telemetry", "--unknown"}, want: true},
		{name: "invalid inline before telemetry", arguments: []string{"--web-access=invalid", "--telemetry"}},
		{name: "invalid inline after telemetry", arguments: []string{"--telemetry", "--web-access=invalid"}, want: true},
		{name: "inline prompt value", arguments: []string{"--prompt=--telemetry"}},
		{name: "error before telemetry", arguments: []string{"--unknown", "--telemetry"}},
		{name: "last false", arguments: []string{"--telemetry", "--telemetry=false"}},
		{name: "last true", arguments: []string{"--telemetry=false", "--telemetry=true"}, want: true},
		{name: "invalid telemetry after opt in", arguments: []string{"--telemetry", "--telemetry=invalid"}, want: true},
		{name: "output conflict before telemetry", arguments: []string{"--output=json", "--output=terminal", "--telemetry"}},
		{name: "output conflict after telemetry", arguments: []string{"--telemetry", "--output=json", "--output=terminal"}, want: true},
		{name: "prompt conflict before telemetry", arguments: []string{"--prompt", "task", "--prompt-file", "task.md", "--telemetry"}},
		{name: "prompt conflict after telemetry", arguments: []string{"--telemetry", "--prompt", "task", "--prompt-file", "task.md"}, want: true},
		{name: "invalid model before telemetry", arguments: []string{"--model", " bad", "--telemetry"}},
		{name: "invalid model after telemetry", arguments: []string{"--telemetry", "--model", " bad"}, want: true},
		{name: "after terminator", arguments: []string{"--", "--telemetry"}},
		{name: "help before telemetry", arguments: []string{"--help", "--telemetry"}},
		{name: "help after telemetry", arguments: []string{"--telemetry", "--help"}, want: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := reviewTelemetryRequested(test.arguments); got != test.want {
				t.Fatalf("reviewTelemetryRequested(%v) = %t, want %t", test.arguments, got, test.want)
			}
		})
	}
}

func TestReviewHelpNeverWritesTelemetry(t *testing.T) {
	for _, arguments := range [][]string{{"review", "--help", "--telemetry"}, {"review", "--telemetry", "--help"}} {
		startCalls := 0
		var stdout bytes.Buffer
		exit := run(t.Context(), arguments, &stdout, io.Discard, dependencies{
			startTelemetry: func(telemetrypkg.Event) error {
				startCalls++
				return nil
			},
		})
		if exit != 0 || startCalls != 0 || !bytes.Contains(stdout.Bytes(), []byte("Usage of slopguard review")) {
			t.Fatalf("arguments=%v exit=%d start_calls=%d output=%q", arguments, exit, startCalls, stdout.String())
		}
	}
}

func TestReviewTelemetryAppendsLocalEvent(t *testing.T) {
	repository := reviewRepository(t)
	path := filepath.Join(t.TempDir(), "telemetry.jsonl")
	reviewer := &scriptedReviewer{results: []reviewStep{{result: cleanResult()}}}
	dependencies := reviewDependencies(t, cleanScanner{}, reviewer)
	dependencies.startTelemetry = func(event telemetrypkg.Event) error {
		return (telemetrypkg.Store{Path: path}).Append(event)
	}
	var stdout bytes.Buffer
	exit := run(t.Context(), []string{
		"review", "--repository", repository, "--mode", "local", "--engine", "codex",
		"--prompt", "Review the target.", "--telemetry", "--output", "json",
	}, &stdout, io.Discard, dependencies)
	if exit != 0 {
		t.Fatalf("exit=%d output=%s", exit, stdout.String())
	}
	var exported bytes.Buffer
	if err := (telemetrypkg.Store{Path: path}).Export(&exported); err != nil {
		t.Fatal(err)
	}
	var result telemetrypkg.Export
	if err := json.Unmarshal(exported.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Events) != 1 || result.Events[0].Provider != string(protocol.ProviderCodex) || result.Events[0].BundleBucket == "unavailable" || result.Events[0].PhaseDurationBuckets["report_write"] == "" {
		t.Fatalf("events=%+v", result.Events)
	}
}

func TestTelemetryRecordInternalUsesStrictBoundedInput(t *testing.T) {
	event := telemetrypkg.NewMetrics().Event("development", protocol.Report{
		SchemaVersion: protocol.SchemaVersion,
		Status:        protocol.StatusClean,
		Review:        &protocol.Review{Findings: []protocol.Finding{}, OverallExplanation: "not recorded", OverallConfidence: 1},
		Metadata:      protocol.Metadata{Attempts: []protocol.Attempt{}},
	})
	payload, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("valid", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "telemetry.jsonl")
		var stderr bytes.Buffer
		exit := runTelemetry([]string{telemetryRecordCommand, base64.RawURLEncoding.EncodeToString(payload)}, io.Discard, &stderr, dependencies{
			telemetryPath: func() (string, error) { return path, nil },
		})
		if exit != 0 || stderr.Len() != 0 || len(exportStoreEvents(t, path)) != 1 {
			t.Fatalf("exit=%d stderr=%q", exit, stderr.String())
		}
	})

	t.Run("unknown field", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "telemetry.jsonl")
		private := bytes.Replace(payload, []byte("{"), []byte(`{"prompt":"PRIVATE",`), 1)
		var stderr bytes.Buffer
		exit := runTelemetry([]string{telemetryRecordCommand, base64.RawURLEncoding.EncodeToString(private)}, io.Discard, &stderr, dependencies{
			telemetryPath: func() (string, error) { return path, nil },
		})
		if exit != 2 || bytes.Contains(stderr.Bytes(), []byte("PRIVATE")) || stderr.String() != "record telemetry: operation failed\n" {
			t.Fatalf("exit=%d stderr=%q", exit, stderr.String())
		}
	})
}

func TestTelemetryExportCommandUsesLocalStore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "telemetry.jsonl")
	store := telemetrypkg.Store{Path: path}
	event := telemetrypkg.NewMetrics().Event("development", protocol.Report{
		SchemaVersion: protocol.SchemaVersion,
		Status:        protocol.StatusClean,
		Review:        &protocol.Review{Findings: []protocol.Finding{}, OverallExplanation: "not exported", OverallConfidence: 1},
		Metadata:      protocol.Metadata{Attempts: []protocol.Attempt{}},
	})
	if err := store.Append(event); err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exit := run(t.Context(), []string{"telemetry", "export"}, &stdout, &stderr, dependencies{
		telemetryPath: func() (string, error) { return path, nil },
	})
	if exit != 0 || stderr.Len() != 0 {
		t.Fatalf("exit=%d stdout=%q stderr=%q", exit, stdout.String(), stderr.String())
	}
	var exported telemetrypkg.Export
	if err := json.Unmarshal(stdout.Bytes(), &exported); err != nil {
		t.Fatal(err)
	}
	if exported.SchemaVersion != telemetrypkg.SchemaVersion || len(exported.Events) != 1 {
		t.Fatalf("export=%+v", exported)
	}
}

func TestTelemetryExportFailureIsSanitized(t *testing.T) {
	privatePath := filepath.Join(t.TempDir(), "private-sentinel")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exit := run(t.Context(), []string{"telemetry", "export"}, &stdout, &stderr, dependencies{
		telemetryPath: func() (string, error) { return privatePath, errors.New("private sentinel") },
	})
	if exit != 2 || stdout.Len() != 0 || stderr.String() != "export telemetry: operation failed\n" {
		t.Fatalf("exit=%d stdout=%q stderr=%q", exit, stdout.String(), stderr.String())
	}
}

func exportStoreEvents(t *testing.T, path string) []telemetrypkg.Event {
	t.Helper()
	var output bytes.Buffer
	if err := (telemetrypkg.Store{Path: path}).Export(&output); err != nil {
		t.Fatal(err)
	}
	var exported telemetrypkg.Export
	if err := json.Unmarshal(output.Bytes(), &exported); err != nil {
		t.Fatal(err)
	}
	return exported.Events
}
