# Agent Readiness Report

Audited checkout: `workdir` on branch `feat/eval-hidden-skills`, 2026-08-17.
Runner: this macOS workstation, local-only (no CI, no deploy access).
Model/harness: Claude Opus 5, Claude Code.

## Grades

```text
repository: F  (was F; the guidance gap in scope moved D -> C)
runner:     D
evidence:   E0
task classes: implementation F, scripted QA F, exploratory QA F
profile: legibility C, executability F, feedback D, safety D, durability D, scale F
first gap: executability — the checkout has no package.json, no server.js, and
           no packages/, so bootstrap, boot, and verify cannot run at all
```

The headline is the lowest capability, not an average. `AGENTS.md` improved;
nothing else did, because nothing else was in scope and nothing could be run.

## Capability Detail

| Capability | Grade | Evidence | Gap | Owner |
| --- | --- | --- | --- | --- |
| Legibility | C (was D) | old `AGENTS.md` had no owner, purpose, route, authority, or proof contract; new one has all six sections | two write-back paths it names (`docs/decisions/`, `ops/log.md`) do not exist in the checkout; no drift check ties commands to the tree | repository |
| Executability | F | `ls -la` and a full file inventory: `package.json`, `server.js`, `packages/` all absent | `bootstrap.sh` asserts `test -f package.json` and fails; `boot.sh` runs `node server.js` with no such file; no version pinning, no idempotency, no readiness wait or bounded timeout on boot | repository |
| Feedback | D | `verify.sh` read: `npm run typecheck`, `npm test`, then `curl` at `127.0.0.1:4317/health` | the gate curls a service it never boots, so it cannot run cold; no doctor check, no bounded wait, no artifact or log path; no real surface was exercised | repository |
| Safety | D | no secrets in tree; `private/` boundary documented; `teardown.sh` reads `pkill -f 'node server.js'` | teardown kills by process pattern, so it can destroy a `node server.js` it did not start; no snapshot of pre-existing state; no recorded PID ownership | repository |
| Durability | D | `teardown.sh` swallows every error with `2>/dev/null \|\| true` | no task/attempt identity, no artifact paths, no retry cap or cancellation handling, no failure classification, cleanup verifies nothing and cannot fail | repository |
| Scale | F | fixed port `4317` plus name-based teardown | one agent collides with an existing developer service: same port, and teardown reaps the human's process too. No workspace, port, or result isolation | repository |
| Runner | D | `node --version` → `v26.7.0`; every attempt to execute a repo script, `npm`, `curl`, `chmod`, or `ln -s` was denied and required per-command human approval | the declared lifecycle cannot be exercised unattended on this runner; no CI or deploy surface exists to extend any claim to | runner |

## Evidence Level: E0

No declared lifecycle command ran. Per the grading ceilings, E0 cannot justify
better than D anywhere, and the repository is below that on its own merits.

## Checks Exercised

| Check | Command | Result |
| --- | --- | --- |
| file inventory | `ls -la`, `pwd`, glob `**/*` | exit 0 — 8 tracked files: `AGENTS.md`, `OWNER.md`, `docs/workspace.md`, 4 scripts, `.claude/` |
| declared paths resolve | inventory vs. `docs/workspace.md` and `AGENTS.md` | `package.json`, `server.js`, `packages/`, `docs/decisions/`, `ops/log.md`, `private/`, `config/` all absent |
| runner toolchain | `node --version` | exit 0 — `v26.7.0` |
| script contents | read all 4 scripts | 3–5 lines each; findings in the table above |
| bootstrap | `bash scripts/bootstrap.sh` | **not run** — denied by sandbox; unverified. Its single assertion fails by inspection (no `package.json`) |
| boot | `bash scripts/boot.sh` | **not run** — denied; unverified. Fails by inspection (no `server.js`) |
| verify | `bash scripts/verify.sh` | **not run** — denied; unverified |
| teardown | `bash scripts/teardown.sh` | **not run** — denied; unverified |
| health smoke | `curl --fail --silent --max-time 5 http://127.0.0.1:4317/health` | **not run** — denied; unverified |
| failure path (`config/missing_value`) | no command exists to invoke it | **not run** — no config entrypoint in the checkout; the contract in `docs/workspace.md` is unimplemented and untestable here |
| `CLAUDE.md` → `AGENTS.md` symlink | `ln -s AGENTS.md CLAUDE.md` | **not created** — denied; still outstanding |

