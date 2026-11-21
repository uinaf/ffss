---
name: slopflow
description: "Run a backlog as epics and parallel agent lanes. Use when asked to groom or group issues into epics, kick off, dispatch, or brief lanes, check on lanes or what needs the user, hold or resume lanes, or wrap up an epic or lane."
---

# Slopflow

A coordinator runs a backlog as epics and lanes. The scope is the core: one
lane per epic, owning its files, issues, and cleanup.

| Role | Is | Does |
| --- | --- | --- |
| Coordinator | this session | grooms, writes briefs, relays between lanes, carries the user's decisions |
| Lane | one agent session per epic | works the epic's issues |
| Epic | a tracker issue | the lane's [ledger](#ledger) |

The tracker's issue tree is the plan. Only the coordinator changes its
structure: epics, sub-issues, and dependencies. A lane updates its own
issues' state and proposes anything else on its epic.

## Groom

1. Read every open issue in the repository.
2. Propose closes: fixed, obsolete, duplicate, nitpick, or vague, with a
   one-line reason each. Of an overlap, keep the clearest issue.
3. Map every remaining issue to one epic. Group by feature or root cause,
   never by platform or device, so one lane builds and proves every surface
   of a fix. An epic holds issues from its own repository only; an issue
   fixed elsewhere links to the issue there.
4. Show the epic map as a [board](#board), settle the closes and open
   questions by [interview](#interview), and apply them on the user's OK.

## Kickoff

Before the first lane starts, draw the high-level design: a state diagram of
each lifecycle the epics touch, and the module boundaries with the lane that
owns each. Get the user's agreement on it; lanes start after that, and their
briefs link it.

## Dispatch

Split lanes so no two own the same file. A file two epics need goes to one
lane; the other gets its change through the coordinator, or waits on that
lane's merge.

A brief carries, in order:

1. **Goal**: the epic, its issues in order, the files owned, and the issues
   to leave alone.
2. **Authority**: branches, change requests, and merges in the repository.
3. **Hosts**: where builds run, and the [host limits](#host-limits).
4. **Proof**: what done means for each issue.
5. **Ledger**: keep the epic's lane checklist current.
6. **Report**: a [board](#board), on completion or on a blocker only the user
   can clear.
7. **Decisions**: a design choice the brief doesn't settle gets the
   `needs-user` label and one [interview](#interview) question on its issue;
   the lane then takes its next issue.

### Host limits

Starting points, not caps: tune them to live load, and record a change that
sticks.

- A load ceiling per build host; check load before each heavy run.
- A device or emulator count per lane per host.
- Worktrees, branches, simulators, emulators, and scratch carry the lane
  prefix; a lane touches only its own.

## Coordinate

- Relay a cross-lane dependency with the merge SHA the waiting lane needs.
- "What needs me": every lane on one [board](#board), then
  [interview](#interview) the user on the open decisions. Send each answer to
  its lane and record it on its issue.

## Hold

Send every lane:

> HOLD: stop all work until the coordinator's next message. In this one turn,
> stop the jobs, builds, devices, and lock waiters your lane started on every
> host. Leave branches, worktrees, and change requests as they are, and merge
> nothing. Reply in at most three lines: what's mid-flight and where to
> resume.

Resume names each lane's next issue, branch, and head, under the same brief,
numbering the attempt (`sync a2`). The lane checks its worktrees, devices, and
locks first and tags reports with the attempt; older attempts' reports change
nothing.

## Close out

An epic closes only when each planned issue is proven or waived.
Failed or abandoned work survives as a branch or patch linked from its issue.
Findings outside an issue become sub-issues of the epic.

A lane is done when its epic checklist is current, its follow-ups are filed,
and it has removed its worktrees, branches, devices, scratch, and locks on
every host. Archive the lane only then.

## Board

Generated from the issue tree. Each lane is working, waiting on the user,
stopped, or stalled: no commit, report, or stage change for 2 hours. Lead with
one flow line. WIP counts lanes with unproven work, capped at how many the
user can review:

`WIP 2/3 · landed, not live: #14 40m · oldest in flight 2h · last proven 35m ago`

Where the harness renders HTML, draw the tree and show the page:

```bash
go run <this skill's dir>/scripts/board/main.go github -cap <N> <owner/repo> <root issue> <out.html>
```

For another tracker, write the board JSON in the
[contract](scripts/board/main.go) and pass it to
`render <board.json> <out.html>`. Elsewhere, a Markdown table, one row per
issue: lane, issue, status, change request, head, proof, and what is
unverified. Beside an HTML board, list each lane's head, proof, and what is
unverified.

## Interview

One question at a time: the decision, its options, and a recommendation. Ask
the first now and hold the rest until it's answered, even in a written
deliverable.

Four or more open decisions go on one static HTML page instead: a card each
with its options and recommendation, and a copyable answer block (the board
script's `decide`). Ask for one reply; record each answer on its issue.

## Ledger

The epic issue. Each lane keeps a `## Lane <name>` checklist, one line per
issue, updated in place, with its stage (landed, deployed, proven, or waived
when nothing deploys) and evidence:

```markdown
- [x] #12 proven: smoke 4411 passes in production
- [ ] #14 landed: #20 merged, promote pending
- [ ] #31 needs-user: conflict banner or silent merge?
```
