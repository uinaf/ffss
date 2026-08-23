package provider

import (
	"context"
	"testing"
	"time"

	"github.com/uinaf/ffss/cli/slopguard/internal/config"
	"github.com/uinaf/ffss/cli/slopguard/internal/phase"
	"github.com/uinaf/ffss/cli/slopguard/internal/protocol"
)

func TestProviderPhaseCoverage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		setup func(*testing.T) (Reviewer, config.Effective)
	}{
		{
			name: "codex",
			setup: func(t *testing.T) (Reviewer, config.Effective) {
				fake := newFakeCodex(t, fakeCodexOptions{})
				return NewCodex(CodexOptions{Repository: t.TempDir(), Executable: fake.path, Environment: []string{"PATH=/usr/bin:/bin", "OPENAI_API_KEY=secret"}}), codexConfig(protocol.IsolationStrict, false, 5*time.Second)
			},
		},
		{
			name: "claude",
			setup: func(t *testing.T) (Reviewer, config.Effective) {
				fake := newFakeClaude(t, fakeClaudeOptions{})
				return NewClaude(ClaudeOptions{Repository: t.TempDir(), Executable: fake.path, Environment: []string{"PATH=/usr/bin:/bin", "ANTHROPIC_API_KEY=secret"}}), claudeConfig(protocol.IsolationStrict, false, 5*time.Second)
			},
		},
		{
			name: "cursor",
			setup: func(t *testing.T) (Reviewer, config.Effective) {
				fake := newFakeCursor(t, fakeCursorOptions{})
				return NewCursor(CursorOptions{Repository: t.TempDir(), Executable: fake.path, Environment: []string{"PATH=/usr/bin:/bin", "CURSOR_API_KEY=secret"}}), cursorConfig(protocol.IsolationStrict, true, 5*time.Second)
			},
		},
		{
			name: "grok",
			setup: func(t *testing.T) (Reviewer, config.Effective) {
				fake := newFakeGrok(t, fakeGrokOptions{})
				return NewGrok(GrokOptions{Repository: t.TempDir(), Executable: fake.path, Environment: []string{"PATH=/usr/bin:/bin", "XAI_API_KEY=secret"}}), grokConfig(protocol.IsolationStrict, false, 5*time.Second)
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			reviewer, effective := test.setup(t)
			clock := &providerSteppingClock{current: time.Unix(0, 0), step: 5 * time.Millisecond}
			var measurements []phase.Measurement
			recorder := phase.New(clock.Now, func(measurement phase.Measurement) {
				measurements = append(measurements, measurement)
			})
			ctx := phase.WithRecorder(context.Background(), recorder)
			if _, err := reviewer.Review(ctx, Request{Prompt: "frozen bundle", Config: effective}); err != nil {
				t.Fatal(err)
			}
			seen := make(map[phase.Name]bool, len(measurements))
			for _, measurement := range measurements {
				if measurement.Duration <= 0 {
					t.Fatalf("phase %s duration = %s", measurement.Name, measurement.Duration)
				}
				seen[measurement.Name] = true
			}
			for _, expected := range []phase.Name{phase.DependencyProbes, phase.ProviderPreparation, phase.ProviderProcess, phase.ProtocolDecode} {
				if !seen[expected] {
					t.Fatalf("phase %s missing from %+v", expected, measurements)
				}
			}
		})
	}
}

type providerSteppingClock struct {
	current time.Time
	step    time.Duration
}

func (clock *providerSteppingClock) Now() time.Time {
	current := clock.current
	clock.current = clock.current.Add(clock.step)
	return current
}
