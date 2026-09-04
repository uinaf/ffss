package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/uinaf/ffss/cli/slopguard/internal/protocol"
	"github.com/uinaf/ffss/cli/slopguard/internal/provider"
)

func TestDoctorCommandWritesReadyJSON(t *testing.T) {
	repository := reviewRepository(t)
	doctorCalls := 0
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exit := run(t.Context(), []string{
		"doctor", "--repository", repository, "--engine", "codex", "--output", "json",
	}, &stdout, &stderr, dependencies{
		lookupEnv: func(string) (string, bool) { return "", false },
		homeDir:   func() (string, error) { return t.TempDir(), nil },
		doctor: func(_ context.Context, options provider.DoctorOptions) provider.Diagnostic {
			doctorCalls++
			return provider.Diagnostic{
				SchemaVersion: provider.DoctorSchemaVersion, Status: provider.DoctorReady,
				Provider: options.Config.Engine.Value, Version: "1.2.3", Compatible: true,
				WebAccess:      options.Config.WebAccess.Value,
				Authentication: provider.AuthenticationDelegated,
			}
		},
	})
	if exit != 0 || doctorCalls != 1 || stderr.Len() != 0 {
		t.Fatalf("exit=%d calls=%d stdout=%q stderr=%q", exit, doctorCalls, stdout.String(), stderr.String())
	}
	var diagnostic provider.Diagnostic
	if err := json.Unmarshal(stdout.Bytes(), &diagnostic); err != nil || diagnostic.Status != provider.DoctorReady {
		t.Fatalf("diagnostic=%+v error=%v", diagnostic, err)
	}
}

func TestDoctorCommandAcceptsJSONAlias(t *testing.T) {
	repository := reviewRepository(t)
	var stdout bytes.Buffer
	exit := run(t.Context(), []string{
		"doctor", "--repository", repository, "--engine", "codex", "--model", "gpt-5.6-sol", "--reasoning-effort", "high", "--json",
	}, &stdout, io.Discard, dependencies{
		lookupEnv: func(string) (string, bool) { return "", false },
		homeDir:   func() (string, error) { return t.TempDir(), nil },
		doctor: func(_ context.Context, options provider.DoctorOptions) provider.Diagnostic {
			return provider.Diagnostic{
				SchemaVersion: provider.DoctorSchemaVersion, Status: provider.DoctorReady,
				Provider: options.Config.Engine.Value, Version: "0.153.2", Compatible: true,
				WebAccess: options.Config.WebAccess.Value, Authentication: provider.AuthenticationDelegated,
			}
		},
	})
	var diagnostic provider.Diagnostic
	if err := json.Unmarshal(stdout.Bytes(), &diagnostic); exit != 0 || err != nil || diagnostic.Version != "0.153.2" || !diagnostic.Compatible {
		t.Fatalf("exit=%d diagnostic=%+v error=%v", exit, diagnostic, err)
	}
}

func TestDoctorJSONAliasCoversEarlyFailures(t *testing.T) {
	for _, arguments := range [][]string{
		{"doctor", "--json", "--unknown"},
		{"doctor", "--json", "--output", "terminal"},
		{"doctor", "--output", "json", "--json=false", "--unknown"},
		{"doctor", "--output", "terminal", "--output", "yaml", "--json"},
		{"doctor", "--json=invalid"},
	} {
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		exit := run(t.Context(), arguments, &stdout, &stderr, dependencies{})
		var diagnostic provider.Diagnostic
		if err := json.Unmarshal(stdout.Bytes(), &diagnostic); exit != 2 || err != nil || stderr.Len() != 0 || diagnostic.FailureClass != protocol.FailureConfig {
			t.Fatalf("arguments=%v exit=%d diagnostic=%+v error=%v stderr=%q", arguments, exit, diagnostic, err, stderr.String())
		}
	}
}

func TestDoctorJSONAliasDoesNotMatchAnotherFlagValue(t *testing.T) {
	var stdout bytes.Buffer
	exit := run(t.Context(), []string{"doctor", "--model", "--json", "--unknown"}, &stdout, io.Discard, dependencies{})
	if exit != 2 || !strings.HasPrefix(stdout.String(), "status: not_ready\n") {
		t.Fatalf("exit=%d stdout=%q", exit, stdout.String())
	}
}

