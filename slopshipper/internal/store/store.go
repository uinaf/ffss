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

	"github.com/uinaf/slopomatic/internal/machine"

	_ "modernc.org/sqlite"
)

const schemaVersion = 1

// Store is the global sqlite-backed run database.
type Store struct {
	db *sql.DB
}

// Open opens or creates the global database at path.
func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	// WAL lets serve read while the CLI writes; -wal/-shm sidecars sit beside the db.
	// _txlock=immediate makes database/sql Begin() acquire a write lock promptly.
	dsn := "file:" + filepath.ToSlash(path) + "?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_txlock=immediate"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func DefaultPath(xdgDataHome, home string) string {
	base := xdgDataHome
	if base == "" {
		base = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(base, "slopomatic", "slopomatic.sqlite")
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
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
		if err := s.exec(q); err != nil {
			return err
		}
	}
	var v string
	err := s.db.QueryRow(`SELECT value FROM meta WHERE key = 'schema_version'`).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return s.exec(`INSERT INTO meta(key, value) VALUES ('schema_version', ?)`, fmt.Sprintf("%d", schemaVersion))
	}
	if err != nil {
		return err
	}
	if v != fmt.Sprintf("%d", schemaVersion) {
		return fmt.Errorf("unsupported schema version %s (want %d)", v, schemaVersion)
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

	now := time.Now().UTC().Format(time.RFC3339Nano)
	var released any
	if run.ReleasedRevision != nil {
		released = *run.ReleasedRevision
	}
	_, err = tx.Exec(`INSERT INTO runs(
		id, repo_key, state, intake_revision, released_revision, revision,
		delivery_mode, review_consent, series_bound, completed_units,
		current_unit_id, blocker_reason, decision_question, return_state,
		open, created_at, updated_at
	) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,1,?,?)`,
		run.ID, run.RepoKey, string(run.State), run.IntakeRevision, released, run.Revision,
		string(run.DeliveryMode), string(run.ReviewConsent), run.SeriesBound, run.CompletedUnits,
		run.CurrentUnitID, run.BlockerReason, run.DecisionQuestion, string(run.ReturnState),
		now, now,
	)
	if err != nil {
		return err
	}
	if err := replaceUnitsTx(tx, run.ID, units); err != nil {
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
		current_unit_id, blocker_reason, decision_question, return_state
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
	var state, delivery, consent, returnState string
	var released sql.NullInt64
	err := row.Scan(
		&run.ID, &run.RepoKey, &state, &run.IntakeRevision, &released, &run.Revision,
		&delivery, &consent, &run.SeriesBound, &run.CompletedUnits,
		&run.CurrentUnitID, &run.BlockerReason, &run.DecisionQuestion, &returnState,
	)
	if err != nil {
		return machine.Run{}, err
	}
	run.State = machine.State(state)
	run.DeliveryMode = machine.DeliveryMode(delivery)
	run.ReviewConsent = machine.ReviewConsent(consent)
	run.ReturnState = machine.State(returnState)
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
func (s *Store) SaveApply(result machine.ApplyResult, evidence any) error {
	tx, err := s.beginImmediate()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	run := result.Run
	prev := run.Revision - 1
	var released any
	if run.ReleasedRevision != nil {
		released = *run.ReleasedRevision
	}
	open := 1
	if run.State == machine.StateRunDone || run.State == machine.StateBlocked {
		open = 0
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := tx.Exec(`UPDATE runs SET
		state=?, intake_revision=?, released_revision=?, revision=?,
		delivery_mode=?, review_consent=?, series_bound=?, completed_units=?,
		current_unit_id=?, blocker_reason=?, decision_question=?, return_state=?,
		open=?, updated_at=?
	WHERE id=? AND revision=?`,
		string(run.State), run.IntakeRevision, released, run.Revision,
		string(run.DeliveryMode), string(run.ReviewConsent), run.SeriesBound, run.CompletedUnits,
		run.CurrentUnitID, run.BlockerReason, run.DecisionQuestion, string(run.ReturnState),
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
	evJSON, err := json.Marshal(evidence)
	if err != nil {
		return err
	}
	if evidence == nil {
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
