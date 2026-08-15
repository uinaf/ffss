package store_test

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/uinaf/slopshipper/internal/machine"
	"github.com/uinaf/slopshipper/internal/store"
)

func testProfile(repoKey string) machine.RepoProfile {
	return machine.RepoProfile{
		RepoKey:       repoKey,
		ForgeKind:     machine.ForgeGitHub,
		TrustTier:     machine.TrustLow,
		VerifyCommand: "mise run verify",
		DeliveryMode:  machine.DeliveryPRHold,
		Readiness:     machine.ReadinessReady,
		Bindings: map[machine.Role][]string{
			machine.RoleReview: {"slopguard", "bugbot"},
		},
	}
}

func TestRepoProfileRoundTrip(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "t.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()

	if _, found, err := s.GetRepoProfile("repo"); err != nil || found {
		t.Fatalf("unregistered repo: found=%v err=%v", found, err)
	}
	// The write transaction itself re-checks review bindings against the
	// reviewer registry, independent of any caller pre-check.
	ghost := testProfile("repo")
	ghost.Bindings = map[machine.Role][]string{machine.RoleReview: {"ghost"}}
	err = s.RegisterRepoProfile(ghost)
	if !errors.Is(err, machine.ErrBadArgs) || !strings.Contains(err.Error(), "ghost") {
		t.Fatalf("unregistered review binding must fail transactionally: %v", err)
	}
	if err := s.RegisterRepoProfile(testProfile("repo")); err != nil {
		t.Fatal(err)
	}
	err = s.RegisterRepoProfile(testProfile("repo"))
	if !errors.Is(err, machine.ErrBadArgs) {
		t.Fatalf("double register must fail: %v", err)
	}
	got, found, err := s.GetRepoProfile("repo")
	if err != nil || !found {
		t.Fatalf("found=%v err=%v", found, err)
	}
	if got.VerifyCommand != "mise run verify" || got.ForgeKind != machine.ForgeGitHub ||
		got.DeliveryMode != machine.DeliveryPRHold || got.Readiness != machine.ReadinessReady ||
		len(got.Bindings[machine.RoleReview]) != 2 {
		t.Fatalf("profile did not round-trip: %+v", got)
	}

	updated := testProfile("repo")
	updated.TrustTier = machine.TrustMedium
	updated.Bindings = map[machine.Role][]string{machine.RoleReview: {"bugbot"}}
	if err := s.UpdateRepoProfile(updated); err != nil {
		t.Fatal(err)
	}
	got, _, err = s.GetRepoProfile("repo")
	if err != nil {
		t.Fatal(err)
	}
	if got.TrustTier != machine.TrustMedium || len(got.Bindings[machine.RoleReview]) != 1 {
		t.Fatalf("update did not replace: %+v", got)
	}

	err = s.UpdateRepoProfile(testProfile("other"))
	if !errors.Is(err, machine.ErrNotFound) {
		t.Fatalf("update of unregistered repo must fail: %v", err)
	}
	if err := s.UnregisterRepoProfile("repo"); err != nil {
		t.Fatal(err)
	}
	if err := s.UnregisterRepoProfile("repo"); err != nil {
		t.Fatalf("unregister must be idempotent: %v", err)
	}
	if _, found, _ := s.GetRepoProfile("repo"); found {
		t.Fatal("profile must be gone")
	}
}

func TestSaveApplyReenforcesProfileBindingsTransactionally(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "t.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()

	run := machine.NewRun("run", "repo")
	if err := s.CreateRun(run, nil, nil); err != nil {
		t.Fatal(err)
	}
	res, err := machine.Apply(run, nil, machine.CmdIntake, machine.ApplyInput{
		Intake: &machine.IntakePatch{Units: []machine.Unit{{ID: "u1", Title: "one"}}},
	})
	if err != nil {
		t.Fatal(err)
	}

	// A profile registered between the machine's check (nil profile) and the
	// commit must still gate the save when it does not bind the reviewer.
	profile := testProfile("repo")
	profile.Bindings = map[machine.Role][]string{machine.RoleReview: {"bugbot"}}
	if err := s.RegisterRepoProfile(profile); err != nil {
		t.Fatal(err)
	}
	err = s.SaveApply(res)
	if !errors.Is(err, machine.ErrUnmetGuard) || !strings.Contains(err.Error(), "slopguard") {
		t.Fatalf("save must re-check profile bindings transactionally: %v", err)
	}

	profile.Bindings = map[machine.Role][]string{machine.RoleReview: {"slopguard"}}
	if err := s.UpdateRepoProfile(profile); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveApply(res); err != nil {
		t.Fatal(err)
	}
}

