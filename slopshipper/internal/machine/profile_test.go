package machine_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/uinaf/slopshipper/internal/machine"
)

func validProfile() machine.RepoProfile {
	return machine.RepoProfile{
		RepoKey:       "repo",
		ForgeKind:     machine.ForgeGitHub,
		TrustTier:     machine.TrustLow,
		VerifyCommand: "mise run verify",
		DeliveryMode:  machine.DeliveryPRHold,
		Readiness:     machine.ReadinessReady,
		Bindings: map[machine.Role][]string{
			machine.RoleReview: {"autoreview", "slopzapper"},
			machine.RoleQA:     {"slopscouter"},
		},
	}
}

func TestValidateProfileAcceptsDeclaredFields(t *testing.T) {
	profile := validProfile()
	if err := machine.ValidateProfile(&profile); err != nil {
		t.Fatal(err)
	}
	empty := machine.RepoProfile{RepoKey: "repo"}
	if err := machine.ValidateProfile(&empty); err != nil {
		t.Fatalf("all-optional profile must validate: %v", err)
	}
}

func TestValidateProfileNormalizesEmptyBindingLists(t *testing.T) {
	profile := validProfile()
	profile.Bindings[machine.RoleVenue] = nil
	if err := machine.ValidateProfile(&profile); err != nil {
		t.Fatal(err)
	}
	if _, ok := profile.Bindings[machine.RoleVenue]; ok {
		t.Fatal("empty binding list must be dropped")
	}
}

func TestValidateProfileFailsClosed(t *testing.T) {
	mutations := map[string]func(*machine.RepoProfile){
		"missing repo key":       func(p *machine.RepoProfile) { p.RepoKey = "" },
		"unknown forge":          func(p *machine.RepoProfile) { p.ForgeKind = "gitlab" },
		"unknown trust":          func(p *machine.RepoProfile) { p.TrustTier = "total" },
		"unknown readiness":      func(p *machine.RepoProfile) { p.Readiness = "maybe" },
		"unknown delivery":       func(p *machine.RepoProfile) { p.DeliveryMode = "yolo" },
		"blank verify command":   func(p *machine.RepoProfile) { p.VerifyCommand = "   " },
		"newline verify command": func(p *machine.RepoProfile) { p.VerifyCommand = "make\nverify" },
		"u2028 verify command":   func(p *machine.RepoProfile) { p.VerifyCommand = "make\u2028verify" },
		"oversized verify command": func(p *machine.RepoProfile) {
			p.VerifyCommand = strings.Repeat("x", 501)
		},
		"unknown role": func(p *machine.RepoProfile) { p.Bindings["reviewer"] = []string{"x"} },
		"invalid binding name": func(p *machine.RepoProfile) {
			p.Bindings[machine.RoleQA] = []string{"bad name"}
		},
		"duplicate binding": func(p *machine.RepoProfile) {
			p.Bindings[machine.RoleReview] = []string{"autoreview", "autoreview"}
		},
	}
	for name, mutate := range mutations {
		profile := validProfile()
		mutate(&profile)
		err := machine.ValidateProfile(&profile)
		if !errors.Is(err, machine.ErrBadArgs) {
			t.Fatalf("%s: want ErrBadArgs, got %v", name, err)
		}
	}
}

func TestProfileAllowsReviewers(t *testing.T) {
	required := []machine.ReviewerIdentity{"autoreview", "slopzapper"}
	if err := machine.ProfileAllowsReviewers(nil, required); err != nil {
		t.Fatalf("nil profile keeps profile-less behavior: %v", err)
	}
	profile := validProfile()
	if err := machine.ProfileAllowsReviewers(&profile, required); err != nil {
		t.Fatal(err)
	}
	profile.Bindings[machine.RoleReview] = []string{"autoreview"}
	err := machine.ProfileAllowsReviewers(&profile, required)
	if !errors.Is(err, machine.ErrUnmetGuard) || !strings.Contains(err.Error(), "slopzapper") {
		t.Fatalf("unbound reviewer must fail closed with the name: %v", err)
	}
	profile.Bindings = nil
	err = machine.ProfileAllowsReviewers(&profile, required)
	if !errors.Is(err, machine.ErrUnmetGuard) || !strings.Contains(err.Error(), "binds no review implementations") {
		t.Fatalf("bindingless profile must fail closed actionably: %v", err)
	}
}

func TestIntakeAndReleaseEnforceProfileBindings(t *testing.T) {
	run := machine.NewRun("run", "repo")
	profile := validProfile()
	profile.Bindings[machine.RoleReview] = []string{"bugbot"}
	units := []machine.Unit{{ID: "u1", Title: "one"}}

	// Intake with a retained set the profile does not bind fails.
	_, err := machine.Apply(run, nil, machine.CmdIntake, machine.ApplyInput{
		Intake:  &machine.IntakePatch{Units: units},
		Profile: &profile,
	})
	if !errors.Is(err, machine.ErrUnmetGuard) {
		t.Fatalf("retained unbound reviewer must fail intake: %v", err)
	}

	// Intake replacing the set with a bound reviewer passes.
	res, err := machine.Apply(run, nil, machine.CmdIntake, machine.ApplyInput{
		Intake: &machine.IntakePatch{
			RequiredReviewers: []machine.ReviewerIdentity{machine.ReviewerBugbot},
			Units:             units,
		},
		Profile: &profile,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Release re-checks the binding: a profile change after intake fails closed.
	narrowed := validProfile()
	narrowed.Bindings[machine.RoleReview] = []string{"autoreview"}
	_, err = machine.Apply(res.Run, res.Units, machine.CmdRelease, machine.ApplyInput{
		IntakeRevision: res.Run.IntakeRevision,
		Profile:        &narrowed,
	})
	if !errors.Is(err, machine.ErrUnmetGuard) {
		t.Fatalf("release must re-check profile bindings: %v", err)
	}
	if _, err := machine.Apply(res.Run, res.Units, machine.CmdRelease, machine.ApplyInput{
		IntakeRevision: res.Run.IntakeRevision,
		Profile:        &profile,
	}); err != nil {
		t.Fatal(err)
	}
}
