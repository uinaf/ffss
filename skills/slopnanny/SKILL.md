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
- Know the repository's review bots before calling anything settled: those
  that reviewed its recent merged change requests, plus any that reacted to,
  commented on, or run a check on this one. They may not appear in review
  requests, and silence on a young change request doesn't mean there are none.
- A bot is done when its review or summary for the head you merge exists, or
  its latest one if it reviews only once. An eyes reaction, a running bot
  check, or a "reviewing" comment means it's still working: wait, re-poll to a
  deadline, and on timeout report the stall instead of merging.
- Don't reprocess findings a push answered; don't assume a push resolved older
  feedback.

## Triage

A person's `CHANGES_REQUESTED` review blocks merge until that person approves
or the user decides. If every point in it is a concrete, unambiguous fix, make
the fixes, reply with the commits, and re-request that person's review. If any
point, or any person's comment, questions the design or is ambiguous, stop
rework on that change request, post nothing, and hand it back with the
evidence you found. Bot findings and a person's other concrete, unambiguous fix
requests stay in this loop.

1. Validate each claim against current code, the task contract, and stronger
   invariants. Reject wrong or out-of-scope findings with evidence; real gaps
   outside the goal become tracker items, not commits.
2. Batch accepted findings into one rework pass, then required gates and
   affected runtime proof. When [slopguard's When](../slopguard/SKILL.md#when)
   applies, run it once on the final head, not per thread. Apply
   [slopguard's convergence rule](../slopguard/SKILL.md#validate-and-close) to
   repeated findings; an unresolved blocking review still prevents merge.
3. Push verified fixes; reply on each addressed thread with the commit hash.
   No force-push without approval. Let requested reviewers finish on the new
   head; don't start duplicates. Re-request review from an approver whose
   approval predates the new head.
4. Visual proof only when it is the clearest evidence:
   [visual-evidence ladder](../slopcourier/references/visual-evidence.md).

Replies are first person as the authenticated account: evidence and commit
hashes, nothing that doesn't advance the thread.

## Settle

- When nothing changed, post nothing on the forge; still tell the user what
  you observed.
- Merge when required checks are green on the latest commit, no requested
  review is pending, every known review bot is done, and
  reviewers and threads are clear. Use the repository's merge method and
  report the merged commit; don't ask first. Hold only when asked.
  Never merge past a blocking human review or an open thread.
- Delete the branch or worktree only after the forge shows the change request
  merged; a deleted head branch closes an open one.
