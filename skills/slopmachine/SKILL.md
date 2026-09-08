---
name: slopmachine
description: "Execute an agreed plan through implementation, verification, review, and delivery. Use when asked to run a plan end to end or work slopmachine-style; not for planning or assessment alone."
---

# Slopmachine

Carry the agreed plan to its requested outcome. Use the current coding harness
and repository workflow; this skill needs no slopmachine binary or database.

## Establish the contract

Read the agreed plan, repository rules, and current worktree. Identify the
acceptance criteria, dependencies, required checks and reviewers, and delivery
endpoint from the conversation and owning sources. Use an existing plan or
harness task list for progress; don't create a second tracking system.

A request to run, start, continue, or resume authorizes the matching work.
Preserve that authorization across turns. Make routine implementation choices
yourself; ask only when missing information materially changes scope, authority,
or correctness. Complete independent work while a decision is pending.
Assessment and preparation requests authorize only those deliverables.

User instructions take precedence over skill guidance. If a skill would stop
authorized work, identify its exact instruction and check whether it applies
before treating it as a blocker. Respect an explicit hold or narrower endpoint.

## Execute

- Work in dependency order. Batch independent reads and checks. Delegate
  bounded independent work when useful and permitted, keep working alongside
  it, and inspect the results before integrating them. After a dependency lands,
  reconcile dependent branches with the new base before verifying and merging.
- Implement the full requested behavior with focused edits. Fix nearby issues
  only when they prevent the requested outcome; report unrelated findings
  separately. Reopen settled choices only when new evidence warrants it.
- Run repository-approved checks and exercise changed behavior. Keep permanent
  tests proportional to the contract and existing coverage. Passing checks
  need repeating only after relevant changes, failures, or concrete concerns.
- Fix change-caused failures and rerun affected checks. For external failures,
  diagnose the cause; retry only transient operations with a bounded strategy.
  Respect provider cooldowns; authentication and approval failures need their
  actual remedy.
  Preserve failing proof and continue other ready work when one part is blocked.
- After checks pass, use [slopguard](../slopguard/SKILL.md) for independent
  review and honor any other required reviewers. Validate findings against the
  contract; follow slopguard's convergence rule instead of chasing a clean
  verdict through repeated calls.

## Deliver and settle

Deliver through the repository's permitted path after required proof and review.
For a change request, use [slopcourier](../slopcourier/SKILL.md), then
[slopnanny](../slopnanny/SKILL.md) through review, CI, and merge unless the user
requested a hold or delivery-only endpoint. For authorized direct delivery,
verify the pushed commit and required remote checks. Creating a change request
is not completion when the agreed outcome includes merge.

Verify delivery against the forge's actual head and status. A missing tool,
unavailable check, or inaccessible forge is a specific limitation to resolve or
report, never passing evidence. Don't bypass a required gate to finish.

## Continue and report

Give brief updates when findings, decisions, or blockers change. Before
compaction or a handoff, preserve the agreed scope, authorization, decisions,
completed work, valid proof and its revision, delivery URLs, and outstanding
work in the existing tracking surface. On resume, reconcile that record with
the worktree and forge; reuse valid proof and continue from the actual state.
Keep user and repository requirements distinct from earlier agent suggestions;
review history must not become a new mandatory gate in the handoff.

End when the requested outcome is verified, or remaining work needs a user
decision or external recovery. Report the result, proof, delivery links, and
anything incomplete with its cause. A promised next step within scope is work
to perform before ending the turn.
