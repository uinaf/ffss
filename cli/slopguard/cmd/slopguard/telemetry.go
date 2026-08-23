package main

import (
	"io"

	telemetrypkg "github.com/uinaf/ffss/cli/slopguard/internal/telemetry"
)

func runTelemetry(arguments []string, stdout, stderr io.Writer, dependencies dependencies) int {
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
