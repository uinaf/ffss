package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/uinaf/ffsstack/cli/slopmachine/internal/forge"
	"github.com/uinaf/ffsstack/cli/slopmachine/internal/machine"
	"github.com/uinaf/ffsstack/cli/slopmachine/internal/watch"
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
	// Existence, openness, and head are the whole delivery proof; the narrow
	// read keeps faults in reads deliver does not need (review threads) out
	// of it.
	hs, err := adapter.Head(ctx, ref)
	if err != nil {
		return observationFailure(fmt.Sprintf("delivery change request %s", ref), err, opts)
	}
	// An already merged or closed change request cannot be the delivery of
	// new work; accepting one lets a later watch pass settle the unit on
	// somebody else's merge.
	if hs.Merged || hs.Closed {
		state := "closed"
		if hs.Merged {
			state = "merged"
		}
		return writeFailure(opts, 3, fmt.Errorf("%w: delivery change request %s is already %s; delivery evidence must name the open change request that ships this unit", machine.ErrUnmetGuard, ref, state))
	}
	head := hs.SHA
	// A payload without a judgeable head cannot corroborate anything; that
	// is an incomplete observation, never a silent pass.
	if !validObservedHead(head) {
		return observationFailure(fmt.Sprintf("delivery change request %s", ref),
			&forge.Error{Kind: forge.ErrorTransient, Err: fmt.Errorf("forge returned no usable head revision")}, opts)
	}
	if ev.CommitSHA != "" && watch.HeadMoved(ev.CommitSHA, head) {
		return writeFailure(opts, 3, fmt.Errorf("%w: delivered head mismatch: evidence records %s but the forge head of %s is %s; re-deliver the current head or fix the evidence", machine.ErrUnmetGuard, ev.CommitSHA, ref, head))
	}
	// Evidence without a recorded revision anchors to the built checkout:
	// the forge head must match the local head before it is adopted as the
	// delivered revision. Adopting an arbitrary observed head would let any
	// open change request stand in for the delivered work.
	if ev.CommitSHA == "" {
		local, lerr := localHeadRevision(ctx)
		if lerr != nil {
			return writeFailure(opts, 3, fmt.Errorf("%w: cannot read the built checkout's head to corroborate the delivery (%v); record deliver.commit_sha for the revision that was actually delivered", machine.ErrUnmetGuard, lerr))
		}
		if watch.HeadMoved(local, head) {
			return writeFailure(opts, 3, fmt.Errorf("%w: forge head of %s is %s but the built checkout head is %s; deliver the built revision or record deliver.commit_sha for the revision that was actually delivered", machine.ErrUnmetGuard, ref, head, local))
		}
		ev.CommitSHA = head
	}
	ev.Verification = machine.VerificationObserved
	return 0
}

// localHeadRevision reads the built checkout's current head; delivery
// evidence without an explicit revision anchors to it.
func localHeadRevision(ctx context.Context) (string, error) {
	out, err := exec.CommandContext(ctx, "git", "rev-parse", "HEAD").Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse HEAD: %v", err)
	}
	head := strings.TrimSpace(string(out))
	if !validObservedHead(head) {
		return "", fmt.Errorf("git rev-parse HEAD returned no usable revision")
	}
	return head, nil
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
		// On a forge-bound repo, a reviewer without a forge mapping must at
		// least leave a resolvable artifact; a verdict with nothing behind
		// it is a claim, not evidence.
		if adapter != nil {
			if err := resolveLocalArtifact(ev.ArtifactRef); err != nil {
				return writeFailure(opts, 3, fmt.Errorf("%w: reviewer %q is recorded input on a forge-bound repo, so review.artifact_ref must resolve to the reviewer's result artifact (%v); point it at the result file, or map the reviewer to its forge login with repo update --forge-reviewer", machine.ErrUnmetGuard, ev.Reviewer, err))
			}
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

// resolveLocalArtifact requires a local review artifact reference to point
// at an existing, non-empty file: a file:// URL or an absolute path.
func resolveLocalArtifact(ref string) error {
	var path string
	switch {
	case strings.HasPrefix(ref, "file://"):
		path = strings.TrimPrefix(ref, "file://")
	case strings.HasPrefix(ref, "/"):
		path = ref
	default:
		return fmt.Errorf("artifact_ref %q is not a file:// URL or absolute path", ref)
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("artifact %q is not readable: %v", path, err)
	}
	if info.IsDir() {
		return fmt.Errorf("artifact %q is a directory", path)
	}
	if info.Size() == 0 {
		return fmt.Errorf("artifact %q is empty", path)
	}
	return nil
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
