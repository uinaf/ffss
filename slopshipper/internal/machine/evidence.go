package machine

import (
	"fmt"
	"strings"
)

func validateVerifyEvidence(ev *VerifyEvidence) error {
	if ev == nil {
		return fmt.Errorf("%w: verify evidence required", ErrUnmetGuard)
	}
	if strings.TrimSpace(ev.Command) == "" {
		return fmt.Errorf("%w: verify.command required", ErrUnmetGuard)
	}
	if ev.ExitCode != 0 {
		return fmt.Errorf("%w: verify.exit_code must be 0 for success path", ErrUnmetGuard)
	}
	return nil
}

func validateReviewEvidence(ev *ReviewEvidence, consent ReviewConsent) error {
	if ev == nil {
		return fmt.Errorf("%w: review evidence required", ErrUnmetGuard)
	}
	if strings.TrimSpace(ev.Verdict) == "" {
		return fmt.Errorf("%w: review.verdict required", ErrUnmetGuard)
	}
	if strings.TrimSpace(ev.ArtifactRef) == "" {
		return fmt.Errorf("%w: review.artifact_ref required", ErrUnmetGuard)
	}
	switch ev.Reviewer {
	case ReviewerAutoreview, ReviewerBugbot, ReviewerHuman:
	default:
		return fmt.Errorf("%w: review.reviewer must be autoreview|bugbot|human", ErrUnmetGuard)
	}
	if !reviewerMatchesConsent(ev.Reviewer, consent) {
		return fmt.Errorf("%w: review.reviewer %q does not match review_consent %q", ErrUnmetGuard, ev.Reviewer, consent)
	}
	return nil
}

func reviewerMatchesConsent(reviewer ReviewerIdentity, consent ReviewConsent) bool {
	switch consent {
	case ReviewBoth:
		return reviewer == ReviewerAutoreview || reviewer == ReviewerBugbot
	case ReviewAutoreview:
		return reviewer == ReviewerAutoreview
	case ReviewBugbot:
		return reviewer == ReviewerBugbot
	case ReviewHuman:
		return reviewer == ReviewerHuman
	default:
		return false
	}
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
	return nil
}
