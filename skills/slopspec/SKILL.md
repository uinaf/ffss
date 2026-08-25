---
name: slopspec
description: "Turn agreed work into a durable, resumable plan in the repository's preferred tracker. Use to plan or save future work, or create epics and tickets; not for in-session checklists or implementation."
disable-model-invocation: true
---

# Slopspec

Make agreed work durable and easy to resume without inventing a second backlog.

## Principles

- Preserve settled intent; investigate facts but do not restart the discussion
- Prefer the repository's work tracker over checked-in tactical plan files
- Create the smallest artifact set that makes the work independently resumable
- Slice by verifiable behavior, not by architectural layer
- The published plan is the builder's completion instrument: work is checked
  against it instead of asking the requester what done means
- Represent real blocking edges; independent work stays independent
- Keep one canonical plan and link to supporting specs, decisions, code, or prototypes
- Treat tracker publication as an external write and protect private or sensitive details

## Boundaries

- Plan and persist the work; do not implement it.
- Do not turn a routine execution checklist for the current session into durable backlog.
- Do not create a new spec when the agreed requirements already fit in a work item.
- Do not silently write `docs/plans/`, `plans/`, or another local fallback when the intended tracker is unavailable.
- If unresolved product or architecture decisions materially change the plan, ask one decision question at a time with a recommended answer before publishing.

## Workflow

### 1. Establish the planning contract

Read the full conversation and every referenced issue, spec, decision, prototype, or source file. Inspect the current repository where needed to verify names, existing seams, constraints, validation commands, and already-tracked work.

Separate:

- settled outcome and decisions
- acceptance criteria and non-goals
- facts that can be resolved from the repository or tracker
- unresolved decisions that block honest decomposition

Do not ask the user for facts you can discover yourself. Do not reopen settled choices merely to make the plan more elaborate.

### 2. Resolve the durable destination

Use [tracker-selection.md](references/tracker-selection.md) to identify the repository's preferred tracker and the available write path.

Use this precedence:

1. destination explicitly named by the user or the source work item
2. repository-owned instructions and established tracker conventions
3. existing related work, issue keys, templates, and development links
4. remote hosting provider as a recommendation signal only

If the user explicitly requested publication and one destination is unambiguous, proceed. Otherwise ask one short question that leads with the recommended destination and artifact shape.

Before publishing sensitive details to a public or broadly visible tracker, show the exposure and get explicit confirmation or offer a redacted/private destination.

### 3. Choose the smallest useful shape

Pick the shape, templates, and slicing from
[artifact-shapes.md](references/artifact-shapes.md). Use the least hierarchy
that keeps the work resumable.

### 4. Draft from the agreed evidence

Carry the agreed evidence into the canonical plan: problem and outcome,
decisions, acceptance criteria, non-goals, approach at stable module boundaries,
verification, risks and stop conditions, and parent/child/blocking
relationships. Use the exact shapes in
[artifact-shapes.md](references/artifact-shapes.md); do not draft your own.

Write every issue, epic, and ticket body in the
[house style](../slopscriber/references/style.md): outcome first, one fact per
line, bullets over paragraphs, links over code spans, no narration or hedging.

Blocking edge example: `Migrate auth tokens` blocks `Wire login UI` because the
UI cannot verify against the new token contract until migration lands; shared
theme alone is not a blocker.

Use exact paths only after verifying them against the current revision. Before
creating multiple tickets, present titles, delivered behavior, and blocking
edges for approval unless the user already approved them.

### 5. Publish or update

Search for an existing canonical issue before you create a duplicate. When the conversation started from an issue, epic, or project, update or attach to it unless the user asked for a new planning root.

- Preserve repository templates, terminology, labels, teams, projects, milestones, and issue types.
- Create blockers before dependents so relationships can reference real identifiers.
- Use native parent/child and dependency relationships when the tracker and available tool support them; otherwise record explicit links in the artifact bodies.

If you cannot write to the preferred tracker, follow the fallback in
[tracker-selection.md](references/tracker-selection.md).

### 6. Verify the resume point

Re-read every created or updated artifact. Confirm titles, bodies, metadata, hierarchy, blocking relationships, and links match the approved plan.

Return a compact handoff. The tracker artifact is the durable record; do not duplicate it:

```text
canonical: PROJ-123
children: PROJ-124, PROJ-125
frontier: PROJ-124 (no blockers)
gaps: none
next: start PROJ-124
```

## References

- [tracker-selection.md](references/tracker-selection.md): destination evidence, confirmation, access, and safety
- [artifact-shapes.md](references/artifact-shapes.md): single-item and parent/child shapes, slicing, and resume contract
