package machine

import (
	"fmt"
	"slices"
	"strings"
	"unicode"
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
	return validateEvidenceVerification("review", &ev.Verification, ev.UnverifiedReason)
}

const maxUnverifiedReasonBytes = 300

// validateEvidenceVerification checks the driver-stamped verification field:
// the enum is closed, an override always carries its reason, and a reason
// without an override is meaningless.
func validateEvidenceVerification(field string, verification *EvidenceVerification, reason string) error {
	switch *verification {
	case "":
		*verification = VerificationRecorded
	case VerificationObserved, VerificationRecorded, VerificationOverridden:
	default:
		return fmt.Errorf("%w: %s.verification must be observed|recorded|overridden", ErrUnmetGuard, field)
	}
	if *verification != VerificationOverridden {
		if reason != "" {
			return fmt.Errorf("%w: %s.unverified_reason applies only to an overridden verification", ErrUnmetGuard, field)
		}
		return nil
	}
	if strings.TrimSpace(reason) == "" {
		return fmt.Errorf("%w: an unverified override requires a reason", ErrUnmetGuard)
	}
	if len(reason) > maxUnverifiedReasonBytes {
		return fmt.Errorf("%w: %s.unverified_reason exceeds %d bytes", ErrUnmetGuard, field, maxUnverifiedReasonBytes)
	}
	for _, r := range reason {
		if unicode.IsControl(r) || r == '\u2028' || r == '\u2029' {
			return fmt.Errorf("%w: %s.unverified_reason must be a single line without control characters", ErrUnmetGuard, field)
		}
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
	return validateEvidenceVerification("deliver", &ev.Verification, ev.UnverifiedReason)
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
