package provider

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/uinaf/ffss/cli/slopguard/internal/config"
	"github.com/uinaf/ffss/cli/slopguard/internal/protocol"
)

func TestProviderPreparationIsCachedAcrossReviewAttempts(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name       string
		newFixture func(*testing.T) (Reviewer, string, string, config.Effective)
		probes     int
	}{
		{
			name: "Codex",
			newFixture: func(t *testing.T) (Reviewer, string, string, config.Effective) {
				fake := newFakeCodex(t, fakeCodexOptions{})
				return NewCodex(CodexOptions{Repository: t.TempDir(), Executable: fake.path, Environment: []string{"PATH=/usr/bin:/bin", "OPENAI_API_KEY=secret"}}), fake.probes, fake.directory, codexConfig(protocol.IsolationStrict, false, 5*time.Second)
			},
			probes: 3,
		},
		{
			name: "Claude",
			newFixture: func(t *testing.T) (Reviewer, string, string, config.Effective) {
				fake := newFakeClaude(t, fakeClaudeOptions{})
				return NewClaude(ClaudeOptions{Repository: t.TempDir(), Executable: fake.path, Environment: []string{"PATH=/usr/bin:/bin", "ANTHROPIC_API_KEY=secret"}}), fake.probes, fake.directory, claudeConfig(protocol.IsolationStrict, false, 5*time.Second)
			},
			probes: 2,
		},
		{
			name: "Cursor",
			newFixture: func(t *testing.T) (Reviewer, string, string, config.Effective) {
				fake := newFakeCursor(t, fakeCursorOptions{})
				return NewCursor(CursorOptions{Repository: t.TempDir(), Executable: fake.path, Environment: []string{"PATH=/usr/bin:/bin", "CURSOR_API_KEY=secret"}}), fake.probes, fake.directory, cursorConfig(protocol.IsolationStrict, true, 5*time.Second)
			},
			probes: 2,
		},
		{
			name: "Grok",
			newFixture: func(t *testing.T) (Reviewer, string, string, config.Effective) {
				fake := newFakeGrok(t, fakeGrokOptions{})
				return NewGrok(GrokOptions{Repository: t.TempDir(), Executable: fake.path, Environment: []string{"PATH=/usr/bin:/bin", "XAI_API_KEY=secret"}}), fake.probes, fake.directory, grokConfig(protocol.IsolationStrict, false, 5*time.Second)
			},
			probes: 2,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			reviewer, probePath, directoryPath, effective := test.newFixture(t)
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
			workspaces := strings.Fields(readTestFile(t, directoryPath))
			if len(workspaces) != 2 || workspaces[0] == workspaces[1] {
				t.Fatalf("provider workspaces = %v, want 2 distinct attempts", workspaces)
			}
		})
	}
}

func TestPreparationCacheCoalescesMissesAndDoesNotCacheFailures(t *testing.T) {
	t.Parallel()

	cache := &preparationCache{}
	key := preparationKey{Isolation: protocol.IsolationStrict}
	var calls atomic.Int32
	var wait sync.WaitGroup
	results := make(chan preparedExecutable, 2)
	for range 2 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			prepared, err := cache.resolve(context.Background(), key, func() (preparedExecutable, error) {
				calls.Add(1)
				time.Sleep(25 * time.Millisecond)
				return preparedExecutable{Path: "/provider", Version: "1.0.0"}, nil
			})
			if err != nil {
				t.Errorf("resolve() error = %v", err)
				return
			}
			results <- prepared
		}()
	}
	wait.Wait()
	close(results)
	if calls.Load() != 1 {
		t.Fatalf("concurrent preparation calls = %d, want 1", calls.Load())
	}
	for prepared := range results {
		if prepared.Path != "/provider" {
			t.Fatalf("prepared = %+v", prepared)
		}
	}

	failureKey := preparationKey{Isolation: protocol.IsolationNative}
	failedCalls := 0
	if _, err := cache.resolve(context.Background(), failureKey, func() (preparedExecutable, error) {
		failedCalls++
		return preparedExecutable{}, errors.New("probe failed")
	}); err == nil {
		t.Fatal("failed preparation was accepted")
	}
	if _, err := cache.resolve(context.Background(), failureKey, func() (preparedExecutable, error) {
		failedCalls++
		return preparedExecutable{Path: "/native-provider", Version: "1.0.0"}, nil
	}); err != nil {
		t.Fatal(err)
	}
	if failedCalls != 2 {
		t.Fatalf("failure preparation calls = %d, want 2", failedCalls)
	}

	policyCalls := 0
	webKey := preparationKey{Isolation: protocol.IsolationNative, WebAccess: true}
	if _, err := cache.resolve(context.Background(), webKey, func() (preparedExecutable, error) {
		policyCalls++
		return preparedExecutable{Path: "/web-provider", Version: "1.0.0"}, nil
	}); err != nil {
		t.Fatal(err)
	}
	if policyCalls != 1 {
		t.Fatalf("policy preparation calls = %d, want 1", policyCalls)
	}
}

