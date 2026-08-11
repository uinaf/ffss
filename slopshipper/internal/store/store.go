package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/uinaf/slopshipper/internal/machine"
	"github.com/uinaf/slopshipper/internal/repo"

	_ "modernc.org/sqlite"
)

const schemaVersion = 2

// Store is the global sqlite-backed run database.
type Store struct {
	db *sql.DB
}

// Open opens or creates the global database at path.
func Open(path string) (*Store, error) {
	resolvedPath, err := resolvedDatabasePath(path)
	if err != nil {
		return nil, err
	}
	directory := filepath.Dir(resolvedPath)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return nil, fmt.Errorf("prepare database %q: create database directory %q: %w", resolvedPath, directory, err)
	}
	databaseFile, err := os.OpenFile(resolvedPath, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, fmt.Errorf("prepare database %q: %w", resolvedPath, err)
	}
	if err := databaseFile.Close(); err != nil {
		return nil, fmt.Errorf("prepare database %q: %w", resolvedPath, err)
	}
	// WAL lets serve read while the CLI writes; -wal/-shm sidecars sit beside the db.
	// _txlock=immediate makes database/sql Begin() acquire a write lock promptly.
	dsn := "file:" + filepath.ToSlash(resolvedPath) + "?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_txlock=immediate"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database %q: %w", resolvedPath, err)
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("initialize database %q: %w", resolvedPath, err)
	}
	return s, nil
}

func OpenReadOnly(path string) (*Store, error) {
	resolvedPath, err := resolvedDatabasePath(path)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(resolvedPath); err != nil {
		return nil, fmt.Errorf("open read-only database %q: %w", resolvedPath, err)
	}
	dsn := "file:" + filepath.ToSlash(resolvedPath) + "?mode=ro&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open read-only database %q: %w", resolvedPath, err)
	}
	db.SetMaxOpenConns(1)
	var version int
	if err := db.QueryRow(`SELECT value FROM meta WHERE key = 'schema_version'`).Scan(&version); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("open read-only database %q: read schema version: %w", resolvedPath, err)
	}
	if version != schemaVersion {
		_ = db.Close()
		return nil, fmt.Errorf("open read-only database %q: schema version %d requires a normal command to migrate to %d", resolvedPath, version, schemaVersion)
	}
	return &Store{db: db}, nil
}

func resolvedDatabasePath(path string) (string, error) {
	if path == "" {
		return "", errors.New("database path is empty")
	}
	resolvedPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve database path %q: %w", path, err)
	}
	return resolvedPath, nil
}

