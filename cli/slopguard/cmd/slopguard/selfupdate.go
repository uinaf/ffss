package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/uinaf/ffsstack/cli/slopguard/internal/buildinfo"
	"github.com/uinaf/ffsstack/cli/slopguard/internal/selfupdate"
)

// runSelfupdate replaces this binary with a published release. It shares
// the installer's endpoint overrides (SLOPGUARD_INSTALL_API_URL,
// SLOPGUARD_INSTALL_REPOSITORY_URL) so tests and mirrors configure one
// knob for both surfaces.
func runSelfupdate(ctx context.Context, arguments []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("slopguard selfupdate", flag.ContinueOnError)
	check := flags.Bool("check", false, "report the target release without touching the binary; exits 5 when an update is available")
	release := flags.String("release", "", "pin the target release (vX.Y.Z); default is the newest published release")
	if err := parseFlags(flags, arguments, stdout, stderr); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if flags.NArg() != 0 {
		report(stderr, "slopguard selfupdate does not accept positional arguments\n")
		return 2
	}
	options := selfupdate.Options{
		Member:         "slopguard",
		CurrentVersion: buildinfo.Release(),
		RequestVersion: *release,
		APIBase:        os.Getenv("SLOPGUARD_INSTALL_API_URL"),
		DownloadBase:   os.Getenv("SLOPGUARD_INSTALL_REPOSITORY_URL"),
	}
	var result selfupdate.Result
	var err error
	if *check {
		result, err = selfupdate.Check(ctx, options)
	} else {
		result, err = selfupdate.Run(ctx, options)
	}
	if err != nil {
		report(stderr, "slopguard selfupdate: %v\n", err)
		if errors.Is(err, selfupdate.ErrNotRelease) || errors.Is(err, selfupdate.ErrBrewManaged) || errors.Is(err, selfupdate.ErrInvalidVersion) {
			return 2
		}
		return 3
	}
	available := result.To != result.From
	switch {
	case result.Updated:
		fmt.Fprintf(stdout, "updated slopguard %s -> %s (%s)\n", result.From, result.To, result.Path)
	case available:
		fmt.Fprintf(stdout, "slopguard %s available (current %s); run slopguard selfupdate\n", result.To, result.From)
	default:
		fmt.Fprintf(stdout, "slopguard %s is up to date\n", result.From)
	}
	if *check && available {
		return 5
	}
	return 0
}
