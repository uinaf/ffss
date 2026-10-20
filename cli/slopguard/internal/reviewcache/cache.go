// Package reviewcache keeps validated review results keyed by everything that
// decides them, so an unchanged target returns its earlier result instantly.
package reviewcache

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"syscall"

	"github.com/uinaf/ffss/cli/slopguard/internal/protocol"
)

const (
	stateKeyBytes   = 32
	maxEntryBytes   = int64(16 << 20)
	reviewsDirName  = "reviews"
	stateKeyName    = "state-key"
	temporaryPrefix = ".tmp-"
)

type Store struct {
	directory string
}

// KeyParts lists every input besides the snapshot that changes a review. The
// snapshot hash already covers the bundle, prompt, and context files.
type KeyParts struct {
	SlopguardVersion string `json:"slopguard_version"`
	SnapshotHash     string `json:"snapshot_hash"`
	Engine           string `json:"engine"`
	Model            string `json:"model"`
	ReasoningEffort  string `json:"reasoning_effort"`
	WebAccess        bool   `json:"web_access"`
	RepositoryAccess bool   `json:"repository_access"`
}

func (parts KeyParts) Key() string {
	encoded, _ := json.Marshal(parts)
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

// DefaultDirectory uses an absolute XDG_CACHE_HOME, else the account home.
func DefaultDirectory(lookupEnv func(string) (string, bool), homeDir func() (string, error)) (string, error) {
	if lookupEnv != nil {
		if base, ok := lookupEnv("XDG_CACHE_HOME"); ok && filepath.IsAbs(base) {
			return filepath.Join(base, "slopguard"), nil
		}
	}
	var home string
	var err error
	if homeDir == nil {
		account, accountErr := user.Current()
		if accountErr != nil {
			err = accountErr
		} else {
			home = account.HomeDir
		}
	} else {
		home, err = homeDir()
	}
	if err != nil {
		return "", fmt.Errorf("resolve cache home: %w", err)
	}
	if !filepath.IsAbs(home) {
		return "", fmt.Errorf("cache home must be absolute")
	}
	return filepath.Join(home, ".cache", "slopguard"), nil
}

func Open(directory string) (*Store, error) {
	if err := os.MkdirAll(filepath.Join(directory, reviewsDirName), 0o700); err != nil {
		return nil, fmt.Errorf("create review cache: %w", err)
	}
	for _, path := range []string{directory, filepath.Join(directory, reviewsDirName)} {
		if err := privateDirectory(path); err != nil {
			return nil, err
		}
	}
	return &Store{directory: directory}, nil
}

// StateKey returns the persistent key for local source-state hashes, creating
// it on first use. It never leaves this directory.
func (store *Store) StateKey() ([]byte, error) {
	path := filepath.Join(store.directory, stateKeyName)
	key, err := readPrivateFile(path, stateKeyBytes)
	if err == nil {
		if len(key) != stateKeyBytes {
			return nil, fmt.Errorf("review cache state key has the wrong size")
		}
		return key, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	key = make([]byte, stateKeyBytes)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generate review cache state key: %w", err)
	}
	// Link the finished key into place so concurrent first runs agree on one.
	file, err := os.CreateTemp(store.directory, temporaryPrefix)
	if err != nil {
		return nil, fmt.Errorf("create review cache state key: %w", err)
	}
	temporary := file.Name()
	defer func() { _ = os.Remove(temporary) }()
	_, writeErr := file.Write(key)
	if closeErr := file.Close(); writeErr == nil {
		writeErr = closeErr
	}
	if writeErr != nil {
		return nil, fmt.Errorf("write review cache state key: %w", writeErr)
	}
	if err := os.Link(temporary, path); err != nil && !errors.Is(err, os.ErrExist) {
		return nil, fmt.Errorf("install review cache state key: %w", err)
	}
	return readPrivateFile(path, stateKeyBytes)
}

// Load returns a cached report only when it decodes, validates, finished
// clean or with findings, and describes the same snapshot.
func (store *Store) Load(key, snapshotHash string) (protocol.Report, bool) {
	data, err := readPrivateFile(store.entryPath(key), maxEntryBytes)
	if err != nil {
		return protocol.Report{}, false
	}
	report, err := protocol.DecodeReport(data)
	if err != nil || report.Validate() != nil {
		return protocol.Report{}, false
	}
	if report.Status != protocol.StatusClean && report.Status != protocol.StatusFindings {
		return protocol.Report{}, false
	}
	if report.Metadata.Target == nil || report.Metadata.Target.SnapshotHash != snapshotHash {
		return protocol.Report{}, false
	}
	return report, true
}

func (store *Store) Save(key string, report protocol.Report) error {
	if report.Status != protocol.StatusClean && report.Status != protocol.StatusFindings {
		return fmt.Errorf("only completed reviews are cached")
	}
	data, err := json.Marshal(report)
	if err != nil {
		return fmt.Errorf("encode cached review: %w", err)
	}
	if int64(len(data)) > maxEntryBytes {
		return fmt.Errorf("cached review exceeds %d bytes", maxEntryBytes)
	}
	return writePrivateFile(filepath.Join(store.directory, reviewsDirName), store.entryPath(key), data)
}

func (store *Store) entryPath(key string) string {
	return filepath.Join(store.directory, reviewsDirName, key+".json")
}

func privateDirectory(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("inspect review cache: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("review cache path %q is not a directory", path)
	}
	// Listing a directory reveals only key hashes; entries are private files.
	return ownedWithout(info, path, 0o022)
}

func ownedWithout(info os.FileInfo, path string, forbidden os.FileMode) error {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || int(stat.Uid) != os.Getuid() {
		return fmt.Errorf("review cache path %q is not owned by the current user", path)
	}
	if info.Mode().Perm()&forbidden != 0 {
		return fmt.Errorf("review cache path %q is open to other users", path)
	}
	return nil
}

func readPrivateFile(path string, limit int64) ([]byte, error) {
	file, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("review cache entry %q is not a regular file", path)
	}
	if err := ownedWithout(info, path, 0o077); err != nil {
		return nil, err
	}
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("review cache entry %q exceeds %d bytes", path, limit)
	}
	return data, nil
}

// writePrivateFile replaces path atomically so a reader never sees a partial
// entry.
func writePrivateFile(directory, path string, data []byte) error {
	file, err := os.CreateTemp(directory, temporaryPrefix)
	if err != nil {
		return fmt.Errorf("create review cache entry: %w", err)
	}
	temporary := file.Name()
	_, writeErr := file.Write(data)
	if closeErr := file.Close(); writeErr == nil {
		writeErr = closeErr
	}
	if writeErr == nil {
		writeErr = os.Rename(temporary, path)
	}
	if writeErr != nil {
		_ = os.Remove(temporary)
		return fmt.Errorf("write review cache entry: %w", writeErr)
	}
	return nil
}
