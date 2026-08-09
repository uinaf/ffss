package store_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/uinaf/slopomatic/internal/machine"
	"github.com/uinaf/slopomatic/internal/store"
)

func TestCreateResolveAndCAS(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Open(filepath.Join(dir, "t.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	run := machine.NewRun("run-a", "repo-a")
	units := []machine.Unit{{ID: "u1", Title: "one"}}
	if err := s.CreateRun(run, units); err != nil {
		t.Fatal(err)
	}

	got, gotUnits, err := s.ResolveActiveRun("repo-a", "")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "run-a" || len(gotUnits) != 1 {
		t.Fatalf("resolve: %+v units=%d", got, len(gotUnits))
	}

	// second open run → ambiguous
	run2 := machine.NewRun("run-b", "repo-a")
	if err := s.CreateRun(run2, units); err != nil {
		t.Fatal(err)
	}
	_, _, err = s.ResolveActiveRun("repo-a", "")
	if !errors.Is(err, machine.ErrAmbiguousRun) {
		t.Fatalf("want ambiguous got %v", err)
	}
	got, _, err = s.ResolveActiveRun("repo-a", "run-b")
	if err != nil || got.ID != "run-b" {
		t.Fatalf("explicit run: %v %+v", err, got)
	}

	other := machine.NewRun("run-other", "repo-b")
	if err := s.CreateRun(other, units); err != nil {
		t.Fatal(err)
	}
	_, _, err = s.ResolveActiveRun("repo-a", "run-other")
	if !errors.Is(err, machine.ErrNotFound) {
		t.Fatalf("want cross-repo not found got %v", err)
	}

	res, err := machine.Apply(got, units, machine.CmdIntake, machine.ApplyInput{
		ExpectedRevision: got.Revision,
		Intake:           &machine.IntakePatch{SeriesBound: intPtr(1)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SaveApply(res); err != nil {
		t.Fatal(err)
	}

	// stale CAS: pretend we advanced further than the DB
	conflict := res
	conflict.Run.Revision = res.Run.Revision + 10
	if err := s.SaveApply(conflict); !errors.Is(err, machine.ErrRevisionConflict) {
		t.Fatalf("want revision conflict got %v", err)
	}
}

func TestResolveStatusRunIncludesClosed(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Open(filepath.Join(dir, "t.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	run := machine.NewRun("run-done", "repo-a")
	units := []machine.Unit{{ID: "u1", Title: "one"}}
	if err := s.CreateRun(run, units); err != nil {
		t.Fatal(err)
	}
	run.State = machine.StateRunDone
	run.Revision++
	if err := s.SaveApply(machine.ApplyResult{
		Run: run, Units: units, Command: machine.CmdDeliver,
		EventFrom: machine.StateDeliver, EventTo: machine.StateRunDone,
	}); err != nil {
		t.Fatal(err)
	}

	_, _, err = s.ResolveActiveRun("repo-a", "")
	if !errors.Is(err, machine.ErrNotFound) {
		t.Fatalf("active should miss closed: %v", err)
	}
	got, _, err := s.ResolveStatusRun("repo-a", "")
	if err != nil || got.ID != "run-done" || got.State != machine.StateRunDone {
		t.Fatalf("status resolve: %v %+v", err, got)
	}
}

func TestListRunsAndEvents(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Open(filepath.Join(dir, "t.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	runA := machine.NewRun("run-a", "repo-a")
	units := []machine.Unit{{ID: "u1", Title: "one"}}
	if err := s.CreateRun(runA, units); err != nil {
		t.Fatal(err)
	}
	res, err := machine.Apply(runA, units, machine.CmdIntake, machine.ApplyInput{
		ExpectedRevision: runA.Revision,
		Intake:           &machine.IntakePatch{SeriesBound: intPtr(1)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SaveApply(res); err != nil {
		t.Fatal(err)
	}
	runA = res.Run
	units = res.Units
	res, err = machine.Apply(runA, units, machine.CmdRelease, machine.ApplyInput{
		ExpectedRevision: runA.Revision,
		IntakeRevision:   runA.IntakeRevision,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SaveApply(res); err != nil {
		t.Fatal(err)
	}

	runB := machine.NewRun("run-b", "repo-a")
	if err := s.CreateRun(runB, units); err != nil {
		t.Fatal(err)
	}
	other := machine.NewRun("run-other", "repo-b")
	if err := s.CreateRun(other, units); err != nil {
		t.Fatal(err)
	}

	summaries, err := s.ListRuns("repo-a")
	if err != nil || len(summaries) != 2 {
		t.Fatalf("ListRuns: %v %+v", err, summaries)
	}
	if summaries[0].ID != "run-b" || summaries[1].ID != "run-a" {
		t.Fatalf("ListRuns order: %+v", summaries)
	}
	if !summaries[1].Released || !summaries[1].Open {
		t.Fatalf("run-a summary flags: %+v", summaries[1])
	}
	if summaries[0].Released {
		t.Fatalf("run-b should be unreleased: %+v", summaries[0])
	}

	events, err := s.ListEvents("repo-a", "run-a")
	if err != nil || len(events) != 3 {
		t.Fatalf("ListEvents: %v %+v", err, events)
	}
	if events[0].Command != string(machine.CmdInit) || events[1].Command != string(machine.CmdIntake) || events[2].Command != string(machine.CmdRelease) {
		t.Fatalf("ListEvents order: %+v", events)
	}
	_, err = s.ListEvents("repo-b", "run-a")
	if !errors.Is(err, machine.ErrNotFound) {
		t.Fatalf("ListEvents cross-repo: %v", err)
	}
	_, _, _, err = s.GetRunProjection("repo-a", "run-a")
	if err != nil {
		t.Fatal(err)
	}
	_, _, _, err = s.GetRunProjection("repo-b", "run-a")
	if !errors.Is(err, machine.ErrNotFound) {
		t.Fatalf("cross-repo projection: %v", err)
	}
}

func TestRunIDsAreGloballyUnique(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "t.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.CreateRun(machine.NewRun("demo", "repo-a"), nil); err != nil {
		t.Fatal(err)
	}
	err = s.CreateRun(machine.NewRun("demo", "repo-b"), nil)
	if err == nil {
		t.Fatal("expected global primary-key conflict for duplicate run id")
	}
}

func TestPersistsCanonicalEvidenceAndReviewProgress(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "t.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	run := machine.NewRun("review", "repo")
	run.State = machine.StateReview
	run.ReviewConsent = machine.ReviewBoth
	run.CurrentUnitID = "u1"
	units := []machine.Unit{{ID: "u1", Attempt: 1}}
	if err := s.CreateRun(run, units); err != nil {
		t.Fatal(err)
	}
	res, err := machine.Apply(run, units, machine.CmdReview, machine.ApplyInput{
		ExpectedRevision: run.Revision,
		Review: &machine.ReviewEvidence{
			Reviewer: machine.ReviewerAutoreview, Verdict: machine.ReviewClean, ArtifactRef: "autoreview://1",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SaveApply(res); err != nil {
		t.Fatal(err)
	}
	got, _, err := s.GetRun("review")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.CompletedReviewers) != 1 || got.CompletedReviewers[0] != machine.ReviewerAutoreview {
		t.Fatalf("review progress: %#v", got.CompletedReviewers)
	}

	deliver := machine.NewRun("deliver", "repo")
	deliver.State = machine.StateDeliver
	deliver.CurrentUnitID = "u1"
	if err := s.CreateRun(deliver, units); err != nil {
		t.Fatal(err)
	}
	deliveryEvidence := &machine.DeliverEvidence{PRURL: "https://example.com/pull/1"}
	res, err = machine.Apply(deliver, units, machine.CmdDeliver, machine.ApplyInput{
		ExpectedRevision: deliver.Revision, Deliver: deliveryEvidence,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SaveApply(res); err != nil {
		t.Fatal(err)
	}
	events, err := s.ListEvents("repo", "deliver")
	if err != nil {
		t.Fatal(err)
	}
	if got := events[len(events)-1].EvidenceJSON; !strings.Contains(got, `"delivery_mode":"pr-hold"`) {
		t.Fatalf("unnormalized delivery evidence: %s", got)
	}
}

func TestDefaultsPreferencesRekeyAndMissingRuns(t *testing.T) {
	if got := store.DefaultPath("/data", "/home/example"); got != filepath.Join("/data", "slopomatic", "slopomatic.sqlite") {
		t.Fatalf("xdg path: %q", got)
	}
	if got := store.DefaultPath("", "/home/example"); got != filepath.Join("/home/example", ".local", "share", "slopomatic", "slopomatic.sqlite") {
		t.Fatalf("home path: %q", got)
	}

	s, err := store.Open(filepath.Join(t.TempDir(), "nested", "t.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, ok, err := s.GetPref("missing"); err != nil || ok {
		t.Fatalf("missing preference: ok=%v err=%v", ok, err)
	}
	if err := s.SetPref("delivery", "pr-hold"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetPref("delivery", "direct-trunk"); err != nil {
		t.Fatal(err)
	}
	if got, ok, err := s.GetPref("delivery"); err != nil || !ok || got != "direct-trunk" {
		t.Fatalf("preference=%q ok=%v err=%v", got, ok, err)
	}
	basicRoot := "/work/basic"
	basicKey := "https://host/basic|" + basicRoot
	run := machine.NewRun("legacy", "https://host/basic?access_token=OLD|"+basicRoot)
	if err := s.CreateRun(run, nil); err != nil {
		t.Fatal(err)
	}
	if err := s.RekeyRepo(basicKey, basicRoot); err != nil {
		t.Fatal(err)
	}
	if err := s.RekeyRepo(basicKey, basicRoot); err != nil {
		t.Fatal(err)
	}
	if got, _, err := s.ResolveStatusRun(basicKey, "legacy"); err != nil || got.RepoKey != basicKey {
		t.Fatalf("rekey: %+v %v", got, err)
	}
	if _, _, err := s.GetRun("missing"); !errors.Is(err, machine.ErrNotFound) {
		t.Fatalf("missing run: %v", err)
	}
	rotated := machine.NewRun("rotated", "https://host/repo?access_token=OLD|/work/repo")
	if err := s.CreateRun(rotated, nil); err != nil {
		t.Fatal(err)
	}
	other := machine.NewRun("other-root", "https://host/repo?access_token=OLD|/work/other")
	if err := s.CreateRun(other, nil); err != nil {
		t.Fatal(err)
	}
	collision := machine.NewRun("delimiter-root", "https://host/repo|/tmp|/work/repo")
	if err := s.CreateRun(collision, nil); err != nil {
		t.Fatal(err)
	}
	if err := s.RekeyRepo("https://host/repo|/work/repo", "/work/repo"); err != nil {
		t.Fatal(err)
	}
	if got, _, err := s.ResolveStatusRun("https://host/repo|/work/repo", "rotated"); err != nil || got.RepoKey != "https://host/repo|/work/repo" {
		t.Fatalf("rotated credential rekey: %+v %v", got, err)
	}
	if got, _, err := s.GetRun("other-root"); err != nil || got.RepoKey != other.RepoKey {
		t.Fatalf("different root changed: %+v %v", got, err)
	}
	if got, _, err := s.GetRun("delimiter-root"); err != nil || got.RepoKey != collision.RepoKey {
		t.Fatalf("delimiter-bearing root changed: %+v %v", got, err)
	}
	repointed := machine.NewRun("repointed", "https://old-host/repo?access_token=OLD|/work/repo")
	if err := s.CreateRun(repointed, nil); err != nil {
		t.Fatal(err)
	}
	if err := s.RekeyRepo("https://new-host/repo|/work/repo", "/work/repo"); err != nil {
		t.Fatal(err)
	}
	if got, _, err := s.GetRun("repointed"); err != nil || got.RepoKey != "https://old-host/repo|/work/repo" {
		t.Fatalf("repointed repository was not independently sanitized: %+v %v", got, err)
	}
}

func TestOpenCreatesPrivateStateAndReportsResolvedFailures(t *testing.T) {
	root := t.TempDir()
	databasePath := filepath.Join(root, "missing", "slopomatic", "state.sqlite")
	s, err := store.Open(databasePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	directoryInfo, err := os.Stat(filepath.Dir(databasePath))
	if err != nil {
		t.Fatal(err)
	}
	if got := directoryInfo.Mode().Perm(); got != 0o700 {
		t.Fatalf("database directory mode=%#o want 0700", got)
	}
	databaseInfo, err := os.Stat(databasePath)
	if err != nil {
		t.Fatal(err)
	}
	if got := databaseInfo.Mode().Perm(); got != 0o600 {
		t.Fatalf("database mode=%#o want 0600", got)
	}

	blockingFile := filepath.Join(root, "not-a-directory")
	if err := os.WriteFile(blockingFile, []byte("blocked"), 0o600); err != nil {
		t.Fatal(err)
	}
	blockedPath := filepath.Join(blockingFile, "state.sqlite")
	_, err = store.Open(blockedPath)
	resolvedPath, resolveErr := filepath.Abs(blockedPath)
	if resolveErr != nil {
		t.Fatal(resolveErr)
	}
	if err == nil || !strings.Contains(err.Error(), filepath.Dir(resolvedPath)) ||
		!strings.Contains(err.Error(), "create database directory") {
		t.Fatalf("contextual open error=%v", err)
	}

	databaseDirectory := filepath.Join(root, "database-directory")
	if err := os.Mkdir(databaseDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Open(databaseDirectory); err == nil ||
		!strings.Contains(err.Error(), databaseDirectory) || !strings.Contains(err.Error(), "prepare database") {
		t.Fatalf("directory database error=%v", err)
	}

	corruptPath := filepath.Join(root, "corrupt.sqlite")
	if err := os.WriteFile(corruptPath, []byte("not sqlite"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Open(corruptPath); err == nil ||
		!strings.Contains(err.Error(), corruptPath) || !strings.Contains(err.Error(), "initialize database") {
		t.Fatalf("corrupt database error=%v", err)
	}

	if _, err := store.Open(""); err == nil || !strings.Contains(err.Error(), "database path is empty") {
		t.Fatalf("empty path error=%v", err)
	}
}

func intPtr(v int) *int { return &v }
