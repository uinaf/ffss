package store_test

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	"github.com/uinaf/slopshipper/internal/machine"
	"github.com/uinaf/slopshipper/internal/store"
)

func TestMigratesVersionOneTransactionally(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v1.sqlite")
	s, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.CreateRun(machine.NewRun("run", "repo"), []machine.Unit{{ID: "u1"}}); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateRun(machine.NewRun("deliver", "repo-deliver"), []machine.Unit{{ID: "u2"}}); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateRun(machine.NewRun("closed-deliver", "repo-closed"), []machine.Unit{{ID: "u3"}}); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateRun(machine.NewRun("parked-deliver", "repo-parked"), []machine.Unit{{ID: "u4"}}); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	db := openSQLite(t, path)
	if _, err := db.Exec(`ALTER TABLE runs DROP COLUMN completed_reviewers_json`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE runs SET state='BLOCKED', current_unit_id='u1', blocker_reason='old verify failure', open=0 WHERE id='run'`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE runs SET state='DELIVER', current_unit_id='u2', review_consent='autoreview', open=1 WHERE id='deliver'`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE runs SET state='DELIVER', current_unit_id='u3', review_consent='human', open=0 WHERE id='closed-deliver'`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE runs SET state='NEEDS_DECISION', return_state='DELIVER', current_unit_id='u4', open=0 WHERE id='parked-deliver'`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE meta SET value='1' WHERE key='schema_version'`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	s, err = store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	run, units, err := s.ResolveActiveRun("repo", "")
	if err != nil {
		t.Fatal(err)
	}
	if run.CompletedReviewers == nil || len(run.CompletedReviewers) != 0 {
		t.Fatalf("completed reviewers: %#v", run.CompletedReviewers)
	}
	res, err := machine.Apply(run, units, machine.CmdRetry, machine.ApplyInput{
		ExpectedRevision: run.Revision, RetryReason: "verified migration recovery",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SaveApply(res); err != nil {
		t.Fatal(err)
	}
	if got, _, err := s.ResolveActiveRun("repo", "run"); err != nil || got.State != machine.StateBuild {
		t.Fatalf("retry after migration: %+v %v", got, err)
	}
	deliver, _, err := s.ResolveActiveRun("repo-deliver", "deliver")
	if err != nil {
		t.Fatal(err)
	}
	if deliver.State != machine.StateReview || len(deliver.CompletedReviewers) != 0 {
		t.Fatalf("legacy review migration did not fail closed: %+v", deliver)
	}
	closed, _, err := s.GetRun("closed-deliver")
	if err != nil {
		t.Fatal(err)
	}
	if closed.State != machine.StateReview || len(closed.CompletedReviewers) != 0 {
		t.Fatalf("closed legacy review migration did not fail closed: %+v", closed)
	}
	if active, _, err := s.ResolveActiveRun("repo-closed", "closed-deliver"); err != nil || active.State != machine.StateReview {
		t.Fatalf("closed legacy review was not reopened: %+v %v", active, err)
	}
	parked, _, err := s.GetRun("parked-deliver")
	if err != nil {
		t.Fatal(err)
	}
	if parked.State != machine.StateNeedsDecision || parked.ReturnState != machine.StateReview {
		t.Fatalf("parked legacy delivery target did not fail closed: %+v", parked)
	}
	if active, _, err := s.ResolveActiveRun("repo-parked", "parked-deliver"); err != nil || active.ReturnState != machine.StateReview {
		t.Fatalf("parked legacy delivery target was not reopened: %+v %v", active, err)
	}
}

func TestFutureSchemaIsRejectedWithoutDDL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "future.sqlite")
	s, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	db := openSQLite(t, path)
	if _, err := db.Exec(`UPDATE meta SET value='99' WHERE key='schema_version'`); err != nil {
		t.Fatal(err)
	}
	before := schemaSnapshot(t, db)
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	if _, err := store.Open(path); err == nil || !strings.Contains(err.Error(), "unsupported schema version 99") {
		t.Fatalf("future schema: %v", err)
	}
	db = openSQLite(t, path)
	defer db.Close()
	if after := schemaSnapshot(t, db); after != before {
		t.Fatalf("future schema mutated:\nbefore=%s\nafter=%s", before, after)
	}
	var version string
	if err := db.QueryRow(`SELECT value FROM meta WHERE key='schema_version'`).Scan(&version); err != nil || version != "99" {
		t.Fatalf("version=%q err=%v", version, err)
	}
}

func openSQLite(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func schemaSnapshot(t *testing.T, db *sql.DB) string {
	t.Helper()
	rows, err := db.Query(`SELECT type, name, COALESCE(sql, '') FROM sqlite_schema ORDER BY type, name`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var b strings.Builder
	for rows.Next() {
		var typ, name, sqlText string
		if err := rows.Scan(&typ, &name, &sqlText); err != nil {
			t.Fatal(err)
		}
		b.WriteString(typ)
		b.WriteByte('|')
		b.WriteString(name)
		b.WriteByte('|')
		b.WriteString(sqlText)
		b.WriteByte('\n')
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return b.String()
}
