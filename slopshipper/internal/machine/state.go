package machine

// State is a resting state for a run (or unit latch).
type State string

const (
	StateIntake        State = "INTAKE"
	StateBuild         State = "BUILD"
	StateVerify        State = "VERIFY"
	StateReview        State = "REVIEW"
	StateDeliver       State = "DELIVER"
	StateRework        State = "REWORK"
	StateNeedsDecision State = "NEEDS_DECISION"
	StateBlocked       State = "BLOCKED"
	StateRunDone       State = "RUN_DONE"
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
)

// DeliveryMode is how work is published.
type DeliveryMode string

const (
	DeliveryPRHold           DeliveryMode = "pr-hold"
	DeliveryPRMergeWhenReady DeliveryMode = "pr-merge-when-ready"
	DeliveryDirectTrunk      DeliveryMode = "direct-trunk"
)

// ReviewConsent records the human's chosen review approach for the run.
type ReviewConsent string

const (
	ReviewAutoreview ReviewConsent = "autoreview"
	ReviewBugbot     ReviewConsent = "bugbot"
	ReviewBoth       ReviewConsent = "both"
	ReviewHuman      ReviewConsent = "human"
)

// ReviewerIdentity is who produced review evidence.
type ReviewerIdentity string

const (
	ReviewerAutoreview ReviewerIdentity = "autoreview"
	ReviewerBugbot     ReviewerIdentity = "bugbot"
	ReviewerHuman      ReviewerIdentity = "human"
)

// ReviewVerdict is the normalized outcome of one independent review.
type ReviewVerdict string

const (
	ReviewClean     ReviewVerdict = "clean"
	ReviewFindings  ReviewVerdict = "findings"
	ReviewAmbiguous ReviewVerdict = "ambiguous"
)
