# Artifact Shapes

Use the least hierarchy that keeps the work resumable.

## Shape Selection

| Work shape | Durable artifact |
|---|---|
| One agent, one fresh context, one independently verifiable outcome | One issue or work item |
| Several independently landable slices or more than one session | Parent issue or epic with child tickets |
| Several teams, products, or planning horizons | Existing project or initiative convention, containing parent work |
| Tracker unavailable | Paste-ready draft for the intended tracker, not a different durable home |

Do not create an epic merely to hold one issue. Do not split a cohesive change into tickets that cannot land or verify independently.

## Canonical Parent

```markdown
## Outcome
What changes for the user or operator when this work is complete.

## Context
Current behavior and the evidence that makes the work necessary.

## Decisions and constraints
- Settled choice or invariant

## Acceptance criteria
- [ ] Observable result

## Non-goals
- Explicit exclusion

## Delivery shape
- Child work items and what each delivers

## Verification
- Required automated and real-surface proof

## Risks and stop conditions
- Risk, mitigation, and when an implementer must stop instead of improvising
```

## Child Ticket

```markdown
## Parent
Canonical parent URL or key.

## What this delivers
One narrow end-to-end behavior that can be demonstrated or verified independently.

## Acceptance criteria
- [ ] Observable result

## Verification
- Expected test, check, or real-surface evidence

## Blocked by
- Native relationship or explicit issue link; `None` when immediately actionable

## Boundaries
- In scope and explicit non-goals for this slice
```

## Slicing Rules

- Prefer tracer-bullet slices through the necessary layers over separate database, API, UI, and test tickets.
- Size each ticket for one fresh agent context, including verification and cleanup.
- A completed ticket should leave the repository in a valid, reviewable state.
- Create only genuine dependency edges. Shared topic, preferred order, or the same parent does not imply blocking.
- Put foundational compatibility work first only when later slices genuinely cannot begin without it.
- For wide mechanical changes that cannot land green as vertical slices, use expand, migrate in reviewable batches, then contract.
- Keep volatile implementation detail out unless it is necessary to preserve a settled decision. Verify every included path and stamp the relevant revision when drift matters.

## Resume Contract

The canonical artifact must let a fresh agent answer:

- What outcome is required?
- Which decisions are already settled?
- What is in and out of scope?
- What has completed, what is blocked, and what is ready now?
- What evidence proves each item complete?
- Where should newly discovered decisions or scope changes be recorded?

Update the canonical tracker artifacts as the plan changes. Comments may record progress, but settled requirements and current status belong in the artifact fields or body rather than being buried in a conversation log.