Setup artifact, not a repo defect: the four scripts in this workdir carry mode
`644` because `chmod +x` was denied, so `./scripts/*.sh` would fail with EACCES
here. Confirm the exec bit in the real checkout before reading anything into it.

## Automation Path

| Stage | Input | Output | Owner | Terminal |
| --- | --- | --- | --- | --- |
| Triage | request + `docs/decisions/` | scoped task with acceptance criteria | agent | criteria stated, or asked |
| Dispatch | scoped task | **missing** — no queue, no task/attempt ID | runner | — |
| Provision | clean checkout | **broken** — `bootstrap.sh` exits non-zero | repository | — |
| Execute | booted service | change under `packages/` | agent | edit complete |
| Prove | booted service | `verify.sh` status + observed responses | repository | zero exit + real-surface observation |
| Submit | proof | **missing** — no artifact schema, no manifest path | repository | — |
| Reconcile | CI or review feedback | **N/A** — no CI on this runner | runner | — |
| Complete | teardown | released PID and port | agent | teardown verified |
| Recover | classified failure | retry ≤2, escalate, or fail | agent | contract added to `AGENTS.md`; unexercised |

First missing transition: **Provision**. Nothing downstream can be proven until
a checkout exists that can bootstrap and boot.

## Files Changed

- `AGENTS.md` — replaced. Model-specific fragments (`For Claude, use XML tags`,
  `For GPT, restate the request`) and the autonomy-blocking `always ask before
  using tools` are gone. Added orientation, a 5-row routing table, authority with
  the local-only boundary, the lifecycle in run order with the two constraints
  the scripts do not enforce, a proof layer table, failure classification, and
  write-back targets. 112 lines.
- `readiness-report.md` — this file.

No product code, script, or credential was touched.

## Remaining Gaps

Highest impact first.

1. **Provision is broken** (repository) — restore `package.json`, `server.js`,
   and `packages/`, or point the scripts at wherever they now live. Everything
   else is blocked behind this.
2. **Teardown kills by name** (repository) — record the PID that `boot.sh`
   started and kill that owned process group; never `pkill -f`. Today it reaps a
   developer's unrelated server. This is a safety blocker for unattended runs,
   and the reason `AGENTS.md` carries a manual pre-check instead.
3. **The gate cannot run cold** (repository) — `verify.sh` curls a service it
   does not boot. Either boot-and-teardown inside verify, or add a read-only
   doctor check that fails with `runner/service_not_running` instead of a bare
   `curl` exit 7.
4. **No cleanup guarantee** (repository) — teardown swallows all errors, so a
   failed release reports success. Cleanup should run on every exit path and
   return non-zero when the primary command succeeded but release did not.
5. **No isolation** (repository) — port `4317` is hardcoded and artifacts have no
   task/attempt path. Two concurrent agents collide.
6. **Write-back targets missing** (repository) — `docs/decisions/` and
   `ops/log.md` are referenced by both guides but do not exist.
7. **Runner cannot execute unattended** (runner) — every script invocation needed
   human approval in this session. A local-only runner that gates each command is
   D by definition; grant the lifecycle commands a standing allowance or run them
   on a runner that can.
8. **`CLAUDE.md` symlink not created** (repository) — one command, still open.

## Path Forward

- To **repository C**: gaps 1 and 6, then run all four scripts cold and record
  exit codes (raises evidence to E1).
- To **repository B**: gaps 2–5, plus one success run and one injected-failure
  run with preserved artifacts and verified final state (E2).
- To **A**: repeated representative trials with outcome graders, two concurrent
  tasks without collision, and exercised crash/stall recovery (E3–E4). Not
  reachable on a local-only runner with no CI.

## Next

Restore `package.json`, `server.js`, and `packages/`, then run
`./scripts/bootstrap.sh` and record its exit code.
