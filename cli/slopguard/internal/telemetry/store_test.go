package telemetry

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/uinaf/ffss/cli/slopguard/internal/protocol"
	"golang.org/x/sys/unix"
)

func TestStoreBoundsConcurrentAppends(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), "state", "telemetry.jsonl")}
	const writers = 64
	var wait sync.WaitGroup
	errs := make(chan error, writers)
	for range writers {
		wait.Add(1)
		go func() {
			defer wait.Done()
			errs <- store.Append(validEvent())
		}()
	}
	wait.Wait()
	close(errs)
	succeeded := 0
	for err := range errs {
		if err == nil {
			succeeded++
		} else if !errors.Is(err, errStoreBusy) {
			t.Fatal(err)
		}
	}
	exported := exportStore(t, store)
	if succeeded == 0 || len(exported.Events) != succeeded {
		t.Fatalf("events = %d, succeeded = %d", len(exported.Events), succeeded)
	}
	info, err := os.Stat(store.Path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 || info.Size() > maximumStoreBytes {
		t.Fatalf("store mode=%o size=%d", info.Mode().Perm(), info.Size())
	}
}

func TestStoreSerializesConcurrentProcesses(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), "telemetry.jsonl")}
	t.Setenv("SLOPGUARD_TEST_TELEMETRY_HELPER", "ambient")
	t.Setenv("SLOPGUARD_TEST_TELEMETRY_PATH", filepath.Join(t.TempDir(), "ambient.jsonl"))
	const processes = 12
	commands := make([]*exec.Cmd, 0, processes)
	outputs := make([]bytes.Buffer, processes)
	for index := range processes {
		command := exec.Command(os.Args[0], "-test.run=^TestStoreAppendHelperProcess$", "-test.count=1")
		command.Env = []string{
			"SLOPGUARD_TEST_TELEMETRY_HELPER=1",
			"SLOPGUARD_TEST_TELEMETRY_PATH=" + store.Path,
		}
		for _, entry := range os.Environ() {
			if !strings.HasPrefix(entry, "SLOPGUARD_TEST_TELEMETRY_HELPER=") &&
				!strings.HasPrefix(entry, "SLOPGUARD_TEST_TELEMETRY_PATH=") {
				command.Env = append(command.Env, entry)
			}
		}
		command.Stdout = &outputs[index]
		command.Stderr = &outputs[index]
		if err := command.Start(); err != nil {
			t.Fatal(err)
		}
		commands = append(commands, command)
	}
	for index, command := range commands {
		if err := command.Wait(); err != nil {
			t.Fatalf("helper %d: %v: %s", index, err, outputs[index].String())
		}
	}
	if events := exportStore(t, store).Events; len(events) == 0 || len(events) > processes {
		t.Fatalf("events = %d, want 1..%d", len(events), processes)
	}
}

func TestStoreAppendHelperProcess(t *testing.T) {
	if os.Getenv("SLOPGUARD_TEST_TELEMETRY_HELPER") != "1" {
		return
	}
	path := os.Getenv("SLOPGUARD_TEST_TELEMETRY_PATH")
	if path == "" {
		t.Fatal("helper path is missing")
	}
	if err := (Store{Path: path}).Append(validEvent()); err != nil && !errors.Is(err, errStoreBusy) {
		t.Fatal(err)
	}
}

func TestStoreAppendDoesNotWaitForAnotherProcess(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), "telemetry.jsonl")}
	lock, err := os.OpenFile(store.Path+".lock", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = lock.Close() }()
	if err := unix.Flock(int(lock.Fd()), unix.LOCK_EX); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = unix.Flock(int(lock.Fd()), unix.LOCK_UN) }()

	started := time.Now()
	err = store.Append(validEvent())
	if !errors.Is(err, errStoreBusy) {
		t.Fatalf("error = %v, want store busy", err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("contended append took %s", elapsed)
	}
}

