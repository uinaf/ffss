package buildinfo

import (
	"fmt"
	"regexp"
	"runtime/debug"

	"golang.org/x/mod/module"
	"golang.org/x/mod/semver"
)

var (
	version = "dev"
	commit  = "unknown"
)

func Version() string {
	build, ok := debug.ReadBuildInfo()
	resolvedVersion, resolvedCommit := resolve(version, commit, build, ok)
	return format(resolvedVersion, resolvedCommit)
}

// Release returns the bare release version (for example v1.0.1) when this
// binary is a published release, and "" otherwise; selfupdate refuses to
// replace non-release builds. Published member tags are exactly vX.Y.Z, so
// anything else — snapshots, pseudo-versions from go install, pre-releases —
// is not a release.
func Release() string {
	build, ok := debug.ReadBuildInfo()
	resolvedVersion, _ := resolve(version, commit, build, ok)
	if !releaseTagPattern.MatchString(resolvedVersion) {
		return ""
	}
	if ok && vcsModified(build) {
		return ""
	}
	return resolvedVersion
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
	if resolvedCommit == "unknown" && isReleaseModuleVersion(resolvedVersion) {
		return fmt.Sprintf("slopguard %s", resolvedVersion)
	}
	return fmt.Sprintf("slopguard %s (%s)", resolvedVersion, resolvedCommit)
}

func resolve(linkedVersion, linkedCommit string, build *debug.BuildInfo, ok bool) (string, string) {
	if !ok || build == nil {
		return linkedVersion, linkedCommit
	}
	if linkedVersion == "dev" && isReleaseModuleVersion(build.Main.Version) {
		linkedVersion = build.Main.Version
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

func isReleaseModuleVersion(value string) bool {
	return semver.IsValid(value) && !module.IsPseudoVersion(value)
}
