package provider

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/uinaf/ffss/cli/slopguard/internal/config"
	"github.com/uinaf/ffss/cli/slopguard/internal/protocol"
)

func TestDoctorUsesProviderPreflightWithoutModelCalls(t *testing.T) {
	tests := []struct {
		name     string
		fixture  func(*testing.T) (string, string, string, config.Effective, []string, []string)
		probes   int
		provider protocol.ProviderName
	}{
		{
			name: "codex", provider: protocol.ProviderCodex, probes: 3,
			fixture: func(t *testing.T) (string, string, string, config.Effective, []string, []string) {
				fake := newFakeCodex(t, fakeCodexOptions{})
				return fake.path, fake.arguments, fake.probes, codexConfig(protocol.IsolationStrict, false, 5*time.Second), []string{"PATH=/usr/bin:/bin", "OPENAI_API_KEY=secret"}, []string{"PATH=/usr/bin:/bin", "HOME=/native/home", "CODEX_HOME=/native/codex"}
			},
		},
		{
			name: "claude", provider: protocol.ProviderClaude, probes: 2,
			fixture: func(t *testing.T) (string, string, string, config.Effective, []string, []string) {
				fake := newFakeClaude(t, fakeClaudeOptions{})
				return fake.path, fake.arguments, fake.probes, claudeConfig(protocol.IsolationStrict, false, 5*time.Second), []string{"PATH=/usr/bin:/bin", "ANTHROPIC_API_KEY=secret"}, []string{"PATH=/usr/bin:/bin", "HOME=/native/home", "CLAUDE_CONFIG_DIR=/native/claude", "ANTHROPIC_AUTH_TOKEN=test-helper", "ANTHROPIC_BASE_URL=https://gateway.invalid"}
			},
		},
		{
			name: "cursor", provider: protocol.ProviderCursor, probes: 2,
			fixture: func(t *testing.T) (string, string, string, config.Effective, []string, []string) {
				fake := newFakeCursor(t, fakeCursorOptions{})
				return fake.path, fake.arguments, fake.probes, cursorConfig(protocol.IsolationStrict, true, 5*time.Second), []string{"PATH=/usr/bin:/bin", "CURSOR_API_KEY=secret"}, []string{"PATH=/usr/bin:/bin", "HOME=/native/home", "CURSOR_CONFIG_DIR=/native/cursor"}
			},
		},
		{
			name: "grok", provider: protocol.ProviderGrok, probes: 2,
			fixture: func(t *testing.T) (string, string, string, config.Effective, []string, []string) {
				fake := newFakeGrok(t, fakeGrokOptions{})
				return fake.path, fake.arguments, fake.probes, grokConfig(protocol.IsolationStrict, false, 5*time.Second), []string{"PATH=/usr/bin:/bin", "XAI_API_KEY=secret"}, []string{"PATH=/usr/bin:/bin", "HOME=/native/home", "GROK_HOME=/native/grok", "XAI_BASE_URL=https://gateway.invalid"}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			executable, arguments, probes, effective, environment, nativeEnvironment := test.fixture(t)
			diagnostic := Doctor(t.Context(), DoctorOptions{
				Repository: t.TempDir(), Executable: executable, Environment: environment, Config: effective,
			})
			if err := diagnostic.Validate(); err != nil {
				t.Fatal(err)
			}
			if diagnostic.Status != DoctorReady || diagnostic.Provider != test.provider || !diagnostic.Compatible || diagnostic.Version == "" || diagnostic.Authentication != AuthenticationReady {
				t.Fatalf("diagnostic = %+v", diagnostic)
			}
			if _, err := os.Stat(arguments); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("model invocation was attempted: %v", err)
			}
			content, err := os.ReadFile(probes)
			if err != nil || len(strings.Fields(string(content))) != test.probes {
				t.Fatalf("probes=%q error=%v", content, err)
			}
			native := effective
			native.Isolation = config.Value[protocol.Isolation]{Value: protocol.IsolationNative, Source: config.SourceDefault}
			nativeExecutable := doctorProbeWrapper(t, executable, nativeEnvironment[1:], "")
			nativeDiagnostic := Doctor(t.Context(), DoctorOptions{
				Repository: t.TempDir(), Executable: nativeExecutable, Environment: nativeEnvironment, Config: native,
			})
			if nativeDiagnostic.Status != DoctorReady || nativeDiagnostic.Authentication != AuthenticationDelegated {
				t.Fatalf("native diagnostic = %+v", nativeDiagnostic)
			}
			if _, err := os.Stat(arguments); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("native doctor invoked model: %v", err)
			}
		})
	}
}

