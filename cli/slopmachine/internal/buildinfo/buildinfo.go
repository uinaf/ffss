package buildinfo

import (
	"fmt"
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
