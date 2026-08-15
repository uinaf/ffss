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
	SeriesBound        int
	CompletedUnits     int
	CurrentUnitID      string
	CompletedReviewers []ReviewerIdentity
	BlockerReason      string
	DecisionQuestion   string
	ReturnState        State // resume target after NEEDS_DECISION; empty defaults to INTAKE
}

// Unit is one graph node.
type Unit struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Blockers []string `json:"blockers"`
	Attempt  int      `json:"attempt"`
	Done     bool     `json:"done"`
}

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

// DeliverEvidence gates DELIVER completion for a unit.
type DeliverEvidence struct {
	DeliveryMode DeliveryMode `json:"delivery_mode"`
	PRURL        string       `json:"pr_url,omitempty"`
	CommitSHA    string       `json:"commit_sha,omitempty"`
}

// IntakePatch updates contract fields; always bumps intake revision.
type IntakePatch struct {
	DeliveryMode      *DeliveryMode
	RequiredReviewers []ReviewerIdentity // replaces the required set when non-nil
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

// ApplyInput carries command-specific payloads.
type ApplyInput struct {
	ExpectedRevision int64 // 0 means "use current" for tests; store always passes real revision
	IntakeRevision   int64 // required for release
	// RegisteredReviewers is the custom registry snapshot the caller loaded;
	// built-in identities are always allowed in addition to this set.
	RegisteredReviewers []ReviewerIdentity
	Intake              *IntakePatch
	Verify              *VerifyEvidence
	Review              *ReviewEvidence
	Deliver             *DeliverEvidence
	Decision            *Decision
	RetryReason         string
	BlockReason         string
	Question            string // when forcing NEEDS_DECISION from a command path
}

// ApplyResult is the post-transition snapshot hint for status/next_action.
type ApplyResult struct {
	Run       Run
	Units     []Unit
	EventFrom State
	EventTo   State
	Command   Command
	Evidence  any
}
