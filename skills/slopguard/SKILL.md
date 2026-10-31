---
name: slopguard
description: "Get an independent review of a finished change from the other model family and close out its findings. Use when the user asks for a slopguard or independent review, or before delivery in a repo with no review bots or required reviews."
---

# Slopguard

A model reviewing its own change shares its blind spots, so the review comes
from the other model family. It never edits, and its verdict is advisory
evidence, not a merge decision.

## When

- Once, after the change passes its checks and before delivery. Not per edit
  or per thread.
- Re-review after later edits only if they change behavior, contracts, or
  risk.

## Steps

1. **Pick the reviewer** from the other model family than the change's author,
   or than this session's for a human-written change: Claude wrote it, Codex
   reviews; Codex wrote it, Claude reviews. The user's choice wins. If that
   CLI is missing or signed out, report it; a same-family review
   (`/code-review` in Claude Code, `/review` in Codex) runs only if the user
   accepts it, and the report calls it not independent.
2. **Run it** from the repo root, in the foreground:
   - Codex: `codex review "<prompt>"`. The prompt names the target
     (`git diff <base>...HEAD`, a commit, or the uncommitted changes), the
     objective, acceptance criteria, and non-goals. It asks for every
     suspected finding regardless of severity or confidence, and for changed
     verification (thresholds, checks, assertions, skips, suppressions)
     compared against the base. `--base`, `--commit`, and `--uncommitted`
     refuse a prompt; use them only for an unsteered pass.
   - Claude: `claude -p "/code-review <PR number, branch, or path>"`. It
     takes a target, not a prompt. From Codex, request escalated
     permissions: inside the sandbox it fails with a 401.

   Both send the diff and every file they read to their provider under the
   user's login. Both exit 0 with findings, and `claude -p` also exits 0 on
   an auth or API error, so read stdout: an error there or a non-zero exit
   is a failed run.
3. **Validate.** Findings are hypotheses. Accept one only with evidence of a
   defect or unmet requirement; reject the rest with a short reason. Name
   each accepted finding's defect class and fix every instance in the
   change, not only the flagged line.
4. **Re-review** after accepted fixes: the fixed target, same reviewer.
   After the third round, stop patching; reconsider the approach and tell
   the user.

## Done

Every finding is fixed and verified, or rejected with a reason. A review with
findings is never "clean", even when all were rejected. A failed run is a
blocker; rerun once only if the tree changed during the review.

**Convergence:** a repeated finding reopens only with new evidence. Another
model's verdict on the same inputs is not evidence, so don't rerun unchanged
inputs, switch reviewers, or run a panel to get agreement. If a disagreement
needs the user's judgment, say so and pause only that part.

## Report

To the user, not the PR body: the command and target, the reviewer, accepted
and rejected findings, and the verdict or blocker.
