package provider

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/uinaf/ffss/cli/slopguard/internal/config"
	"github.com/uinaf/ffss/cli/slopguard/internal/protocol"
)

const (
	DoctorSchemaVersion   = "1"
	doctorVersionMaxBytes = 64
)

type DoctorStatus string

const (
	DoctorReady    DoctorStatus = "ready"
	DoctorNotReady DoctorStatus = "not_ready"
)

type AuthenticationReadiness string

const (
	AuthenticationReady     AuthenticationReadiness = "ready"
	AuthenticationDelegated AuthenticationReadiness = "delegated"
	AuthenticationMissing   AuthenticationReadiness = "missing"
)

type Diagnostic struct {
	SchemaVersion  string                  `json:"schema_version"`
	Status         DoctorStatus            `json:"status"`
	Provider       protocol.ProviderName   `json:"provider"`
	Version        string                  `json:"version,omitempty"`
	Compatible     bool                    `json:"compatible"`
	WebAccess      bool                    `json:"web_access"`
	Authentication AuthenticationReadiness `json:"authentication"`
	FailureClass   protocol.FailureClass   `json:"failure_class,omitempty"`
	Message        string                  `json:"message,omitempty"`
}

type DoctorOptions struct {
	Repository   string
	Executable   string
	Environment  []string
	Config       config.Effective
	closeRuntime func(*config.Runtime) error
}

func Doctor(ctx context.Context, options DoctorOptions) (diagnostic Diagnostic) {
	diagnostic = Diagnostic{
		SchemaVersion:  DoctorSchemaVersion,
		Status:         DoctorNotReady,
		Provider:       options.Config.Engine.Value,
		WebAccess:      options.Config.WebAccess.Value,
		Authentication: authenticationReadiness(options.Config.Engine.Value, options.Environment),
	}
	if err := validateDoctorConfig(options.Config); err != nil {
		return diagnostic.withFailure(protocol.FailureConfig)
	}
	environment := options.Environment
	if environment == nil {
		environment = os.Environ()
		diagnostic.Authentication = authenticationReadiness(options.Config.Engine.Value, environment)
	}
	repository, err := filepath.Abs(options.Repository)
	if err != nil {
		return diagnostic.withFailure(protocol.FailureConfig)
	}
	runtime, err := config.PrepareRuntime(environment)
	if err != nil {
		return diagnostic.withFailure(protocol.FailureInternal)
	}
	closeRuntime := options.closeRuntime
	if closeRuntime == nil {
		closeRuntime = func(runtime *config.Runtime) error { return runtime.Close() }
	}
	defer func() {
		if err := closeRuntime(runtime); err != nil {
			diagnostic = diagnostic.withFailure(protocol.FailureInternal)
		}
	}()
	probeContext, cancelProbe := context.WithTimeout(ctx, time.Duration(options.Config.Timeout.Value))
	defer cancelProbe()

	executable := options.Executable
	var preflight func(string) (string, error)
	switch options.Config.Engine.Value {
	case protocol.ProviderCodex:
		adapter := NewCodex(CodexOptions{Repository: repository, Executable: executable, Environment: environment})
		executable = adapter.executable
		preflight = func(candidate string) (string, error) {
			return adapter.preflight(probeContext, candidate, runtime, options.Config)
		}
	case protocol.ProviderClaude:
		adapter := NewClaude(ClaudeOptions{Repository: repository, Executable: executable, Environment: environment})
		executable = adapter.executable
		providerEnvironment := runtime.Environment()
		preflight = func(candidate string) (string, error) {
			return adapter.preflight(probeContext, candidate, runtime.Workspace, providerEnvironment, options.Config)
		}
	case protocol.ProviderCursor:
		adapter := NewCursor(CursorOptions{Repository: repository, Executable: executable, Environment: environment})
		executable = adapter.executable
		providerEnvironment := runtime.Environment()
		preflight = func(candidate string) (string, error) {
			return adapter.preflight(probeContext, candidate, runtime.Workspace, providerEnvironment, options.Config)
		}
	case protocol.ProviderGrok:
		adapter := NewGrok(GrokOptions{Repository: repository, Executable: executable, Environment: environment})
		executable = adapter.executable
		providerEnvironment := setEnvironmentValue(runtime.Environment(), "GROK_MEMORY", "0")
		providerEnvironment = setEnvironmentValue(providerEnvironment, "GROK_SUBAGENTS", "0")
		preflight = func(candidate string) (string, error) {
			return adapter.preflight(probeContext, candidate, runtime.Workspace, providerEnvironment, options.Config)
		}
	default:
		return diagnostic.withFailure(protocol.FailureConfig)
	}
	candidates, err := discoverExecutableCandidates(executable, repository, environment)
	if err != nil {
		return diagnostic.withFailure(protocol.FailureCapability)
	}
	prepared, err := selectCompatibleExecutable(candidates, preflight)
	if err != nil {
		return diagnostic.withFailure(classifyDoctorFailure(err))
	}
	if !validDoctorVersion(diagnostic.Provider, prepared.Version) || doctorVersionContainsCredential(diagnostic.Provider, prepared.Version, environment) {
		return diagnostic.withFailure(protocol.FailureCapability)
	}
	diagnostic.Compatible = true
	diagnostic.Version = prepared.Version
	if diagnostic.Authentication == AuthenticationMissing {
		return diagnostic.withFailure(protocol.FailureAuth)
	}
	diagnostic.Status = DoctorReady
	return diagnostic
}

