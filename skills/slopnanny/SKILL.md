---
name: slopnanny
description: "Resolve review feedback and CI on an open change request, then merge when green unless asked to hold. Use for babysitting delivered work."
---

# Slopnanny

Settle one delivered change request from real forge evidence. Babysitting
carries merge authority. Rework stays in its contract; independent review
belongs to the required reviewer.

## Observe

- Poll the forge CLI (`gh` / `glab`): checks, pending review requests, verdicts
  (a `CHANGES_REQUESTED` review may have no inline thread), unresolved threads,
  and top-level comments, where bots often post findings.
- Green checks and no threads are not settled while a requested reviewer,
  human or bot, is pending. After it submits, re-read everything.
- Don't reprocess findings a push answered; don't assume a push resolved older
  feedback.

## Triage

A person's `CHANGES_REQUESTED` review, or a comment questioning the design, is
the user's to answer: stop rework on that change request, post nothing, and
hand the objection back with the evidence you found. Bot findings and a
person's concrete, unambiguous fix requests stay in this loop.

1. Validate each claim against current code, the task contract, and stronger
   invariants. Reject wrong or out-of-scope findings with evidence; real gaps
   outside the goal become tracker items, not commits.
2. Batch accepted findings into one rework pass, then required gates and
   affected runtime proof. When the rework changed behavior, or repository
   policy requires it, run [slopguard](../slopguard/SKILL.md#when) once on the
   final head, not per thread. Apply
   [slopguard's convergence rule](../slopguard/SKILL.md#convergence) to
   repeated findings; an unresolved blocking review still prevents merge.
3. Push verified fixes; reply on each addressed thread with the commit hash.
   No force-push without approval. Let requested reviewers finish on the new
   head; don't start duplicates.
4. Visual proof only when it is the clearest evidence:
   [visual-evidence ladder](../slopcourier/references/visual-evidence.md).

Replies are first person as the authenticated account: evidence and commit
hashes, nothing that doesn't advance the thread.

## Settle

- When nothing changed, post nothing on the forge; still tell the user what
  you observed.
- Merge when required checks are green on the latest commit, no review is
  pending, and reviewers and threads are clear. Use the repository's merge
  method and report the merged commit; don't ask first. Hold only when asked.
  Never merge past a blocking human review or an open thread.