func TestDoctorReportsStrictMissingTimeoutAndCancellationForEveryProvider(t *testing.T) {
	providers := []struct {
		name    string
		fixture func(*testing.T, protocol.Isolation, time.Duration) (string, config.Effective)
	}{
		{name: "codex", fixture: func(t *testing.T, isolation protocol.Isolation, timeout time.Duration) (string, config.Effective) {
			return newFakeCodex(t, fakeCodexOptions{}).path, codexConfig(isolation, false, timeout)
		}},
		{name: "claude", fixture: func(t *testing.T, isolation protocol.Isolation, timeout time.Duration) (string, config.Effective) {
			return newFakeClaude(t, fakeClaudeOptions{}).path, claudeConfig(isolation, false, timeout)
		}},
		{name: "cursor", fixture: func(t *testing.T, isolation protocol.Isolation, timeout time.Duration) (string, config.Effective) {
			return newFakeCursor(t, fakeCursorOptions{}).path, cursorConfig(isolation, true, timeout)
		}},
		{name: "grok", fixture: func(t *testing.T, isolation protocol.Isolation, timeout time.Duration) (string, config.Effective) {
			return newFakeGrok(t, fakeGrokOptions{}).path, grokConfig(isolation, false, timeout)
		}},
	}
	for _, test := range providers {
		t.Run(test.name, func(t *testing.T) {
			strictExecutable, strictConfig := test.fixture(t, protocol.IsolationStrict, 5*time.Second)
			strict := Doctor(t.Context(), DoctorOptions{
				Repository: t.TempDir(), Executable: strictExecutable, Environment: []string{"PATH=/usr/bin:/bin"}, Config: strictConfig,
			})
			if strict.Status != DoctorNotReady || !strict.Compatible || strict.Authentication != AuthenticationMissing || strict.FailureClass != protocol.FailureAuth {
				t.Fatalf("strict diagnostic = %+v", strict)
			}

			timeoutExecutable, timeoutConfig := test.fixture(t, protocol.IsolationNative, 25*time.Millisecond)
			timedOut := Doctor(t.Context(), DoctorOptions{
				Repository: t.TempDir(), Executable: doctorProbeWrapper(t, timeoutExecutable, nil, "0.2"), Environment: []string{"PATH=/usr/bin:/bin"}, Config: timeoutConfig,
			})
			if timedOut.FailureClass != protocol.FailureTimeout {
				t.Fatalf("timeout diagnostic = %+v", timedOut)
			}

			cancelExecutable, cancelConfig := test.fixture(t, protocol.IsolationNative, 5*time.Second)
			cancelledContext, cancel := context.WithCancel(t.Context())
			cancel()
			cancelled := Doctor(cancelledContext, DoctorOptions{
				Repository: t.TempDir(), Executable: cancelExecutable, Environment: []string{"PATH=/usr/bin:/bin"}, Config: cancelConfig,
			})
			if cancelled.FailureClass != protocol.FailureCancelled {
				t.Fatalf("cancelled diagnostic = %+v", cancelled)
			}
		})
	}
}