func TestCompatibleSelectionPreservesCandidateOrderAndStopsOnNonCapabilityFailure(t *testing.T) {
	t.Parallel()

	var order []string
	prepared, err := selectCompatibleExecutable([]string{"first", "second"}, func(candidate string) (string, error) {
		order = append(order, candidate)
		if candidate == "first" {
			return "", &Error{Class: protocol.FailureCapability, Message: "incompatible"}
		}
		return "2.0.0", nil
	})
	if err != nil || prepared.Path != "second" || strings.Join(order, ",") != "first,second" {
		t.Fatalf("selection = %+v, order = %v, error = %v", prepared, order, err)
	}

	order = nil
	_, err = selectCompatibleExecutable([]string{"first", "second"}, func(candidate string) (string, error) {
		order = append(order, candidate)
		return "", &Error{Class: protocol.FailureAuth, Message: "authentication failed"}
	})
	if err == nil || strings.Join(order, ",") != "first" {
		t.Fatalf("non-capability selection order = %v, error = %v", order, err)
	}
}

func TestPreparationCacheScopesInflightCallsByPolicyAndHonorsWaiterContext(t *testing.T) {
	t.Parallel()

	cache := &preparationCache{}
	slowKey := preparationKey{Isolation: protocol.IsolationStrict}
	started := make(chan struct{})
	release := make(chan struct{})
	finished := make(chan error, 1)
	go func() {
		_, err := cache.resolve(context.Background(), slowKey, func() (preparedExecutable, error) {
			close(started)
			<-release
			return preparedExecutable{Path: "/strict", Version: "1.0.0"}, nil
		})
		finished <- err
	}()
	<-started

	otherKey := preparationKey{Isolation: protocol.IsolationNative}
	if _, err := cache.resolve(context.Background(), otherKey, func() (preparedExecutable, error) {
		return preparedExecutable{Path: "/native", Version: "1.0.0"}, nil
	}); err != nil {
		t.Fatalf("independent policy preparation blocked: %v", err)
	}

	waitContext, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := cache.resolve(waitContext, slowKey, func() (preparedExecutable, error) {
		t.Fatal("waiter started duplicate preparation")
		return preparedExecutable{}, nil
	}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("waiter error = %v, want context deadline", err)
	}

	close(release)
	if err := <-finished; err != nil {
		t.Fatal(err)
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

func TestImplicitCandidateStopsAfterProviderProbeFailure(t *testing.T) {
	t.Parallel()

	shimDirectory := t.TempDir()
	provider := filepath.Join(shimDirectory, "codex")
	writeTestExecutableAt(t, provider, "#!/bin/sh\nprintf '%s\\n' 'rate limited' >&2\nexit 7\n")
	fake := newFakeCodex(t, fakeCodexOptions{})
	environment := []string{
		"PATH=" + strings.Join([]string{shimDirectory, filepath.Dir(fake.path), "/usr/bin", "/bin"}, string(os.PathListSeparator)),
		"OPENAI_API_KEY=secret",
	}
	reviewer := NewCodex(CodexOptions{Repository: t.TempDir(), Environment: environment})
	_, err := reviewer.Review(context.Background(), Request{Prompt: "bundle", Config: codexConfig(protocol.IsolationStrict, false, 5*time.Second)})
	_ = assertProviderError(t, err, protocol.FailureProvider)
	if _, statErr := os.Stat(fake.arguments); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("compatible candidate was invoked after provider probe failure: %v", statErr)
	}
}
