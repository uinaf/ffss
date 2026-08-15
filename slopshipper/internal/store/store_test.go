package store_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/uinaf/slopshipper/internal/machine"
	"github.com/uinaf/slopshipper/internal/store"
)

func TestOpenReadOnlyRejectsEmptyPath(t *testing.T) {
	if _, err := store.OpenReadOnly(""); err == nil {
		t.Fatal("empty read-only database path accepted")
	}
}

func TestCreateResolveAndCAS(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Open(filepath.Join(dir, "t.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	run := machine.NewRun("run-a", "repo-a")
	units := []machine.Unit{{ID: "u1", Title: "one"}}
	if err := s.CreateRun(run, units, nil); err != nil {
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
	if err := s.CreateRun(run2, units, nil); err != nil {
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
	if err := s.CreateRun(other, units, nil); err != nil {
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

func TestOpenReadOnlyResolvesWithoutWriting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "readonly.sqlite")
	s, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.CreateRun(machine.NewRun("read-only", "repo"), nil, nil); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	readOnly, err := store.OpenReadOnly(path)
	if err != nil {
		t.Fatal(err)
	}
	run, _, err := readOnly.ResolveStatusRun("repo", "read-only")
	if err != nil || run.ID != "read-only" {
		t.Fatalf("resolve=%+v err=%v", run, err)
	}
	if err := readOnly.SetPref("read-only", "rejected"); err == nil {
		t.Fatal("read-only store accepted a write")
	}
	if err := readOnly.Close(); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !after.ModTime().Equal(before.ModTime()) || after.Size() != before.Size() {
		t.Fatalf("read-only open changed database: before=%+v after=%+v", before, after)
	}
}

func TestOpenReadOnlyRejectsMissingDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.sqlite")
	if _, err := store.OpenReadOnly(path); err == nil || !strings.Contains(err.Error(), path) {
		t.Fatalf("missing read-only database: %v", err)
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
	if err := s.CreateRun(run, units, nil); err != nil {
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
	if err := s.CreateRun(runA, units, nil); err != nil {
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
	if err := s.CreateRun(runB, units, nil); err != nil {
		t.Fatal(err)
	}
	other := machine.NewRun("run-other", "repo-b")
	if err := s.CreateRun(other, units, nil); err != nil {
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

func TestReviewerRegistryRoundTrip(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "registry.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	reviewers, err := s.ListReviewers()
	if err != nil || len(reviewers) != 0 {
		t.Fatalf("initial registry: %v %v", reviewers, err)
	}
	for _, name := range []machine.ReviewerIdentity{"slopzapper", "slopzapper", "qa-bot"} {
		if err := s.RegisterReviewer(name); err != nil {
			t.Fatalf("register %s: %v", name, err)
		}
	}
	reviewers, err = s.ListReviewers()
	if err != nil || len(reviewers) != 2 {
		t.Fatalf("registry after adds: %v %v", reviewers, err)
	}
	for _, name := range []machine.ReviewerIdentity{"slopzapper", "slopzapper"} {
		if err := s.UnregisterReviewer(name); err != nil {
			t.Fatalf("unregister %s: %v", name, err)
		}
	}
	reviewers, err = s.ListReviewers()
	if err != nil || len(reviewers) != 1 || reviewers[0] != "qa-bot" {
		t.Fatalf("registry after removes: %v %v", reviewers, err)
	}

	// Unit phases and rework causes survive a round trip.
	phased := machine.NewRun("phased", "repo-phased")
	if err := s.CreateRun(phased, []machine.Unit{
		{ID: "d1", Phase: machine.PhaseDelivered},
		{ID: "r1", Phase: machine.PhaseRework, ReworkCause: "checks_failed: ci", Attempt: 2},
		{ID: "p1"},
	}, nil); err != nil {
		t.Fatal(err)
	}
	_, phasedUnits, err := s.GetRun("phased")
	if err != nil {
		t.Fatal(err)
	}
	if phasedUnits[0].Phase != machine.PhaseDelivered ||
		phasedUnits[1].Phase != machine.PhaseRework || phasedUnits[1].ReworkCause != "checks_failed: ci" ||
		phasedUnits[2].Phase != machine.PhasePending {
		t.Fatalf("phase round trip: %+v", phasedUnits)
	}

	// A run stored with a nil required set reads back as empty, not null.
	bare := machine.NewRun("bare", "repo-bare")
	bare.RequiredReviewers = nil
	if err := s.CreateRun(bare, nil, nil); err != nil {
		t.Fatal(err)
	}
	loaded, _, err := s.GetRun("bare")
	if err != nil || loaded.RequiredReviewers == nil || len(loaded.RequiredReviewers) != 0 {
		t.Fatalf("nil reviewers round trip: %+v %v", loaded.RequiredReviewers, err)
	}
}

func TestResolveStatusRunBranchesAndReadOnlyVersionGate(t *testing.T) {
	if _, err := store.Open(""); err == nil {
		t.Fatal("empty database path accepted")
	}
	path := filepath.Join(t.TempDir(), "branches.sqlite")
	s, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.CreateRun(machine.NewRun("first", "repo-x"), []machine.Unit{{ID: "u1"}}, nil); err != nil {
		t.Fatal(err)
	}
	if run, _, err := s.ResolveStatusRun("repo-x", ""); err != nil || run.ID != "first" {
		t.Fatalf("single-open status resolution: %+v %v", run, err)
	}
	if _, _, err := s.GetRun("nope"); !errors.Is(err, machine.ErrNotFound) {
		t.Fatalf("missing run: %v", err)
	}
	if err := s.CreateRun(machine.NewRun("second", "repo-x"), []machine.Unit{{ID: "u1"}}, nil); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.ResolveStatusRun("repo-x", ""); !errors.Is(err, machine.ErrAmbiguousRun) {
		t.Fatalf("ambiguous status resolution: %v", err)
	}
	if run, _, err := s.ResolveStatusRun("repo-x", "first"); err != nil || run.ID != "first" {
		t.Fatalf("explicit status resolution: %+v %v", run, err)
	}
	if _, _, err := s.ResolveStatusRun("repo-y", "first"); !errors.Is(err, machine.ErrNotFound) {
		t.Fatalf("cross-repo status resolution: %v", err)
	}
	if _, _, err := s.ResolveStatusRun("repo-none", ""); !errors.Is(err, machine.ErrNotFound) {
		t.Fatalf("empty repo status resolution: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	db := openSQLite(t, path)
	if _, err := db.Exec(`UPDATE meta SET value='2' WHERE key='schema_version'`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := store.OpenReadOnly(path); err == nil || !strings.Contains(err.Error(), "requires a normal command to migrate") {
		t.Fatalf("read-only version gate: %v", err)
	}
}

func TestOpenFailureBranches(t *testing.T) {
	blocking := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(blocking, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Open(filepath.Join(blocking, "db.sqlite")); !errors.Is(err, store.ErrStateUnavailable) {
		t.Fatalf("unpreparable open: %v", err)
	}
	if _, err := store.OpenReadOnly(filepath.Join(t.TempDir(), "missing.sqlite")); err == nil {
		t.Fatal("read-only open of missing database succeeded")
	}
	// A present file that is not a slopshipper database fails at the
	// schema-version read, not silently.
	if _, err := store.OpenReadOnly(blocking); err == nil {
		t.Fatal("read-only open of a non-database succeeded")
	}
}

func TestReadPathsFailClosedOnWrongRepo(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "reads.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.CreateRun(machine.NewRun("reads", "repo-a"), []machine.Unit{{ID: "u1"}}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ListEvents("repo-b", "reads"); !errors.Is(err, machine.ErrNotFound) {
		t.Fatalf("cross-repo events: %v", err)
	}
	if _, _, _, err := s.GetRunProjection("repo-b", "reads"); !errors.Is(err, machine.ErrNotFound) {
		t.Fatalf("cross-repo projection: %v", err)
	}
	run, units, events, err := s.GetRunProjection("repo-a", "reads")
	if err != nil || run.ID != "reads" || len(units) != 1 || len(events) != 1 {
		t.Fatalf("projection: %+v units=%d events=%d err=%v", run, len(units), len(events), err)
	}
}

func TestRekeyRepoBranches(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "rekey.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	// Empty inputs are a declared no-op.
	if err := s.RekeyRepo("", "/root"); err != nil {
		t.Fatal(err)
	}
	if err := s.RekeyRepo("key", ""); err != nil {
		t.Fatal(err)
	}
	// A root that matches no persisted run leaves state untouched.
	if err := s.CreateRun(machine.NewRun("keep", "identity|/somewhere/else"), nil, nil); err != nil {
		t.Fatal(err)
	}
	if err := s.RekeyRepo("new-key|/root", "/root"); err != nil {
		t.Fatal(err)
	}
	kept, _, err := s.GetRun("keep")
	if err != nil || kept.RepoKey != "identity|/somewhere/else" {
		t.Fatalf("unmatched rekey mutated state: %+v %v", kept, err)
	}
}

func TestSaveApplyReenforcesReviewerRegistryTransactionally(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "guard.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.RegisterReviewer("slopzapper"); err != nil {
		t.Fatal(err)
	}
	run := machine.NewRun("guarded", "repo-guard")
	if err := s.CreateRun(run, nil, nil); err != nil {
		t.Fatal(err)
	}
	res, err := machine.Apply(run, nil, machine.CmdIntake, machine.ApplyInput{
		ExpectedRevision:    run.Revision,
		RegisteredReviewers: []machine.ReviewerIdentity{"slopzapper"},
		Intake: &machine.IntakePatch{
			RequiredReviewers: []machine.ReviewerIdentity{"slopzapper"},
			Units:             []machine.Unit{{ID: "u1", Title: "one"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	// The registry changes between the machine check and the store commit.
	if err := s.UnregisterReviewer("slopzapper"); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveApply(res); !errors.Is(err, machine.ErrUnmetGuard) {
		t.Fatalf("stale registry commit: %v", err)
	}
	if err := s.RegisterReviewer("slopzapper"); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveApply(res); err != nil {
		t.Fatal(err)
	}
}

func TestRunIDsAreGloballyUnique(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "t.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.CreateRun(machine.NewRun("demo", "repo-a"), nil, nil); err != nil {
		t.Fatal(err)
	}
	err = s.CreateRun(machine.NewRun("demo", "repo-b"), nil, nil)
	if !errors.Is(err, machine.ErrRunExists) {
		t.Fatalf("duplicate run error=%v", err)
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
	run.RequiredReviewers = []machine.ReviewerIdentity{machine.ReviewerAutoreview, machine.ReviewerBugbot}
	run.CurrentUnitID = "u1"
	units := []machine.Unit{{ID: "u1", Attempt: 1}}
	if err := s.CreateRun(run, units, nil); err != nil {
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
	if err := s.CreateRun(deliver, units, nil); err != nil {
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
	if got := store.DefaultPath("/data", "/home/example"); got != filepath.Join("/data", "slopshipper", "slopshipper.sqlite") {
		t.Fatalf("xdg path: %q", got)
	}
	if got := store.DefaultPath("", "/home/example"); got != filepath.Join("/home/example", ".local", "share", "slopshipper", "slopshipper.sqlite") {
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
	if err := s.CreateRun(run, nil, nil); err != nil {
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
	if err := s.CreateRun(rotated, nil, nil); err != nil {
		t.Fatal(err)
	}
	other := machine.NewRun("other-root", "https://host/repo?access_token=OLD|/work/other")
	if err := s.CreateRun(other, nil, nil); err != nil {
		t.Fatal(err)
	}
	collision := machine.NewRun("delimiter-root", "https://host/repo|/tmp|/work/repo")
	if err := s.CreateRun(collision, nil, nil); err != nil {
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
	if err := s.CreateRun(repointed, nil, nil); err != nil {
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
	databasePath := filepath.Join(root, "missing", "slopshipper", "state.sqlite")
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
