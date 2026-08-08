---
name: planning
description: "Turn an agreed change, specification, or conversation into a durable, resumable work plan in the repository's preferred tracker. Use when asked to plan future work, write or make a plan, save a plan, create an epic or tickets, break work into tickets, decompose work for parallel execution, or prepare work for another session. Do not use for routine in-session checklists, implementation, general documentation cleanup, or open-ended discovery where requirements still need substantive product decisions."
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

The canonical plan should preserve: problem/outcome, decisions, acceptance
criteria, non-goals, approach at stable module boundaries, verification, risks
and stop conditions, plus parent/child/blocking relationships.

Child-ticket skeleton (expand via [artifact-shapes.md](references/artifact-shapes.md)):

```markdown
## Parent
<canonical parent URL or key>
## What this delivers
<one independently verifiable behavior>
## Acceptance criteria
- [ ] <observable result>
## Verification
- <test, check, or real-surface evidence>
## Blocked by
- <issue link or None>
```

Blocking edge example: `Migrate auth tokens` blocks `Wire login UI` because the
UI cannot verify against the new token contract until migration lands; shared
theme alone is not a blocker.

Use exact paths only when verified against the current revision. Before creating
multiple tickets, present titles, delivered behavior, and blocking edges for
approval unless already approved.

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

Return a compact handoff (tracker artifact is durable; do not duplicate it):

```text
canonical: PROJ-123
children: PROJ-124, PROJ-125
frontier: PROJ-124 (no blockers)
gaps: none
next: start PROJ-124
```

## References

- [tracker-selection.md](references/tracker-selection.md) — destination evidence, confirmation, access, and safety
- [artifact-shapes.md](references/artifact-shapes.md) — single-item and parent/child shapes, slicing, and resume contract
