package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/uinaf/ffsstack/cli/slopmachine/internal/buildinfo"
	"github.com/uinaf/ffsstack/cli/slopmachine/internal/selfupdate"
)

// cmdSelfupdate replaces this binary with a published release. It shares
// the installer's endpoint overrides (SLOPMACHINE_INSTALL_API_URL,
// SLOPMACHINE_INSTALL_REPOSITORY_URL) so tests and mirrors configure one
// knob for both surfaces.
func cmdSelfupdate(args []string, opts runOptions) int {
	fs, code := requireFlags("selfupdate", args, opts)
	if code != 0 {
		return code
	}
	_, check := fs["check"]
	options := selfupdate.Options{
		Member:         "slopmachine",
		CurrentVersion: buildinfo.Release(),
		RequestVersion: fs["release"],
		APIBase:        os.Getenv("SLOPMACHINE_INSTALL_API_URL"),
		DownloadBase:   os.Getenv("SLOPMACHINE_INSTALL_REPOSITORY_URL"),
	}
	var result selfupdate.Result
	var err error
	if check {
		result, err = selfupdate.Check(options)
	} else {
		result, err = selfupdate.Run(options)
	}
	if err != nil {
		return writeFailure(opts, selfupdateErrorExit(err), err)
	}
	return writeSelfupdateResult(result, check, opts)
}

// selfupdateErrorExit maps refusals (wrong install kind) to invalid-input
// and everything else (resolution, download, verification) to unmet guard.
func selfupdateErrorExit(err error) int {
	if errors.Is(err, selfupdate.ErrNotRelease) || errors.Is(err, selfupdate.ErrBrewManaged) {
		return 2
	}
	return 3
}

// writeSelfupdateResult renders one pass; --check exits 4 when an update is
// available so automation can branch without parsing output.
func writeSelfupdateResult(result selfupdate.Result, check bool, opts runOptions) int {
	available := result.To != result.From
	if opts.json {
		if err := writeJSON(map[string]any{
			"schema_version":   1,
			"current":          result.From,
			"target":           result.To,
			"update_available": available,
			"updated":          result.Updated,
			"path":             result.Path,
		}); err != nil {
			return writeFailure(opts, 10, err)
		}
	} else {
		switch {
		case result.Updated:
			fmt.Printf("updated slopmachine %s -> %s (%s)\n", result.From, result.To, result.Path)
		case available:
			fmt.Printf("slopmachine %s available (current %s); run slopmachine selfupdate\n", result.To, result.From)
		default:
			fmt.Printf("slopmachine %s is up to date\n", result.From)
		}
	}
	if check && available {
		return 4
	}
	return 0
}
