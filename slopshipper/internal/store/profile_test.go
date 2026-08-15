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
			machine.RoleReview: {"autoreview", "bugbot"},
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
	if !errors.Is(err, machine.ErrUnmetGuard) || !strings.Contains(err.Error(), "autoreview") {
		t.Fatalf("save must re-check profile bindings transactionally: %v", err)
	}

	profile.Bindings = map[machine.Role][]string{machine.RoleReview: {"autoreview"}}
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
