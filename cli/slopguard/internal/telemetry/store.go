package telemetry

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/sys/unix"
)

const (
	maximumEvents        = 256
	maximumStoreBytes    = int64(1 << 20)
	maximumRecoveryBytes = int64(8 << 20)
)

type Store struct {
	Path string
}

var processStoreLock sync.Mutex

type Export struct {
	SchemaVersion string  `json:"schema_version"`
	Events        []Event `json:"events"`
}

func DefaultPath(homeDir func() (string, error)) (string, error) {
	var home string
	var err error
	if homeDir == nil {
		home, err = os.UserHomeDir()
	} else {
		home, err = homeDir()
	}
	if err != nil {
		return "", fmt.Errorf("resolve telemetry home: %w", err)
	}
	if !filepath.IsAbs(home) {
		return "", fmt.Errorf("telemetry home must be absolute")
	}
	return filepath.Join(home, ".local", "state", "slopguard", "telemetry.jsonl"), nil
}

func (store Store) Append(event Event) error {
	if err := event.Validate(); err != nil {
		return err
	}
	return store.withLock(func() error {
		events, _, err := store.load()
		if err != nil {
			return err
		}
		events = append(events, event)
		if len(events) > maximumEvents {
			events = events[len(events)-maximumEvents:]
		}
		return store.writeBounded(events)
	})
}

func (store Store) Export(output io.Writer) error {
	if output == nil {
		return fmt.Errorf("telemetry export output is unavailable")
	}
	var exported Export
	if err := store.withLock(func() error {
		events, corrupted, err := store.load()
		if err != nil {
			return err
		}
		if corrupted {
			if err := store.writeBounded(events); err != nil {
				return err
			}
		}
		exported = Export{SchemaVersion: SchemaVersion, Events: events}
		return nil
	}); err != nil {
		return err
	}
	encoder := json.NewEncoder(output)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(exported)
}

func (store Store) withLock(operation func() error) error {
	processStoreLock.Lock()
	defer processStoreLock.Unlock()
	if !filepath.IsAbs(store.Path) {
		return fmt.Errorf("telemetry store path must be absolute")
	}
	directory := filepath.Dir(store.Path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("create telemetry directory: %w", err)
	}
	lock, err := os.OpenFile(store.Path+".lock", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("open telemetry lock: %w", err)
	}
	defer func() { _ = lock.Close() }()
	if err := unix.Flock(int(lock.Fd()), unix.LOCK_EX); err != nil {
		return fmt.Errorf("lock telemetry store: %w", err)
	}
	defer func() { _ = unix.Flock(int(lock.Fd()), unix.LOCK_UN) }()
	return operation()
}

func (store Store) load() ([]Event, bool, error) {
	pathInfo, err := os.Lstat(store.Path)
	if errors.Is(err, os.ErrNotExist) {
		return []Event{}, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("inspect telemetry store path: %w", err)
	}
	if !pathInfo.Mode().IsRegular() {
		return []Event{}, true, nil
	}
	fileDescriptor, err := unix.Open(store.Path, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NONBLOCK|unix.O_NOFOLLOW, 0)
	if errors.Is(err, unix.ENOENT) || errors.Is(err, unix.ELOOP) || errors.Is(err, unix.ENXIO) {
		return []Event{}, true, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("open telemetry store: %w", err)
	}
	file := os.NewFile(uintptr(fileDescriptor), store.Path)
	defer func() { _ = file.Close() }()
	info, err := file.Stat()
	if err != nil {
		return nil, false, fmt.Errorf("inspect telemetry store: %w", err)
	}
	if !info.Mode().IsRegular() {
		return []Event{}, true, nil
	}
	corrupted := info.Size() > maximumStoreBytes
	start := int64(0)
	skippingLine := false
	if info.Size() > maximumRecoveryBytes {
		start = info.Size() - maximumRecoveryBytes
		var previous [1]byte
		if _, err := file.ReadAt(previous[:], start-1); err != nil {
			return nil, false, fmt.Errorf("inspect telemetry recovery boundary: %w", err)
		}
		skippingLine = previous[0] != '\n'
		if _, err := file.Seek(start, io.SeekStart); err != nil {
			return nil, false, fmt.Errorf("seek telemetry store: %w", err)
		}
		corrupted = true
	}
	reader := bufio.NewReaderSize(io.LimitReader(file, maximumRecoveryBytes), 64<<10)
	eventRing := make([]Event, maximumEvents)
	eventCount := 0
	eventStart := 0
	for {
		line, readErr := reader.ReadSlice('\n')
		completeLine := !errors.Is(readErr, bufio.ErrBufferFull)
		switch {
		case skippingLine:
			corrupted = true
			if completeLine {
				skippingLine = false
			}
		case errors.Is(readErr, bufio.ErrBufferFull):
			corrupted = true
			skippingLine = true
		case len(line) != 0:
			line = bytes.TrimSpace(line)
			if len(line) == 0 {
				corrupted = true
			} else {
				event, err := decodeEvent(line)
				if err != nil {
					corrupted = true
				} else {
					if eventCount < maximumEvents {
						eventRing[eventCount] = event
						eventCount++
					} else {
						eventRing[eventStart] = event
						eventStart = (eventStart + 1) % maximumEvents
						corrupted = true
					}
				}
			}
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if errors.Is(readErr, bufio.ErrBufferFull) {
			continue
		}
		if readErr != nil {
			corrupted = true
			break
		}
	}
	events := make([]Event, eventCount)
	for index := range eventCount {
		events[index] = eventRing[(eventStart+index)%maximumEvents]
	}
	return events, corrupted, nil
}

func (store Store) writeBounded(events []Event) error {
	encoded := make([][]byte, 0, len(events))
	total := int64(0)
	for _, event := range events {
		line, err := json.Marshal(event)
		if err != nil {
			return fmt.Errorf("encode telemetry event: %w", err)
		}
		line = append(line, '\n')
		encoded = append(encoded, line)
		total += int64(len(line))
	}
	for total > maximumStoreBytes && len(encoded) > 0 {
		total -= int64(len(encoded[0]))
		encoded = encoded[1:]
	}
	temporary, err := os.CreateTemp(filepath.Dir(store.Path), ".telemetry-*")
	if err != nil {
		return fmt.Errorf("create telemetry store: %w", err)
	}
	temporaryPath := temporary.Name()
	closed := false
	defer func() {
		if !closed {
			_ = temporary.Close()
		}
		_ = os.Remove(temporaryPath)
	}()
	if err := temporary.Chmod(0o600); err != nil {
		return err
	}
	for _, line := range encoded {
		if _, err := temporary.Write(line); err != nil {
			return fmt.Errorf("write telemetry store: %w", err)
		}
	}
	if err := temporary.Sync(); err != nil {
		return fmt.Errorf("sync telemetry store: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close telemetry store: %w", err)
	}
	closed = true
	if info, err := os.Lstat(store.Path); err == nil && !info.Mode().IsRegular() {
		if err := os.Remove(store.Path); err != nil {
			return fmt.Errorf("remove non-regular telemetry store: %w", err)
		}
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect telemetry store destination: %w", err)
	}
	if err := os.Rename(temporaryPath, store.Path); err != nil {
		return fmt.Errorf("replace telemetry store: %w", err)
	}
	return nil
}
