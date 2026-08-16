package buildinfo

import (
	"fmt"
	"regexp"
	"runtime/debug"
	"strings"
)

// Set by GoReleaser / -ldflags at release time.
var (
	version = "dev"
	commit  = "unknown"
)

// Version returns the human-facing version line for `slopmachine version`.
func Version() string {
	build, ok := debug.ReadBuildInfo()
	v, c := resolve(version, commit, build, ok)
	return format(v, c)
}

// Release returns the bare release version (for example v1.0.1) when this
// binary is a published release, and "" otherwise; selfupdate refuses to
// replace non-release builds. Published member tags are exactly vX.Y.Z, so
// anything else — snapshots, pseudo-versions from go install, dirty builds —
// is not a release.
func Release() string {
	build, ok := debug.ReadBuildInfo()
	v, _ := resolve(version, commit, build, ok)
	if !releaseTagPattern.MatchString(v) {
		return ""
	}
	if ok && vcsModified(build) {
		return ""
	}
	return v
}

// vcsModified reports a build stamped from a dirty checkout; a release
// version string on modified sources is not a release.
func vcsModified(build *debug.BuildInfo) bool {
	for _, setting := range build.Settings {
		if setting.Key == "vcs.modified" {
			return setting.Value == "true"
		}
	}
	return false
}

var releaseTagPattern = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+$`)

func format(resolvedVersion, resolvedCommit string) string {
	if resolvedCommit == "unknown" && isReleaseVersion(resolvedVersion) {
		return fmt.Sprintf("slopmachine %s", resolvedVersion)
	}
	return fmt.Sprintf("slopmachine %s (%s)", resolvedVersion, resolvedCommit)
}

func resolve(linkedVersion, linkedCommit string, build *debug.BuildInfo, ok bool) (string, string) {
	if !ok || build == nil {
		return linkedVersion, linkedCommit
	}
	if linkedVersion == "dev" {
		if mv := build.Main.Version; mv != "" && mv != "(devel)" {
			linkedVersion = mv
		}
	}
	if linkedCommit == "unknown" {
		for _, setting := range build.Settings {
			if setting.Key == "vcs.revision" && setting.Value != "" {
				linkedCommit = setting.Value
				break
			}
		}
	}
	return linkedVersion, linkedCommit
}

func isReleaseVersion(value string) bool {
	if value == "" || value == "dev" || value == "(devel)" {
		return false
	}
	if strings.Contains(value, "-snapshot-") || strings.Contains(value, "-dirty") {
		return false
	}
	return strings.HasPrefix(value, "v") || (len(value) > 0 && value[0] >= '0' && value[0] <= '9')
}