func TestMigratesVersionFiveAddsRepos(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v5.sqlite")
	s, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.CreateRun(machine.NewRun("run", "repo"), nil, nil); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	db := openSQLite(t, path)
	if _, err := db.Exec(`DROP TABLE repos`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`ALTER TABLE events DROP COLUMN telemetry_json`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE meta SET value = '5' WHERE key = 'schema_version'`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	s, err = store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	if err := s.RegisterRepoProfile(testProfile("repo")); err != nil {
		t.Fatalf("migrated database must accept profiles: %v", err)
	}
	if _, _, err := s.GetRun("run"); err != nil {
		t.Fatalf("existing runs must keep reading: %v", err)
	}
}

func TestRekeyRepoMovesProfiles(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "t.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()

	root := "/checkout"
	credentialed := "x://user:password@host/r.git|" + root
	sanitized := "x://host/r.git|" + root
	if err := s.RegisterRepoProfile(testProfile(credentialed)); err != nil {
		t.Fatal(err)
	}
	if err := s.RekeyRepo(sanitized, root); err != nil {
		t.Fatal(err)
	}
	if _, found, _ := s.GetRepoProfile(credentialed); found {
		t.Fatal("credentialed profile key must be gone")
	}
	if _, found, _ := s.GetRepoProfile(sanitized); !found {
		t.Fatal("profile must follow the sanitized key")
	}

	// A sanitized twin already present wins; the credentialed row is dropped.
	credentialed2 := "x://user2:password2@host/r.git|" + root
	if err := s.RegisterRepoProfile(testProfile(credentialed2)); err != nil {
		t.Fatal(err)
	}
	if err := s.RekeyRepo(sanitized, root); err != nil {
		t.Fatal(err)
	}
	if _, found, _ := s.GetRepoProfile(credentialed2); found {
		t.Fatal("credentialed twin must be dropped")
	}
	if _, found, _ := s.GetRepoProfile(sanitized); !found {
		t.Fatal("sanitized profile must survive")
	}
}

func TestForgeReviewersRoundTrip(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "t.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()

	if err := s.RegisterReviewer("slopzapper"); err != nil {
		t.Fatal(err)
	}
	profile := testProfile("repo")
	profile.ForgeReviewers = map[string]string{"slopzapper": "zapbot[bot]"}
	if err := s.RegisterRepoProfile(profile); err != nil {
		t.Fatal(err)
	}
	got, found, err := s.GetRepoProfile("repo")
	if err != nil || !found {
		t.Fatalf("found=%v err=%v", found, err)
	}
	if got.ForgeReviewers["slopzapper"] != "zapbot[bot]" {
		t.Fatalf("forge reviewers did not round-trip: %+v", got.ForgeReviewers)
	}

	// The write transaction re-checks forge reviewer identities against the
	// reviewer registry, like review bindings.
	ghost := testProfile("repo")
	ghost.ForgeReviewers = map[string]string{"ghost": "ghostbot"}
	err = s.UpdateRepoProfile(ghost)
	if !errors.Is(err, machine.ErrBadArgs) || !strings.Contains(err.Error(), "ghost") {
		t.Fatalf("unregistered forge reviewer must fail transactionally: %v", err)
	}

	// Replacing without the mapping clears it.
	if err := s.UpdateRepoProfile(testProfile("repo")); err != nil {
		t.Fatal(err)
	}
	got, _, err = s.GetRepoProfile("repo")
	if err != nil || len(got.ForgeReviewers) != 0 {
		t.Fatalf("replacement must clear forge reviewers: %+v err=%v", got.ForgeReviewers, err)
	}
}

func TestMigratesVersionSevenAddsForgeReviewers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v7.sqlite")
	s, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.CreateRun(machine.NewRun("run", "repo"), nil, nil); err != nil {
		t.Fatal(err)
	}
	if err := s.RegisterRepoProfile(testProfile("repo")); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	db := openSQLite(t, path)
	if _, err := db.Exec(`ALTER TABLE repos DROP COLUMN forge_reviewers_json`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE meta SET value = '7' WHERE key = 'schema_version'`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	s, err = store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	got, found, err := s.GetRepoProfile("repo")
	if err != nil || !found || len(got.ForgeReviewers) != 0 {
		t.Fatalf("migrated profile must read with empty forge reviewers: %+v found=%v err=%v", got.ForgeReviewers, found, err)
	}
	if err := s.RegisterReviewer("slopzapper"); err != nil {
		t.Fatal(err)
	}
	got.ForgeReviewers = map[string]string{"slopzapper": "zapbot"}
	if err := s.UpdateRepoProfile(got); err != nil {
		t.Fatalf("migrated database must accept forge reviewers: %v", err)
	}
}

func TestMigratesVersionEightRenamesRetiredReviewer(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v8.sqlite")
	s, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	run := machine.NewRun("run", "repo")
	run.RequiredReviewers = []machine.ReviewerIdentity{machine.ReviewerSlopguard, machine.ReviewerBugbot}
	if err := s.CreateRun(run, nil, nil); err != nil {
		t.Fatal(err)
	}
	if err := s.RegisterRepoProfile(testProfile("repo")); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	// Rewind to v8 with the retired identity in live state, exactly as a
	// pre-rename database would hold it.
	db := openSQLite(t, path)
	if _, err := db.Exec(`UPDATE runs SET
		required_reviewers_json = '["autoreview","bugbot"]',
		completed_reviewers_json = '["autoreview"]'`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE repos SET
		bindings_json = '{"review":["autoreview","bugbot"]}',
		forge_reviewers_json = '{"autoreview":"autoreview"}'`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE meta SET value = '8' WHERE key = 'schema_version'`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	s, err = store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	got, _, err := s.GetRun("run")
	if err != nil {
		t.Fatal(err)
	}
	if got.RequiredReviewers[0] != machine.ReviewerSlopguard || got.CompletedReviewers[0] != machine.ReviewerSlopguard {
		t.Fatalf("live run state must follow the rename: %+v", got)
	}
	profile, _, err := s.GetRepoProfile("repo")
	if err != nil {
		t.Fatal(err)
	}
	if profile.Bindings[machine.RoleReview][0] != "slopguard" {
		t.Fatalf("bindings must follow the rename: %+v", profile.Bindings)
	}
	// The mapping KEY is an identity and renames; the VALUE is a forge
	// login and must stay untouched.
	if login, ok := profile.ForgeReviewers["slopguard"]; !ok || login != "autoreview" {
		t.Fatalf("forge reviewer keys rename, values stay: %+v", profile.ForgeReviewers)
	}
}

func TestMigrationRefusesReviewerRenameCollision(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v8-collision.sqlite")
	s, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.CreateRun(machine.NewRun("run", "repo"), nil, nil); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	db := openSQLite(t, path)
	// A v8 world where slopguard already existed as a custom identity next
	// to the built-in autoreview: merging them would weaken the gate.
	if _, err := db.Exec(`UPDATE runs SET required_reviewers_json = '["autoreview","slopguard"]'`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE meta SET value = '8' WHERE key = 'schema_version'`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Open(path); err == nil || !strings.Contains(err.Error(), "refusing to merge distinct identities") {
		t.Fatalf("collision must refuse the migration: %v", err)
	}
}

func TestMigrationSupersedesCustomSlopguardRegistration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v8-custom.sqlite")
	s, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.RegisterReviewer("shadow"); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	db := openSQLite(t, path)
	if _, err := db.Exec(`UPDATE reviewers SET name = 'slopguard' WHERE name = 'shadow'`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE meta SET value = '8' WHERE key = 'schema_version'`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	registered, err := s.ListReviewers()
	if err != nil {
		t.Fatal(err)
	}
	for _, reviewer := range registered {
		if reviewer == "slopguard" {
			t.Fatal("custom slopguard row must be superseded by the built-in")
		}
	}
}
