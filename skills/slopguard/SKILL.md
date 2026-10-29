---
name: slopguard
description: "Review a finished change with the slopguard CLI and close out its findings. Use when the user asks for a slopguard or independent review, or before delivery in a repo with no review bots or required reviews."
---

# Slopguard

`slopguard` sends a frozen snapshot of the change to one model provider and
reports findings. It never edits, and its verdict is advisory evidence, not
a merge decision. What leaves the machine: [security.md](references/security.md).

## When

- Once, after the change passes its checks and before delivery. Not per edit
  or per thread.
- Re-review after later edits only if they change behavior, contracts, or
  risk.

## Steps

1. **Check the CLI.** `slopguard --version`. If it's missing, report it. Doc
   links in the references point at `main`; swap in `slopguard/v<version>` to
   read the installed release.
2. **Write the prompt.** Objective, acceptance criteria, non-goals, source
   ids. Ask for every suspected finding regardless of severity or confidence.
   Ask the reviewer to compare changed verification (thresholds, checks,
   assertions, skips, suppressions) against the base.
3. **Pick one provider.** User's choice, then trusted config, else Codex at
   high reasoning. Details: [providers.md](references/providers.md).
   Config and `slopguard doctor`: [configuration.md](references/configuration.md).
4. **Run.** Mode `local`, `branch` (against the PR's real base), or `commit`:
   [run-modes.md](references/run-modes.md).
5. **Validate.** Findings are hypotheses. Accept one only with evidence of a
   defect or unmet requirement; reject the rest with a short reason. Name
   each accepted finding's defect class and fix every instance in the
   change, not only the flagged line. Fix them together, in the next frozen
   target (worktree for local, a commit for branch, an amended commit for
   commit mode).
6. **Re-review** after accepted fixes change the target: the final target,
   same provider and mode. After the third round, stop patching; reconsider
   the approach and tell the user.

## Done

- Exit 0: clean.
- Exit 1: findings. Done once every finding is fixed and verified, or
  rejected with a reason. Exit 1 is never "clean".
- Exit 2: no trustworthy result. `source_changed` means the tree moved during
  review; discard and rerun once edits stop. Report other failure classes
  ([result contract](https://github.com/uinaf/ffss/blob/main/cli/slopguard/docs/RESULT_SCHEMA.md))
  as blockers.

**Convergence:** a repeated finding reopens only with new evidence. Another
model's verdict on the same inputs is not evidence, so don't rerun unchanged
inputs, switch providers, or run a panel to get agreement. If a disagreement
needs the user's judgment, say so and pause only that part.

## Report

To the user, not the PR body: redacted command and target, provider, web
access state, builder proof, accepted and rejected findings, verdict or
blocker.
