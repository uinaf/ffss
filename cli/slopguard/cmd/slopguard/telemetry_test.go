package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/uinaf/ffss/cli/slopguard/internal/protocol"
	telemetrypkg "github.com/uinaf/ffss/cli/slopguard/internal/telemetry"
)

func TestReviewTelemetryDisabledDoesNoFilesystemWork(t *testing.T) {
	repository := reviewRepository(t)
	reviewer := &scriptedReviewer{results: []reviewStep{{result: cleanResult()}}}
	dependencies := reviewDependencies(t, cleanScanner{}, reviewer)
	pathCalls := 0
	appendCalls := 0
	dependencies.telemetryPath = func() (string, error) {
		pathCalls++
		return filepath.Join(t.TempDir(), "telemetry.jsonl"), nil
	}
	dependencies.appendTelemetry = func(string, telemetrypkg.Event) error {
		appendCalls++
		return nil
	}
	var stdout bytes.Buffer
	exit := run(t.Context(), []string{
		"review", "--repository", repository, "--mode", "local", "--engine", "codex",
		"--prompt", "Review the target.", "--output", "json",
	}, &stdout, io.Discard, dependencies)
	if exit != 0 || pathCalls != 0 || appendCalls != 0 {
		t.Fatalf("exit=%d path_calls=%d append_calls=%d output=%s", exit, pathCalls, appendCalls, stdout.String())
	}
}

func TestConflictingTelemetryFlagsDoNoFilesystemWork(t *testing.T) {
	repository := reviewRepository(t)
	reviewer := &scriptedReviewer{results: []reviewStep{{result: cleanResult()}}}
	dependencies := reviewDependencies(t, cleanScanner{}, reviewer)
	pathCalls := 0
	appendCalls := 0
	dependencies.telemetryPath = func() (string, error) {
		pathCalls++
		return filepath.Join(t.TempDir(), "telemetry.jsonl"), nil
	}
	dependencies.appendTelemetry = func(string, telemetrypkg.Event) error {
		appendCalls++
		return nil
	}
	var stdout bytes.Buffer
	exit := run(t.Context(), []string{
		"review", "--repository", repository, "--mode", "local", "--engine", "codex", "--prompt", "Review the target.",
		"--telemetry=false", "--telemetry=true", "--output", "json",
	}, &stdout, io.Discard, dependencies)
	if exit != 0 || pathCalls != 0 || appendCalls != 0 {
		t.Fatalf("exit=%d path_calls=%d append_calls=%d output=%s", exit, pathCalls, appendCalls, stdout.String())
	}
}

func TestTelemetryStorageFailureCannotChangeReviewResult(t *testing.T) {
	repository := reviewRepository(t)
	runReview := func(enabled bool) (protocol.Report, int, int, telemetrypkg.Event) {
		t.Helper()
		reviewer := &scriptedReviewer{results: []reviewStep{{result: cleanResult()}}}
		clock := &steppingClock{current: time.Unix(0, 0), step: 5 * time.Millisecond}
		dependencies := reviewDependencies(t, cleanScanner{}, reviewer)
		dependencies.now = clock.Now
		appendCalls := 0
		var event telemetrypkg.Event
		dependencies.telemetryPath = func() (string, error) { return filepath.Join(t.TempDir(), "telemetry.jsonl"), nil }
		dependencies.appendTelemetry = func(_ string, value telemetrypkg.Event) error {
			appendCalls++
			event = value
			return errors.New("injected storage failure")
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
		return report, exit, appendCalls, event
	}
	baseline, baselineExit, baselineAppends, _ := runReview(false)
	enabled, enabledExit, enabledAppends, event := runReview(true)
	if baselineExit != 0 || enabledExit != baselineExit || baselineAppends != 0 || enabledAppends != 1 {
		t.Fatalf("baseline_exit=%d enabled_exit=%d baseline_appends=%d enabled_appends=%d", baselineExit, enabledExit, baselineAppends, enabledAppends)
	}
	if !reflect.DeepEqual(baseline, enabled) {
		t.Fatalf("telemetry changed report:\nbaseline=%+v\nenabled=%+v", baseline, enabled)
	}
	if err := event.Validate(); err != nil || event.Outcome != string(protocol.StatusClean) {
		t.Fatalf("event=%+v error=%v", event, err)
	}
}

func TestExplicitTelemetryRecordsConfigurationFailure(t *testing.T) {
	appendCalls := 0
	var event telemetrypkg.Event
	dependencies := dependencies{
		telemetryPath: func() (string, error) { return filepath.Join(t.TempDir(), "telemetry.jsonl"), nil },
		appendTelemetry: func(_ string, value telemetrypkg.Event) error {
			appendCalls++
			event = value
			return nil
		},
	}
	var stdout bytes.Buffer
	exit := run(t.Context(), []string{
		"review", "--repository", filepath.Join(t.TempDir(), "missing"), "--mode", "local", "--engine", "codex",
		"--prompt", "Review the target.", "--telemetry", "--output", "json",
	}, &stdout, io.Discard, dependencies)
	if exit != 2 || appendCalls != 1 || event.Outcome != string(protocol.StatusFailure) || event.FailureClass != string(protocol.FailureConfig) {
		t.Fatalf("exit=%d append_calls=%d event=%+v output=%s", exit, appendCalls, event, stdout.String())
	}
}

func TestExplicitTelemetryRecordsEarlyOutputFailure(t *testing.T) {
	appendCalls := 0
	var event telemetrypkg.Event
	var stdout bytes.Buffer
	exit := run(t.Context(), []string{
		"review", "--telemetry", "--output=json", "--output=terminal",
	}, &stdout, io.Discard, dependencies{
		telemetryPath: func() (string, error) { return filepath.Join(t.TempDir(), "telemetry.jsonl"), nil },
		appendTelemetry: func(_ string, value telemetrypkg.Event) error {
			appendCalls++
			event = value
			return nil
		},
	})
	if exit != 2 || appendCalls != 1 || event.FailureClass != string(protocol.FailureConfig) {
		t.Fatalf("exit=%d append_calls=%d event=%+v output=%s", exit, appendCalls, event, stdout.String())
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
		{name: "conflicting", arguments: []string{"--telemetry", "--telemetry=false"}},
		{name: "conflicting false then true", arguments: []string{"--telemetry=false", "--telemetry=true"}},
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
		pathCalls := 0
		appendCalls := 0
		var stdout bytes.Buffer
		exit := run(t.Context(), arguments, &stdout, io.Discard, dependencies{
			telemetryPath: func() (string, error) {
				pathCalls++
				return filepath.Join(t.TempDir(), "telemetry.jsonl"), nil
			},
			appendTelemetry: func(string, telemetrypkg.Event) error {
				appendCalls++
				return nil
			},
		})
		if exit != 0 || pathCalls != 0 || appendCalls != 0 || !bytes.Contains(stdout.Bytes(), []byte("Usage of slopguard review")) {
			t.Fatalf("arguments=%v exit=%d path_calls=%d append_calls=%d output=%q", arguments, exit, pathCalls, appendCalls, stdout.String())
		}
	}
}

func TestReviewTelemetryAppendsLocalEvent(t *testing.T) {
	repository := reviewRepository(t)
	path := filepath.Join(t.TempDir(), "telemetry.jsonl")
	reviewer := &scriptedReviewer{results: []reviewStep{{result: cleanResult()}}}
	dependencies := reviewDependencies(t, cleanScanner{}, reviewer)
	dependencies.telemetryPath = func() (string, error) { return path, nil }
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
