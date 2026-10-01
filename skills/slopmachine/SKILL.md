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

## Deliver

Review, open, and babysit the change with [slopguard](../slopguard/SKILL.md)
when it applies, before the change request opens, then
[slopcourier](../slopcourier/SKILL.md) and [slopnanny](../slopnanny/SKILL.md)
through merge, unless the user requested a hold or delivery-only endpoint. For
authorized direct delivery, verify the pushed commit and required remote
checks. Verify delivery against the forge's actual head and status, and don't
finish past a failing required check.

## Handoff

As milestones land and before a handoff, record in the existing tracking
surface: the agreed scope, authorization, decisions, completed work, valid
proof and its revision, delivery URLs, and outstanding work. On resume, continue
from that record and the actual state.
