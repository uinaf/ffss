package provider

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/uinaf/ffss/cli/slopguard/internal/config"
	"github.com/uinaf/ffss/cli/slopguard/internal/protocol"
)

const (
	DefaultCodexModel  = "gpt-5.6-sol"
	DefaultClaudeModel = "claude-opus-5"
	DefaultCursorModel = "cursor-grok-4.6-high-fast"
	DefaultGrokModel   = "grok-4.6"
)

type Request struct {
	Prompt        string
	TrustedSuffix string
	Config        config.Effective
	Target        protocol.Target
}

func (request Request) validPrompt() bool {
	return utf8.ValidString(request.Prompt) && utf8.ValidString(request.TrustedSuffix) && strings.TrimSpace(request.Prompt) != ""
}

func (request Request) promptBytes() (int64, bool) {
	if len(request.TrustedSuffix) > int(^uint(0)>>1)-len(request.Prompt) {
		return 0, false
	}
	return int64(len(request.Prompt) + len(request.TrustedSuffix)), true
}

func (request Request) promptReader(additionalSuffix string) io.Reader {
	readers := []io.Reader{strings.NewReader(request.Prompt)}
	if request.TrustedSuffix != "" {
		readers = append(readers, strings.NewReader(request.TrustedSuffix))
	}
	if additionalSuffix != "" {
		readers = append(readers, strings.NewReader(additionalSuffix))
	}
	if len(readers) == 1 {
		return readers[0]
	}
	return io.MultiReader(readers...)
}

type Result struct {
	Review           protocol.Review
	Provider         protocol.Provider
	Attempt          protocol.Attempt
	Duration         time.Duration
	Isolation        protocol.Isolation
	WebAccess        bool
	ProtocolRecovery protocol.ProtocolRecovery
}

type Execution struct {
	Provider         protocol.Provider
	Isolation        protocol.Isolation
	WebAccess        bool
	ProtocolRecovery protocol.ProtocolRecovery
}

type Reviewer interface {
	Review(context.Context, Request) (Result, error)
}

type Error struct {
	Class     protocol.FailureClass
	Message   string
	Attempt   *protocol.Attempt
	Reason    protocol.ProtocolReason
	Execution *Execution
}

type reportedProviderError struct {
	Class   protocol.FailureClass
	Message string
}

type preparationKey struct {
	Isolation protocol.Isolation
	WebAccess bool
}

type preparedExecutable struct {
	Path    string
	Version string
}

type preparationCache struct {
	mu       sync.Mutex
	values   map[preparationKey]preparedExecutable
	inFlight map[preparationKey]*preparationCall
}

type preparationCall struct {
	done     chan struct{}
	prepared preparedExecutable
	err      error
}

func (cache *preparationCache) get(key preparationKey) (preparedExecutable, bool) {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	prepared, ok := cache.values[key]
	return prepared, ok
}

func (cache *preparationCache) resolve(ctx context.Context, key preparationKey, prepare func() (preparedExecutable, error)) (preparedExecutable, error) {
	for {
		cache.mu.Lock()
		if prepared, ok := cache.values[key]; ok {
			cache.mu.Unlock()
			return prepared, nil
		}
		if call, ok := cache.inFlight[key]; ok {
			cache.mu.Unlock()
			select {
			case <-call.done:
				if call.err != nil && ctx.Err() == nil && (errors.Is(call.err, context.Canceled) || errors.Is(call.err, context.DeadlineExceeded)) {
					continue
				}
				return call.prepared, call.err
			case <-ctx.Done():
				return preparedExecutable{}, ctx.Err()
			}
		}
		if cache.inFlight == nil {
			cache.inFlight = make(map[preparationKey]*preparationCall)
		}
		call := &preparationCall{done: make(chan struct{})}
		cache.inFlight[key] = call
		cache.mu.Unlock()

		call.prepared, call.err = prepare()

		cache.mu.Lock()
		delete(cache.inFlight, key)
		if call.err == nil {
			if cache.values == nil {
				cache.values = make(map[preparationKey]preparedExecutable)
			}
			cache.values[key] = call.prepared
		}
		close(call.done)
		cache.mu.Unlock()

		if call.err != nil {
			return preparedExecutable{}, call.err
		}
		return call.prepared, nil
	}
}

func effectivePreparationKey(effective config.Effective) preparationKey {
	return preparationKey{Isolation: effective.Isolation.Value, WebAccess: effective.WebAccess.Value}
}

func (failure *reportedProviderError) Error() string {
	return failure.Message
}

func (failure *Error) withExecution(execution Execution) *Error {
	failure.Execution = &execution
	return failure
}

func strictCredentialFailure(effective config.Effective, provider protocol.ProviderName, environment []string) *Error {
	if effective.Isolation.Value != protocol.IsolationStrict {
		return nil
	}
	label, names := providerCredentialNames(provider)
	if len(names) == 0 {
		return newFailure(
			protocol.FailureAuth,
			fmt.Sprintf("%s strict isolation has no configured credential contract; use --isolation native or update the provider adapter", label),
			environment,
			nil,
		)
	}
	for _, name := range names {
		if environmentValue(environment, name) != "" {
			return nil
		}
	}
	return newFailure(
		protocol.FailureAuth,
		fmt.Sprintf("%s strict isolation requires %s; set a supported API key or use --isolation native for provider or session authentication", label, strings.Join(names, " or ")),
		environment,
		nil,
	)
}

func strictCredentialRecovery(effective config.Effective, provider protocol.ProviderName) string {
	if effective.Isolation.Value != protocol.IsolationStrict {
		return ""
	}
	_, names := providerCredentialNames(provider)
	if len(names) == 0 {
		return "use --isolation native or update the provider adapter credential contract"
	}
	return fmt.Sprintf("verify %s or use --isolation native for provider or session authentication", strings.Join(names, " or "))
}

func providerCredentialNames(name protocol.ProviderName) (string, []string) {
	switch name {
	case protocol.ProviderCodex:
		return "Codex", []string{"CODEX_API_KEY", "OPENAI_API_KEY"}
	case protocol.ProviderClaude:
		return "Claude", []string{"ANTHROPIC_API_KEY"}
	case protocol.ProviderCursor:
		return "Cursor", []string{"CURSOR_API_KEY"}
	case protocol.ProviderGrok:
		return "Grok", []string{"XAI_API_KEY"}
	default:
		return string(name), nil
	}
}

func (failure *Error) Error() string {
	return failure.Message
}
