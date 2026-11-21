---
name: slopflow
description: "Run a backlog as epics and parallel agent lanes: intake, groom, kickoff, dispatch, coordinate, hold, land, close out. Use when asked to triage incoming requests, groom or group issues into epics, dispatch or brief lanes, check on lanes or what needs the user, hold or resume lanes, or wrap up a finished lane."
---

# Slopflow

A coordinator runs a backlog as epics and lanes: intake, plan, lanes, land,
report. The scope is the core: one lane per epic, owning its files, issues,
and cleanup.

| Role | Is | Does |
| --- | --- | --- |
| Coordinator | this session | triages, grooms, writes briefs, relays between lanes, carries the user's decisions; edits, `git`, and the forge CLI only |
| Lane | one agent session per epic | runs its own workflow and subagents over the epic's issues |
| Epic | a tracker issue | the lane's [ledger](#ledger) |

## Graph

The issue tree is the only graph: sub-issues for containment, "blocked by"
links for dependencies, labels for state (`ready`, `claimed`, `needs-user`,
`failed`, `done`). A node is ready when it is open, unclaimed, and has no open
blockers. Keep no second store: no plan file, DAG, or hand-kept board.

- Only the coordinator changes the graph. A lane proposes a sub-lane in a
  comment on its issue; the coordinator creates it after checking file
  ownership. Depth stops at coordinator, lane, and the lane's own subagents.
- Review capacity, not machine capacity, caps how many lanes build at once.
- A lane facing a design decision its brief doesn't settle labels the node
  `needs-user` with one question, then takes its next ready issue.
- Results come back as references: the change request link and its check
  result. Check the plan against the results before calling an epic done.
  "Stopped trying" and "doesn't work" are different outcomes; failed work
  survives as a branch or patch.

## Intake

Requests from outside (chat threads, review requests, mentions, reports)
become tracker issues first, then join an epic or a lane. Match each to an
existing issue before filing a new one. The coordinator works none of them ad
hoc. Reply to the requester with the issue link, and put anything that changes
an epic's scope or priority to the user.

## Groom

1. Read every open issue in the repository.
2. Propose closes: fixed, obsolete, duplicate, nitpick, or vague, with a
   one-line reason each. Of an overlap, keep the clearest issue.
3. Map every remaining issue to one epic in its own repository. Group by
   feature or root cause, never by platform or device, so one lane builds and
   proves every surface of a fix. An issue fixed in another repository goes
   to an epic there.
4. Show the epic map as a [board](#board), settle the closes and open
   questions by [interview](#interview), and apply them on the user's OK.

An epic is titled `Epic: <area>` and holds its issues as sub-issues. Its body
states the outcome, the groups or root causes, and when it closes.

Groom is done when every open issue has an epic or a close, and every decision
only the user can make sits on its issue.

## Kickoff

Before the first lane starts, draw the high-level design: a state diagram of
each lifecycle the epics touch, and the module boundaries with the lane that
owns each. Get the user's agreement on it; lanes start after that and their
briefs link it.

## Dispatch

Split lanes so no two own the same file. A file two epics need goes to one
lane; the other gets its change through the coordinator, or waits on that
lane's merge. Check each lane's proof target (tools, test accounts, hosts,
devices) first; a lane whose target fails stays undispatched, with the gap
reported.

A brief carries, in order:

1. **Goal**: the epic, its issues sized S/M/L, their order, the files owned,
   and the issues to leave alone.
2. **Authority**: branches, change requests, and merges in the repository;
   each shared device or account the lane may use.
3. **Hosts**: where builds run, and the [host limits](#host-limits).
4. **Proof**: what done means for each issue.
5. **Ledger**: keep the epic's lane checklist current.
6. **Report**: a [board](#board), on completion or on a blocker only the user
   can clear.

How a lane runs its work (workflow, subagents, stacking) is its call.

### Host limits

Starting points, not caps. Tune them to live load and the lane's work, and
record a change that sticks.

- A load ceiling per build host; check load before each heavy run.
- A device or emulator count per lane per host.
- Worktrees, branches, simulators, emulators, and scratch carry the lane
  prefix; a lane touches only its own.
- Lanes merging into one shared branch take a merge lock.
- A rate-limited shared resource, such as test sign-ins, serves one lane at a
  time under a lock.

## Coordinate

- Relay a cross-lane dependency with the merge SHA the waiting lane needs.
- "What needs me": show every lane on one [board](#board), then
  [interview](#interview) the user on the open decisions. Send each answer to
  its lane and record it on its issue.
- A lane silent past its expected report gets checked, not waited on.
- Out of usage or context: write a handoff with state, open decisions, and
  the next prompt.

## Hold

Send every lane:

> HOLD: stop all work until the coordinator's next message. In this one turn,
> stop the jobs, builds, devices, and lock waiters your lane started on every
> host. Leave branches, worktrees, and change requests as they are, and merge
> nothing. Reply in at most three lines: what's mid-flight and where to
> resume.

Resume names each lane's next issue, branch, and head, under the same brief.
The lane checks its worktrees, devices, and locks first.

## Land

- **Stack**: dependent issues can land as a stack, one change request per
  issue on top of the previous, so each layer reviews fast. Review each layer
  against the layer below it, and merge bottom-up.
- **Review**: one independent review of each final head comes back clean
  ([slopguard](../slopguard/SKILL.md)); every review-bot comment on that head
  is fixed or disproved with evidence.
- **Rounds**: one fix-and-review round per issue, three at most. A fourth
  needs the user.
- **Scope**: findings outside the issue become sub-issues of the epic.

## Close out

A lane is done when its epic checklist is current, its follow-ups are filed,
and it has removed its worktrees, branches, devices, scratch, and locks on
every host, and signed out any account it signed in. The coordinator then
archives the lane, prunes leftovers on each host, and reports what still
waits on the user.

## Board

Generated from the tree, one row per issue: lane, issue, status, change
request, head, proof, and what is unverified. Each lane is working, waiting on
the user, or stopped. A lane waiting on another lane or on CI is not waiting
on the user; only `needs-user` nodes and layers awaiting the user's review
are. Render it in what the harness shows: a Markdown table or
Mermaid diagram, or an HTML render where one is available. Follow it with at
most a few lines on follow-ups and cleanup.

## Interview

One question at a time: the decision, its options, and a recommendation.
Queue the rest until the answer comes back.

## Ledger

The epic issue. Each lane keeps a `## Lane <name>` checklist, one line per
issue, updated in place:

```markdown
- [x] #12 merged #20
- [ ] #31 blocked on #27: waits for the signing key
```

Statuses: `merged`, `blocked`, `needs-user`, `skipped`, `gap`.
