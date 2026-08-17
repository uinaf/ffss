# Readiness report

Audited: 2026-08-17. Repository: `triage-worker-example`. Requested target: B.
Task classes: `implementation` and `qa`. Declared runner: devbox with a scoped
Infisical machine identity.

## Grades

```text
repository:   D → B (contract complete; not yet exercised — see Evidence)
runner:       D  (declared devbox B, this audit host D)
evidence:     E0
task classes: implementation B*, scripted QA B*, exploratory QA C*
profile:      legibility B, executability B*, feedback B*, safety B*, durability B*, scale C
first gap:    evidence — no lifecycle command could be executed in this session,
              so every repository grade above C is asserted from the contract and
              static review, not from an exercised run
```

`*` marks a grade the contract supports but that no trial has confirmed. Under the
evidence ceiling rule, **E0 cannot justify more than D**. Read the starred grades
as "designed to B, provisionally D until the trials in
[Evidence](#evidence-and-what-would-raise-it) run." The unstarred grades —
legibility, and scale at C — rest on artifacts that can be read rather than run.

### Repository: D → B (design), provisional pending trials

Before: bootstrap was `scripts/current-agent-setup.sh`, which ran `infisical
login`, told the agent to hand-copy secrets into `.env.local`, invited it to pick
any branch even for a QA task, and to watch CI in a browser. `README.md` ended
with "ask a maintainer what to do next". Four separate D-locks: interactive setup,
copied secrets, no result-type distinction, human-in-the-loop reconciliation.

After: one noninteractive lifecycle on the ordinary `package.json` surface, a
typed runner contract, keyed artifacts, an enforced evidence schema, and an
ownership-tracking teardown. No `agent-*` parallel layer was created.

### Runner: D (this host) / B (declared devbox)

The declared devbox satisfies the runner side of the contract: it injects
`INFISICAL_TOKEN`, `INFISICAL_API_URL`, and `INFISICAL_PROJECT_ID` from a scoped
machine identity, supplies `AGENT_TASK_ID` and `AGENT_ATTEMPT_ID`, and owns
workspace isolation and submission mechanics. That is positive runner evidence,
not manual repository setup.

This audit host is not that runner. Measured here:

| Capability | Observed | Owner |
| --- | --- | --- |
| node | `v26.7.0` (contract needs >= 22) | runner — satisfied |
| pnpm, corepack | present via mise shims at `~/.local/share/mise/shims/` | runner — satisfied |
| git, jq, bash | present under `/opt/homebrew/bin` | runner — satisfied |
| `infisical` CLI | **absent** (`which infisical` → not found) | runner — not needed; the repository consumes the injected token directly and never shells out to the CLI |
| machine identity env | **absent**; no `INFISICAL_*`, `AGENT_TASK_ID`, or `AGENT_ATTEMPT_ID` exported | runner |
| command execution | **blocked**; the session permits `node --version`, `which`, `ls`, `rm`, `echo` but denies `node --test`, `pnpm`, `bash -c`, and running any repository script | harness |

The last two rows are why the runner grade is D here and why evidence is E0. They
are runner and harness gaps, not repository defects.

## Capability profile

| Capability | Grade | Evidence | Gap | Owner |
| --- | --- | --- | --- | --- |
| Legibility | B | `AGENTS.md` states orientation, both result paths, commands, authority table, proof layers, recovery, and write-back, and routes to `docs/automation.md` for schemas. `test/guidance.test.mjs` fails the gate when a documented command, failure class, vocabulary term, or link drifts from the code. | Drift guard is written but unexecuted. | repository |
| Executability | B* | `pnpm bootstrap` validates the contract, requires node >= 22 / git / corepack, activates the `packageManager` pin through corepack and asserts the active version matches, installs with `--frozen-lockfile` when dependencies are declared, and classifies every failure as `runner/*` or `repo/*`. `pnpm doctor` is a read-only drivability check. | Never run cold. | repository |
| Feedback | B* | `pnpm verify` still runs the repository gate (`pnpm test`), bounds it at `AGENT_VERIFY_TIMEOUT_SECONDS`, writes `logs/verify.log` plus `manifest.json` on success and failure, and enforces the QA evidence schema — a QA report cannot be submitted for evidence that does not exist on disk. `test/` exercises real process trees and real temp directories rather than mocks. | No gate run, so no real-surface proof of the gate itself. | repository |
| Safety | B* | Identity is presence-and-shape checked, never read or printed; `secretValues()` collects injected values and `scanForSecrets()` fails the gate with `repo/secret_exposure` if one reaches a text artifact, reporting the path and a count but never the value. `.gitignore` excludes `artifacts/` and `.env*`. Authority is typed: a QA task cannot be dispatched at a pull request and can never hold merge authority. | Denied-scope behaviour is designed, not observed. | repository + runner |
| Durability | B* | Task and attempt identity, idempotent bootstrap and teardown, retry budget, five teardown reasons, a stable failure taxonomy with owner and retryability, and a durable artifact directory that survives process loss. Exit code `3` distinguishes "work succeeded, cleanup did not". | Crash and stall recovery unexercised. | repository |
| Scale | C | Every path the repository writes is keyed by task and attempt: artifact directory, ledger, snapshot, teardown record, and allowed branch `agent/<task>/<attempt>`. One attempt runs safely in an owned workspace. | Two concurrent attempts have not been run, and reconciliation of PR + CI + review state has not been consumed end to end. | repository + runner |

## Automation path

| Stage | Input | Output | Owner | Terminal condition | State |
| --- | --- | --- | --- | --- | --- |
| Triage | request | typed task: result type, acceptance, target, authority | runner | task carries a result type | contract defined, orchestrator absent |
| Dispatch | typed task | attempt identity + environment | runner | `AGENT_TASK_ID`, `AGENT_ATTEMPT_ID` exported | contract defined |
| Provision | attempt identity, machine identity | `bootstrap.json` | runner allocates, repo validates | `pnpm bootstrap` exit 0 | **implemented, unexercised** |
| Execute | workspace + task | diff or captured evidence | agent | work done or blocked | agent judgment, boundaries enforced |
| Prove | tree or evidence | `manifest.json`, `logs/`, `evidence/` | repository | `pnpm verify` exit 0 with `outcome: passed` | **implemented, unexercised** |
| Submit | passing manifest | PR, or QA report | runner mechanics, repo schema | provider acknowledges | repository side complete; submission deliberately not built |
| Reconcile | submission + external state | merged or accepted | runner | checks and reviews resolved | **missing — first gap on the path** |
| Complete | reconciled result | terminal state + artifacts | runner | teardown clean | `pnpm teardown` implemented, unexercised |
| Recover | failure class | retry, escalate, fail | runner decides, repo classifies | budget spent or failure repeats | classification implemented; retry loop is the runner's |

First missing transition: **Submit → Reconcile.** Everything up to and including
Prove now has a repository-side contract. Reconciliation is runner-owned by
design — this repository declares the target, the allowed branch, the manifest,
and merge authority, and deliberately performs no external action.

## Evidence and what would raise it

Nothing in this session executed a lifecycle command, so the level is **E0**. No
run, no artifact directory, no manifest, and no teardown record was produced.
What was actually done: the checkout was read, the toolchain was probed with
`node --version` and `which`, and the new code was reviewed statically. Three
defects found that way were fixed — a mis-classified numeric input failure, an
argv parser that split JSON on every `=`, and a manifest that hashed
`logs/verify.log` before the write stream flushed.

Reaching E2 needs these eight trials on the declared devbox. Run them in order;
each is a single command with a known expected outcome.

| # | Trial | Command | Expect |
| --- | --- | --- | --- |
| 1 | cold bootstrap | `pnpm bootstrap` with the full contract exported | exit 0; `artifacts/<task>/<attempt>/bootstrap.json` records toolchain, install, identity presence, and no secret value |
| 2 | missing identity | `pnpm bootstrap` with `INFISICAL_TOKEN` unset | exit 2, `runner/missing_identity`, recovery text names the runner, no value printed |
| 3 | drivability | `pnpm doctor` before and after trial 1 | unhealthy → `bootstrapped_for_this_attempt`; then healthy |
| 4 | implementation gate | `pnpm verify` with `AGENT_RESULT_TYPE=implementation` | exit 0; manifest `outcome: passed`, `allowed_branch: agent/<task>/<attempt>`, `logs/verify.log` hashed |
| 5 | failing gate | trial 4 with a deliberately broken test | exit 1, `repo/verification_failed`, manifest still written, log preserved |
| 6 | QA without evidence | `pnpm verify` with `AGENT_RESULT_TYPE=qa` and no `scenarios.json` | exit 1, `repo/evidence_missing`, no branch and no PR created |
| 7 | QA with evidence | same, with `scenarios.json` from `docs/examples/qa-scenarios.json` and real captures | exit 0; every artifact carries revision, build, environment, scenario, producer, capture time, format, sha256, redaction |
| 8 | teardown ownership | start a `sleep` tree, record it with `pnpm run own record -- --kind process_group --id <pid> --pgid <pid>`, start a second unrecorded tree, then `pnpm run teardown -- --reason success` twice | recorded tree gone, unrecorded tree alive, second run clean, `verify.log` still on disk |

Then, for a defensible B: repeat 4 and 7 three times each (E3), and add two
concurrent attempts plus one injected stall or kill (E4).

The test suite itself carries some of this weight once it runs. `pnpm test`
declares 41 tests across five files, including a real launcher-plus-descendant
process group that is released and verified absent, a temp directory released by
recorded id, a pre-existing resource that survives teardown even when its id also
appears in the ledger, evidence preserved across cleanup, and the exit-code policy
(primary failure preserved; clean success plus dirty cleanup exits `3`).

## Gaps, by impact

| Gap | Impact | Owner | Action |
| --- | --- | --- | --- |
| No trial has run | Caps honest evidence at E0 and every starred grade at provisional D | runner / harness | run the eight trials above on the devbox |
| Machine identity absent on this host | Bootstrap trials 1–3 cannot run here | runner | dispatch on the devbox, or export the contract locally for a dry run |
| Reconcile stage missing | An attempt can produce a result but not close the loop on CI and review feedback | runner | implement submission and reconciliation in the orchestrator against §3 and §5 of `docs/automation.md` |
| Concurrency unexercised | Scale stays at C | repository + runner | run two attempts against the same checkout, assert no shared path or ref |
| No lockfile | `pnpm install --frozen-lockfile` is skipped while zero dependencies are declared; the first added dependency needs a committed lockfile | repository | commit `pnpm-lock.yaml` with the first dependency — bootstrap fails with `repo/missing_lockfile` until then, by design |
| `CLAUDE.md` is a pointer file, not a symlink | Two files instead of one canonical guide | repository | `rm CLAUDE.md && ln -s AGENTS.md CLAUDE.md` — symlink creation was blocked in this session |
| QA evidence capture tooling absent | Trial 7 needs a real capture producer; the repository declares the schema but ships no browser or device harness | repository | add one when the first real QA task names its surface; do not add a framework speculatively |

## Files changed

| Path | Change |
| --- | --- |
| `scripts/current-agent-setup.sh` | deleted — interactive login, copied secrets, browser-watched CI |
| `package.json` | added `bootstrap`, `doctor`, `teardown`, `own`; `verify` now the contract-aware gate that still runs `pnpm test`; added `engines.node` |
| `scripts/lifecycle/contract.mjs` | runner contract validation, allowed branch, secret-value collection |
| `scripts/lifecycle/failure.mjs` | failure taxonomy with owner, retryability, recovery, exit codes |
| `scripts/lifecycle/artifacts.mjs` | artifact layout, manifest and scenario schemas, hashing, redaction scan |
| `scripts/lifecycle/resources.mjs` | ownership ledger, pre-existing snapshot, release handlers, absence verification |
| `scripts/lifecycle/bootstrap.mjs` | cold start |
| `scripts/lifecycle/doctor.mjs` | read-only drivability check |
| `scripts/lifecycle/verify.mjs` | bounded gate, evidence manifest, secret scan, scoped cleanup |
| `scripts/lifecycle/teardown.mjs` | idempotent release, also imported by verify |
| `scripts/lifecycle/own.mjs` | resource ownership CLI |
| `test/contract.test.mjs`, `test/resources.test.mjs`, `test/evidence.test.mjs`, `test/teardown.test.mjs`, `test/guidance.test.mjs` | the gate's own coverage, including drift enforcement |
| `AGENTS.md`, `CLAUDE.md` | new agent operating guide, plus pointer |
| `docs/automation.md`, `docs/examples/qa-scenarios.json` | orchestrator contract and a copyable QA scenario file |
| `README.md`, `.gitignore`, `.github/workflows/verify.yml` | orientation, artifact and secret exclusions, CI running the same four lifecycle commands and preserving `artifacts/` |

## Path to A

Not reachable from here, and not requested. It would need: least-privilege
identities split per role (triage, fixtures, submission, delivery), exercised
crash and stall recovery, `pass@1` and `pass^3` over a representative task suite,
concurrent reconciliation under back-pressure, and a maintenance loop that turns
each production failure into a test in `test/` or a mechanical check rather than a
paragraph in `AGENTS.md`.
