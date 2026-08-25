---
name: slopnanny
description: "Babysit an open change request through review and CI: verify bot claims, fix real findings, answer threads with commit hashes, merge once green unless asked to hold. Not for creating change requests or reviewing."
---

# Slopnanny

Walk one delivered change request to a settled outcome. Respond to what the
forge shows; never invent signals, review your own work, or widen the
change.

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
- Act only on checks, reviews, and comments newer than the latest push;
  everything older was already answered by that push.

## Triage findings

1. Treat every bot or reviewer claim as a hypothesis. Verify it against the
   exact code and the change's contract before acting; bots are helpful and
   sometimes wrong.
2. Fix a real finding with the smallest change at the owning boundary, rerun
   the repository's own gates, push, and reply on the thread with the commit
   hash. Never force-push without explicit approval, even when rework
   after `head_moved` tempts a rebase.
3. Reject an incorrect or out-of-scope finding in a thread reply with
   concrete evidence (file, line, invariant); never fix-to-appease.
4. Never let feedback expand the change beyond its original goal. Real
   shortcomings outside the goal become tracker items, not commits.
5. Attach visual proof only when it is the clearest evidence; use the
   [slopcourier visual-evidence ladder](../slopcourier/references/visual-evidence.md).

## Reply voice

Your replies post under the authenticated account and are that identity
speaking:

- first-person neutral voice, never third-person self-reference
- structured markdown: short paragraphs, backticked identifiers
- only content that advances the thread

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