func TestDoctorClassifiesMissingIncompatibleTimeoutAndCancellation(t *testing.T) {
	base := codexConfig(protocol.IsolationNative, false, 5*time.Second)
	providers := []struct {
		name      string
		effective config.Effective
	}{
		{name: "codex", effective: base},
		{name: "claude", effective: claudeConfig(protocol.IsolationNative, false, 5*time.Second)},
		{name: "cursor", effective: cursorConfig(protocol.IsolationNative, true, 5*time.Second)},
		{name: "grok", effective: grokConfig(protocol.IsolationNative, false, 5*time.Second)},
	}
	for _, test := range providers {
		t.Run(test.name, func(t *testing.T) {
			missing := Doctor(t.Context(), DoctorOptions{Repository: t.TempDir(), Executable: "missing-doctor-provider", Environment: []string{"PATH=/usr/bin:/bin"}, Config: test.effective})
			if missing.FailureClass != protocol.FailureCapability || missing.Compatible || strings.Contains(missing.Message, "missing-doctor-provider") {
				t.Fatalf("missing diagnostic = %+v", missing)
			}
			incompatible := filepath.Join(t.TempDir(), "provider")
			writeTestExecutableAt(t, incompatible, "#!/bin/sh\nprintf 'mise 2026.8.6\\n'\n")
			wrong := Doctor(t.Context(), DoctorOptions{Repository: t.TempDir(), Executable: incompatible, Environment: []string{"PATH=/usr/bin:/bin"}, Config: test.effective})
			if wrong.FailureClass != protocol.FailureCapability || wrong.Compatible {
				t.Fatalf("incompatible diagnostic = %+v", wrong)
			}
		})
	}
	slow := newFakeCodex(t, fakeCodexOptions{probeDelay: "2"})
	timeoutConfig := codexConfig(protocol.IsolationNative, false, 25*time.Millisecond)
	timedOut := Doctor(t.Context(), DoctorOptions{Repository: t.TempDir(), Executable: slow.path, Environment: []string{"PATH=/usr/bin:/bin"}, Config: timeoutConfig})
	if timedOut.FailureClass != protocol.FailureTimeout {
		t.Fatalf("timeout diagnostic = %+v", timedOut)
	}
	cancelledContext, cancel := context.WithCancel(t.Context())
	cancel()
	cancelled := Doctor(cancelledContext, DoctorOptions{Repository: t.TempDir(), Executable: slow.path, Environment: []string{"PATH=/usr/bin:/bin"}, Config: base})
	if cancelled.FailureClass != protocol.FailureCancelled {
		t.Fatalf("cancelled diagnostic = %+v", cancelled)
	}
	authProse := filepath.Join(t.TempDir(), "provider")
	writeTestExecutableAt(t, authProse, "#!/bin/sh\nprintf '%s\\n' 'not authenticated: private provider detail' >&2\nexit 1\n")
	proseDiagnostic := Doctor(t.Context(), DoctorOptions{Repository: t.TempDir(), Executable: authProse, Environment: []string{"PATH=/usr/bin:/bin"}, Config: base})
	if proseDiagnostic.FailureClass != protocol.FailureCapability || proseDiagnostic.Authentication != AuthenticationDelegated || strings.Contains(proseDiagnostic.Message, "private") {
		t.Fatalf("auth prose diagnostic = %+v", proseDiagnostic)
	}
}

func TestDoctorUsesSingleTimeoutBudget(t *testing.T) {
	slow := newFakeCodex(t, fakeCodexOptions{probeDelay: "0.06"})
	diagnostic := Doctor(t.Context(), DoctorOptions{
		Repository: t.TempDir(), Executable: slow.path, Environment: []string{"PATH=/usr/bin:/bin"},
		Config: codexConfig(protocol.IsolationNative, false, 100*time.Millisecond),
	})
	if diagnostic.FailureClass != protocol.FailureTimeout {
		t.Fatalf("diagnostic = %+v", diagnostic)
	}
}

func TestDoctorRejectsCanonicalCursorVersionContainingCredential(t *testing.T) {
	fake := newFakeCursor(t, fakeCursorOptions{version: "2026.08.23-deadbee"})
	diagnostic := Doctor(t.Context(), DoctorOptions{
		Repository: t.TempDir(), Executable: fake.path,
		Environment: []string{"PATH=/usr/bin:/bin", "CURSOR_API_KEY=deadbee"},
		Config:      cursorConfig(protocol.IsolationNative, true, 5*time.Second),
	})
	if diagnostic.FailureClass != protocol.FailureCapability || diagnostic.Compatible || diagnostic.Version != "" {
		t.Fatalf("diagnostic = %+v", diagnostic)
	}
}

