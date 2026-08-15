package machine

// State is a resting state for a run (or unit latch).
type State string

const (
	StateIntake  State = "INTAKE"
	StateBuild   State = "BUILD"
	StateVerify  State = "VERIFY"
	StateReview  State = "REVIEW"
	StateDeliver State = "DELIVER"
	StateRework  State = "REWORK"
	// AWAITING_SIGNALS rests when no unit is buildable but delivered units
	// still wait on external signals (checks, review feedback, merge).
	StateAwaitingSignals State = "AWAITING_SIGNALS"
	StateNeedsDecision   State = "NEEDS_DECISION"
	StateBlocked         State = "BLOCKED"
	StateRunDone         State = "RUN_DONE"
)

// UnitPhase is the authoritative per-unit lifecycle position. The run state
// is a projection of unit phases plus the active unit's pipeline position.
type UnitPhase string

const (
	// PhasePending waits for its blockers and a build claim.
	PhasePending UnitPhase = "pending"
	// PhaseActive is the single unit currently in the build pipeline.
	PhaseActive UnitPhase = "active"
	// PhaseRework was pulled back by an external signal and awaits re-build.
	PhaseRework UnitPhase = "rework"
	// PhaseDelivered shipped a change request and awaits external signals.
	PhaseDelivered UnitPhase = "delivered"
	// PhaseDone is settled: the delivered work was accepted.
	PhaseDone UnitPhase = "done"
)

// ObserveSignal is one externally observed event about a delivered unit.
type ObserveSignal string

const (
	SignalMerged         ObserveSignal = "merged"
	SignalChecksFailed   ObserveSignal = "checks_failed"
	SignalReviewFeedback ObserveSignal = "review_feedback"
)

// Command is a named edge command.
type Command string

const (
	CmdInit    Command = "init"
	CmdIntake  Command = "intake"
	CmdRelease Command = "release"
	CmdBuild   Command = "build"
	CmdVerify  Command = "verify"
	CmdReview  Command = "review"
	CmdRework  Command = "rework"
	CmdDeliver Command = "deliver"
	CmdAsk     Command = "ask"
	CmdDecide  Command = "decide"
	CmdRetry   Command = "retry"
	CmdBlock   Command = "block"
	CmdObserve Command = "observe"
)

// RiskTier classifies how much can go wrong when a run's work is wrong.
// Recorded contract data: routing and review-depth policy consume it later.
type RiskTier string

const (
	RiskLow    RiskTier = "low"
	RiskMedium RiskTier = "medium"
	RiskHigh   RiskTier = "high"
)

// Complexity classifies how hard one unit is expected to be.
type Complexity string

const (
	ComplexityLow    Complexity = "low"
	ComplexityMedium Complexity = "medium"
	ComplexityHigh   Complexity = "high"
)

// DeliveryMode is how work is published.
type DeliveryMode string

const (
	DeliveryPRHold           DeliveryMode = "pr-hold"
	DeliveryPRMergeWhenReady DeliveryMode = "pr-merge-when-ready"
	DeliveryDirectTrunk      DeliveryMode = "direct-trunk"
)

// ReviewerIdentity names one registered independent reviewer. Identities
// follow resource-ID rules; the built-ins below are always registered and
// custom identities register through the reviewers registry.
type ReviewerIdentity string

const (
	ReviewerAutoreview ReviewerIdentity = "autoreview"
	ReviewerBugbot     ReviewerIdentity = "bugbot"
	ReviewerHuman      ReviewerIdentity = "human"
)

// BuiltinReviewers returns the identities that are registered in every
// installation. Humans hold the release and recovery latches, not a default
// review identity; a human sign-off reviewer must be registered explicitly.
func BuiltinReviewers() []ReviewerIdentity {
	return []ReviewerIdentity{ReviewerAutoreview, ReviewerBugbot}
}

// ReviewVerdict is the normalized outcome of one independent review.
type ReviewVerdict string

const (
	ReviewClean     ReviewVerdict = "clean"
	ReviewFindings  ReviewVerdict = "findings"
	ReviewAmbiguous ReviewVerdict = "ambiguous"
)
