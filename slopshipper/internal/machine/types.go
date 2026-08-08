package machine

// Run is the durable orchestration record for one slopomatic run.
type Run struct {
	ID               string
	RepoKey          string
	State            State
	IntakeRevision   int64
	ReleasedRevision *int64
	Revision         int64
	DeliveryMode     DeliveryMode
	ReviewConsent    ReviewConsent
	SeriesBound      int
	CompletedUnits   int
	CurrentUnitID    string
	BlockerReason    string
	DecisionQuestion string
	ReturnState      State // resume target after NEEDS_DECISION; empty defaults to INTAKE
}

// Unit is one graph node.
type Unit struct {
	ID       string
	Title    string
	Blockers []string
	Attempt  int
	Done     bool
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
	Verdict     string           `json:"verdict"`
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
	DeliveryMode  *DeliveryMode
	ReviewConsent *ReviewConsent
	SeriesBound   *int
	Units         []Unit // replaces unit graph when non-nil
}

// Decision resolves NEEDS_DECISION.
type Decision struct {
	Answer string
}

// ApplyInput carries command-specific payloads.
type ApplyInput struct {
	ExpectedRevision int64 // 0 means "use current" for tests; store always passes real revision
	IntakeRevision   int64 // required for release
	Intake           *IntakePatch
	Verify           *VerifyEvidence
	Review           *ReviewEvidence
	Deliver          *DeliverEvidence
	Decision         *Decision
	BlockReason      string
	Question         string // when forcing NEEDS_DECISION from a command path
}

// ApplyResult is the post-transition snapshot hint for status/next_action.
type ApplyResult struct {
	Run              Run
	Units            []Unit
	EventFrom        State
	EventTo          State
	Command          Command
	AllowedCommands  []Command
	NextAction       string
	RequiredEvidence []string
}