func (diagnostic Diagnostic) Validate() error {
	if diagnostic.SchemaVersion != DoctorSchemaVersion {
		return fmt.Errorf("unsupported doctor schema")
	}
	if diagnostic.Provider != "" {
		switch diagnostic.Provider {
		case protocol.ProviderCodex, protocol.ProviderClaude, protocol.ProviderCursor, protocol.ProviderGrok:
		default:
			return fmt.Errorf("invalid doctor provider")
		}
	} else if diagnostic.Status != DoctorNotReady || (diagnostic.FailureClass != protocol.FailureConfig && diagnostic.FailureClass != protocol.FailureTarget) {
		return fmt.Errorf("doctor provider is required")
	}
	if diagnostic.Status != DoctorReady && diagnostic.Status != DoctorNotReady {
		return fmt.Errorf("invalid doctor status")
	}
	if diagnostic.Authentication != AuthenticationReady && diagnostic.Authentication != AuthenticationDelegated && diagnostic.Authentication != AuthenticationMissing {
		return fmt.Errorf("invalid authentication readiness")
	}
	if diagnostic.Compatible != (diagnostic.Version != "") || diagnostic.Version != "" && !validDoctorVersion(diagnostic.Provider, diagnostic.Version) {
		return fmt.Errorf("invalid doctor version")
	}
	if diagnostic.Status == DoctorReady && (!diagnostic.Compatible || diagnostic.Version == "" || diagnostic.FailureClass != "" || diagnostic.Message != "") {
		return fmt.Errorf("ready doctor result is inconsistent")
	}
	if diagnostic.Status == DoctorNotReady {
		message, ok := doctorFailureMessage(diagnostic.FailureClass)
		if !ok || diagnostic.Message != message {
			return fmt.Errorf("not-ready doctor result is invalid")
		}
	}
	return nil
}

func (diagnostic Diagnostic) withFailure(class protocol.FailureClass) Diagnostic {
	diagnostic.Status = DoctorNotReady
	message, ok := doctorFailureMessage(class)
	if !ok {
		diagnostic.FailureClass = protocol.FailureCapability
		diagnostic.Message, _ = doctorFailureMessage(protocol.FailureCapability)
		return diagnostic
	}
	diagnostic.FailureClass = class
	diagnostic.Message = message
	return diagnostic
}

