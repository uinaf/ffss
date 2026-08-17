# Planning result — team invitations

**Destination:** Jira project `TEAM`. Evidence: `AGENTS.md:3` — "Planned engineering
work lives in Jira project TEAM. Multi-session features use an Epic with child Tasks
and `blocks` links for hard dependencies."

**Publication status:** not published. This session has no Jira write path (no Jira
connector, no verified `jira`/`acli` CLI). The artifacts below are paste-ready in the
shape `AGENTS.md` prescribes. Keys `TEAM-A`…`TEAM-D` are placeholders; replace them
with the real keys at creation time, creating the epic and `TEAM-A` before `TEAM-B`
and `TEAM-C` so the `blocks` links can reference real identifiers.

## Artifact set

| Placeholder | Type | Title | Parent | Blocked by |
|---|---|---|---|---|
| `TEAM-A` | Epic | Team invitations: create, accept, revoke | — | — |
| `TEAM-B` | Task | Create and send an invitation from the admin UI | `TEAM-A` | none |
| `TEAM-C` | Task | Accept an invitation and join the team | `TEAM-A` | `TEAM-B` |
| `TEAM-D` | Task | Revoke a pending invitation from the admin UI | `TEAM-A` | `TEAM-B` |

## Dependency graph

```text
                 TEAM-B  create + send
                 (frontier — no blockers)
                        |
              blocks    |    blocks
        +---------------+---------------+
        |                               |
   TEAM-C  accept + join           TEAM-D  revoke pending
   (parallel with TEAM-D)          (parallel with TEAM-C)
```

`blocks` links to create, and only these:

- `TEAM-B` **blocks** `TEAM-C`
- `TEAM-B` **blocks** `TEAM-D`

No link between `TEAM-C` and `TEAM-D`. They touch the same feature and the same
parent, which is not a dependency. Acceptance never reads revocation state and
revocation never reads acceptance state, so once `TEAM-B` lands both can run
concurrently in separate contexts.

**Current frontier:** `TEAM-B` only. Nothing else is actionable until it lands.

---

## `TEAM-A` — Epic: Team invitations: create, accept, revoke

```markdown
## Outcome
A team admin can invite someone to the team, that person can accept and become a
member, and the admin can revoke an invitation that has not been accepted.

## Context
There is no invitation path today. The work was agreed as three independently
reviewable end-to-end slices so that two of them can proceed in parallel once the
creation slice lands.

## Decisions and constraints
- Three child Tasks, each a vertical slice through admin UI and API, not one ticket
  per layer.
- Creation lands first because it owns the invitation record and its API contract.
- Acceptance and revocation are independent of each other and must not be
  serialized against each other.
- Each child leaves the repository in a valid, reviewable state on its own.

## Acceptance criteria
- [ ] An admin creates and sends an invitation from the admin UI.
- [ ] An invited person accepts an invitation and appears as a team member.
- [ ] An admin revokes a pending invitation and it can no longer be accepted.

## Non-goals
- Resending, bulk invitation, invitation expiry policy, and role or permission
  changes were not part of the agreed scope.

## Delivery shape
- `TEAM-B` — create and send an invitation (unblocked).
- `TEAM-C` — accept an invitation and join the team (blocked by `TEAM-B`).
- `TEAM-D` — revoke a pending invitation (blocked by `TEAM-B`).

## Verification
- Each child ships automated coverage for its own behavior plus a demonstration on
  the real admin UI surface.
- Repository gates run per child; the epic closes only when all three pass.

## Risks and stop conditions
- Risk: the invitation state model chosen in `TEAM-B` does not support revocation.
  Mitigation: `TEAM-B` defines and documents the states that `TEAM-C` and `TEAM-D`
  consume.
- Stop condition: if `TEAM-C` or `TEAM-D` needs a change to the `TEAM-B` contract,
  stop and record the change on this epic before diverging.
- Stop condition: if implementing a slice reveals a product decision that was not
  settled here, stop and ask rather than deciding it in code.
```

## `TEAM-B` — Task: Create and send an invitation from the admin UI

