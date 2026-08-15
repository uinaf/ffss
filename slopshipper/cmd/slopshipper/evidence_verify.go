package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/uinaf/slopshipper/internal/forge"
	"github.com/uinaf/slopshipper/internal/machine"
	"github.com/uinaf/slopshipper/internal/watch"
)

// evidenceOverride carries an explicit --unverified request from the caller.
type evidenceOverride struct {
	Requested bool
	Reason    string
}

func overrideFromFlags(fs map[string]string) (evidenceOverride, error) {
	_, unverified := fs["unverified"]
	reason, hasReason := fs["reason"]
	if hasReason && !unverified {
		return evidenceOverride{}, fmt.Errorf("--reason requires --unverified")
	}
	if unverified && strings.TrimSpace(reason) == "" {
		return evidenceOverride{}, fmt.Errorf("--unverified requires --reason explaining why verification is bypassed")
	}
	return evidenceOverride{Requested: unverified, Reason: reason}, nil
}

func overrideFromInput(unverified *bool, reason *string) (evidenceOverride, error) {
	requested := unverified != nil && *unverified
	if reason != nil && !requested {
		return evidenceOverride{}, fmt.Errorf("unverified_reason requires unverified: true")
	}
	if requested && (reason == nil || strings.TrimSpace(*reason) == "") {
		return evidenceOverride{}, fmt.Errorf("unverified: true requires an unverified_reason explaining why verification is bypassed")
	}
	if reason == nil {
		return evidenceOverride{Requested: requested}, nil
	}
	return evidenceOverride{Requested: requested, Reason: *reason}, nil
}

// forgeProfile returns the adapter for a registered profile that binds a
// forge kind, or nil when the repo keeps recorded-input behavior.
func forgeProfile(profile machine.RepoProfile, found bool) (forge.Forge, error) {
	if !found || profile.ForgeKind == "" {
		return nil, nil
	}
	return forge.New(forge.Kind(profile.ForgeKind))
}

// resolveDeliverVerification stamps how the delivery evidence was
// established, observing the change request when the profile binds a forge.
// It returns a non-zero exit code on rejection or an unprovable observation.
func resolveDeliverVerification(ctx context.Context, ev *machine.DeliverEvidence, profile machine.RepoProfile, found bool, mode machine.DeliveryMode, override evidenceOverride, opts runOptions) int {
	adapter, err := forgeProfile(profile, found)
	if err != nil {
		return writeFailure(opts, 10, err)
	}
	prMode := mode == machine.DeliveryPRHold || mode == machine.DeliveryPRMergeWhenReady
	applies := adapter != nil && prMode && strings.TrimSpace(ev.PRURL) != ""
	if !applies {
		if override.Requested {
			return writeFailure(opts, 2, fmt.Errorf("--unverified applies only when delivery evidence would be verified against the forge (a forge-bound repo profile and a change-request delivery); this delivery is recorded input already"))
		}
		ev.Verification = machine.VerificationRecorded
		return 0
	}
	if override.Requested {
		ev.Verification = machine.VerificationOverridden
		ev.UnverifiedReason = override.Reason
		return 0
	}
	ref, err := adapter.ParseChangeRequestURL(ev.PRURL)
	if err != nil {
		return writeFailure(opts, 3, fmt.Errorf("%w: deliver.pr_url is not a %s change request URL: %q", machine.ErrUnmetGuard, profile.ForgeKind, ev.PRURL))
	}
	// Existence and head are the whole delivery proof; the narrow read keeps
	// faults in reads deliver does not need (review threads) out of it.
	head, err := adapter.Head(ctx, ref)
	if err != nil {
		return observationFailure(fmt.Sprintf("delivery change request %s", ref), err, opts)
	}
	// A payload without a judgeable head cannot corroborate anything; that
	// is an incomplete observation, never a silent pass.
	if !validObservedHead(head) {
		return observationFailure(fmt.Sprintf("delivery change request %s", ref),
			&forge.Error{Kind: forge.ErrorTransient, Err: fmt.Errorf("forge returned no usable head revision")}, opts)
	}
	if ev.CommitSHA != "" && watch.HeadMoved(ev.CommitSHA, head) {
		return writeFailure(opts, 3, fmt.Errorf("%w: delivered head mismatch: evidence records %s but the forge head of %s is %s; re-deliver the current head or fix the evidence", machine.ErrUnmetGuard, ev.CommitSHA, ref, head))
	}
	// A verified delivery without a recorded revision adopts the observed
	// head, giving watch a baseline for head_moved.
	if ev.CommitSHA == "" {
		ev.CommitSHA = head
	}
	ev.Verification = machine.VerificationObserved
	return 0
}