func DefaultPath(xdgDataHome, home string) string {
	base := xdgDataHome
	if base == "" {
		base = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(base, "slopshipper", "slopshipper.sqlite")
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	tx, err := s.beginImmediate()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var hasMeta int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM sqlite_schema WHERE type='table' AND name='meta'`).Scan(&hasMeta); err != nil {
		return err
	}
	if hasMeta == 0 {
		if err := createCurrentSchema(tx); err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO meta(key, value) VALUES ('schema_version', ?)`, fmt.Sprintf("%d", schemaVersion)); err != nil {
			return err
		}
		return tx.Commit()
	}

	var version int
	if err := tx.QueryRow(`SELECT value FROM meta WHERE key = 'schema_version'`).Scan(&version); err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}
	if version > schemaVersion {
		return fmt.Errorf("unsupported schema version %d (want <= %d)", version, schemaVersion)
	}
	for version < schemaVersion {
		switch version {
		case 1:
			if _, err := tx.Exec(`ALTER TABLE runs ADD COLUMN completed_reviewers_json TEXT NOT NULL DEFAULT '[]'`); err != nil {
				return fmt.Errorf("migrate schema 1 to 2: %w", err)
			}
			if _, err := tx.Exec(`UPDATE runs SET state = 'REVIEW', open = 1 WHERE state = 'DELIVER'`); err != nil {
				return fmt.Errorf("reset unverifiable reviews from schema 1: %w", err)
			}
			if _, err := tx.Exec(`UPDATE runs SET return_state = 'REVIEW', open = 1 WHERE return_state = 'DELIVER'`); err != nil {
				return fmt.Errorf("reset parked unverifiable reviews from schema 1: %w", err)
			}
			if _, err := tx.Exec(`UPDATE runs SET open = 1 WHERE state = 'BLOCKED'`); err != nil {
				return fmt.Errorf("reopen blocked schema 1 runs: %w", err)
			}
			version = 2
		default:
			return fmt.Errorf("unsupported schema version %d", version)
		}
		if _, err := tx.Exec(`UPDATE meta SET value = ? WHERE key = 'schema_version'`, fmt.Sprintf("%d", version)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func createCurrentSchema(tx *sql.Tx) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS meta (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS prefs (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS runs (
			id TEXT PRIMARY KEY,
			repo_key TEXT NOT NULL,
			state TEXT NOT NULL,
			intake_revision INTEGER NOT NULL,
			released_revision INTEGER,
			revision INTEGER NOT NULL,
			delivery_mode TEXT NOT NULL,
			review_consent TEXT NOT NULL,
			series_bound INTEGER NOT NULL,
			completed_units INTEGER NOT NULL,
				current_unit_id TEXT NOT NULL DEFAULT '',
				completed_reviewers_json TEXT NOT NULL DEFAULT '[]',
				blocker_reason TEXT NOT NULL DEFAULT '',
			decision_question TEXT NOT NULL DEFAULT '',
			return_state TEXT NOT NULL DEFAULT '',
			open INTEGER NOT NULL DEFAULT 1,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_runs_repo_open ON runs(repo_key, open)`,
		`CREATE TABLE IF NOT EXISTS units (
			run_id TEXT NOT NULL,
			id TEXT NOT NULL,
			title TEXT NOT NULL,
			blockers_json TEXT NOT NULL,
			attempt INTEGER NOT NULL,
			done INTEGER NOT NULL,
			PRIMARY KEY (run_id, id),
			FOREIGN KEY (run_id) REFERENCES runs(id)
		)`,
		`CREATE TABLE IF NOT EXISTS events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			run_id TEXT NOT NULL,
			seq INTEGER NOT NULL,
			at TEXT NOT NULL,
			command TEXT NOT NULL,
			from_state TEXT NOT NULL,
			to_state TEXT NOT NULL,
			evidence_json TEXT NOT NULL,
			UNIQUE(run_id, seq)
		)`,
	}
	for _, q := range stmts {
		if _, err := tx.Exec(q); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) exec(q string, args ...any) error {
	_, err := s.db.Exec(q, args...)
	return err
}

// CreateRun inserts a new run and optional units.
func (s *Store) CreateRun(run machine.Run, units []machine.Unit) error {
	tx, err := s.beginImmediate()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var existing int
	if err := tx.QueryRow(`SELECT 1 FROM runs WHERE id = ?`, run.ID).Scan(&existing); err == nil {
		return fmt.Errorf("%w: run id %q", machine.ErrRunExists, run.ID)
	} else if !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	completedReviewers, err := json.Marshal(run.CompletedReviewers)
	if err != nil {
		return err
	}
	var released any
	if run.ReleasedRevision != nil {
		released = *run.ReleasedRevision
	}
	_, err = tx.Exec(`INSERT INTO runs(
		id, repo_key, state, intake_revision, released_revision, revision,
		delivery_mode, review_consent, series_bound, completed_units,
		current_unit_id, completed_reviewers_json, blocker_reason, decision_question, return_state,
		open, created_at, updated_at
	) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,1,?,?)`,
		run.ID, run.RepoKey, string(run.State), run.IntakeRevision, released, run.Revision,
		string(run.DeliveryMode), string(run.ReviewConsent), run.SeriesBound, run.CompletedUnits,
		run.CurrentUnitID, string(completedReviewers), run.BlockerReason, run.DecisionQuestion, string(run.ReturnState),
		now, now,
	)
	if err != nil {
		return err
	}
	if err := replaceUnitsTx(tx, run.ID, units); err != nil {
		return err
	}
	initEvidence, err := json.Marshal(machine.InitEvidence{
		DeliveryMode: run.DeliveryMode, ReviewConsent: run.ReviewConsent, SeriesBound: run.SeriesBound,
	})
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO events(run_id, seq, at, command, from_state, to_state, evidence_json)
		VALUES (?,?,?,?,?,?,?)`, run.ID, 1, now, string(machine.CmdInit), "", string(run.State), string(initEvidence)); err != nil {
		return err
	}
	return tx.Commit()
}

// ResolveActiveRun returns the open run for repoKey, or the explicit runID.
// When runID is set, the loaded run must still belong to repoKey.
// Empty runID never falls back to closed runs (mutations stay fail-closed).
func (s *Store) ResolveActiveRun(repoKey, runID string) (machine.Run, []machine.Unit, error) {
	if runID != "" {
		return s.getRunForRepo(repoKey, runID)
	}
	ids, err := s.runIDs(repoKey, true)
	if err != nil {
		return machine.Run{}, nil, err
	}
	switch len(ids) {
	case 0:
		return machine.Run{}, nil, fmt.Errorf("%w: no open run for repo", machine.ErrNotFound)
	case 1:
		return s.GetRun(ids[0])
	default:
		return machine.Run{}, nil, fmt.Errorf("%w: %d open runs; pass --run (%s)", machine.ErrAmbiguousRun, len(ids), strings.Join(ids, ", "))
	}
}

// ResolveStatusRun resolves a run for read-only status: explicit ID, else the
// single open run, else the most recently updated run for the repo (including
// RUN_DONE / BLOCKED) so terminal state remains reportable.
func (s *Store) ResolveStatusRun(repoKey, runID string) (machine.Run, []machine.Unit, error) {
	if runID != "" {
		return s.getRunForRepo(repoKey, runID)
	}
	ids, err := s.runIDs(repoKey, true)
	if err != nil {
		return machine.Run{}, nil, err
	}
	switch len(ids) {
	case 1:
		return s.GetRun(ids[0])
	case 0:
		// fall through to latest closed/any
	default:
		return machine.Run{}, nil, fmt.Errorf("%w: %d open runs; pass --run (%s)", machine.ErrAmbiguousRun, len(ids), strings.Join(ids, ", "))
	}
	all, err := s.runIDs(repoKey, false)
	if err != nil {
		return machine.Run{}, nil, err
	}
	if len(all) == 0 {
		return machine.Run{}, nil, fmt.Errorf("%w: no run for repo", machine.ErrNotFound)
	}
	return s.GetRun(all[0])
}

func (s *Store) getRunForRepo(repoKey, runID string) (machine.Run, []machine.Unit, error) {
	run, units, err := s.GetRun(runID)
	if err != nil {
		return machine.Run{}, nil, err
	}
	if run.RepoKey != repoKey {
		return machine.Run{}, nil, fmt.Errorf("%w: run %s belongs to a different repo", machine.ErrNotFound, runID)
	}
	return run, units, nil
}

func (s *Store) runIDs(repoKey string, openOnly bool) ([]string, error) {
	q := `SELECT id FROM runs WHERE repo_key = ?`
	if openOnly {
		q += ` AND open = 1`
	}
	q += ` ORDER BY updated_at DESC, created_at DESC`
	rows, err := s.db.Query(q, repoKey)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (s *Store) GetRun(runID string) (machine.Run, []machine.Unit, error) {
	row := s.db.QueryRow(`SELECT
		id, repo_key, state, intake_revision, released_revision, revision,
		delivery_mode, review_consent, series_bound, completed_units,
		current_unit_id, completed_reviewers_json, blocker_reason, decision_question, return_state
	FROM runs WHERE id = ?`, runID)
	run, err := scanRun(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return machine.Run{}, nil, fmt.Errorf("%w: run %s", machine.ErrNotFound, runID)
		}
		return machine.Run{}, nil, err
	}
	units, err := s.loadUnits(runID)
	return run, units, err
}

type scannable interface {
	Scan(dest ...any) error
}

func scanRun(row scannable) (machine.Run, error) {
	var run machine.Run
	var state, delivery, consent, returnState, completedReviewers string
	var released sql.NullInt64
	err := row.Scan(
		&run.ID, &run.RepoKey, &state, &run.IntakeRevision, &released, &run.Revision,
		&delivery, &consent, &run.SeriesBound, &run.CompletedUnits,
		&run.CurrentUnitID, &completedReviewers, &run.BlockerReason, &run.DecisionQuestion, &returnState,
	)
	if err != nil {
		return machine.Run{}, err
	}
	run.State = machine.State(state)
	run.DeliveryMode = machine.DeliveryMode(delivery)
	run.ReviewConsent = machine.ReviewConsent(consent)
	run.ReturnState = machine.State(returnState)
	if err := json.Unmarshal([]byte(completedReviewers), &run.CompletedReviewers); err != nil {
		return machine.Run{}, fmt.Errorf("decode completed reviewers: %w", err)
	}
	if released.Valid {
		v := released.Int64
		run.ReleasedRevision = &v
	}
	return run, nil
}

func (s *Store) loadUnits(runID string) ([]machine.Unit, error) {
	rows, err := s.db.Query(`SELECT id, title, blockers_json, attempt, done FROM units WHERE run_id = ? ORDER BY rowid`, runID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var units []machine.Unit
	for rows.Next() {
		var u machine.Unit
		var blockers string
		var done int
		if err := rows.Scan(&u.ID, &u.Title, &blockers, &u.Attempt, &done); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(blockers), &u.Blockers); err != nil {
			return nil, err
		}
		u.Done = done == 1
		units = append(units, u)
	}
	return units, rows.Err()
}

// SaveApply persists an ApplyResult with CAS on revision-1 (pre-apply).
func (s *Store) SaveApply(result machine.ApplyResult) error {
	tx, err := s.beginImmediate()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	run := result.Run
	completedReviewers, err := json.Marshal(run.CompletedReviewers)
	if err != nil {
		return err
	}
	prev := run.Revision - 1
	var released any
	if run.ReleasedRevision != nil {
		released = *run.ReleasedRevision
	}
	open := 1
	if run.State == machine.StateRunDone {
		open = 0
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := tx.Exec(`UPDATE runs SET
		state=?, intake_revision=?, released_revision=?, revision=?,
		delivery_mode=?, review_consent=?, series_bound=?, completed_units=?,
		current_unit_id=?, completed_reviewers_json=?, blocker_reason=?, decision_question=?, return_state=?,
		open=?, updated_at=?
	WHERE id=? AND revision=?`,
		string(run.State), run.IntakeRevision, released, run.Revision,
		string(run.DeliveryMode), string(run.ReviewConsent), run.SeriesBound, run.CompletedUnits,
		run.CurrentUnitID, string(completedReviewers), run.BlockerReason, run.DecisionQuestion, string(run.ReturnState),
		open, now, run.ID, prev,
	)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return fmt.Errorf("%w: run %s revision %d", machine.ErrRevisionConflict, run.ID, prev)
	}
	if err := replaceUnitsTx(tx, run.ID, result.Units); err != nil {
		return err
	}
	evJSON, err := json.Marshal(result.Evidence)
	if err != nil {
		return err
	}
	if result.Evidence == nil {
		evJSON = []byte("{}")
	}
	var seq int
	if err := tx.QueryRow(`SELECT COALESCE(MAX(seq), 0) + 1 FROM events WHERE run_id = ?`, run.ID).Scan(&seq); err != nil {
		return err
	}
	_, err = tx.Exec(`INSERT INTO events(run_id, seq, at, command, from_state, to_state, evidence_json)
		VALUES (?,?,?,?,?,?,?)`,
		run.ID, seq, now, string(result.Command), string(result.EventFrom), string(result.EventTo), string(evJSON),
	)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func replaceUnitsTx(tx *sql.Tx, runID string, units []machine.Unit) error {
	if _, err := tx.Exec(`DELETE FROM units WHERE run_id = ?`, runID); err != nil {
		return err
	}
	for _, u := range units {
		b, err := json.Marshal(u.Blockers)
		if err != nil {
			return err
		}
		if b == nil {
			b = []byte("[]")
		}
		done := 0
		if u.Done {
			done = 1
		}
		if _, err := tx.Exec(`INSERT INTO units(run_id, id, title, blockers_json, attempt, done) VALUES (?,?,?,?,?,?)`,
			runID, u.ID, u.Title, string(b), u.Attempt, done); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) beginImmediate() (*sql.Tx, error) {
	// Open DSN sets _txlock=immediate; MaxOpenConns(1) serializes writers for v0.
	return s.db.Begin()
}

// SetPref / GetPref for global defaults.
func (s *Store) SetPref(key, value string) error {
	return s.exec(`INSERT INTO prefs(key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
}

func (s *Store) GetPref(key string) (string, bool, error) {
	var v string
	err := s.db.QueryRow(`SELECT value FROM prefs WHERE key = ?`, key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	return v, err == nil, err
}

// RekeyRepo replaces persisted identities for this checkout with the sanitized form.
func (s *Store) RekeyRepo(newKey, root string) error {
	if newKey == "" || root == "" {
		return nil
	}
	tx, err := s.beginImmediate()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	suffix := "|" + root
	rows, err := tx.Query(`SELECT id, repo_key FROM runs WHERE repo_key = ? OR substr(repo_key, -length(?)) = ?`, root, suffix, suffix)
	if err != nil {
		return err
	}
	targets := make(map[string]string)
	for rows.Next() {
		var id, candidate string
		if err := rows.Scan(&id, &candidate); err != nil {
			_ = rows.Close()
			return err
		}
		if sanitized, ok := repo.SanitizeKey(candidate, root); ok {
			if repo.MatchesKey(candidate, newKey, root) {
				sanitized = newKey
			}
			targets[id] = sanitized
		}
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for id, target := range targets {
		if _, err := tx.Exec(`UPDATE runs SET repo_key = ? WHERE id = ?`, target, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}
