---
name: slopmachine
description: "Execute an agreed plan through implementation, verification, review, and delivery. Use when asked to run a plan end to end or work slopmachine-style; not for planning or assessment alone."
---

# Slopmachine

Carry the agreed plan to its requested outcome. Use the current coding harness
and repository workflow; this skill needs no slopmachine binary or database.

## Authority

A request to run, start, continue, or resume authorizes the matching work
through the agreed endpoint, and that authorization persists across turns.
Assessment and preparation requests authorize only those deliverables. Respect
an explicit hold or narrower endpoint. Track progress in the existing plan or
harness task list; don't create a second tracking system.

User instructions take precedence over skill guidance. If a skill would stop
authorized work, identify its exact instruction and check whether it applies
before treating it as a blocker.

## Execute

- After a dependency lands, reconcile dependent branches with the new base
  before verifying and merging.
- After checks pass, use [slopguard](../slopguard/SKILL.md) for independent
  review and honor any other required reviewers. Follow
  [slopguard's convergence rule](../slopguard/SKILL.md#convergence) instead of
  chasing a clean verdict through repeated calls.

## Deliver

For a change request, open it with [slopcourier](../slopcourier/SKILL.md), then
run [slopnanny](../slopnanny/SKILL.md) through review, CI, and merge unless the
user requested a hold or delivery-only endpoint. Creating a change request is
not completion when the agreed outcome includes merge. For authorized direct
delivery, verify the pushed commit and required remote checks.

Verify delivery against the forge's actual head and status. A missing tool,
unavailable check, or inaccessible forge is a limitation to resolve or report,
never passing evidence. Don't bypass a required gate to finish.

## Handoff

As milestones land and before a handoff, record in the existing tracking
surface: the agreed scope, authorization, decisions, completed work, valid
proof and its revision, delivery URLs, and outstanding work. On resume,
reconcile that record with the worktree and forge, reuse valid proof, and
continue from the actual state. Keep user and repository requirements distinct
from earlier agent suggestions; review history must not become a new mandatory
gate in the handoff.
