---
name: slopspec
description: "Save agreed future work in the repository tracker as issues, epics, or durable plans; not an in-session checklist."
disable-model-invocation: true
---

# Slopspec

Preserve settled intent in the smallest artifact a fresh agent can resume.
Do not implement the plan or invent a second backlog.

Inspect the conversation, source work item, and repository evidence. Resolve
discoverable facts yourself; ask only about decisions that block an honest
plan. Keep outcomes, acceptance, constraints, and non-goals separate from
speculative implementation detail.

- Resolve the destination and write authority with
  [tracker-selection.md](references/tracker-selection.md).
- Choose the minimum useful hierarchy with
  [artifact-shapes.md](references/artifact-shapes.md).
- Use the repository's templates and terminology;
  [writing defaults](../slopscriber/references/style.md) fill gaps.

Search for existing work before creating artifacts. Update the canonical item
when the request started there, preserving its agreed criteria and metadata.
For multiple tickets, show the delivered behavior and real blocking edges
before publication unless that decomposition is already approved.

Create blockers before dependents. Use native hierarchy/dependency operations
when available, otherwise explicit links in the same tracker. Re-read the
published artifacts to confirm content, metadata, relationships, and links.

Return the canonical URL or key, what can start now, and any blocker. When
publication is unavailable, return a paste-ready draft for the intended tracker
and the exact missing access; do not switch trackers or save local plan files.
