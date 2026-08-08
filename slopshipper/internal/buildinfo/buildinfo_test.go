package buildinfo

import (
	"runtime/debug"
	"testing"
)

func TestFormat(t *testing.T) {
	if got := format("v1.2.3", "abc1234deadbeef"); got != "slopomatic v1.2.3 (abc1234deadbeef)" {
		t.Fatalf("got %q", got)
	}
	if got := format("v1.2.3", "unknown"); got != "slopomatic v1.2.3" {
		t.Fatalf("omit unknown commit: %q", got)
	}
	if got := format("dev", "unknown"); got != "slopomatic dev (unknown)" {
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