func TestDoctorCommandReportsTargetFailureOutsideGitWorktree(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exit := run(t.Context(), []string{
		"doctor", "--repository", t.TempDir(), "--engine", "codex", "--output", "json",
	}, &stdout, &stderr, dependencies{
		lookupEnv: func(string) (string, bool) { return "", false },
		homeDir:   func() (string, error) { return t.TempDir(), nil },
		doctor: func(context.Context, provider.DoctorOptions) provider.Diagnostic {
			t.Fatal("doctor must not run without a resolved repository")
			return provider.Diagnostic{}
		},
	})
	var diagnostic provider.Diagnostic
	if err := json.Unmarshal(stdout.Bytes(), &diagnostic); err != nil {
		t.Fatalf("decode diagnostic: %v: %s", err, stdout.String())
	}
	if exit != 2 || diagnostic.FailureClass != protocol.FailureTarget || diagnostic.Message != "repository path is not inside a Git worktree" {
		t.Fatalf("exit=%d diagnostic=%+v stderr=%q", exit, diagnostic, stderr.String())
	}
}

func TestDoctorCommandSanitizesConfigurationFailure(t *testing.T) {
	privatePath := filepath.Join(t.TempDir(), "private-repository")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exit := run(t.Context(), []string{
		"doctor", "--repository", privatePath, "--engine", "codex", "--output", "json",
	}, &stdout, &stderr, dependencies{})
	if exit != 2 || stderr.Len() != 0 || strings.Contains(stdout.String(), privatePath) {
		t.Fatalf("exit=%d stdout=%q stderr=%q", exit, stdout.String(), stderr.String())
	}
	var diagnostic provider.Diagnostic
	if err := json.Unmarshal(stdout.Bytes(), &diagnostic); err != nil || diagnostic.FailureClass != protocol.FailureTarget {
		t.Fatalf("diagnostic=%+v error=%v", diagnostic, err)
	}
}

func TestDoctorCommandReturnsExit2ForProviderConfigRejection(t *testing.T) {
	repository := reviewRepository(t)
	var stdout bytes.Buffer
	exit := run(t.Context(), []string{
		"doctor", "--repository", repository, "--engine", "cursor", "--web-access=false", "--output", "json",
	}, &stdout, io.Discard, dependencies{
		lookupEnv: func(string) (string, bool) { return "", false },
		homeDir:   func() (string, error) { return t.TempDir(), nil },
	})
	var diagnostic provider.Diagnostic
	if err := json.Unmarshal(stdout.Bytes(), &diagnostic); exit != 2 || err != nil || diagnostic.FailureClass != protocol.FailureConfig {
		t.Fatalf("exit=%d diagnostic=%+v error=%v", exit, diagnostic, err)
	}
}

func TestDoctorCommandReturnsExit1ForInternalDiagnostic(t *testing.T) {
	repository := reviewRepository(t)
	var stdout bytes.Buffer
	exit := run(t.Context(), []string{
		"doctor", "--repository", repository, "--engine", "codex", "--output", "json",
	}, &stdout, io.Discard, dependencies{
		lookupEnv: func(string) (string, bool) { return "", false },
		homeDir:   func() (string, error) { return t.TempDir(), nil },
		doctor: func(context.Context, provider.DoctorOptions) provider.Diagnostic {
			return provider.Diagnostic{
				SchemaVersion: provider.DoctorSchemaVersion, Status: provider.DoctorNotReady,
				Provider:       protocol.ProviderCodex,
				Authentication: provider.AuthenticationDelegated, FailureClass: protocol.FailureInternal,
				Message: "provider diagnostic failed internally",
			}
		},
	})
	var diagnostic provider.Diagnostic
	if err := json.Unmarshal(stdout.Bytes(), &diagnostic); exit != 1 || err != nil || diagnostic.FailureClass != protocol.FailureInternal {
		t.Fatalf("exit=%d diagnostic=%+v error=%v", exit, diagnostic, err)
	}
}

