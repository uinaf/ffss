package serve

import (
	"testing"

	"github.com/uinaf/ffss/cli/slopmachine/internal/store"
)

func TestFormatTotals(t *testing.T) {
	if got := formatTotals(store.Totals{}); got != "" {
		t.Fatalf("no recorded telemetry renders nothing: %q", got)
	}
	got := formatTotals(store.Totals{RecordedEvents: 3, DurationMS: 2549, Tokens: 4500, CostCents: 123})
	want := "3 event(s) · 2.549s · 4500 tokens · $1.23"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if got := formatTotals(store.Totals{RecordedEvents: 1, DurationMS: 1}); got != "1 event(s) · 1ms" {
		t.Fatalf("sub-second totals must stay exact: %q", got)
	}
	if got := formatTotals(store.Totals{RecordedEvents: 1, DurationMS: 2500}); got != "1 event(s) · 2.5s" {
		t.Fatalf("trailing zeros trim: %q", got)
	}
	if got := formatTotals(store.Totals{RecordedEvents: 1, Tokens: 10}); got != "1 event(s) · 10 tokens" {
		t.Fatalf("zero dimensions must be omitted: %q", got)
	}
}
