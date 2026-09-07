---
name: slopnanny
description: "Resolve review feedback and CI on an open change request, then merge when green unless asked to hold. Use for babysitting delivered work."
---

# Slopnanny

Settle one delivered change request from real forge evidence. Rework stays in
its contract; independent review belongs to the required reviewer.

## Observe

- With an active slopmachine run, `slopmachine watch --once` (or
  `--interval SECONDS`) is the authority. Act on the cause it records;
  [slopmachine](../slopmachine/SKILL.md) owns the signal vocabulary.
- Otherwise poll the forge CLI (`gh` / `glab`): checks, pending review
  requests, verdicts (a `CHANGES_REQUESTED` review may have no inline thread),
  unresolved threads, top-level comments. Bots often post findings as plain
  comments.
- Green checks and no threads are not settled while a requested reviewer,
  human or bot, is pending. After it submits, re-read everything.
- Require checks and review evidence for the latest commit. Don't reprocess
  findings a push answered; don't assume a push resolved older feedback.
- Unchanged observations need no new tests, review, or reply.

## Triage

1. Validate each claim against current code, the task contract, and stronger
   invariants. Reject wrong or out-of-scope findings with evidence; real gaps
   outside the goal become tracker items, not commits.
2. Batch accepted findings into one rework pass. Focused checks while editing,
   then required gates and affected runtime proof. Refresh independent review
   once on the final target, not per thread.
3. Push verified fixes; reply on each addressed thread with the commit hash.
   No force-push without approval. Let requested reviewers finish on the new
   head; don't start duplicates.
4. Visual proof only when it is the clearest evidence:
   [visual-evidence ladder](../slopcourier/references/visual-evidence.md).

Replies are first person as the authenticated account: evidence and commit
hashes, nothing that doesn't advance the thread.

## Quiet discipline

- Nothing changed → post nothing. Your report to the user still says what was
  observed.
- Merge when required checks are green on the latest commit, no review is
  pending, and reviewers and threads are clear. Use the repository's merge
  method and report the merged commit; babysitting carries merge authority.
  Hold only when asked. Never merge past a blocking human review or an open
  thread.
- On a slopmachine run, route rework through `slopmachine` commands and let
  `watch` observe the merge; settlement never comes from your narration.