func TestDoctorCommandRejectsRepositoryExecutableFromNestedPath(t *testing.T) {
	repository := reviewRepository(t)
	nested := filepath.Join(repository, "nested")
	providerBin := filepath.Join(repository, "bin")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(providerBin, 0o755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(t.TempDir(), "invoked")
	executable := filepath.Join(providerBin, "codex")
	if err := os.WriteFile(executable, []byte("#!/bin/sh\nprintf invoked > "+shellLiteral(marker)+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", providerBin+string(os.PathListSeparator)+"/usr/bin:/bin")
	var stdout bytes.Buffer
	exit := run(t.Context(), []string{
		"doctor", "--repository", nested, "--engine", "codex", "--output", "json",
	}, &stdout, io.Discard, dependencies{
		homeDir: func() (string, error) { return t.TempDir(), nil },
	})
	var diagnostic provider.Diagnostic
	if err := json.Unmarshal(stdout.Bytes(), &diagnostic); exit != 1 || err != nil || diagnostic.FailureClass != protocol.FailureCapability {
		t.Fatalf("exit=%d diagnostic=%+v error=%v", exit, diagnostic, err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("repository provider executable was invoked: %v", err)
	}
}

func TestDoctorCommandDiscardsRawFlagDiagnostics(t *testing.T) {
	const private = "PRIVATE-CREDENTIAL-VALUE"
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exit := run(t.Context(), []string{"doctor", "--engine", "codex", "--web-access=" + private}, &stdout, &stderr, dependencies{})
	if exit != 2 || stderr.Len() != 0 || strings.Contains(stdout.String(), private) || !strings.Contains(stdout.String(), "provider configuration is invalid") {
		t.Fatalf("exit=%d stdout=%q stderr=%q", exit, stdout.String(), stderr.String())
	}
}

func TestDoctorCommandRejectsInjectedUnboundedDiagnostic(t *testing.T) {
	repository := reviewRepository(t)
	const private = "PRIVATE RAW PROVIDER OUTPUT"
	var stdout bytes.Buffer
	exit := run(t.Context(), []string{"doctor", "--repository", repository, "--engine", "codex", "--output", "json"}, &stdout, io.Discard, dependencies{
		lookupEnv: func(string) (string, bool) { return "", false },
		homeDir:   func() (string, error) { return t.TempDir(), nil },
		doctor: func(context.Context, provider.DoctorOptions) provider.Diagnostic {
			return provider.Diagnostic{
				SchemaVersion: provider.DoctorSchemaVersion, Status: provider.DoctorNotReady,
				Provider:       protocol.ProviderCodex,
				Authentication: provider.AuthenticationDelegated, FailureClass: protocol.FailureProvider, Message: private,
			}
		},
	})
	if exit != 2 || strings.Contains(stdout.String(), private) || !strings.Contains(stdout.String(), "provider configuration is invalid") {
		t.Fatalf("exit=%d output=%q", exit, stdout.String())
	}
}

func TestDoctorCommandWritesNotReadyTerminal(t *testing.T) {
	repository := reviewRepository(t)
	var stdout bytes.Buffer
	exit := run(t.Context(), []string{"doctor", "--repository", repository, "--engine", "codex"}, &stdout, io.Discard, dependencies{
		lookupEnv: func(string) (string, bool) { return "", false },
		homeDir:   func() (string, error) { return t.TempDir(), nil },
		doctor: func(context.Context, provider.DoctorOptions) provider.Diagnostic {
			return provider.Diagnostic{
				SchemaVersion: provider.DoctorSchemaVersion, Status: provider.DoctorNotReady,
				Provider:       protocol.ProviderCodex,
				Authentication: provider.AuthenticationDelegated, FailureClass: protocol.FailureCapability,
				Message: "provider executable is missing or incompatible",
			}
		},
	})
	if exit != 1 || !strings.Contains(stdout.String(), "status: not_ready") || !strings.Contains(stdout.String(), "failure: capability") {
		t.Fatalf("exit=%d output=%q", exit, stdout.String())
	}
}
