package provider

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/uinaf/ffss/cli/slopguard/internal/config"
	"github.com/uinaf/ffss/cli/slopguard/internal/protocol"
)

func TestProviderPreparationIsCachedAcrossReviewAttempts(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name       string
		newFixture func(*testing.T) (Reviewer, string, config.Effective)
		probes     int
	}{
		{
			name: "Codex",
			newFixture: func(t *testing.T) (Reviewer, string, config.Effective) {
				fake := newFakeCodex(t, fakeCodexOptions{})
				return NewCodex(CodexOptions{Repository: t.TempDir(), Executable: fake.path, Environment: []string{"PATH=/usr/bin:/bin", "OPENAI_API_KEY=secret"}}), fake.probes, codexConfig(protocol.IsolationStrict, false, 5*time.Second)
			},
			probes: 3,
		},
		{
			name: "Claude",
			newFixture: func(t *testing.T) (Reviewer, string, config.Effective) {
				fake := newFakeClaude(t, fakeClaudeOptions{})
				return NewClaude(ClaudeOptions{Repository: t.TempDir(), Executable: fake.path, Environment: []string{"PATH=/usr/bin:/bin", "ANTHROPIC_API_KEY=secret"}}), fake.probes, claudeConfig(protocol.IsolationStrict, false, 5*time.Second)
			},
			probes: 2,
		},
		{
			name: "Cursor",
			newFixture: func(t *testing.T) (Reviewer, string, config.Effective) {
				fake := newFakeCursor(t, fakeCursorOptions{})
				return NewCursor(CursorOptions{Repository: t.TempDir(), Executable: fake.path, Environment: []string{"PATH=/usr/bin:/bin", "CURSOR_API_KEY=secret"}}), fake.probes, cursorConfig(protocol.IsolationStrict, true, 5*time.Second)
			},
			probes: 2,
		},
		{
			name: "Grok",
			newFixture: func(t *testing.T) (Reviewer, string, config.Effective) {
				fake := newFakeGrok(t, fakeGrokOptions{})
				return NewGrok(GrokOptions{Repository: t.TempDir(), Executable: fake.path, Environment: []string{"PATH=/usr/bin:/bin", "XAI_API_KEY=secret"}}), fake.probes, grokConfig(protocol.IsolationStrict, false, 5*time.Second)
			},
			probes: 2,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			reviewer, probePath, effective := test.newFixture(t)
			for attempt := 1; attempt <= 2; attempt++ {
				if _, err := reviewer.Review(context.Background(), Request{Prompt: "bundle", Config: effective}); err != nil {
					t.Fatalf("Review() attempt %d: %v", attempt, err)
				}
			}
			content, err := os.ReadFile(probePath)
			if err != nil {
				t.Fatal(err)
			}
			if got := len(strings.Fields(string(content))); got != test.probes {
				t.Fatalf("provider probes = %d, want %d: %s", got, test.probes, content)
			}
		})
	}
}

