package reviewcache

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/uinaf/ffss/cli/slopguard/internal/protocol"
)

func fixture(t *testing.T) protocol.Report {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "protocol", "testdata", "report-findings.json"))
	if err != nil {
		t.Fatal(err)
	}
	report, err := protocol.DecodeReport(data)
	if err != nil {
		t.Fatal(err)
	}
	return report
}

func TestStoreRoundTripsOnlyMatchingCompletedReviews(t *testing.T) {
	t.Parallel()

	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	report := fixture(t)
	snapshot := report.Metadata.Target.SnapshotHash
	key := KeyParts{SnapshotHash: snapshot, Engine: "codex"}.Key()
	if err := store.Save(key, report); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if got, ok := store.Load(key, snapshot); !ok || got.Status != report.Status {
		t.Fatalf("Load = %+v, %t", got, ok)
	}
	if _, ok := store.Load(key, "sha256:other"); ok {
		t.Fatal("Load accepted a different snapshot")
	}
	if _, ok := store.Load(KeyParts{SnapshotHash: snapshot, Engine: "claude"}.Key(), snapshot); ok {
		t.Fatal("Load accepted another engine's key")
	}
	failure := report
	failure.Status = protocol.StatusFailure
	if err := store.Save(key, failure); err == nil {
		t.Fatal("Save accepted a failed review")
	}
}

func TestStoreRejectsSharedOrLinkedEntries(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	store, err := Open(directory)
	if err != nil {
		t.Fatal(err)
	}
	report := fixture(t)
	snapshot := report.Metadata.Target.SnapshotHash
	key := KeyParts{SnapshotHash: snapshot}.Key()
	if err := store.Save(key, report); err != nil {
		t.Fatal(err)
	}
	entry := store.entryPath(key)
	if err := os.Chmod(entry, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok := store.Load(key, snapshot); ok {
		t.Fatal("Load accepted a world-readable entry")
	}
	if err := os.Remove(entry); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(KeyParts{}.Key(), report); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(store.entryPath(KeyParts{}.Key()), entry); err != nil {
		t.Fatal(err)
	}
	if _, ok := store.Load(key, snapshot); ok {
		t.Fatal("Load followed a symlinked entry")
	}
	if err := os.Chmod(directory, 0o777); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(directory); err == nil {
		t.Fatal("Open accepted a directory other users can write")
	}
}

func TestStateKeyPersists(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	first, err := Open(directory)
	if err != nil {
		t.Fatal(err)
	}
	key, err := first.StateKey()
	if err != nil || len(key) != stateKeyBytes {
		t.Fatalf("StateKey = %x, %v", key, err)
	}
	second, err := Open(directory)
	if err != nil {
		t.Fatal(err)
	}
	again, err := second.StateKey()
	if err != nil || string(again) != string(key) {
		t.Fatalf("second StateKey = %x, %v; want %x", again, err, key)
	}
}

func TestStorePrunesToNewestEntries(t *testing.T) {
	t.Parallel()

	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	report := fixture(t)
	for index := range maxEntries + 3 {
		if err := store.Save(KeyParts{Retries: index}.Key(), report); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := os.ReadDir(filepath.Join(store.directory, reviewsDirName))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != maxEntries {
		t.Fatalf("cache holds %d entries, want %d", len(entries), maxEntries)
	}
}
