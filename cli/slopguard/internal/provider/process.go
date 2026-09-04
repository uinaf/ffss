package provider

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"

	"github.com/uinaf/ffss/cli/slopguard/internal/processgroup"
)

type processErrorKind string

const (
	processStart       processErrorKind = "start"
	processExit        processErrorKind = "exit"
	processTimeout     processErrorKind = "timeout"
	processCancelled   processErrorKind = "cancelled"
	processOutputLimit processErrorKind = "output_limit"
	processCleanup     processErrorKind = "cleanup"
)

type processSpec struct {
	Path        string
	Arguments   []string
	Directory   string
	Environment []string
	Input       io.Reader
	Timeout     time.Duration
	StdoutLimit int64
	StderrLimit int64
}

type processResult struct {
	Stdout                []byte
	Stderr                []byte
	AuthenticationFailure bool
	ExitCode              int
	Duration              time.Duration
}

var authenticationFailureMarkers = []string{
	"not logged in", "not authenticated", "not signed in", "unauthorized",
	"authentication", "login required", "401",
}

type processError struct {
	Kind          processErrorKind
	Result        processResult
	Err           error
	ContextCaused bool
}

func (failure *processError) Error() string {
	return fmt.Sprintf("provider process %s: %v", failure.Kind, failure.Err)
}

func (failure *processError) Unwrap() error {
	return failure.Err
}

func runProcess(ctx context.Context, spec processSpec) (processResult, error) {
	if spec.Timeout <= 0 {
		return processResult{}, &processError{Kind: processStart, Err: fmt.Errorf("timeout must be positive")}
	}
	if spec.StdoutLimit < 0 || spec.StderrLimit < 0 {
		return processResult{}, &processError{Kind: processStart, Err: fmt.Errorf("output limits must not be negative")}
	}
	runContext, cancel := context.WithTimeout(ctx, spec.Timeout)
	defer cancel()
	//nolint:noctx // processgroup.Run owns cancellation so it can kill the group before reaping the leader.
	command := exec.Command(spec.Path, spec.Arguments...)
	command.Dir = spec.Directory
	command.Env = append(make([]string, 0, len(spec.Environment)), spec.Environment...)
	command.Stdin = spec.Input
	var stdoutOverflow atomic.Bool
	stdout := newBoundedBuffer(spec.StdoutLimit, func() {
		stdoutOverflow.Store(true)
		cancel()
	})
	// Provider CLIs may emit large non-fatal hook or progress diagnostics on
	// stderr. Retain bounded head and tail segments for classification without
	// terminating an otherwise valid machine-readable result.
	stderr := newHeadTailBuffer(spec.StderrLimit, authenticationFailureMarkers)
	command.Stdout = stdout
	command.Stderr = stderr
	started := time.Now()
	runResult := processgroup.Run(runContext, command)
	result := processResult{
		Stdout:                stdout.Bytes(),
		Stderr:                stderr.Bytes(),
		AuthenticationFailure: stderr.Matched(),
		ExitCode:              0,
		Duration:              time.Since(started),
	}
	if runResult.CommandErr == nil && runResult.CleanupErr == nil {
		if stdoutOverflow.Load() {
			return result, &processError{Kind: processOutputLimit, Result: result, Err: errors.New("stdout exceeded limit")}
		}
		return result, nil
	}
	if runResult.CommandErr == nil {
		result.Stdout = nil
		result.Stderr = nil
		return result, &processError{Kind: processCleanup, Result: result, Err: runResult.CleanupErr}
	}
	err := errors.Join(runResult.CommandErr, runResult.CleanupErr)
	if exitError := new(exec.ExitError); errors.As(err, &exitError) {
		result.ExitCode = exitError.ExitCode()
	} else {
		result.ExitCode = -1
	}
	kind := processExit
	switch {
	case stdoutOverflow.Load():
		kind = processOutputLimit
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		kind = processTimeout
	case ctx.Err() != nil:
		kind = processCancelled
	case errors.Is(runContext.Err(), context.DeadlineExceeded):
		kind = processTimeout
	case result.ExitCode == -1:
		kind = processStart
	}
	contextCaused := (kind == processTimeout || kind == processCancelled) && runResult.ContextCaused
	return result, &processError{Kind: kind, Result: result, Err: err, ContextCaused: contextCaused}
}

func isOrdinaryProcessExit(err error) bool {
	var failure *processError
	return errors.As(err, &failure) && failure.Kind == processExit
}