func TestStoreRetentionAndCorruptionRecovery(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), "telemetry.jsonl")}
	for range maximumEvents + 20 {
		if err := store.Append(validEvent()); err != nil {
			t.Fatal(err)
		}
	}
	exported := exportStore(t, store)
	if len(exported.Events) != maximumEvents {
		t.Fatalf("retained events = %d, want %d", len(exported.Events), maximumEvents)
	}
	valid, err := json.Marshal(validEvent())
	if err != nil {
		t.Fatal(err)
	}
	corrupt := append([]byte("PRIVATE-CORRUPTION\n"), valid...)
	corrupt = append(corrupt, '\n')
	if err := os.WriteFile(store.Path, corrupt, 0o600); err != nil {
		t.Fatal(err)
	}
	exported = exportStore(t, store)
	if len(exported.Events) != 1 {
		t.Fatalf("recovered events = %d", len(exported.Events))
	}
	content, err := os.ReadFile(store.Path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(content, []byte("PRIVATE-CORRUPTION")) {
		t.Fatalf("corruption was retained: %s", content)
	}
	unknown := append([]byte(`{"prompt":"PRIVATE-UNKNOWN",`), valid[1:]...)
	withUnknown := append(append(append(append([]byte{}, valid...), '\n'), unknown...), '\n')
	withUnknown = append(withUnknown, valid...)
	withUnknown = append(withUnknown, '\n')
	if err := os.WriteFile(store.Path, withUnknown, 0o600); err != nil {
		t.Fatal(err)
	}
	if events := exportStore(t, store).Events; len(events) != 2 {
		t.Fatalf("events around unknown field = %d, want 2", len(events))
	}
	content, err = os.ReadFile(store.Path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(content, []byte("PRIVATE-UNKNOWN")) {
		t.Fatalf("unknown private field was retained: %s", content)
	}
	oversized := append(append(append([]byte{}, valid...), '\n'), bytes.Repeat([]byte("x"), int(maximumStoreBytes)+(70<<10))...)
	oversized = append(oversized, '\n')
	oversized = append(oversized, valid...)
	oversized = append(oversized, '\n')
	if err := os.WriteFile(store.Path, oversized, 0o600); err != nil {
		t.Fatal(err)
	}
	if events := exportStore(t, store).Events; len(events) != 2 {
		t.Fatalf("events around oversized corruption = %d, want 2", len(events))
	}
	line := append(append([]byte{}, valid...), '\n')
	fillerSize := int(maximumRecoveryBytes) - (2 * len(line))
	tail := append([]byte{}, line...)
	tail = append(tail, bytes.Repeat([]byte("x"), fillerSize-1)...)
	tail = append(tail, '\n')
	tail = append(tail, line...)
	if len(tail) != int(maximumRecoveryBytes) {
		t.Fatalf("tail bytes = %d, want %d", len(tail), maximumRecoveryBytes)
	}
	prefix := []byte("PRIVATE-PREFIX\n")
	if err := os.WriteFile(store.Path, append(prefix, tail...), 0o600); err != nil {
		t.Fatal(err)
	}
	if events := exportStore(t, store).Events; len(events) != 2 {
		t.Fatalf("events at exact recovery boundary = %d, want 2", len(events))
	}
	if err := os.WriteFile(store.Path, bytes.Repeat(line, maximumEvents+500), 0o600); err != nil {
		t.Fatal(err)
	}
	if events := exportStore(t, store).Events; len(events) != maximumEvents {
		t.Fatalf("events from oversized valid store = %d, want %d", len(events), maximumEvents)
	}
}

func TestExportFailureLeavesStoreReadable(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), "telemetry.jsonl")}
	if err := store.Append(validEvent()); err != nil {
		t.Fatal(err)
	}
	if err := store.Export(failingWriter{}); err == nil {
		t.Fatal("expected export failure")
	}
	if events := exportStore(t, store).Events; len(events) != 1 {
		t.Fatalf("events after failed export = %d", len(events))
	}
}

func TestStoreRecoversFIFOWithoutBlocking(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), "telemetry.jsonl")}
	if err := unix.Mkfifo(store.Path, 0o600); err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	if err := store.Append(validEvent()); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("FIFO recovery took %s", elapsed)
	}
	info, err := os.Lstat(store.Path)
	if err != nil {
		t.Fatal(err)
	}
	if !info.Mode().IsRegular() || len(exportStore(t, store).Events) != 1 {
		t.Fatalf("recovered store mode=%s", info.Mode())
	}
}

func TestStoreRecoversEmptyDirectoryWithoutDeletingContent(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		store := Store{Path: filepath.Join(t.TempDir(), "telemetry.jsonl")}
		if err := os.Mkdir(store.Path, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := store.Append(validEvent()); err != nil {
			t.Fatal(err)
		}
		info, err := os.Lstat(store.Path)
		if err != nil || !info.Mode().IsRegular() || len(exportStore(t, store).Events) != 1 {
			t.Fatalf("recovered store info=%v error=%v", info, err)
		}
	})

	t.Run("non-empty", func(t *testing.T) {
		store := Store{Path: filepath.Join(t.TempDir(), "telemetry.jsonl")}
		if err := os.Mkdir(store.Path, 0o700); err != nil {
			t.Fatal(err)
		}
		preserved := filepath.Join(store.Path, "preserved")
		if err := os.WriteFile(preserved, []byte("keep"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := store.Append(validEvent()); err == nil {
			t.Fatal("expected non-empty directory recovery to fail")
		}
		if content, err := os.ReadFile(preserved); err != nil || string(content) != "keep" {
			t.Fatalf("preserved content=%q error=%v", content, err)
		}
	})
}

func TestDefaultPathRejectsRelativeHome(t *testing.T) {
	if _, err := DefaultPath(func() (string, error) { return "relative", nil }); err == nil {
		t.Fatal("expected relative home to fail")
	}
}

func TestDefaultPathUsesAccountHomeInsteadOfEnvironment(t *testing.T) {
	account, err := user.Current()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", filepath.Join(t.TempDir(), "overridden-home"))
	path, err := DefaultPath(nil)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(account.HomeDir, ".local", "state", "slopguard", "telemetry.jsonl")
	if path != want {
		t.Fatalf("path = %q, want %q", path, want)
	}
}

func validEvent() Event {
	return Event{
		SchemaVersion: SchemaVersion, CLIVersion: "development", ReportSchemaVersion: protocol.SchemaVersion,
		Outcome: string(protocol.StatusClean), AttemptOutcomes: []string{string(protocol.AttemptValid)},
		BundleBucket: "1-64KiB", FindingCountBucket: "0", PhaseDurationBuckets: map[string]string{},
	}
}

func exportStore(t *testing.T, store Store) Export {
	t.Helper()
	var output bytes.Buffer
	if err := store.Export(&output); err != nil {
		t.Fatal(err)
	}
	var exported Export
	if err := json.Unmarshal(output.Bytes(), &exported); err != nil {
		t.Fatal(err)
	}
	return exported
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errors.New("injected write failure")
}
