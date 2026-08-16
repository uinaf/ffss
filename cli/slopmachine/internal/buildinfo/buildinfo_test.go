package buildinfo

import (
	"runtime/debug"
	"strings"
	"testing"
)

func TestFormat(t *testing.T) {
	if got := format("v1.2.3", "abc1234deadbeef"); got != "slopmachine v1.2.3 (abc1234deadbeef)" {
		t.Fatalf("got %q", got)
	}
	if got := format("v1.2.3", "unknown"); got != "slopmachine v1.2.3" {
		t.Fatalf("omit unknown commit: %q", got)
	}
	if got := format("dev", "unknown"); got != "slopmachine dev (unknown)" {
		t.Fatalf("dev keeps unknown: %q", got)
	}
}

func TestResolveBackfillsFromBuildInfo(t *testing.T) {
	v, c := resolve("dev", "unknown", &debug.BuildInfo{
		Main: debug.Module{Version: "v9.9.9"},
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "deadbeef"},
		},
	}, true)
	if v != "v9.9.9" || c != "deadbeef" {
		t.Fatalf("got %s %s", v, c)
	}
}

func TestResolveKeepsLinkedReleaseFields(t *testing.T) {
	v, c := resolve("v1.2.3", "abc", &debug.BuildInfo{
		Main: debug.Module{Version: "v9.9.9"},
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "deadbeef"},
		},
	}, true)
	if v != "v1.2.3" || c != "abc" {
		t.Fatalf("got %s %s", v, c)
	}
}

func TestVersionAndReleaseClassification(t *testing.T) {
	if got := Version(); !strings.HasPrefix(got, "slopmachine ") {
		t.Fatalf("Version=%q", got)
	}
	tests := map[string]bool{
		"": false, "dev": false, "(devel)": false,
		"v1.2.3": true, "1.2.3": true, "release": false,
		"v1.2.3-snapshot-1": false, "v1.2.3-dirty": false,
	}
	for value, want := range tests {
		if got := isReleaseVersion(value); got != want {
			t.Errorf("isReleaseVersion(%q)=%v want %v", value, got, want)
		}
	}
}

func TestResolveWithoutBuildInfoAndEmptySettings(t *testing.T) {
	if v, c := resolve("dev", "unknown", nil, false); v != "dev" || c != "unknown" {
		t.Fatalf("no build info: %s %s", v, c)
	}
	v, c := resolve("dev", "unknown", &debug.BuildInfo{
		Main:     debug.Module{Version: "(devel)"},
		Settings: []debug.BuildSetting{{Key: "vcs.revision", Value: ""}},
	}, true)
	if v != "dev" || c != "unknown" {
		t.Fatalf("empty settings: %s %s", v, c)
	}
}

func TestVCSModifiedRefusesDirtyRelease(t *testing.T) {
	dirty := &debug.BuildInfo{Settings: []debug.BuildSetting{{Key: "vcs.modified", Value: "true"}}}
	if !vcsModified(dirty) {
		t.Fatal("a dirty checkout must be detected")
	}
	clean := &debug.BuildInfo{Settings: []debug.BuildSetting{{Key: "vcs.modified", Value: "false"}}}
	if vcsModified(clean) {
		t.Fatal("a clean checkout is not dirty")
	}
	if vcsModified(&debug.BuildInfo{}) {
		t.Fatal("absent vcs metadata (release builds with -trimpath) is not dirty")
	}
}

func TestReleaseOnDevBinaryIsEmpty(t *testing.T) {
	// The test binary is never a release build.
	if got := Release(); got != "" {
		t.Fatalf("test binary must not classify as a release: %q", got)
	}
}