func doctorFailureMessage(class protocol.FailureClass) (string, bool) {
	switch class {
	case protocol.FailureAuth:
		return "provider authentication is not ready", true
	case protocol.FailureTarget:
		return "repository path is not inside a Git worktree", true
	case protocol.FailureTimeout:
		return "provider capability probe timed out", true
	case protocol.FailureCancelled:
		return "provider capability probe was cancelled", true
	case protocol.FailureConfig:
		return "provider configuration is invalid", true
	case protocol.FailureInternal:
		return "provider diagnostic failed internally", true
	case protocol.FailureCapability:
		return "provider executable is missing or incompatible", true
	default:
		return "", false
	}
}

func validateDoctorConfig(effective config.Effective) error {
	if err := effective.Validate(); err != nil {
		return err
	}
	switch effective.Engine.Value {
	case protocol.ProviderClaude:
		switch effective.ReasoningEffort.Value {
		case config.ReasoningLow, config.ReasoningMedium, config.ReasoningHigh, config.ReasoningXHigh, config.ReasoningMax:
		default:
			return fmt.Errorf("Claude reasoning effort is unsupported")
		}
	case protocol.ProviderCursor:
		if effective.ReasoningEffort.Source != config.SourceDefault || !effective.WebAccess.Value {
			return fmt.Errorf("Cursor execution policy is unsupported")
		}
	}
	return nil
}

func authenticationReadiness(name protocol.ProviderName, environment []string) AuthenticationReadiness {
	_, credentials := providerCredentialNames(name)
	for _, credential := range credentials {
		if environmentValue(environment, credential) != "" {
			return AuthenticationReady
		}
	}
	return AuthenticationDelegated
}

func classifyDoctorFailure(err error) protocol.FailureClass {
	var failure *Error
	if errors.As(err, &failure) {
		switch failure.Class {
		case protocol.FailureTimeout, protocol.FailureCancelled:
			return failure.Class
		default:
			return protocol.FailureCapability
		}
	}
	switch {
	case errors.Is(err, context.Canceled):
		return protocol.FailureCancelled
	case errors.Is(err, context.DeadlineExceeded):
		return protocol.FailureTimeout
	default:
		return protocol.FailureCapability
	}
}

func validDoctorVersion(name protocol.ProviderName, version string) bool {
	if version == "" || len(version) > doctorVersionMaxBytes {
		return false
	}
	var matched string
	switch name {
	case protocol.ProviderCodex:
		matched = codexVersionPattern.FindString(version)
	case protocol.ProviderClaude:
		matched = claudeVersionPattern.FindString(version)
	case protocol.ProviderCursor:
		matched = cursorVersionPattern.FindString(version)
	case protocol.ProviderGrok:
		matched = grokVersionPattern.FindString(version)
	}
	return matched == version
}

func doctorVersionContainsCredential(provider protocol.ProviderName, version string, environment []string) bool {
	var build string
	if provider == protocol.ProviderCursor {
		_, build, _ = strings.Cut(version, "-")
	}
	for _, entry := range environment {
		name, value, found := strings.Cut(entry, "=")
		if !found || value == "" || !credentialEnvironmentName(name) {
			continue
		}
		if version == value || build == value {
			return true
		}
	}
	return false
}

func credentialEnvironmentName(name string) bool {
	name = strings.ToUpper(name)
	for _, marker := range []string{
		"AUTH", "AUTHORIZATION", "BEARER", "COOKIE", "COOKIES", "CRED", "CREDENTIAL", "CREDENTIALS",
		"DSN", "KEY", "OAUTH", "PASS", "PASSWORD", "PAT", "PRIVATE", "SECRET", "SECRETS",
		"SESSION", "SESSIONS", "TOKEN",
	} {
		if name == marker || strings.HasSuffix(name, "_"+marker) {
			return true
		}
	}
	return false
}
