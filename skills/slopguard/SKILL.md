---
name: slopguard
description: "Get an independent review of a finished change from the other model family and close out its findings. Use when the user, a brief, or the repository guide asks for a slopguard or independent review."
---

# Slopguard

Run an outside review of a finished change, then validate and fix what it
finds.

## When

- When the user, a brief, or the repository guide asks for it, after the
  change passes its checks.
- Again after later edits that change behavior, contracts, or risk.

## Steps

1. **Pick the reviewer** from the other model family than the change's author,
   or than this session's for a human-written change. The user's choice
   wins. If that CLI is missing or signed out, report it and offer a
   same-family review (`/code-review` in Claude Code, `/review` in Codex);
   the report calls that one same-family.
2. **Run it** from the repo root, in the foreground:
   - Codex: `codex review "<prompt>"`. The prompt names the target
     (`git diff <base>...HEAD`, a commit, or the uncommitted changes), the
     objective, acceptance criteria, and non-goals. It asks for every
     suspected finding regardless of severity or confidence, and for changed
     verification (thresholds, checks, assertions, skips, suppressions)
     compared against the base. For a risky change, it also asks the
     reviewer to exercise one consumer a symbol search misses: a wire format,
     persisted data, or a pinned dependency. `--base`, `--commit`, and
     `--uncommitted` refuse a prompt; they suit an unsteered pass.
   - Claude: `claude -p "/code-review <PR number, branch, or path>"`. It
     takes a target. From Codex, request escalated
     permissions: inside the sandbox it fails with a 401.

   Both send the diff and every file they read to their provider under the
   user's login. Both exit 0 with findings, and `claude -p` also exits 0 on
   an auth or API error, so read stdout: an error there or a non-zero exit
   is a failed run.
3. **Validate.** Accept a finding on evidence of a defect or unmet
   requirement; reject the rest with a short reason. Name each accepted
   finding's defect class and fix every instance in the change.
4. **Re-review** after accepted fixes: the fixed target, same reviewer.
   After the third round, reconsider the approach with the user.

## Done

Every finding is fixed and verified, or rejected with a reason. Report a
review that had findings with each one's outcome; "clean" is for a review
with none. A failed run is a blocker to report; rerun once when the tree
changed during the review.

**Convergence:** rerun on changed inputs. A repeated finding reopens on new
evidence. A disagreement that needs judgment goes to the user while the
rest proceeds.

## Report

To the user: the command and target, the reviewer, accepted
and rejected findings, and the verdict or blocker.
