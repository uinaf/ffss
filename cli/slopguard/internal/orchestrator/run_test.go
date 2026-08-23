package orchestrator

import (
	"context"
	"testing"
	"time"

	"github.com/uinaf/ffss/cli/slopguard/internal/protocol"
)

func TestRunIncludesElapsedWorkBeforeOrchestration(t *testing.T) {
	t.Parallel()

	started := time.Unix(0, 0)
	report := Run(context.Background(), Options{
		Started: started,
		Now:     func() time.Time { return started.Add(25 * time.Millisecond) },
	})
	if report.Status != protocol.StatusFailure || report.Failure == nil || report.Failure.Class != protocol.FailureInternal {
		t.Fatalf("report = %+v", report)
	}
	if report.Metadata.DurationMS != 25 {
		t.Fatalf("duration_ms = %d", report.Metadata.DurationMS)
	}
}
