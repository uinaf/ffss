package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"

	telemetrypkg "github.com/uinaf/ffss/cli/slopguard/internal/telemetry"
)

const (
	telemetryRecordCommand  = "record-internal"
	maximumTelemetryHandoff = 8 << 10
)

func runTelemetry(arguments []string, stdout, stderr io.Writer, dependencies dependencies) int {
	if len(arguments) == 2 && arguments[0] == telemetryRecordCommand {
		return runTelemetryRecord(arguments[1], stderr, dependencies)
	}
	if len(arguments) == 1 && (arguments[0] == "--help" || arguments[0] == "-h" || arguments[0] == "help") {
		report(stdout, "usage: slopguard telemetry export\n")
		return 0
	}
	if len(arguments) != 1 || arguments[0] != "export" {
		report(stderr, "usage: slopguard telemetry export\n")
		return 2
	}
	path, err := telemetryStorePath(dependencies)
	if err != nil {
		report(stderr, "export telemetry: operation failed\n")
		return 2
	}
	if err := (telemetrypkg.Store{Path: path}).Export(stdout); err != nil {
		report(stderr, "export telemetry: operation failed\n")
		return 2
	}
	return 0
}

func startTelemetryRecorder(event telemetrypkg.Event) error {
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve telemetry recorder: %w", err)
	}
	return startTelemetryRecorderWithExecutable(executable, event)
}

func startTelemetryRecorderWithExecutable(executable string, event telemetrypkg.Event) error {
	if err := event.Validate(); err != nil {
		return err
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("encode telemetry handoff: %w", err)
	}
	if len(payload) > maximumTelemetryHandoff {
		return fmt.Errorf("telemetry handoff is too large")
	}
	command := exec.Command(executable, "telemetry", telemetryRecordCommand, base64.RawURLEncoding.EncodeToString(payload))
	command.Env = []string{}
	if err := command.Start(); err != nil {
		return fmt.Errorf("start telemetry recorder: %w", err)
	}
	go func() { _ = command.Wait() }()
	return nil
}

func runTelemetryRecord(encoded string, stderr io.Writer, dependencies dependencies) int {
	if len(encoded) > base64.RawURLEncoding.EncodedLen(maximumTelemetryHandoff) {
		report(stderr, "record telemetry: operation failed\n")
		return 2
	}
	payload, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil || len(payload) > maximumTelemetryHandoff {
		report(stderr, "record telemetry: operation failed\n")
		return 2
	}
	path, err := telemetryStorePath(dependencies)
	if err != nil {
		report(stderr, "record telemetry: operation failed\n")
		return 2
	}
	if err := (telemetrypkg.Store{Path: path}).AppendEncoded(payload); err != nil {
		report(stderr, "record telemetry: operation failed\n")
		return 2
	}
	return 0
}
