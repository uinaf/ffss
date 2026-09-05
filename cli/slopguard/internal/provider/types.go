package provider

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/uinaf/ffss/cli/slopguard/internal/config"
	"github.com/uinaf/ffss/cli/slopguard/internal/protocol"
)

const (
	DefaultCodexModel  = "gpt-6-astra"
	DefaultClaudeModel = "claude-fable-5-1"
	DefaultCursorModel = "cursor-grok-4.6-high"
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
	WebAccess        bool
	ProtocolRecovery protocol.ProtocolRecovery
}

type Execution struct {
	Provider         protocol.Provider
	WebAccess        bool
	ProtocolRecovery protocol.ProtocolRecovery
}

type Reviewer interface {
	Review(context.Context, Request) (Result, error)
}

type Error struct {
	Class         protocol.FailureClass
	Message       string
	Attempt       *protocol.Attempt
	Reason        protocol.ProtocolReason
	Execution     *Execution
	ContextCaused bool
}

type reportedProviderError struct {
	Class   protocol.FailureClass
	Message string
}

type preparationKey struct {
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
	done       chan struct{}
	prepared   preparedExecutable
	err        error
	contextErr error
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
				if call.err != nil && ctx.Err() == nil && preparationFailureCausedByContext(call.err, call.contextErr) {
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
		call.contextErr = ctx.Err()

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

func preparationFailureCausedByContext(err, contextErr error) bool {
	if err == nil || contextErr == nil {
		return false
	}
	if errors.Is(err, contextErr) {
		return true
	}
	var failure *Error
	if !errors.As(err, &failure) {
		return false
	}
	if !failure.ContextCaused {
		return false
	}
	switch {
	case errors.Is(contextErr, context.Canceled):
		return failure.Class == protocol.FailureCancelled
	case errors.Is(contextErr, context.DeadlineExceeded):
		return failure.Class == protocol.FailureTimeout
	default:
		return false
	}
}

func effectivePreparationKey(effective config.Effective) preparationKey {
	return preparationKey{WebAccess: effective.WebAccess.Value}
}

func (failure *reportedProviderError) Error() string {
	return failure.Message
}

func (failure *Error) withExecution(execution Execution) *Error {
	failure.Execution = &execution
	return failure
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
