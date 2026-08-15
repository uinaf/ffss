package machine

import (
	"fmt"
	"slices"
	"strings"
)

func validateVerifyCommand(ev *VerifyEvidence) error {
	if ev == nil {
		return fmt.Errorf("%w: verify evidence required", ErrUnmetGuard)
	}
	if strings.TrimSpace(ev.Command) == "" {
		return fmt.Errorf("%w: verify.command required", ErrUnmetGuard)
	}
	return nil
}

func validateReviewEvidence(ev *ReviewEvidence, required []ReviewerIdentity) error {
	if ev == nil {
		return fmt.Errorf("%w: review evidence required", ErrUnmetGuard)
	}
	switch ev.Verdict {
	case ReviewClean, ReviewFindings, ReviewAmbiguous:
	default:
		return fmt.Errorf("%w: review.verdict must be clean|findings|ambiguous", ErrUnmetGuard)
	}
	if strings.TrimSpace(ev.ArtifactRef) == "" {
		return fmt.Errorf("%w: review.artifact_ref required", ErrUnmetGuard)
	}
	if err := ValidateResourceID("review.reviewer", string(ev.Reviewer)); err != nil {
		return err
	}
	if !slices.Contains(required, ev.Reviewer) {
		return fmt.Errorf("%w: review.reviewer %q is not a required reviewer for this run (%s)", ErrUnmetGuard, ev.Reviewer, joinReviewers(required))
	}
	return nil
}

func joinReviewers(reviewers []ReviewerIdentity) string {
	names := make([]string, 0, len(reviewers))
	for _, reviewer := range reviewers {
		names = append(names, string(reviewer))
	}
	return strings.Join(names, ", ")
}

func validateDeliverEvidence(ev *DeliverEvidence, mode DeliveryMode) error {
	if ev == nil {
		return fmt.Errorf("%w: deliver evidence required", ErrUnmetGuard)
	}
	if ev.DeliveryMode == "" {
		ev.DeliveryMode = mode
	}
	if ev.DeliveryMode != mode {
		return fmt.Errorf("%w: deliver.delivery_mode must match run (%s)", ErrUnmetGuard, mode)
	}
	switch mode {
	case DeliveryPRHold, DeliveryPRMergeWhenReady:
		if strings.TrimSpace(ev.PRURL) == "" {
			return fmt.Errorf("%w: deliver.pr_url required for %s", ErrUnmetGuard, mode)
		}
	case DeliveryDirectTrunk:
		if strings.TrimSpace(ev.CommitSHA) == "" {
			return fmt.Errorf("%w: deliver.commit_sha required for direct-trunk", ErrUnmetGuard)
		}
	default:
		return fmt.Errorf("%w: unknown delivery mode %q", ErrUnmetGuard, mode)
	}
	// A recorded revision must be a judgeable commit identity, or head
	// movement could never be detected against it.
	if ev.CommitSHA != "" {
		if err := validCommitSHA(ev.CommitSHA); err != nil {
			return err
		}
	}
	return nil
}

func validCommitSHA(sha string) error {
	if len(sha) < 7 || len(sha) > 64 {
		return fmt.Errorf("%w: deliver.commit_sha must be 7-64 hex characters", ErrUnmetGuard)
	}
	for _, r := range sha {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') && (r < 'A' || r > 'F') {
			return fmt.Errorf("%w: deliver.commit_sha must be hexadecimal", ErrUnmetGuard)
		}
	}
	return nil
}
