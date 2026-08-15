// Package forge is the observation seam between the state machine and code
// hosts. The machine and store never import an adapter; they consume the
// five observation reads through this interface, and unknown forge kinds
// fail closed. Adapters read only — delivery and merges stay human- and
// driver-owned.
package forge

import (
	"context"
	"errors"
	"fmt"
)

// Kind names a supported forge implementation.
type Kind string

const KindGitHub Kind = "github"

// ErrUnknownKind marks a forge kind with no registered adapter.
var ErrUnknownKind = errors.New("unknown forge kind")

// ErrorKind is the stable failure taxonomy for observation reads.
type ErrorKind string

const (
	ErrorAuth      ErrorKind = "auth"
	ErrorRateLimit ErrorKind = "rate_limit"
	ErrorTransient ErrorKind = "transient"
	ErrorNotFound  ErrorKind = "not_found"
)

// Error wraps an observation failure with its classification.
type Error struct {
	Kind ErrorKind
	Err  error
}

func (e *Error) Error() string { return fmt.Sprintf("%s: %v", e.Kind, e.Err) }
func (e *Error) Unwrap() error { return e.Err }

// ChangeRequestRef locates one change request on a forge.
type ChangeRequestRef struct {
	Owner  string
	Repo   string
	Number int
}

func (r ChangeRequestRef) String() string {
	return fmt.Sprintf("%s/%s#%d", r.Owner, r.Repo, r.Number)
}

// ChecksState is the aggregate CI outcome for a head revision.
type ChecksState string

const (
	ChecksPending ChecksState = "pending"
	ChecksPassing ChecksState = "passing"
	ChecksFailing ChecksState = "failing"
	ChecksNone    ChecksState = "none"
)

// Mergeability is the forge's view of whether the change request can land.
type Mergeability string

const (
	MergeableClean       Mergeability = "clean"
	MergeableConflicting Mergeability = "conflicting"
	MergeableUnknown     Mergeability = "unknown"
	MergeableMerged      Mergeability = "merged"
	MergeableClosed      Mergeability = "closed"
)

// ReviewThread is one unresolved review conversation on a change request.
type ReviewThread struct {
	// ID and LastCommentID are the forge's stable identifiers; feedback
	// identity prefers them over location and text. LastCommentEdited
	// changes when the newest comment is edited in place.
	ID                string
	LastCommentID     string
	LastCommentEdited string
	Author            string
	Path              string
	Line              int
	Snippet           string // first line of the newest comment, bounded by the adapter
	Resolved          bool
}

// Observation is one read of a change request's externally visible state.
type Observation struct {
	Ref          ChangeRequestRef
	HeadSHA      string
	Checks       ChecksState
	Mergeability Mergeability
	// UnresolvedThreads counts open review conversations; Threads carries a
	// bounded sample of them for evidence, newest first. ThreadsDigest
	// fingerprints EVERY unresolved thread's identity and newest comment —
	// not just the sample — so feedback beyond the sample bound is still
	// distinguishable.
	UnresolvedThreads int
	Threads           []ReviewThread
	ThreadsDigest     string
}

// Review is one submitted (or pending) review on a change request, read for
// evidence corroboration only — adapters never create or mutate reviews.
type Review struct {
	Author      string
	State       string // forge-native state, e.g. APPROVED, CHANGES_REQUESTED, COMMENTED, DISMISSED, PENDING
	SubmittedAt string
}

// Forge exposes the read-only observation seam over one change request.
type Forge interface {
	// Kind names the adapter.
	Kind() Kind
	// ParseChangeRequestURL validates that url belongs to this forge and
	// extracts its reference.
	ParseChangeRequestURL(url string) (ChangeRequestRef, error)
	// Observe fetches the current head SHA, checks state, mergeability, and
	// unresolved review threads for ref in one read.
	Observe(ctx context.Context, ref ChangeRequestRef) (Observation, error)
	// Head fetches only existence and the current head revision; delivery
	// verification must not fail on faults in reads it does not need.
	Head(ctx context.Context, ref ChangeRequestRef) (string, error)
	// Reviews fetches the change request's reviews for evidence
	// corroboration.
	Reviews(ctx context.Context, ref ChangeRequestRef) ([]Review, error)
}

// New returns the adapter for kind, failing closed on unknown kinds.
func New(kind Kind) (Forge, error) {
	switch kind {
	case KindGitHub:
		return NewGitHub(nil), nil
	default:
		return nil, fmt.Errorf("%w: %q (supported: %s)", ErrUnknownKind, kind, KindGitHub)
	}
}
