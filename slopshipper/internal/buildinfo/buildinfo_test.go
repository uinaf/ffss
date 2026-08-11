package buildinfo

import (
	"runtime/debug"
	"strings"
	"testing"
)

func TestFormat(t *testing.T) {
	if got := format("v1.2.3", "abc1234deadbeef"); got != "slopshipper v1.2.3 (abc1234deadbeef)" {
		t.Fatalf("got %q", got)
	}
	if got := format("v1.2.3", "unknown"); got != "slopshipper v1.2.3" {
		t.Fatalf("omit unknown commit: %q", got)
	}
	if got := format("dev", "unknown"); got != "slopshipper dev (unknown)" {
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
	if got := Version(); !strings.HasPrefix(got, "slopshipper ") {
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
