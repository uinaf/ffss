---
name: planning
description: "Turn an agreed change, specification, or conversation into a durable, resumable work plan in the repository's preferred tracker. Use when asked to plan future work, save a plan, create an epic or tickets, decompose work for parallel execution, or prepare work for another session. Do not use for routine in-session checklists, implementation, general documentation cleanup, or open-ended discovery where requirements still need substantive product decisions."
---

# Planning

Make agreed work durable and easy to resume without inventing a second backlog.

## Principles

- Preserve settled intent; investigate facts but do not restart the discussion
- Prefer the repository's work tracker over checked-in tactical plan files
- Create the smallest artifact set that makes the work independently resumable
- Slice by verifiable behavior, not by architectural layer
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

Do not ask the user for facts that can be discovered. Do not reopen settled choices merely to make the plan more elaborate.

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

Use [artifact-shapes.md](references/artifact-shapes.md) for templates and sizing.

- Use one issue for work that one agent can complete and verify in one fresh context.
- Use a parent issue, epic, or equivalent plus child tickets when the work spans sessions, owners, or independently landable slices.
- Use a project or initiative only when the repository already uses that level for comparable work.

Each child should deliver a narrow end-to-end behavior and be independently demonstrable or verifiable. Wide mechanical changes may use an expand-migrate-contract sequence when no vertical slice can remain green.

### 4. Draft from the agreed evidence

The canonical plan should preserve:

- problem or outcome
- agreed decisions and constraints
- acceptance criteria
- non-goals
- implementation approach at stable module or contract boundaries
- verification expectations
- risks and stop conditions
- parent, child, and blocking relationships

Use exact paths or code excerpts only when they materially help execution and have been verified against the current revision. Prefer stable contracts and module names over brittle line-by-line instructions.

Before creating multiple tickets, present the proposed titles, delivered behavior, and blocking edges for approval unless the user already approved that decomposition.

### 5. Publish or update

Search for an existing canonical issue before creating a duplicate. When the conversation started from an issue, epic, or project, update or attach to it unless the user asked for a new planning root.

Preserve repository templates, terminology, labels, teams, projects, milestones, and issue types. Create blockers before dependents so relationships can reference real identifiers. Use native parent/child and dependency relationships when the tracker and available tool support them; otherwise record explicit links in the artifact bodies.

If the preferred tracker cannot be written:

- keep the intended destination
- produce a paste-ready draft in the correct shape
- report the missing access or tool
- do not switch to another tracker or commit a local plan without approval

### 6. Verify the resume point

Re-read every created or updated artifact. Confirm titles, bodies, metadata, hierarchy, blocking relationships, and links match the approved plan.

Return a compact handoff:

- canonical plan URL or key
- created or updated child items
- current dependency frontier: items with no unresolved blockers
- publication or relationship gaps
- next action to resume work

The tracker artifact is the durable handoff. Do not create a second handoff document that repeats it.

## References

- [tracker-selection.md](references/tracker-selection.md) — destination evidence, confirmation, access, and safety
- [artifact-shapes.md](references/artifact-shapes.md) — single-item and parent/child shapes, slicing, and resume contract