func isCapabilityProbeFailure(err error) bool {
	var failure *processError
	if !errors.As(err, &failure) {
		return false
	}
	return failure.Kind == processOutputLimit
}

type boundedBuffer struct {
	mu       sync.Mutex
	buffer   bytes.Buffer
	limit    int64
	overflow sync.Once
	onLimit  func()
}

type headTailBuffer struct {
	mu              sync.Mutex
	head            bytes.Buffer
	tail            []byte
	limit           int64
	truncated       bool
	markers         [][]byte
	matchWindow     []byte
	maxMarkerLength int
	matched         bool
}

func newBoundedBuffer(limit int64, onLimit func()) *boundedBuffer {
	return &boundedBuffer{limit: limit, onLimit: onLimit}
}

func newHeadTailBuffer(limit int64, markers []string) *headTailBuffer {
	writer := &headTailBuffer{limit: limit}
	for _, marker := range markers {
		lower := bytes.ToLower([]byte(marker))
		writer.markers = append(writer.markers, lower)
		writer.maxMarkerLength = max(writer.maxMarkerLength, len(lower))
	}
	return writer
}

func (writer *boundedBuffer) Write(data []byte) (int, error) {
	written := len(data)
	writer.mu.Lock()
	remaining := writer.limit - int64(writer.buffer.Len())
	if remaining > 0 {
		keep := int64(len(data))
		if keep > remaining {
			keep = remaining
		}
		_, _ = writer.buffer.Write(data[:keep])
	}
	exceeded := int64(len(data)) > remaining
	writer.mu.Unlock()
	if exceeded && writer.onLimit != nil {
		writer.overflow.Do(writer.onLimit)
	}
	return written, nil
}

func (writer *boundedBuffer) Bytes() []byte {
	writer.mu.Lock()
	defer writer.mu.Unlock()
	return append([]byte(nil), writer.buffer.Bytes()...)
}

func (writer *headTailBuffer) Write(data []byte) (int, error) {
	written := len(data)
	writer.mu.Lock()
	defer writer.mu.Unlock()
	writer.match(data)
	if !writer.truncated && int64(writer.head.Len()+len(writer.tail)+len(data)) > writer.limit {
		writer.truncated = true
	}

	headLimit := (writer.limit + 1) / 2
	remaining := headLimit - int64(writer.head.Len())
	if remaining > 0 {
		keep := min(int64(len(data)), remaining)
		_, _ = writer.head.Write(data[:keep])
		data = data[keep:]
	}
	tailLimit := writer.limit - headLimit
	if tailLimit <= 0 || len(data) == 0 {
		return written, nil
	}
	if int64(len(data)) >= tailLimit {
		writer.tail = append(writer.tail[:0], data[int64(len(data))-tailLimit:]...)
		return written, nil
	}
	overflow := int64(len(writer.tail)+len(data)) - tailLimit
	if overflow > 0 {
		writer.tail = append(writer.tail[:0], writer.tail[overflow:]...)
	}
	writer.tail = append(writer.tail, data...)
	return written, nil
}

func (writer *headTailBuffer) match(data []byte) {
	if writer.matched || writer.maxMarkerLength == 0 {
		return
	}
	candidate := make([]byte, 0, len(writer.matchWindow)+len(data))
	candidate = append(candidate, writer.matchWindow...)
	candidate = append(candidate, data...)
	candidate = bytes.ToLower(candidate)
	for _, marker := range writer.markers {
		if bytes.Contains(candidate, marker) {
			writer.matched = true
			return
		}
	}
	keep := min(len(candidate), writer.maxMarkerLength-1)
	writer.matchWindow = append(writer.matchWindow[:0], candidate[len(candidate)-keep:]...)
}

func (writer *headTailBuffer) Bytes() []byte {
	writer.mu.Lock()
	defer writer.mu.Unlock()
	result := make([]byte, 0, writer.head.Len()+len(writer.tail))
	head := writer.head.Bytes()
	separate := writer.truncated && len(head) != 0 && len(writer.tail) != 0
	if separate {
		head = head[:len(head)-1]
	}
	result = append(result, head...)
	if separate {
		result = append(result, '\n')
	}
	return append(result, writer.tail...)
}

func (writer *headTailBuffer) Matched() bool {
	writer.mu.Lock()
	defer writer.mu.Unlock()
	return writer.matched
}