func TestDoctorAllowsShortCredentialPlaceholderInNumericVersion(t *testing.T) {
	fake := newFakeCodex(t, fakeCodexOptions{})
	diagnostic := Doctor(t.Context(), DoctorOptions{
		Repository: t.TempDir(), Executable: fake.path,
		Environment: []string{"PATH=/usr/bin:/bin", "OPENAI_API_KEY=1"},
		Config:      codexConfig(protocol.IsolationStrict, false, 5*time.Second),
	})
	if diagnostic.Status != DoctorReady || diagnostic.Version != "0.146.0" || diagnostic.Authentication != AuthenticationReady {
		t.Fatalf("diagnostic = %+v", diagnostic)
	}
}

func TestDoctorSurfacesCleanupFailureAfterNotReady(t *testing.T) {
	fake := newFakeCodex(t, fakeCodexOptions{})
	diagnostic := Doctor(t.Context(), DoctorOptions{
		Repository: t.TempDir(), Executable: fake.path, Environment: []string{"PATH=/usr/bin:/bin"},
		Config: codexConfig(protocol.IsolationStrict, false, 5*time.Second),
		closeRuntime: func(runtime *config.Runtime) error {
			if err := runtime.Close(); err != nil {
				return err
			}
			return errors.New("injected cleanup failure with private detail")
		},
	})
	if diagnostic.Status != DoctorNotReady || diagnostic.FailureClass != protocol.FailureInternal || diagnostic.Message != "provider diagnostic failed internally" || strings.Contains(diagnostic.Message, "private") {
		t.Fatalf("diagnostic = %+v", diagnostic)
	}
}

func TestDiagnosticRejectsUnboundedFailure(t *testing.T) {
	diagnostic := Diagnostic{
		SchemaVersion: DoctorSchemaVersion, Status: DoctorNotReady, Provider: protocol.ProviderCodex,
		Isolation: protocol.IsolationNative, Authentication: AuthenticationDelegated,
		FailureClass: protocol.FailureProvider, Message: "PRIVATE RAW PROVIDER OUTPUT",
	}
	if err := diagnostic.Validate(); err == nil {
		t.Fatal("expected unallowlisted failure to be rejected")
	}
	diagnostic.FailureClass = protocol.FailureCapability
	if err := diagnostic.Validate(); err == nil {
		t.Fatal("expected non-canonical message to be rejected")
	}
	tooLong := Diagnostic{
		SchemaVersion: DoctorSchemaVersion, Status: DoctorReady, Provider: protocol.ProviderCursor,
		Version: "2026.08.23-" + strings.Repeat("a", doctorVersionMaxBytes), Compatible: true,
		Isolation: protocol.IsolationNative, Authentication: AuthenticationDelegated,
	}
	if err := tooLong.Validate(); err == nil {
		t.Fatal("expected oversized provider version to be rejected")
	}
	tooLong.Version = "2026.08.23-ok private-output"
	if err := tooLong.Validate(); err == nil {
		t.Fatal("expected non-canonical provider version to be rejected")
	}
}

func doctorProbeWrapper(t *testing.T, executable string, required []string, delay string) string {
	t.Helper()
	var script strings.Builder
	script.WriteString("#!/bin/sh\nset -eu\n")
	for _, entry := range required {
		name, value, ok := strings.Cut(entry, "=")
		if !ok {
			t.Fatalf("invalid environment entry %q", entry)
		}
		fmt.Fprintf(&script, "[ \"${%s:-}\" = %s ] || exit 70\n", name, shellQuote(value))
	}
	if delay != "" {
		fmt.Fprintf(&script, "sleep %s\n", shellQuote(delay))
	}
	fmt.Fprintf(&script, "exec %s \"$@\"\n", shellQuote(executable))
	path := filepath.Join(t.TempDir(), "provider")
	writeTestExecutableAt(t, path, script.String())
	return path
}
