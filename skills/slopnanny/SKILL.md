---
name: slopnanny
description: "Babysit an open change request through review and CI: verify bot claims, fix real findings, answer threads with commit hashes, merge once green unless asked to hold. Not for creating change requests or reviewing."
---

# Slopnanny

Settle one delivered change request from real forge evidence. Keep rework
within its contract; independent review belongs to the required reviewer.

## Observe

- With an active slopmachine run, the binary is the observation authority:
  `slopmachine watch --once` (or `--interval SECONDS` for a bounded poll).
  Act on the cause it records; [slopmachine](../slopmachine/SKILL.md) owns
  the signal vocabulary.
- Without a run, poll through the forge CLI the delivery dispatched to
  (`gh` / `glab`): checks state, pending review requests, review verdicts (a
  `CHANGES_REQUESTED` review may carry no inline thread), unresolved review
  threads, and top-level comments; bots often report findings as ordinary
  comments.
- Don't treat green checks and an empty thread list as settled while a
  requested reviewer is still pending, including an automated reviewer
  still working. After it submits, re-read verdicts, comments, and threads
  before deciding.
- Require checks and head-specific review evidence for the latest commit.
  Do not reprocess findings already answered by a push, but do not assume a
  push resolved older feedback: assess remaining blocking verdicts and open
  threads against the current code.
- Unchanged observations need no new tests, review invocation, or reply.

## Triage findings

1. Validate claims against current code, the task contract, and any stronger
   invariant. Reject incorrect or out-of-scope findings with concrete evidence;
   real shortcomings outside the goal become tracker items, not commits.
2. Batch accepted findings into one scoped rework pass. Run focused checks
   during editing, then the repository's required gates and affected runtime
   proof once the batch is ready. Refresh required independent review for the
   final changed target at its prescribed gate, not after each thread fix.
3. Push verified fixes and reply on each addressed thread with its commit hash.
   Never force-push without explicit approval. Let requested forge reviewers
   finish on the new head; do not start duplicate reviews while they work.
4. Attach visual proof only when it is the clearest evidence; use the
   [slopcourier visual-evidence ladder](../slopcourier/references/visual-evidence.md).

Replies speak as the authenticated account: first person, concise evidence,
and commit hashes. Post only content that advances the thread.

## Quiet discipline

- Nothing changed → say nothing. No filler comments, no status noise.
- When required checks are green on the latest commit, no review request is
  pending, and reviewers and threads are green, merge with the repository's
  merge method and report the merged commit; the babysit request carries merge
  authority. Hold at green only when the request says to, and never merge past
  a blocking human review or an unresolved thread.
- On a slopmachine run, keep driving status: route rework causes through
  `slopmachine` commands, and let settlement come from `watch` observing the
  merge, never from your narration.
