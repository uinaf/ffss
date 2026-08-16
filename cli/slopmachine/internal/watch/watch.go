// Package watch turns forge observations about delivered units into observe
// signals. Decisions are pure: one observation maps to at most one signal,
// and unit phases make recording naturally idempotent — a signal that
// already moved its unit out of the delivered phase is never re-recorded.
package watch

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/uinaf/ffsstack/cli/slopmachine/internal/forge"
	"github.com/uinaf/ffsstack/cli/slopmachine/internal/machine"
)

// Target is one delivered unit with its recorded delivery evidence.
type Target struct {
	UnitID    string
	PRURL     string
	CommitSHA string // delivered head; empty when the evidence recorded none
}

// Outcome is the decision for one delivered unit after one observation.
// Signal is empty when there is nothing to record; Note then says why.
type Outcome struct {
	UnitID    string
	Signal    machine.ObserveSignal
	Reference string
	Note      string
}

// Decide maps one forge observation onto at most one observe signal.
// Precedence: merged settles the unit; a moved head invalidates the delivery
// evidence before check or review outcomes are trusted for it.
func Decide(target Target, obs forge.Observation) Outcome {
	out := Outcome{UnitID: target.UnitID}
	switch obs.Mergeability {
	case forge.MergeableMerged:
		out.Signal = machine.SignalMerged
		out.Reference = target.PRURL
		return out
	case forge.MergeableClosed:
		out.Note = "change request closed without merge; ask for a decision or re-deliver"
		return out
	}
	if HeadMoved(target.CommitSHA, obs.HeadSHA) {
		out.Signal = machine.SignalHeadMoved
		out.Reference = fmt.Sprintf("delivered %s but head is %s", shortSHA(target.CommitSHA), shortSHA(obs.HeadSHA))
		return out
	}
	if obs.Checks == forge.ChecksFailing {
		out.Signal = machine.SignalChecksFailed
		out.Reference = fmt.Sprintf("checks failing at %s", shortSHA(obs.HeadSHA))
		return out
	}
	if obs.UnresolvedThreads > 0 {
		out.Signal = machine.SignalReviewFeedback
		out.Reference = reviewReference(obs)
		return out
	}
	out.Note = fmt.Sprintf("waiting: checks %s, mergeable %s", obs.Checks, obs.Mergeability)
	return out
}

const maxReferenceBytes = 200

// maxTokens keeps the token set within the machine's observe evidence
// bounds; adapters already sample far fewer threads than this.
const maxTokens = 64

// ThreadTokens returns one stable token per sampled unresolved thread
// (thread id + newest comment id), the unit of feedback identity: a token
// set that is a subset of what was already recorded carries nothing new.
func ThreadTokens(obs forge.Observation) []string {
	tokens := make([]string, 0, len(obs.Threads))
	for _, thread := range obs.Threads {
		if thread.Resolved {
			continue
		}
		if len(tokens) == maxTokens {
			break
		}
		if thread.ID != "" {
			tokens = append(tokens, sanitizeText(thread.ID+"@"+thread.LastCommentID+"@"+thread.LastCommentEdited))
			continue
		}
		tokens = append(tokens, sanitizeText(fmt.Sprintf("%s:%d:%s", thread.Path, thread.Line, thread.Snippet)))
	}
	return tokens
}

// reviewReference identifies the unresolved-feedback set, not just its size:
// the fingerprint hashes every sampled unresolved thread's location and its
// newest comment, so a new comment on an old thread reads as new feedback
// while an unchanged set keeps an identical reference across passes.
func reviewReference(obs forge.Observation) string {
	reference := fmt.Sprintf("%d unresolved review thread(s)", obs.UnresolvedThreads)
	digest := sha256.New()
	fmt.Fprintf(digest, "count=%d", obs.UnresolvedThreads)
	// The adapter's digest covers EVERY unresolved thread; the sample-based
	// fallback exists for adapters that do not provide one.
	if obs.ThreadsDigest != "" {
		fmt.Fprintf(digest, "|all=%s", obs.ThreadsDigest)
	}
	located := false
	for _, thread := range obs.Threads {
		if thread.Resolved {
			continue
		}
		if obs.ThreadsDigest == "" {
			if thread.ID != "" {
				fmt.Fprintf(digest, "|%s@%s", thread.ID, thread.LastCommentID)
			} else {
				fmt.Fprintf(digest, "|%s:%d:%s", thread.Path, thread.Line, thread.Snippet)
			}
		}
		if !located && thread.Path != "" {
			reference += fmt.Sprintf("; first at %s:%d", sanitizeText(thread.Path), thread.Line)
			located = true
		}
	}
	return fmt.Sprintf("%s; fp:%x", reference, digest.Sum(nil)[:6])
}

// sanitizeText neutralizes forge-controlled text before it becomes durable
// evidence: control characters and line separators would otherwise forge
// output lines or escape sequences when the cause is rendered later.
func sanitizeText(value string) string {
	sanitized := strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || r == '\u2028' || r == '\u2029' {
			return '?'
		}
		return r
	}, value)
	if len(sanitized) > maxReferenceBytes {
		// Truncate on a rune boundary: a split UTF-8 sequence would decode
		// differently after a JSON round trip and break reference equality.
		cut := maxReferenceBytes
		for cut > 0 && !utf8.RuneStart(sanitized[cut]) {
			cut--
		}
		return sanitized[:cut]
	}
	return sanitized
}

// HeadMoved reports whether the observed head no longer matches the
// delivered revision. Delivery evidence may record an abbreviated SHA; a
// hex prefix of at least seven characters counts as the same revision, and
// evidence too short or non-hex to judge safely never claims movement.
func HeadMoved(recorded, head string) bool {
	recorded = strings.ToLower(recorded)
	head = strings.ToLower(head)
	if recorded == "" || head == "" {
		return false
	}
	if recorded == head {
		return false
	}
	if !isHex(recorded) || !isHex(head) || len(recorded) < 7 {
		return false
	}
	if len(recorded) <= len(head) {
		return !strings.HasPrefix(head, recorded)
	}
	return !strings.HasPrefix(recorded, head)
}

func isHex(value string) bool {
	for _, r := range value {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') && (r < 'A' || r > 'F') {
			return false
		}
	}
	return true
}

// shortSHA abbreviates for references; the observed value is forge-supplied,
// so it is sanitized like any other forge-controlled text.
func shortSHA(sha string) string {
	if sha == "" {
		return "(unknown)"
	}
	if len(sha) > 12 {
		sha = sha[:12]
	}
	return sanitizeText(sha)
}
