package machine

import (
	"fmt"
	"slices"
	"strings"
	"unicode"
)

// Role names one machine-defined responsibility a repo profile binds to
// concrete implementations. The machine defines roles only; profiles name
// the vendors, so no vendor ever leaks into the state machine.
type Role string

const (
	RoleReview Role = "review"
	RoleQA     Role = "qa"
	RoleVenue  Role = "venue"
	RoleMemory Role = "memory"
)

// Roles returns every bindable role.
func Roles() []Role { return []Role{RoleReview, RoleQA, RoleVenue, RoleMemory} }

// ForgeKind names the forge family hosting a repo's change requests.
// Recorded data only until an adapter consumes it.
type ForgeKind string

const ForgeGitHub ForgeKind = "github"

// TrustTier records how much autonomy the repo has earned.
type TrustTier string

const (
	TrustLow    TrustTier = "low"
	TrustMedium TrustTier = "medium"
	TrustHigh   TrustTier = "high"
)

// Readiness is the recorded agent-readiness verdict for the repo. It is
// declared input (for example from an audit), never probed by the binary.
type Readiness string

const (
	ReadinessReady    Readiness = "ready"
	ReadinessNotReady Readiness = "not_ready"
)

// RepoProfile binds roles to repo-local implementations and records repo
// policy. Every field except RepoKey is optional; an absent field keeps
// profile-less behavior for that concern.
type RepoProfile struct {
	RepoKey       string
	ForgeKind     ForgeKind
	TrustTier     TrustTier
	VerifyCommand string
	DeliveryMode  DeliveryMode
	Readiness     Readiness
	Bindings      map[Role][]string
	// ForgeReviewers maps a registered reviewer identity to the forge login
	// its reviews are submitted under, marking that reviewer as
	// forge-resident: its review evidence is corroborated against the live
	// change request instead of trusted as recorded input.
	ForgeReviewers map[string]string
}

const maxVerifyCommandBytes = 500

// ValidateProfile checks every declared field fail-closed and normalizes
// bindings (empty role lists are dropped).
func ValidateProfile(p *RepoProfile) error {
	if p.RepoKey == "" {
		return fmt.Errorf("%w: profile repo key required", ErrBadArgs)
	}
	switch p.ForgeKind {
	case "", ForgeGitHub:
	default:
		return fmt.Errorf("%w: forge kind must be github", ErrBadArgs)
	}
	switch p.TrustTier {
	case "", TrustLow, TrustMedium, TrustHigh:
	default:
		return fmt.Errorf("%w: trust tier must be low|medium|high", ErrBadArgs)
	}
	switch p.Readiness {
	case "", ReadinessReady, ReadinessNotReady:
	default:
		return fmt.Errorf("%w: readiness must be ready|not_ready", ErrBadArgs)
	}
	if p.DeliveryMode != "" {
		if err := validDelivery(p.DeliveryMode); err != nil {
			return err
		}
	}
	if err := validVerifyCommandText(p.VerifyCommand); err != nil {
		return err
	}
	for role, names := range p.Bindings {
		if !slices.Contains(Roles(), role) {
			return fmt.Errorf("%w: unknown role %q; roles are review|qa|venue|memory", ErrBadArgs, role)
		}
		if len(names) == 0 {
			delete(p.Bindings, role)
			continue
		}
		seen := make(map[string]struct{}, len(names))
		for _, name := range names {
			if err := ValidateResourceID(string(role)+" binding", name); err != nil {
				return err
			}
			if _, dup := seen[name]; dup {
				return fmt.Errorf("%w: duplicate %s binding %q", ErrBadArgs, role, name)
			}
			seen[name] = struct{}{}
		}
	}
	if len(p.ForgeReviewers) > 0 && p.ForgeKind == "" {
		return fmt.Errorf("%w: forge reviewers require a forge kind; declare one with --forge or drop the mapping", ErrBadArgs)
	}
	for identity, login := range p.ForgeReviewers {
		if err := ValidateResourceID("forge reviewer", identity); err != nil {
			return err
		}
		if err := validForgeLogin(identity, login); err != nil {
			return err
		}
	}
	return nil
}

const maxForgeLoginBytes = 64

// validForgeLogin checks a declared forge login: alphanumerics and hyphens
// with an optional [bot] suffix, matching GitHub's login charset. GraphQL
// reads return bot logins without the suffix, so corroboration strips it on
// both sides before comparing.
func validForgeLogin(identity, login string) error {
	if len(login) > maxForgeLoginBytes {
		return fmt.Errorf("%w: forge login for %q exceeds %d bytes", ErrBadArgs, identity, maxForgeLoginBytes)
	}
	base := strings.TrimSuffix(login, "[bot]")
	if base == "" || strings.HasPrefix(base, "-") || strings.HasSuffix(base, "-") {
		return fmt.Errorf("%w: forge login for %q must be a login name with an optional [bot] suffix", ErrBadArgs, identity)
	}
	for _, r := range base {
		if (r < '0' || r > '9') && (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && r != '-' {
			return fmt.Errorf("%w: forge login for %q must contain only alphanumerics and hyphens (optional [bot] suffix)", ErrBadArgs, identity)
		}
	}
	return nil
}

// NormalizeForgeLogin canonicalizes a login for corroboration comparison:
// the [bot] suffix is representation (REST includes it, GraphQL omits it),
// and GitHub logins are case-insensitive.
func NormalizeForgeLogin(login string) string {
	return strings.ToLower(strings.TrimSuffix(login, "[bot]"))
}

func validVerifyCommandText(command string) error {
	if command == "" {
		return nil
	}
	if strings.TrimSpace(command) == "" {
		return fmt.Errorf("%w: verify command must not be blank; omit it instead", ErrBadArgs)
	}
	if len(command) > maxVerifyCommandBytes {
		return fmt.Errorf("%w: verify command exceeds %d bytes", ErrBadArgs, maxVerifyCommandBytes)
	}
	for _, r := range command {
		if unicode.IsControl(r) || r == '\u2028' || r == '\u2029' {
			return fmt.Errorf("%w: verify command must be a single line without control characters", ErrBadArgs)
		}
	}
	return nil
}

// ProfileAllowsReviewers enforces declaration over detection: in a registered
// repo, every contract-required reviewer must be bound to the review role.
// A nil profile keeps profile-less behavior.
func ProfileAllowsReviewers(profile *RepoProfile, required []ReviewerIdentity) error {
	if profile == nil {
		return nil
	}
	bound := profile.Bindings[RoleReview]
	if len(bound) == 0 {
		return fmt.Errorf("%w: the repo profile binds no review implementations; bind them with slopshipper repo update --bind 'review=<name>' or unregister the profile", ErrUnmetGuard)
	}
	for _, reviewer := range required {
		if !slices.Contains(bound, string(reviewer)) {
			return fmt.Errorf("%w: required reviewer %q has no review binding in the repo profile; bind it with slopshipper repo update --bind or drop it from required_reviewers", ErrUnmetGuard, reviewer)
		}
	}
	return nil
}
