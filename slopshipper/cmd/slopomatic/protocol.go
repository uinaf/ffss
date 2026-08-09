package main

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/uinaf/slopomatic/internal/machine"
)

type outputDigester struct {
	mu     sync.Mutex
	hash   hash.Hash
	mirror io.Writer
}

type mirroredOutputDigester struct {
	digest *outputDigester
	mirror io.Writer
}

func newOutputDigester(mirror ...io.Writer) *outputDigester {
	d := &outputDigester{hash: sha256.New()}
	if len(mirror) > 0 {
		d.mirror = mirror[0]
	}
	return d
}

func (d *outputDigester) Write(p []byte) (int, error) {
	return d.writeTo(d.mirror, p)
}

func (d *outputDigester) Mirror(mirror io.Writer) io.Writer {
	return mirroredOutputDigester{digest: d, mirror: mirror}
}

func (d *outputDigester) writeTo(mirror io.Writer, p []byte) (int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if mirror != nil {
		n, err := mirror.Write(p)
		if err != nil {
			return n, err
		}
		if n != len(p) {
			return n, io.ErrShortWrite
		}
	}
	if _, err := d.hash.Write(p); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (d mirroredOutputDigester) Write(p []byte) (int, error) {
	return d.digest.writeTo(d.mirror, p)
}

func (d *outputDigester) String() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return fmt.Sprintf("sha256:%x", d.hash.Sum(nil))
}

func digestOutputs(stdout, stderr *outputDigester) string {
	sum := sha256.Sum256([]byte("stdout=" + stdout.String() + "\nstderr=" + stderr.String()))
	return fmt.Sprintf("sha256:%x", sum)
}

type runOptions struct {
	json   bool
	dryRun bool
	fields []string
}

type errorDocument struct {
	SchemaVersion int         `json:"schema_version"`
	OK            bool        `json:"ok"`
	Error         errorDetail `json:"error"`
}

type errorDetail struct {
	Kind     string `json:"kind"`
	Message  string `json:"message"`
	ExitCode int    `json:"exit_code"`
}

func parseRunOptions(args []string) ([]string, runOptions, error) {
	opts := runOptions{}
	jsonCount := 0
	for _, arg := range args {
		if arg == "--json" {
			opts.json = true
			jsonCount++
		}
		if strings.HasPrefix(arg, "--json=") {
			opts.json = true
			return nil, opts, fmt.Errorf("flag --json does not accept a value")
		}
	}
	if jsonCount > 1 {
		return nil, opts, fmt.Errorf("flag --json may be specified only once")
	}

	cleaned := make([]string, 0, len(args))
	dryRunCount := 0
	for _, arg := range args {
		switch {
		case arg == "--json":
			continue
		case arg == "--dry-run":
			opts.dryRun = true
			dryRunCount++
			continue
		case strings.HasPrefix(arg, "--dry-run="):
			return nil, opts, fmt.Errorf("flag --dry-run does not accept a value")
		default:
			cleaned = append(cleaned, arg)
		}
	}
	if dryRunCount > 1 {
		return nil, opts, fmt.Errorf("flag --dry-run may be specified only once")
	}
	return cleaned, opts, nil
}

func writeFailure(opts runOptions, code int, err error) int {
	if opts.json {
		doc := errorDocument{
			SchemaVersion: 1,
			OK:            false,
			Error: errorDetail{
				Kind:     errorKind(code, err),
				Message:  err.Error(),
				ExitCode: code,
			},
		}
		if encodeErr := writeJSON(doc); encodeErr != nil {
			fmt.Fprintf(os.Stderr, "slopomatic: encode error response: %v\n", encodeErr)
			return 10
		}
		return code
	}
	fmt.Fprintf(os.Stderr, "slopomatic: %v\n", err)
	return code
}

func errorKind(code int, err error) string {
	switch {
	case errors.Is(err, errUnsafeStatePath):
		return "unsafe_state_path"
	case errors.Is(err, errInvalidStateConfig):
		return "invalid_state_config"
	case errors.Is(err, machine.ErrRunExists):
		return "run_exists"
	case errors.Is(err, machine.ErrBadArgs), code == 2:
		return "invalid_input"
	case errors.Is(err, machine.ErrIllegalTransition):
		return "illegal_transition"
	case errors.Is(err, machine.ErrUnmetGuard):
		return "unmet_guard"
	case errors.Is(err, machine.ErrRevisionConflict), code == 4:
		return "revision_conflict"
	case errors.Is(err, machine.ErrAmbiguousRun):
		return "ambiguous_run"
	case errors.Is(err, machine.ErrNotFound):
		return "not_found"
	case errors.Is(err, machine.ErrCorruptState):
		return "corrupt_state"
	case code == 6:
		return "verification_failed"
	default:
		return "internal"
	}
}

func writeJSON(value any) error {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	fmt.Fprintln(os.Stdout, string(encoded))
	return nil
}

func parseFields(value string) ([]string, error) {
	if strings.TrimSpace(value) == "" {
		return nil, fmt.Errorf("--fields requires a comma-separated value")
	}
	seen := map[string]struct{}{}
	fields := make([]string, 0)
	for _, raw := range strings.Split(value, ",") {
		field := strings.TrimSpace(raw)
		if field == "" {
			return nil, fmt.Errorf("--fields contains an empty field")
		}
		if _, ok := seen[field]; ok {
			return nil, fmt.Errorf("--fields repeats %q", field)
		}
		seen[field] = struct{}{}
		fields = append(fields, field)
	}
	return fields, nil
}

func validateRunID(runID string) error {
	if runID == "" {
		return nil
	}
	return machine.ValidateResourceID("run id", runID)
}
