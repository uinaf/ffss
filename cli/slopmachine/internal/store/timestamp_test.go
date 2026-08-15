package store

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestTimestampFixedWidth pins the ordering invariant: RFC3339Nano trims
// trailing zeros, which made lexicographic ORDER BY disagree with time
// order whenever a timestamp ended in zeros.
func TestTimestampFixedWidth(t *testing.T) {
	stamp := timestampNow()
	if len(stamp) != len("2006-01-02T15:04:05.000000000Z") {
		t.Fatalf("timestamp must be fixed width: %q", stamp)
	}
	if _, err := time.Parse(time.RFC3339Nano, stamp); err != nil {
		t.Fatalf("timestamp must stay RFC3339-parseable: %v", err)
	}
	// The exact collision from CI: a trimmed-zeros stamp sorted after a
	// later, longer one. Fixed width keeps string order == time order.
	early := "2026-08-15T12:31:13.899000000Z"
	late := "2026-08-15T12:31:13.899263000Z"
	if !(strings.Compare(early, late) < 0) {
		t.Fatal("fixed-width stamps must order lexicographically by time")
	}
}

func TestOpenRejectsURIDelimiterPaths(t *testing.T) {
	for _, path := range []string{
		filepath.Join(t.TempDir(), "state?variant.sqlite"),
		filepath.Join(t.TempDir(), "state#frag.sqlite"),
	} {
		if _, err := Open(path); err == nil || !strings.Contains(err.Error(), "URI delimiter") {
			t.Fatalf("path %q must be rejected: %v", path, err)
		}
		if _, err := OpenReadOnly(path); err == nil || !strings.Contains(err.Error(), "URI delimiter") {
			t.Fatalf("read-only %q must be rejected: %v", path, err)
		}
	}
}
