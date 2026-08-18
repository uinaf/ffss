package machine_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/uinaf/ffss/cli/slopmachine/internal/machine"
)

func deliverAt(t *testing.T) (machine.Run, []machine.Unit) {
	t.Helper()
	run, units := releasedAtReview(t)
	res, err := machine.Apply(run, units, machine.CmdReview, machine.ApplyInput{
		ExpectedRevision: run.Revision,
		Review:           &machine.ReviewEvidence{Reviewer: machine.ReviewerSlopguard, Verdict: "clean", ArtifactRef: "test://1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	return res.Run, res.Units
}

func TestDeliverVerificationDefaultsRecorded(t *testing.T) {
	run, units := deliverAt(t)
	ev := &machine.DeliverEvidence{DeliveryMode: machine.DeliveryPRHold, PRURL: "https://example.com/pr/1"}
	if _, err := machine.Apply(run, units, machine.CmdDeliver, machine.ApplyInput{ExpectedRevision: run.Revision, Deliver: ev}); err != nil {
		t.Fatal(err)
	}
	if ev.Verification != machine.VerificationRecorded {
		t.Fatalf("empty verification must default to recorded, got %q", ev.Verification)
	}
}

func TestDeliverVerificationEnumIsClosed(t *testing.T) {
	run, units := deliverAt(t)
	ev := &machine.DeliverEvidence{DeliveryMode: machine.DeliveryPRHold, PRURL: "https://example.com/pr/1", Verification: "trusted"}
	_, err := machine.Apply(run, units, machine.CmdDeliver, machine.ApplyInput{ExpectedRevision: run.Revision, Deliver: ev})
	if !errors.Is(err, machine.ErrUnmetGuard) {
		t.Fatalf("unknown verification must fail closed: %v", err)
	}
}

func TestDeliverOverrideRequiresReason(t *testing.T) {
	run, units := deliverAt(t)
	ev := &machine.DeliverEvidence{DeliveryMode: machine.DeliveryPRHold, PRURL: "https://example.com/pr/1", Verification: machine.VerificationOverridden}
	_, err := machine.Apply(run, units, machine.CmdDeliver, machine.ApplyInput{ExpectedRevision: run.Revision, Deliver: ev})
	if !errors.Is(err, machine.ErrUnmetGuard) || !strings.Contains(err.Error(), "requires a reason") {
		t.Fatalf("override without reason must fail: %v", err)
	}
}

func TestDeliverReasonRequiresOverride(t *testing.T) {
	run, units := deliverAt(t)
	for _, verification := range []machine.EvidenceVerification{"", machine.VerificationObserved, machine.VerificationRecorded} {
		ev := &machine.DeliverEvidence{DeliveryMode: machine.DeliveryPRHold, PRURL: "https://example.com/pr/1", Verification: verification, UnverifiedReason: "why"}
		_, err := machine.Apply(run, units, machine.CmdDeliver, machine.ApplyInput{ExpectedRevision: run.Revision, Deliver: ev})
		if !errors.Is(err, machine.ErrUnmetGuard) {
			t.Fatalf("reason with %q verification must fail: %v", verification, err)
		}
	}
}

func TestDeliverOverrideReasonBounds(t *testing.T) {
	run, units := deliverAt(t)
	for name, reason := range map[string]string{
		"oversized":    strings.Repeat("x", 301),
		"control":      "line\nbreak",
		"unicode-line": "a\u2028b",
		"blank":        "   ",
	} {
		ev := &machine.DeliverEvidence{DeliveryMode: machine.DeliveryPRHold, PRURL: "https://example.com/pr/1", Verification: machine.VerificationOverridden, UnverifiedReason: reason}
		_, err := machine.Apply(run, units, machine.CmdDeliver, machine.ApplyInput{ExpectedRevision: run.Revision, Deliver: ev})
		if !errors.Is(err, machine.ErrUnmetGuard) {
			t.Fatalf("%s reason must fail: %v", name, err)
		}
	}
	ev := &machine.DeliverEvidence{DeliveryMode: machine.DeliveryPRHold, PRURL: "https://example.com/pr/1", Verification: machine.VerificationOverridden, UnverifiedReason: "forge outage, verified manually"}
	if _, err := machine.Apply(run, units, machine.CmdDeliver, machine.ApplyInput{ExpectedRevision: run.Revision, Deliver: ev}); err != nil {
		t.Fatalf("valid override must pass: %v", err)
	}
}

func TestReviewVerificationValidatedToo(t *testing.T) {
	run, units := releasedAtReview(t)
	bad := &machine.ReviewEvidence{Reviewer: machine.ReviewerSlopguard, Verdict: "clean", ArtifactRef: "test://1", Verification: "narrated"}
	if _, err := machine.Apply(run, units, machine.CmdReview, machine.ApplyInput{ExpectedRevision: run.Revision, Review: bad}); !errors.Is(err, machine.ErrUnmetGuard) {
		t.Fatalf("unknown review verification must fail closed: %v", err)
	}
	good := &machine.ReviewEvidence{Reviewer: machine.ReviewerSlopguard, Verdict: "clean", ArtifactRef: "test://1", Verification: machine.VerificationOverridden, UnverifiedReason: "corroboration bypassed for fixture"}
	if _, err := machine.Apply(run, units, machine.CmdReview, machine.ApplyInput{ExpectedRevision: run.Revision, Review: good}); err != nil {
		t.Fatalf("overridden review with reason must pass: %v", err)
	}
	if good.Verification != machine.VerificationOverridden {
		t.Fatalf("verification must survive: %q", good.Verification)
	}
}

func TestProfileForgeReviewersRequireForgeKind(t *testing.T) {
	profile := &machine.RepoProfile{RepoKey: "k", ForgeReviewers: map[string]string{"slopzapper": "zapbot"}}
	if err := machine.ValidateProfile(profile); !errors.Is(err, machine.ErrBadArgs) {
		t.Fatalf("forge reviewers without forge kind must fail: %v", err)
	}
	profile.ForgeKind = machine.ForgeGitHub
	if err := machine.ValidateProfile(profile); err != nil {
		t.Fatalf("forge-bound mapping must pass: %v", err)
	}
}

func TestProfileForgeLoginCharset(t *testing.T) {
	for name, login := range map[string]string{
		"empty":        "",
		"bare-bot":     "[bot]",
		"space":        "a b",
		"slash":        "a/b",
		"leading-dash": "-a",
		"oversized":    strings.Repeat("a", 65),
	} {
		profile := &machine.RepoProfile{RepoKey: "k", ForgeKind: machine.ForgeGitHub, ForgeReviewers: map[string]string{"slopzapper": login}}
		if err := machine.ValidateProfile(profile); !errors.Is(err, machine.ErrBadArgs) {
			t.Fatalf("login %s must fail: %v", name, err)
		}
	}
	for _, login := range []string{"zapbot", "zap-bot", "zapbot[bot]", "Zapbot"} {
		profile := &machine.RepoProfile{RepoKey: "k", ForgeKind: machine.ForgeGitHub, ForgeReviewers: map[string]string{"slopzapper": login}}
		if err := machine.ValidateProfile(profile); err != nil {
			t.Fatalf("login %q must pass: %v", login, err)
		}
	}
}

func TestNormalizeForgeLogin(t *testing.T) {
	if machine.NormalizeForgeLogin("Zapbot[bot]") != machine.NormalizeForgeLogin("zapbot") {
		t.Fatal("bot suffix and case must normalize away")
	}
	if machine.NormalizeForgeLogin("zapbot") == machine.NormalizeForgeLogin("other") {
		t.Fatal("distinct logins must stay distinct")
	}
}

func TestRetiredReviewerNameIsRejectedWithRename(t *testing.T) {
	run := machine.NewRun("r", "repo")
	_, err := machine.Apply(run, nil, machine.CmdIntake, machine.ApplyInput{
		ExpectedRevision: 1,
		Intake: &machine.IntakePatch{
			RequiredReviewers: []machine.ReviewerIdentity{"autoreview"},
			Units:             []machine.Unit{{ID: "u1", Title: "t"}},
		},
	})
	if !errors.Is(err, machine.ErrBadArgs) || !strings.Contains(err.Error(), `renamed to "slopguard"`) {
		t.Fatalf("retired identity must point at its successor: %v", err)
	}
}
