---
name: slopnanny
description: "Babysit an open change request through review and CI to a settled outcome: observe checks and reviewer feedback, verify every bot claim before acting, fix real findings without scope creep, answer threads with commit hashes, and merge once green unless asked to hold. Use when asked to babysit, monitor, or watch a pull/merge/change request, or after slopcourier delivers one. Do not use to create change requests or run a review yourself."
---

# Slopnanny

Walk one delivered change request to a settled outcome. This skill responds
to what the forge shows; it never invents signals, reviews its own work, or
widens the change.

## Observe

- With an active slopmachine run, the binary is the observation authority:
  `slopmachine watch --once` (or `--interval SECONDS` for a bounded poll).
  `merged` settles the unit; `checks_failed`, `review_feedback`, and
  `head_moved` return it to the build loop with the cause recorded, and act on
  that cause.
- Without a run, poll through the forge CLI the delivery dispatched to
  (`gh` / `glab`): checks state, review verdicts (a `CHANGES_REQUESTED`
  review may carry no inline thread), unresolved review threads, and
  top-level comments; bots often report findings as ordinary comments.
- Act only on checks, reviews, and comments newer than the latest push;
  everything older was already answered by that push.

## Triage findings

1. Every bot or reviewer claim is a hypothesis. Verify it against the exact
   code and the change's contract before acting; bots are helpful and
   sometimes wrong.
2. Fix a real finding with the smallest change at the owning boundary, rerun
   the repository's own gates, push, and reply on the thread with the commit
   hash. Never force-push without explicit approval, even when rework
   after `head_moved` tempts a rebase.
3. Reject an incorrect or out-of-scope finding in a thread reply with
   concrete evidence (file, line, invariant); never fix-to-appease.
4. Never let feedback expand the change beyond its original goal. Real
   shortcomings outside the goal become tracker items, not commits.
5. Attach visual proof only when it is the clearest evidence; reuse the
   slopcourier visual-evidence ladder; never commit proof assets.

## Reply voice

Replies post under the authenticated account and are that identity speaking:

- first-person neutral voice, never third-person self-reference
- structured markdown: short paragraphs, backticked identifiers
- only content that advances the thread

## Quiet discipline

- Nothing changed → say nothing. No filler comments, no status noise.
- When required checks and reviewers are green on the latest commit, merge
  with the repository's merge method and report the merged commit; the
  babysit request carries merge authority. Hold at green only when the
  request says to, and never merge past a blocking human review or an
  unresolved thread.
- On a slopmachine run, keep driving status: rework causes route through
  `slopmachine` commands, and settlement comes from `watch` observing the
  merge, never from narrating it.
