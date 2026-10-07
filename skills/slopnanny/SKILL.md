---
name: slopnanny
description: "Resolve review feedback and CI on an open change request, merge when green unless asked to hold, then watch the runs the merge starts. Use for babysitting delivered work."
---

# Slopnanny

Settle one delivered change request from real forge evidence. Babysitting
carries merge authority. Rework stays in its contract; independent review
belongs to the required reviewer.

## Observe

- Never write a polling loop: wait with
  `go run <this skill's directory>/scripts/prwatch/main.go <number or URL>`
  on GitHub or GitLab (`-R <repository path>` outside the checkout; `-forge`
  when the host name doesn't say which). It reads checks, review requests,
  verdicts, unresolved threads, and top-level comments, waits for late checks
  and working bots, and exits on the first thing to act on: `ready` (0);
  `checks-failed`, `activity`, `pushed`, `blocked`, or `closed` (1); a `gh` /
  `glab` failure (2); or `timeout` after 30 minutes (3). Act on what it
  prints, then rerun it with `-since` set to its `since:` line so nothing
  between runs is missed. Without Go, poll `gh` / `glab` for the same
  evidence and say prwatch didn't run.
- A `CHANGES_REQUESTED` review may have no inline thread, and bots often post
  findings as top-level comments.
- Green checks and no threads are not settled while a requested human
  reviewer is pending. After they submit, re-read everything.
- Review bots are advisory. One is pending only while it visibly works on the
  head you merge (an eyes reaction, a running check, an in-progress or
  "reviewing" comment, a review request to it), or for a few minutes after a
  push when it reviewed an earlier head of this change request. A bot that
  never touched this change request isn't pending, even if it reviews others;
  some run only for certain authors or accounts.
- Wait for a pending bot at most 5 minutes, or to the user's deadline if
  sooner, re-polling; then continue without it and name it in the receipt. A
  usage-limit or quota notice means unavailable: say so and continue.
- Never summon a review bot with a mention, comment, or review request unless
  the user asks.
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
   affected runtime proof. Run [slopguard](../slopguard/SKILL.md#when) only
   when the user asked for it or the repository has no review bot or required
   review, once on the final head, not per thread. Apply
   [slopguard's convergence rule](../slopguard/SKILL.md#validate-and-close) to
   repeated findings; an unresolved blocking review still prevents merge.
3. Push verified fixes; reply on each addressed thread with the commit hash.
   Rework that changes what the change request does updates its title and
   body through [slopcourier](../slopcourier/SKILL.md); otherwise leave them
   alone. The body never gains fix hashes, finding counts, or test runs.
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
  human review or review bot is pending, and reviewers and threads are clear. Use the repository's merge method and
  report the merged commit; don't ask first. Hold only when asked.
  Never merge past a blocking human review or an open thread.

## After merge

Once the change request merges, by you or anyone else, watch the runs the merge
started on the default branch.

- Run prwatch on the merged change request once. It finds the merge commit,
  skips scheduled and manual runs, waits for the runs or pipelines it starts
  on the target branch and the ones they trigger, and prints each with its
  link and failed jobs:
  `runs-passed` or `no-runs` (0), `runs-failed` (1), `timeout` (3).
- Without prwatch, do the same by hand:
  - Take the full merge SHA; an abbreviated one lists nothing. Read
    `mergeCommit` from `gh pr view <number> --json mergeCommit,headRefOid`, or
    `merge_commit_sha`, then `squash_commit_sha`, from
    `glab mr view <number> -F json`. A fast-forward or indirect merge has
    neither; use the head commit that landed (`headRefOid` or `sha`).
  - List the runs with `gh run list --commit <merge-sha> --limit 100` or
    `glab ci list --sha <merge-sha>`, skipping scheduled and manual runs. Runs
    can take a minute to appear; if none do, say so and stop.
  - Wait once, re-polling, until they finish: at most 30 minutes unless the
    user set a deadline. Runs they trigger, such as a deploy after CI or a
    downstream pipeline, join the wait; re-list a minute after the last one
    finishes.
- At the deadline, report what is still running, with links, and stop.
- On a failure, report the run link, the likely cause from the failed job's
  log, and the exact revert command repository policy allows:
  `gh pr revert <number>` (on GitLab, a revert branch and `glab mr create`),
  or, where the default branch takes direct pushes, `git revert <merge-sha>`
  on an up-to-date default branch, with `-m 1` for a merge commit or the full
  range for a rebase merge. Revert or redeploy only under authority the user
  already gave.
- On success, say so in one line.
- Delete the branch or worktree only once the forge shows the change request
  merged and this watch ends; a deleted head branch closes an open one.
