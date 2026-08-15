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

	"github.com/uinaf/ffsstack/cli/slopmachine/internal/machine"
	"github.com/uinaf/ffsstack/cli/slopmachine/internal/repo"

	_ "modernc.org/sqlite"
)

const schemaVersion = 9

// timestampNow returns a fixed-width UTC timestamp. RFC3339Nano trims
// trailing zeros, which breaks the lexicographic ordering the run queries
// rely on; a constant-width fraction keeps string order equal to time order.
func timestampNow() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05.000000000Z")
}

// ErrStateUnavailable marks a resolved state location that cannot be
// prepared (directory or database file creation failed). Callers recover by
// selecting a writable location, not by treating the failure as internal.
var ErrStateUnavailable = errors.New("state storage unavailable")

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
		return nil, fmt.Errorf("%w: prepare database %q: create database directory %q: %w", ErrStateUnavailable, resolvedPath, directory, err)
	}
	databaseFile, err := os.OpenFile(resolvedPath, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, fmt.Errorf("%w: prepare database %q: %w", ErrStateUnavailable, resolvedPath, err)
	}
	if err := databaseFile.Close(); err != nil {
		return nil, fmt.Errorf("%w: prepare database %q: %w", ErrStateUnavailable, resolvedPath, err)
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
	if strings.ContainsAny(path, "?#") {
		return "", fmt.Errorf("%w: database path %q contains a SQLite URI delimiter ('?' or '#'); use a path without them", ErrStateUnavailable, path)
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
	return filepath.Join(base, "slopmachine", "slopmachine.sqlite")
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
		case 2:
			var unknownConsent int
			if err := tx.QueryRow(`SELECT COUNT(*) FROM runs
				WHERE review_consent NOT IN ('autoreview','bugbot','both','human')`).Scan(&unknownConsent); err != nil {
				return fmt.Errorf("inspect legacy review consent: %w", err)
			}
			if unknownConsent > 0 {
				return fmt.Errorf("%d run(s) carry an unknown legacy review_consent; refusing to map them to a weaker policy", unknownConsent)
			}
			if _, err := tx.Exec(`ALTER TABLE runs ADD COLUMN required_reviewers_json TEXT NOT NULL DEFAULT '[]'`); err != nil {
				return fmt.Errorf("migrate schema 2 to 3: %w", err)
			}
			if _, err := tx.Exec(`UPDATE runs SET required_reviewers_json = CASE review_consent
				WHEN 'autoreview' THEN '["autoreview"]'
				WHEN 'bugbot' THEN '["bugbot"]'
				WHEN 'both' THEN '["autoreview","bugbot"]'
				WHEN 'human' THEN '["human"]'
			END`); err != nil {
				return fmt.Errorf("map review_consent to required reviewers: %w", err)
			}
			if _, err := tx.Exec(`ALTER TABLE runs DROP COLUMN review_consent`); err != nil {
				return fmt.Errorf("drop review_consent from schema 2: %w", err)
			}
			if _, err := tx.Exec(`CREATE TABLE IF NOT EXISTS reviewers (
				name TEXT PRIMARY KEY,
				created_at TEXT NOT NULL
			)`); err != nil {
				return fmt.Errorf("create reviewers registry from schema 2: %w", err)
			}
			// Human is no longer a built-in reviewer; keep legacy human-consent
			// runs valid by registering the identity those runs require.
			if _, err := tx.Exec(`INSERT OR IGNORE INTO reviewers(name, created_at)
				SELECT 'human', ? WHERE EXISTS (
					SELECT 1 FROM runs WHERE required_reviewers_json = '["human"]'
				)`, timestampNow()); err != nil {
				return fmt.Errorf("register legacy human reviewer from schema 2: %w", err)
			}
			version = 3
		case 3:
			for _, ddl := range []string{
				`ALTER TABLE runs ADD COLUMN risk_tier TEXT NOT NULL DEFAULT ''`,
				`ALTER TABLE runs ADD COLUMN budget_json TEXT NOT NULL DEFAULT '{}'`,
				`ALTER TABLE units ADD COLUMN acceptance_criteria_json TEXT NOT NULL DEFAULT '[]'`,
				`ALTER TABLE units ADD COLUMN complexity TEXT NOT NULL DEFAULT ''`,
			} {
				if _, err := tx.Exec(ddl); err != nil {
					return fmt.Errorf("migrate schema 3 to 4: %w", err)
				}
			}
			version = 4
		case 4:
			for _, ddl := range []string{
				`ALTER TABLE units ADD COLUMN phase TEXT NOT NULL DEFAULT 'pending'`,
				`ALTER TABLE units ADD COLUMN rework_cause TEXT NOT NULL DEFAULT ''`,
			} {
				if _, err := tx.Exec(ddl); err != nil {
					return fmt.Errorf("migrate schema 4 to 5: %w", err)
				}
			}
			// Schema 4 marked settled units done at delivery; map that onto
			// phases and re-mark the active pipeline unit.
			if _, err := tx.Exec(`UPDATE units SET phase = CASE done WHEN 1 THEN 'done' ELSE 'pending' END`); err != nil {
				return fmt.Errorf("map unit done flags to phases: %w", err)
			}
			if _, err := tx.Exec(`UPDATE units SET phase = 'active'
				WHERE phase = 'pending' AND EXISTS (
					SELECT 1 FROM runs WHERE runs.id = units.run_id AND runs.current_unit_id = units.id
				)`); err != nil {
				return fmt.Errorf("mark active units from schema 4: %w", err)
			}
			if _, err := tx.Exec(`ALTER TABLE units DROP COLUMN done`); err != nil {
				return fmt.Errorf("drop unit done flag from schema 4: %w", err)
			}
			version = 5
		case 5:
			// The historical v6 shape; case 7 adds forge_reviewers_json, so
			// replaying old databases lands on the same columns as a fresh
			// create.
			if _, err := tx.Exec(`CREATE TABLE IF NOT EXISTS repos (
	repo_key TEXT PRIMARY KEY,
	forge_kind TEXT NOT NULL DEFAULT '',
	trust_tier TEXT NOT NULL DEFAULT '',
	verify_command TEXT NOT NULL DEFAULT '',
	delivery_mode TEXT NOT NULL DEFAULT '',
	readiness TEXT NOT NULL DEFAULT '',
	bindings_json TEXT NOT NULL DEFAULT '{}',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
)`); err != nil {
				return fmt.Errorf("migrate schema 5 to 6: %w", err)
			}
			version = 6
		case 6:
			if _, err := tx.Exec(`ALTER TABLE events ADD COLUMN telemetry_json TEXT NOT NULL DEFAULT '{}'`); err != nil {
				return fmt.Errorf("migrate schema 6 to 7: %w", err)
			}
			version = 7
		case 7:
			if _, err := tx.Exec(`ALTER TABLE repos ADD COLUMN forge_reviewers_json TEXT NOT NULL DEFAULT '{}'`); err != nil {
				return fmt.Errorf("migrate schema 7 to 8: %w", err)
			}
			version = 8
		case 8:
			// The autoreview built-in identity was renamed to slopguard.
			// Live run state and profile bindings follow the rename; the
			// event ledger keeps historical evidence verbatim. The quoted
			// JSON tokens are unambiguous because identities are whole JSON
			// strings in these documents.
			//
			// At v8 "slopguard" can only be a CUSTOM identity; renaming
			// autoreview onto it would merge two distinct reviewers and
			// weaken existing review gates, in any row or field. Refuse
			// whenever the name occupies the reviewer-identity namespace —
			// the registry, run reviewer arrays, REVIEW role bindings, and
			// forge-reviewer keys. Other role bindings and forge-reviewer
			// values are vendor/login names, not reviewer identities, and
			// are deliberately neither inspected nor renamed.
			var occupied int
			if err := tx.QueryRow(`SELECT
				(SELECT COUNT(*) FROM reviewers WHERE name = 'slopguard')
				+ (SELECT COUNT(*) FROM runs
					WHERE instr(required_reviewers_json, '"slopguard"') > 0
					   OR instr(completed_reviewers_json, '"slopguard"') > 0)`).Scan(&occupied); err != nil {
				return fmt.Errorf("inspect reviewer rename collisions: %w", err)
			}
			type profileRewrite struct {
				key, bindings, forge string
			}
			var rewrites []profileRewrite
			rows, err := tx.Query(`SELECT repo_key, bindings_json, forge_reviewers_json FROM repos`)
			if err != nil {
				return fmt.Errorf("read profiles for reviewer rename: %w", err)
			}
			for rows.Next() {
				var key, rawBindings, rawForge string
				if err := rows.Scan(&key, &rawBindings, &rawForge); err != nil {
					_ = rows.Close()
					return fmt.Errorf("scan profile for reviewer rename: %w", err)
				}
				var bindings map[string][]string
				var forge map[string]string
				if err := json.Unmarshal([]byte(rawBindings), &bindings); err != nil {
					_ = rows.Close()
					return fmt.Errorf("decode profile %q bindings for reviewer rename: %w", key, err)
				}
				if err := json.Unmarshal([]byte(rawForge), &forge); err != nil {
					_ = rows.Close()
					return fmt.Errorf("decode profile %q forge reviewers for reviewer rename: %w", key, err)
				}
				changed := false
				for i, name := range bindings["review"] {
					if name == "slopguard" {
						occupied++
					}
					if name == "autoreview" {
						bindings["review"][i] = "slopguard"
						changed = true
					}
				}
				if _, taken := forge["slopguard"]; taken {
					occupied++
				}
				if login, renamed := forge["autoreview"]; renamed {
					delete(forge, "autoreview")
					forge["slopguard"] = login
					changed = true
				}
				if !changed {
					continue
				}
				encodedBindings, err := json.Marshal(bindings)
				if err != nil {
					_ = rows.Close()
					return err
				}
				encodedForge, err := json.Marshal(forge)
				if err != nil {
					_ = rows.Close()
					return err
				}
				rewrites = append(rewrites, profileRewrite{key: key, bindings: string(encodedBindings), forge: string(encodedForge)})
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				return fmt.Errorf("iterate profiles for reviewer rename: %w", err)
			}
			if err := rows.Close(); err != nil {
				return err
			}
			if occupied > 0 {
				return fmt.Errorf("%d row(s) already use a custom reviewer identity \"slopguard\", which the renamed built-in \"autoreview\" would silently merge with; rename or remove the custom identity and its references, then rerun", occupied)
			}
			for _, rewrite := range rewrites {
				if _, err := tx.Exec(`UPDATE repos SET bindings_json = ?, forge_reviewers_json = ? WHERE repo_key = ?`,
					rewrite.bindings, rewrite.forge, rewrite.key); err != nil {
					return fmt.Errorf("migrate schema 8 to 9 profiles: %w", err)
				}
			}
			if _, err := tx.Exec(`UPDATE runs SET
				required_reviewers_json = REPLACE(required_reviewers_json, '"autoreview"', '"slopguard"'),
				completed_reviewers_json = REPLACE(completed_reviewers_json, '"autoreview"', '"slopguard"')`); err != nil {
				return fmt.Errorf("migrate schema 8 to 9: %w", err)
			}
			version = 9
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
			required_reviewers_json TEXT NOT NULL DEFAULT '[]',
			risk_tier TEXT NOT NULL DEFAULT '',
			budget_json TEXT NOT NULL DEFAULT '{}',
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
			acceptance_criteria_json TEXT NOT NULL DEFAULT '[]',
			complexity TEXT NOT NULL DEFAULT '',
			attempt INTEGER NOT NULL,
			phase TEXT NOT NULL DEFAULT 'pending',
			rework_cause TEXT NOT NULL DEFAULT '',
			PRIMARY KEY (run_id, id),
			FOREIGN KEY (run_id) REFERENCES runs(id)
		)`,
		`CREATE TABLE IF NOT EXISTS reviewers (
			name TEXT PRIMARY KEY,
			created_at TEXT NOT NULL
		)`,
		createReposTable,
		`CREATE TABLE IF NOT EXISTS events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			run_id TEXT NOT NULL,
			seq INTEGER NOT NULL,
			at TEXT NOT NULL,
			command TEXT NOT NULL,
			from_state TEXT NOT NULL,
			to_state TEXT NOT NULL,
			evidence_json TEXT NOT NULL,
			telemetry_json TEXT NOT NULL DEFAULT '{}',
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

const createReposTable = `CREATE TABLE IF NOT EXISTS repos (
	repo_key TEXT PRIMARY KEY,
	forge_kind TEXT NOT NULL DEFAULT '',
	trust_tier TEXT NOT NULL DEFAULT '',
	verify_command TEXT NOT NULL DEFAULT '',
	delivery_mode TEXT NOT NULL DEFAULT '',
	readiness TEXT NOT NULL DEFAULT '',
	bindings_json TEXT NOT NULL DEFAULT '{}',
	forge_reviewers_json TEXT NOT NULL DEFAULT '{}',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
)`

func (s *Store) exec(q string, args ...any) error {
	_, err := s.db.Exec(q, args...)
	return err
}

// CreateRun inserts a new run and optional units; telemetry, when present,
// rides the init event.
func (s *Store) CreateRun(run machine.Run, units []machine.Unit, telemetry *machine.Telemetry) error {
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

	now := timestampNow()
	completedReviewers, err := json.Marshal(run.CompletedReviewers)
	if err != nil {
		return err
	}
	requiredReviewers, err := marshalReviewers(run.RequiredReviewers)
	if err != nil {
		return err
	}
	var released any
	if run.ReleasedRevision != nil {
		released = *run.ReleasedRevision
	}
	budget, err := json.Marshal(run.Budget)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`INSERT INTO runs(
		id, repo_key, state, intake_revision, released_revision, revision,
		delivery_mode, required_reviewers_json, risk_tier, budget_json, series_bound, completed_units,
		current_unit_id, completed_reviewers_json, blocker_reason, decision_question, return_state,
		open, created_at, updated_at
	) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,1,?,?)`,
		run.ID, run.RepoKey, string(run.State), run.IntakeRevision, released, run.Revision,
		string(run.DeliveryMode), requiredReviewers, string(run.RiskTier), string(budget), run.SeriesBound, run.CompletedUnits,
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
		DeliveryMode: run.DeliveryMode, RequiredReviewers: run.RequiredReviewers, SeriesBound: run.SeriesBound,
	})
	if err != nil {
		return err
	}
	initTelemetry := []byte("{}")
	if telemetry != nil {
		if err := machine.ValidateTelemetry(telemetry); err != nil {
			return err
		}
		initTelemetry, err = json.Marshal(telemetry)
		if err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`INSERT INTO events(run_id, seq, at, command, from_state, to_state, evidence_json, telemetry_json)
		VALUES (?,?,?,?,?,?,?,?)`, run.ID, 1, now, string(machine.CmdInit), "", string(run.State), string(initEvidence), string(initTelemetry)); err != nil {
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
	q += ` ORDER BY updated_at DESC, created_at DESC, rowid DESC`
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
		delivery_mode, required_reviewers_json, risk_tier, budget_json, series_bound, completed_units,
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
	var state, delivery, requiredReviewers, riskTier, budget, returnState, completedReviewers string
	var released sql.NullInt64
	err := row.Scan(
		&run.ID, &run.RepoKey, &state, &run.IntakeRevision, &released, &run.Revision,
		&delivery, &requiredReviewers, &riskTier, &budget, &run.SeriesBound, &run.CompletedUnits,
		&run.CurrentUnitID, &completedReviewers, &run.BlockerReason, &run.DecisionQuestion, &returnState,
	)
	if err != nil {
		return machine.Run{}, err
	}
	run.State = machine.State(state)
	run.DeliveryMode = machine.DeliveryMode(delivery)
	run.RiskTier = machine.RiskTier(riskTier)
	if err := json.Unmarshal([]byte(budget), &run.Budget); err != nil {
		return machine.Run{}, fmt.Errorf("decode budget: %w", err)
	}
	run.ReturnState = machine.State(returnState)
	if err := json.Unmarshal([]byte(requiredReviewers), &run.RequiredReviewers); err != nil {
		return machine.Run{}, fmt.Errorf("decode required reviewers: %w", err)
	}
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
	rows, err := s.db.Query(`SELECT id, title, blockers_json, acceptance_criteria_json, complexity, attempt, phase, rework_cause FROM units WHERE run_id = ? ORDER BY rowid`, runID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var units []machine.Unit
	for rows.Next() {
		var u machine.Unit
		var blockers, criteria, complexity, phase string
		if err := rows.Scan(&u.ID, &u.Title, &blockers, &criteria, &complexity, &u.Attempt, &phase, &u.ReworkCause); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(blockers), &u.Blockers); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(criteria), &u.AcceptanceCriteria); err != nil {
			return nil, err
		}
		u.Complexity = machine.Complexity(complexity)
		u.Phase = machine.UnitPhase(phase)
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
	// The registry predicate holds transactionally: a reviewer unregistered
	// between the machine's check and this commit still fails the latch.
	if result.Command == machine.CmdIntake || result.Command == machine.CmdRelease {
		if err := ensureReviewersRegisteredTx(tx, run.RequiredReviewers); err != nil {
			return err
		}
		// The profile predicate also holds transactionally: a binding removed
		// between the machine's check and this commit still fails the latch.
		profile, found, err := repoProfileTx(tx, run.RepoKey)
		if err != nil {
			return err
		}
		var snapshot *machine.RepoProfile
		if found {
			snapshot = &profile
		}
		if err := machine.ProfileAllowsReviewers(snapshot, run.RequiredReviewers); err != nil {
			return err
		}
	}
	completedReviewers, err := json.Marshal(run.CompletedReviewers)
	if err != nil {
		return err
	}
	requiredReviewers, err := marshalReviewers(run.RequiredReviewers)
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
	now := timestampNow()
	budget, err := json.Marshal(run.Budget)
	if err != nil {
		return err
	}
	res, err := tx.Exec(`UPDATE runs SET
		state=?, intake_revision=?, released_revision=?, revision=?,
		delivery_mode=?, required_reviewers_json=?, risk_tier=?, budget_json=?, series_bound=?, completed_units=?,
		current_unit_id=?, completed_reviewers_json=?, blocker_reason=?, decision_question=?, return_state=?,
		open=?, updated_at=?
	WHERE id=? AND revision=?`,
		string(run.State), run.IntakeRevision, released, run.Revision,
		string(run.DeliveryMode), requiredReviewers, string(run.RiskTier), string(budget), run.SeriesBound, run.CompletedUnits,
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
	telemetryJSON := []byte("{}")
	if result.Telemetry != nil {
		telemetryJSON, err = json.Marshal(result.Telemetry)
		if err != nil {
			return err
		}
	}
	_, err = tx.Exec(`INSERT INTO events(run_id, seq, at, command, from_state, to_state, evidence_json, telemetry_json)
		VALUES (?,?,?,?,?,?,?,?)`,
		run.ID, seq, now, string(result.Command), string(result.EventFrom), string(result.EventTo), string(evJSON), string(telemetryJSON),
	)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func ensureReviewersRegisteredTx(tx *sql.Tx, required []machine.ReviewerIdentity) error {
	builtin := make(map[machine.ReviewerIdentity]struct{})
	for _, reviewer := range machine.BuiltinReviewers() {
		builtin[reviewer] = struct{}{}
	}
	for _, reviewer := range required {
		if _, ok := builtin[reviewer]; ok {
			continue
		}
		var one int
		err := tx.QueryRow(`SELECT 1 FROM reviewers WHERE name = ?`, string(reviewer)).Scan(&one)
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("%w: required reviewer %q is no longer registered", machine.ErrUnmetGuard, reviewer)
		}
		if err != nil {
			return err
		}
	}
	return nil
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
		criteria, err := json.Marshal(u.AcceptanceCriteria)
		if err != nil {
			return err
		}
		if criteria == nil || string(criteria) == "null" {
			criteria = []byte("[]")
		}
		phase := u.Phase
		if phase == "" {
			phase = machine.PhasePending
		}
		if _, err := tx.Exec(`INSERT INTO units(run_id, id, title, blockers_json, acceptance_criteria_json, complexity, attempt, phase, rework_cause) VALUES (?,?,?,?,?,?,?,?,?)`,
			runID, u.ID, u.Title, string(b), string(criteria), string(u.Complexity), u.Attempt, string(phase), u.ReworkCause); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) beginImmediate() (*sql.Tx, error) {
	// Open DSN sets _txlock=immediate; MaxOpenConns(1) serializes writers for v0.
	return s.db.Begin()
}

func marshalReviewers(reviewers []machine.ReviewerIdentity) (string, error) {
	if reviewers == nil {
		reviewers = []machine.ReviewerIdentity{}
	}
	encoded, err := json.Marshal(reviewers)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

type profileQuerier interface {
	QueryRow(query string, args ...any) *sql.Row
}

func repoProfileTx(q profileQuerier, repoKey string) (machine.RepoProfile, bool, error) {
	profile := machine.RepoProfile{RepoKey: repoKey}
	var forgeKind, trustTier, deliveryMode, readiness, bindings, forgeReviewers string
	err := q.QueryRow(`SELECT forge_kind, trust_tier, verify_command, delivery_mode, readiness, bindings_json, forge_reviewers_json
		FROM repos WHERE repo_key = ?`, repoKey).
		Scan(&forgeKind, &trustTier, &profile.VerifyCommand, &deliveryMode, &readiness, &bindings, &forgeReviewers)
	if errors.Is(err, sql.ErrNoRows) {
		return machine.RepoProfile{}, false, nil
	}
	if err != nil {
		return machine.RepoProfile{}, false, err
	}
	profile.ForgeKind = machine.ForgeKind(forgeKind)
	profile.TrustTier = machine.TrustTier(trustTier)
	profile.DeliveryMode = machine.DeliveryMode(deliveryMode)
	profile.Readiness = machine.Readiness(readiness)
	if err := json.Unmarshal([]byte(bindings), &profile.Bindings); err != nil {
		return machine.RepoProfile{}, false, fmt.Errorf("decode repo profile bindings: %w", err)
	}
	if err := json.Unmarshal([]byte(forgeReviewers), &profile.ForgeReviewers); err != nil {
		return machine.RepoProfile{}, false, fmt.Errorf("decode repo profile forge reviewers: %w", err)
	}
	return profile, true, nil
}

// GetRepoProfile returns the registered profile for repoKey, if any.
func (s *Store) GetRepoProfile(repoKey string) (machine.RepoProfile, bool, error) {
	return repoProfileTx(s.db, repoKey)
}

// RegisterRepoProfile records a new profile; a registered repo must update.
func (s *Store) RegisterRepoProfile(profile machine.RepoProfile) error {
	return s.writeRepoProfile(profile, false)
}

// UpdateRepoProfile replaces a registered profile; unregistered repos must register.
func (s *Store) UpdateRepoProfile(profile machine.RepoProfile) error {
	return s.writeRepoProfile(profile, true)
}

func (s *Store) writeRepoProfile(profile machine.RepoProfile, mustExist bool) error {
	// Defense in depth: the CLI validates too, but no caller may persist an
	// invalid profile row.
	if err := machine.ValidateProfile(&profile); err != nil {
		return err
	}
	tx, err := s.beginImmediate()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var one int
	err = tx.QueryRow(`SELECT 1 FROM repos WHERE repo_key = ?`, profile.RepoKey).Scan(&one)
	exists := err == nil
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if exists && !mustExist {
		return fmt.Errorf("%w: this repo is already registered; inspect it with slopmachine repo show or change it with slopmachine repo update", machine.ErrBadArgs)
	}
	if !exists && mustExist {
		return fmt.Errorf("%w: this repo has no profile; create one with slopmachine repo register", machine.ErrNotFound)
	}
	bindings := profile.Bindings
	if bindings == nil {
		bindings = map[machine.Role][]string{}
	}
	// Review bindings must be registered reviewer identities at the moment
	// this transaction commits, not just when the caller pre-checked them.
	for _, name := range bindings[machine.RoleReview] {
		if err := ensureReviewersRegisteredTx(tx, []machine.ReviewerIdentity{machine.ReviewerIdentity(name)}); err != nil {
			return fmt.Errorf("%w: review binding %q is not a registered reviewer identity; register it first with slopmachine reviewers --add %s", machine.ErrBadArgs, name, name)
		}
	}
	encoded, err := json.Marshal(bindings)
	if err != nil {
		return err
	}
	forgeReviewers := profile.ForgeReviewers
	if forgeReviewers == nil {
		forgeReviewers = map[string]string{}
	}
	// Forge reviewer identities must also be registered reviewer identities
	// at commit time, for the same reason as review bindings.
	for identity := range forgeReviewers {
		if err := ensureReviewersRegisteredTx(tx, []machine.ReviewerIdentity{machine.ReviewerIdentity(identity)}); err != nil {
			return fmt.Errorf("%w: forge reviewer %q is not a registered reviewer identity; register it first with slopmachine reviewers --add %s", machine.ErrBadArgs, identity, identity)
		}
	}
	encodedForgeReviewers, err := json.Marshal(forgeReviewers)
	if err != nil {
		return err
	}
	now := timestampNow()
	if exists {
		if _, err := tx.Exec(`UPDATE repos SET forge_kind=?, trust_tier=?, verify_command=?, delivery_mode=?, readiness=?, bindings_json=?, forge_reviewers_json=?, updated_at=?
			WHERE repo_key=?`,
			string(profile.ForgeKind), string(profile.TrustTier), profile.VerifyCommand,
			string(profile.DeliveryMode), string(profile.Readiness), string(encoded), string(encodedForgeReviewers), now, profile.RepoKey); err != nil {
			return err
		}
	} else if _, err := tx.Exec(`INSERT INTO repos(repo_key, forge_kind, trust_tier, verify_command, delivery_mode, readiness, bindings_json, forge_reviewers_json, created_at, updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?)`,
		profile.RepoKey, string(profile.ForgeKind), string(profile.TrustTier), profile.VerifyCommand,
		string(profile.DeliveryMode), string(profile.Readiness), string(encoded), string(encodedForgeReviewers), now, now); err != nil {
		return err
	}
	return tx.Commit()
}

// UnregisterRepoProfile removes the profile; the repo returns to profile-less
// behavior. Idempotent.
func (s *Store) UnregisterRepoProfile(repoKey string) error {
	return s.exec(`DELETE FROM repos WHERE repo_key = ?`, repoKey)
}

// RegisterReviewer records a custom reviewer identity; idempotent.
func (s *Store) RegisterReviewer(name machine.ReviewerIdentity) error {
	now := timestampNow()
	return s.exec(`INSERT INTO reviewers(name, created_at) VALUES (?, ?)
		ON CONFLICT(name) DO NOTHING`, string(name), now)
}

// UnregisterReviewer removes a custom reviewer identity; idempotent.
func (s *Store) UnregisterReviewer(name machine.ReviewerIdentity) error {
	return s.exec(`DELETE FROM reviewers WHERE name = ?`, string(name))
}

// ListReviewers returns the custom registry in registration order. Built-in
// identities are not stored; callers combine them via machine.BuiltinReviewers.
func (s *Store) ListReviewers() ([]machine.ReviewerIdentity, error) {
	rows, err := s.db.Query(`SELECT name FROM reviewers ORDER BY created_at, name`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var reviewers []machine.ReviewerIdentity
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		reviewers = append(reviewers, machine.ReviewerIdentity(name))
	}
	return reviewers, rows.Err()
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
	// Profiles carry the same identity; keep them attached across sanitization.
	profileRows, err := tx.Query(`SELECT repo_key FROM repos WHERE repo_key = ? OR substr(repo_key, -length(?)) = ?`, root, suffix, suffix)
	if err != nil {
		return err
	}
	profileTargets := make(map[string]string)
	for profileRows.Next() {
		var candidate string
		if err := profileRows.Scan(&candidate); err != nil {
			_ = profileRows.Close()
			return err
		}
		if sanitized, ok := repo.SanitizeKey(candidate, root); ok {
			if repo.MatchesKey(candidate, newKey, root) {
				sanitized = newKey
			}
			if sanitized != candidate {
				profileTargets[candidate] = sanitized
			}
		}
	}
	if err := profileRows.Close(); err != nil {
		return err
	}
	if err := profileRows.Err(); err != nil {
		return err
	}
	for candidate, target := range profileTargets {
		// A sanitized row that already exists wins; drop the credentialed twin.
		var one int
		err := tx.QueryRow(`SELECT 1 FROM repos WHERE repo_key = ?`, target).Scan(&one)
		if err == nil {
			if _, err := tx.Exec(`DELETE FROM repos WHERE repo_key = ?`, candidate); err != nil {
				return err
			}
			continue
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if _, err := tx.Exec(`UPDATE repos SET repo_key = ? WHERE repo_key = ?`, target, candidate); err != nil {
			return err
		}
	}
	return tx.Commit()
}