func TestCodexClaudeAndCursorSkipImplicitIncompatibleCandidate(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"codex", "claude", "cursor-agent"} {
		t.Run(name, func(t *testing.T) {
			shimDirectory := t.TempDir()
			manager := filepath.Join(t.TempDir(), "mise")
			writeTestExecutableAt(t, manager, "#!/bin/sh\nif [ \"${1:-}\" = '--version' ]; then printf '%s\\n' 'mise 2026.8.6'; exit 0; fi\nif [ \"${1:-}\" = '--help' ]; then printf '%s\\n' 'mise command help'; exit 0; fi\nexit 1\n")
			if err := os.Symlink(manager, filepath.Join(shimDirectory, name)); err != nil {
				t.Fatal(err)
			}

			switch name {
			case "codex":
				fake := newFakeCodex(t, fakeCodexOptions{})
				environment := []string{"PATH=" + strings.Join([]string{shimDirectory, filepath.Dir(fake.path), "/usr/bin", "/bin"}, string(os.PathListSeparator)), "OPENAI_API_KEY=secret"}
				reviewer := NewCodex(CodexOptions{Repository: t.TempDir(), Environment: environment})
				if _, err := reviewer.Review(context.Background(), Request{Prompt: "bundle", Config: codexConfig(protocol.IsolationStrict, false, 5*time.Second)}); err != nil {
					t.Fatal(err)
				}
			case "claude":
				fake := newFakeClaude(t, fakeClaudeOptions{})
				environment := []string{"PATH=" + strings.Join([]string{shimDirectory, filepath.Dir(fake.path), "/usr/bin", "/bin"}, string(os.PathListSeparator)), "ANTHROPIC_API_KEY=secret"}
				reviewer := NewClaude(ClaudeOptions{Repository: t.TempDir(), Environment: environment})
				if _, err := reviewer.Review(context.Background(), Request{Prompt: "bundle", Config: claudeConfig(protocol.IsolationStrict, false, 5*time.Second)}); err != nil {
					t.Fatal(err)
				}
			case "cursor-agent":
				fake := newFakeCursor(t, fakeCursorOptions{})
				environment := []string{"PATH=" + strings.Join([]string{shimDirectory, filepath.Dir(fake.path), "/usr/bin", "/bin"}, string(os.PathListSeparator)), "CURSOR_API_KEY=secret"}
				reviewer := NewCursor(CursorOptions{Repository: t.TempDir(), Environment: environment})
				if _, err := reviewer.Review(context.Background(), Request{Prompt: "bundle", Config: cursorConfig(protocol.IsolationStrict, true, 5*time.Second)}); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestExplicitProviderExecutableDoesNotFallback(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"codex", "claude", "cursor-agent", "grok"} {
		t.Run(name, func(t *testing.T) {
			manager := filepath.Join(t.TempDir(), "mise")
			writeTestExecutableAt(t, manager, "#!/bin/sh\nif [ \"${1:-}\" = '--version' ]; then printf '%s\\n' 'mise 2026.8.6'; exit 0; fi\nif [ \"${1:-}\" = '--help' ]; then printf '%s\\n' 'mise command help'; exit 0; fi\nexit 1\n")

			var err error
			var arguments string
			switch name {
			case "codex":
				fake := newFakeCodex(t, fakeCodexOptions{})
				arguments = fake.arguments
				reviewer := NewCodex(CodexOptions{Repository: t.TempDir(), Executable: manager, Environment: []string{"PATH=" + strings.Join([]string{filepath.Dir(fake.path), "/usr/bin", "/bin"}, string(os.PathListSeparator)), "OPENAI_API_KEY=secret"}})
				_, err = reviewer.Review(context.Background(), Request{Prompt: "bundle", Config: codexConfig(protocol.IsolationStrict, false, 5*time.Second)})
			case "claude":
				fake := newFakeClaude(t, fakeClaudeOptions{})
				arguments = fake.arguments
				reviewer := NewClaude(ClaudeOptions{Repository: t.TempDir(), Executable: manager, Environment: []string{"PATH=" + strings.Join([]string{filepath.Dir(fake.path), "/usr/bin", "/bin"}, string(os.PathListSeparator)), "ANTHROPIC_API_KEY=secret"}})
				_, err = reviewer.Review(context.Background(), Request{Prompt: "bundle", Config: claudeConfig(protocol.IsolationStrict, false, 5*time.Second)})
			case "cursor-agent":
				fake := newFakeCursor(t, fakeCursorOptions{})
				arguments = fake.arguments
				reviewer := NewCursor(CursorOptions{Repository: t.TempDir(), Executable: manager, Environment: []string{"PATH=" + strings.Join([]string{filepath.Dir(fake.path), "/usr/bin", "/bin"}, string(os.PathListSeparator)), "CURSOR_API_KEY=secret"}})
				_, err = reviewer.Review(context.Background(), Request{Prompt: "bundle", Config: cursorConfig(protocol.IsolationStrict, true, 5*time.Second)})
			case "grok":
				fake := newFakeGrok(t, fakeGrokOptions{})
				arguments = fake.arguments
				reviewer := NewGrok(GrokOptions{Repository: t.TempDir(), Executable: manager, Environment: []string{"PATH=" + strings.Join([]string{filepath.Dir(fake.path), "/usr/bin", "/bin"}, string(os.PathListSeparator)), "XAI_API_KEY=secret"}})
				_, err = reviewer.Review(context.Background(), Request{Prompt: "bundle", Config: grokConfig(protocol.IsolationStrict, false, 5*time.Second)})
			}
			_ = assertProviderError(t, err, protocol.FailureCapability)
			if _, statErr := os.Stat(arguments); !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("compatible PATH candidate was invoked after explicit failure: %v", statErr)
			}
		})
	}
}