```markdown
## Parent
TEAM-A

## What this delivers
An admin fills in an invitee on the admin UI, submits, the request reaches the API,
an invitation is recorded in a pending state, and it is sent to the invitee.

## Acceptance criteria
- [ ] The admin UI exposes a way to create an invitation and submit it.
- [ ] The API persists the invitation with an identifier and a pending state.
- [ ] The invitee receives the invitation with whatever token or link acceptance
      will need.
- [ ] The admin sees the resulting pending invitation and sees an error state,
      with input preserved, when creation fails.

## Verification
- Automated coverage for the create endpoint, including its failure responses.
- Automated coverage for the admin UI submit path and its loading, error, and
  success states.
- Real-surface evidence: an invitation created from the admin UI and delivered.

## Blocked by
- None. This is the current frontier.

## Boundaries
- In scope: creation, sending, pending state, and the invitation state model that
  the other two slices read.
- Out of scope: acceptance (`TEAM-C`), revocation (`TEAM-D`), resend, expiry.
```

## `TEAM-C` — Task: Accept an invitation and join the team

```markdown
## Parent
TEAM-A

## What this delivers
An invitee follows a pending invitation, accepts it, and becomes a member of the
team; the invitation is no longer pending.

## Acceptance criteria
- [ ] Following a valid pending invitation presents an accept action.
- [ ] Accepting adds the invitee to the team and marks the invitation accepted.
- [ ] An invitation that is not pending cannot be accepted and says why.
- [ ] Accepting twice does not create a second membership.

## Verification
- Automated coverage for the accept endpoint, including the already-accepted and
  invalid-invitation cases.
- Automated coverage for the accept surface states.
- Real-surface evidence: an invitation created via `TEAM-B` accepted end to end,
  with the invitee visible as a member.

## Blocked by
- TEAM-B — there is no invitation to accept, and no token or state contract to
  verify against, until creation lands.

## Boundaries
- In scope: acceptance and the resulting membership.
- Out of scope: revocation (`TEAM-D`) and any change to roles or permissions.
- Do not serialize against `TEAM-D`; these two slices are independent.
```

## `TEAM-D` — Task: Revoke a pending invitation from the admin UI

```markdown
## Parent
TEAM-A

## What this delivers
An admin revokes a pending invitation from the admin UI; the invitation moves out
of pending and can no longer be used.

## Acceptance criteria
- [ ] The admin UI offers a revoke action on a pending invitation.
- [ ] Revoking marks the invitation revoked and it is no longer listed as pending.
- [ ] A revoked invitation cannot be redeemed.
- [ ] Revoke shows submitting and error states and does not silently no-op.

## Verification
- Automated coverage for the revoke endpoint, including revoking an invitation
  that is already not pending.
- Automated coverage for the admin UI revoke path and its states.
- Real-surface evidence: an invitation created via `TEAM-B` revoked from the
  admin UI.

## Blocked by
- TEAM-B — there is no pending invitation to revoke until creation lands.
- Not blocked by TEAM-C. Revocation is defined against the pending state and does
  not need acceptance to exist.

## Boundaries
- In scope: revoking a pending invitation.
- Out of scope: acceptance (`TEAM-C`), deleting invitation history, resend.
- Do not serialize against `TEAM-C`; these two slices are independent.
```

---

## Handoff

```text
canonical: TEAM-A (epic, not yet created)
children: TEAM-B, TEAM-C, TEAM-D (not yet created)
blocks: TEAM-B -> TEAM-C, TEAM-B -> TEAM-D
frontier: TEAM-B (no blockers)
parallel after TEAM-B: TEAM-C and TEAM-D, no edge between them
gaps: no Jira write path in this session; nothing published
next: create TEAM-A, then TEAM-B, then TEAM-C and TEAM-D with the two blocks links
```

## Assumptions

- The three slices are exactly the ones agreed; nothing was added to scope. The
  non-goals list only names adjacent work as excluded, it does not schedule it.
- No repository file paths are cited: this working tree contains only `AGENTS.md`,
  so there are no verified seams to point an implementer at.
