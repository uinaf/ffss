# Inventory

Read sources in this order; a later source wins only where it is the owner.

## Sources

| Source | What to take |
| --- | --- |
| Agent guide (`AGENTS.md`, `CLAUDE.md`), README, CONTRIBUTING | Gate command, proof map, hazards, setup |
| Testing, harness, QA, release docs | Lanes, devices, accounts, fixtures, evidence, cleanup |
| CI workflows | Which jobs gate a merge, path filters, scheduled or dispatch-only lanes, secrets each job uses |
| Task graph: `package.json` scripts, Gradle tasks, `Makefile`, `justfile`, `mise.toml`, `Taskfile`, Xcode schemes, Cargo aliases | The real command names and what the gate depends on |
| Proof scripts: header comments, `--help`, exit codes, stdout markers | Flags and contracts; these are Lookup rows |
| Test sources: opt-in instrumented or end-to-end tests, their argument names and skip conditions | Lane names and opt-in flags |
| Ignore files and evidence directories | Where captures land and what never gets committed |
| Recent merged changes and their review threads | What reviewers asked for as proof; repeated misses are hazards |

For each row record: the fact, its owner, and whether a command proves it
(runs, prints it, or fails without it).

## Gaps

Ask only those the sources leave open:

- Which lanes gate a merge, and which are opt-in or manual?
- Which test identity is sanctioned for live proof, and how does it sign in
  (CLI profile, OAuth, device code)? Who approves a fresh login?
- Which devices, emulators, simulators, or accounts are shared with other
  people or agents?
- Where does heavy work run (slow builds, full suites, emulators, Docker)?
  Is there a routing skill or runbook to name?
- Which fixtures may a run create, and where do run ledgers live?
- Where does reviewed evidence go (PR comment, attachment host)?
- Which actions need a person: paid calls, production writes, store uploads?

A question whose answer would not change the skill is not a gap.
