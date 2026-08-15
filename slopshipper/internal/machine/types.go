package machine

// Run is the durable orchestration record for one slopshipper run.
type Run struct {
	ID                 string
	RepoKey            string
	State              State
	IntakeRevision     int64
	ReleasedRevision   *int64
	Revision           int64
	DeliveryMode       DeliveryMode
	RequiredReviewers  []ReviewerIdentity
	RiskTier           RiskTier // empty until the contract declares it
	Budget             Budget
	SeriesBound        int
	CompletedUnits     int
	CurrentUnitID      string
	CompletedReviewers []ReviewerIdentity
	BlockerReason      string
	DecisionQuestion   string
	ReturnState        State // resume target after NEEDS_DECISION; empty defaults to INTAKE
}

// Unit is one graph node. Phase is authoritative per-unit state: a unit's
// blockers release once every blocker reaches delivered or done, so later
// units can build while earlier ones await external signals.
type Unit struct {
	ID                 string     `json:"id"`
	Title              string     `json:"title"`
	Blockers           []string   `json:"blockers"`
	AcceptanceCriteria []string   `json:"acceptance_criteria,omitempty"`
	Complexity         Complexity `json:"complexity,omitempty"`
	Attempt            int        `json:"attempt"`
	Phase              UnitPhase  `json:"phase"`
	// ReworkCause records why a delivered unit re-entered the build loop.
	ReworkCause string `json:"rework_cause,omitempty"`
}

// Settled reports whether the unit no longer blocks dependents.
func (u Unit) Settled() bool { return u.Phase == PhaseDelivered || u.Phase == PhaseDone }

// Budget bounds a run's spend as recorded contract data. Zero means the
// dimension is unbounded; enforcement belongs to routing, not the machine.
type Budget struct {
	Tokens  int `json:"tokens,omitempty"`
	Minutes int `json:"minutes,omitempty"`
}

// IsZero reports whether no budget dimension is set.
func (b Budget) IsZero() bool { return b.Tokens == 0 && b.Minutes == 0 }

// Released reports whether the human release latch is valid for the current intake.
func (r *Run) Released() bool {
	return r.ReleasedRevision != nil && *r.ReleasedRevision == r.IntakeRevision
}

// VerifyEvidence gates VERIFY → REVIEW.
type VerifyEvidence struct {
	Command      string `json:"command"`
	ExitCode     int    `json:"exit_code"`
	OutputDigest string `json:"output_digest,omitempty"`
}

// ReviewEvidence gates REVIEW → DELIVER.
type ReviewEvidence struct {
	Reviewer    ReviewerIdentity `json:"reviewer"`
	Verdict     ReviewVerdict    `json:"verdict"`
	ArtifactRef string           `json:"artifact_ref"`
}

// DeliverEvidence gates DELIVER completion for a unit. UnitID is stamped by
// the machine from the unit being delivered, so later observation can
// correlate forge state back to its unit.
type DeliverEvidence struct {
	UnitID       string       `json:"unit,omitempty"`
	DeliveryMode DeliveryMode `json:"delivery_mode"`
	PRURL        string       `json:"pr_url,omitempty"`
	CommitSHA    string       `json:"commit_sha,omitempty"`
}

// IntakePatch updates contract fields; always bumps intake revision.
type IntakePatch struct {
	DeliveryMode      *DeliveryMode
	RequiredReviewers []ReviewerIdentity // replaces the required set when non-nil
	RiskTier          *RiskTier
	Budget            *Budget // replaces the whole budget when non-nil
	SeriesBound       *int
	Units             []Unit // replaces unit graph when non-nil
}

// Decision resolves NEEDS_DECISION.
type Decision struct {
	Answer string
}

// IntakeEvidence is the complete contract snapshot created by an intake event.
type IntakeEvidence struct {
	IntakeRevision    int64              `json:"intake_revision"`
	DeliveryMode      DeliveryMode       `json:"delivery_mode"`
	RequiredReviewers []ReviewerIdentity `json:"required_reviewers"`
	RiskTier          RiskTier           `json:"risk_tier,omitempty"`
	Budget            Budget             `json:"budget,omitzero"`
	SeriesBound       int                `json:"series_bound"`
	Units             []Unit             `json:"units"`
}

type InitEvidence struct {
	DeliveryMode      DeliveryMode       `json:"delivery_mode"`
	RequiredReviewers []ReviewerIdentity `json:"required_reviewers"`
	SeriesBound       int                `json:"series_bound"`
}

type ReleaseEvidence struct {
	IntakeRevision int64 `json:"intake_revision"`
}

type QuestionEvidence struct {
	Question string `json:"question"`
}

type DecisionEvidence struct {
	Answer string `json:"answer"`
}

type RetryEvidence struct {
	Reason string `json:"reason"`
}

type BlockEvidence struct {
	Reason string `json:"reason"`
}

// ObserveEvidence records one external signal about a delivered unit.
// UnitID may be empty when exactly one delivered unit exists.
type ObserveEvidence struct {
	UnitID    string        `json:"unit,omitempty"`
	Signal    ObserveSignal `json:"signal"`
	Reference string        `json:"reference,omitempty"`
	// ThreadTokens identify the unresolved feedback set behind a
	// review_feedback signal (one stable token per thread); watch compares
	// them across observations so only genuinely new feedback re-triggers.
	ThreadTokens []string `json:"thread_tokens,omitempty"`
}

// ApplyInput carries command-specific payloads.
type ApplyInput struct {
	ExpectedRevision int64 // 0 means "use current" for tests; store always passes real revision
	IntakeRevision   int64 // required for release
	// RegisteredReviewers is the custom registry snapshot the caller loaded;
	// built-in identities are always allowed in addition to this set.
	RegisteredReviewers []ReviewerIdentity
	// Profile is the registered repo profile snapshot, nil when the repo is
	// unregistered. A registered profile must bind every required reviewer.
	Profile *RepoProfile
	// Telemetry is optional recorded input for this transition; absent
	// telemetry never blocks a command.
	Telemetry   *Telemetry
	Intake      *IntakePatch
	Verify      *VerifyEvidence
	Review      *ReviewEvidence
	Deliver     *DeliverEvidence
	Observe     *ObserveEvidence
	Decision    *Decision
	RetryReason string
	BlockReason string
	Question    string // when forcing NEEDS_DECISION from a command path
}

// ApplyResult is the post-transition snapshot hint for status/next_action.
type ApplyResult struct {
	Run       Run
	Units     []Unit
	EventFrom State
	EventTo   State
	Command   Command
	Evidence  any
	// Telemetry is the validated recorded input for this transition, nil
	// when none was supplied.
	Telemetry *Telemetry
}