// resolveReviewVerification corroborates forge-resident reviewer evidence
// against the change request's submitted reviews; other reviewers keep
// recorded-input behavior.
func resolveReviewVerification(ctx context.Context, ev *machine.ReviewEvidence, profile machine.RepoProfile, found bool, override evidenceOverride, opts runOptions) int {
	adapter, err := forgeProfile(profile, found)
	if err != nil {
		return writeFailure(opts, 10, err)
	}
	login, resident := "", false
	if adapter != nil {
		login, resident = profile.ForgeReviewers[string(ev.Reviewer)]
	}
	if !resident {
		if override.Requested {
			return writeFailure(opts, 2, fmt.Errorf("--unverified applies only to forge-corroborated reviewers (a forge-bound repo profile with a --forge-reviewer mapping for %q); this review is recorded input already", ev.Reviewer))
		}
		ev.Verification = machine.VerificationRecorded
		return 0
	}
	if override.Requested {
		ev.Verification = machine.VerificationOverridden
		ev.UnverifiedReason = override.Reason
		return 0
	}
	ref, err := adapter.ParseChangeRequestURL(ev.ArtifactRef)
	if err != nil {
		return writeFailure(opts, 3, fmt.Errorf("%w: reviewer %q is forge-corroborated, so review.artifact_ref must be a %s change request URL; got %q", machine.ErrUnmetGuard, ev.Reviewer, profile.ForgeKind, ev.ArtifactRef))
	}
	reviews, err := adapter.Reviews(ctx, ref)
	if err != nil {
		return observationFailure(fmt.Sprintf("reviews on %s", ref), err, opts)
	}
	want := machine.NormalizeForgeLogin(login)
	for _, review := range reviews {
		if submittedReviewState(review.State) && machine.NormalizeForgeLogin(review.Author) == want {
			ev.Verification = machine.VerificationObserved
			return 0
		}
	}
	return writeFailure(opts, 3, fmt.Errorf("%w: no submitted review by %q found on %s; the forge does not corroborate this evidence", machine.ErrUnmetGuard, login, ref))
}

// validObservedHead accepts only a judgeable commit identity: 7-64 hex
// characters, matching the machine's evidence rule.
func validObservedHead(head string) bool {
	if len(head) < 7 || len(head) > 64 {
		return false
	}
	for _, r := range head {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') && (r < 'A' || r > 'F') {
			return false
		}
	}
	return true
}

// submittedReviewState recognizes the closed set of forge states a submitted
// review can carry; anything else (including a missing state on a malformed
// payload) does not corroborate.
func submittedReviewState(state string) bool {
	switch state {
	case "APPROVED", "CHANGES_REQUESTED", "COMMENTED", "DISMISSED":
		return true
	default:
		return false
	}
}

// observationFailure classifies a forge read that blocked verification:
// a missing object refutes the evidence (unmet guard), everything else
// leaves it unprovable (exit 7, observation_<kind>), never accepted quietly.
func observationFailure(what string, err error, opts runOptions) int {
	var forgeErr *forge.Error
	if errors.As(err, &forgeErr) && forgeErr.Kind == forge.ErrorNotFound {
		return writeFailure(opts, 3, fmt.Errorf("%w: %s not found on the forge: %v", machine.ErrUnmetGuard, what, forgeErr.Err))
	}
	return writeFailure(opts, 7, fmt.Errorf("could not verify %s: %w; retry, or record an explicit override with --unverified --reason", what, err))
}
