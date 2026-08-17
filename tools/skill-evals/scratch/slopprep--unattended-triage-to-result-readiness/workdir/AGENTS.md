# Agent guide

## What this repository is

`triage-worker-example` is a Node worker driven by an external orchestrator. A
request is triaged into a typed task, dispatched to an isolated devbox workspace,
and finished without a human watching. Two result paths exist and they are not
interchangeable:

- **implementation** — change code, prove it, hand off a pull request, and merge
  only when the task granted that authority.
- **qa** — test an existing revision or build, capture real evidence, and hand off
  a report. A qa task never creates a branch, commit, or pull request.

The outcome that must not regress: a result an orchestrator can reconcile without
asking a human what happened. That means every terminal state carries a stable
failure class, an owner (`runner/` or `repo/`), and preserved evidence.

## Commands

Run these from the repository root. They are the same commands humans and CI use;
there is no agent-only surface.

| Command | When |
| --- | --- |
| `pnpm bootstrap` | once per attempt, before anything else |
| `pnpm doctor` | before driving the workspace, and after anything surprising |
| `pnpm verify` | the canonical gate; produces the result manifest |
| `pnpm run own record -- --kind <kind> --id <id>` | before driving any runtime resource you raised |
| `pnpm run own list` | what this attempt still holds |
| `pnpm run teardown -- --reason <success\|failure\|cancelled\|timeout\|retry>` | on every exit path |
| `pnpm test` | the raw test gate; `pnpm verify` runs it for you |

`pnpm bootstrap` is noninteractive: it validates the runner contract and the
injected machine identity, activates the pinned pnpm, and installs reproducibly.
It never logs in, never writes `.env.local`, and never prints a secret.

`pnpm verify` is bounded (`AGENT_VERIFY_TIMEOUT_SECONDS`, default 900s) and always
writes `artifacts/<task>/<attempt>/manifest.json`, on success and on failure.

`pnpm teardown` is idempotent and releases only what this attempt recorded.

Exit codes: `0` success · `1` repository failure · `2` runner failure · `3` work
succeeded but cleanup did not · `130` cancelled.

## Task acceptance

The task arrives as environment variables, not as a conversation. Required:
`AGENT_TASK_ID`, `AGENT_ATTEMPT_ID`, `AGENT_RESULT_TYPE`, plus `INFISICAL_TOKEN`,
`INFISICAL_API_URL`, `INFISICAL_PROJECT_ID`. A qa task also needs
`AGENT_TARGET_REVISION`. Full table:
[`docs/automation.md` §1](docs/automation.md#1-task-input).

If a required variable is missing, the failure is the runner's
(`runner/missing_task_identity`, `runner/missing_identity`). Do not work around
it, do not invent values, do not log in interactively. Report and stop.

## Authority

| Action | Allowed |
| --- | --- |
| edit files, run the lifecycle commands, write under `artifacts/<task>/<attempt>/` | yes |
| create a branch | implementation only, exactly `agent/<task>/<attempt>` |
| open or update a pull request | implementation only, when `AGENT_SUBMISSION_TARGET=pull_request` |
| merge | only when `AGENT_MERGE_AUTHORITY=merge`, and only after checks and review resolve |
| create a branch, commit, or PR on a qa task | never |
| print, copy, commit, or write a secret value anywhere | never |
| release a resource this attempt did not record | never |
| kill by process name, stop all containers, shut down all simulators | never |

Credentials are injected by the runner and consumed noninteractively. Humans own
provisioning, rotation, revocation, and emergency recovery.

## Proof

What each layer does and does not prove:

- `pnpm test` proves the repository's own gate on the checked-out revision.
- `pnpm verify` additionally proves the evidence is complete, described, and free
  of injected secret values. It does not prove a browser flow, a device, a
  provider, or a deployment.
- A qa attempt's proof is `scenarios.json` plus the files it references. Each
  scenario needs an `id`, a `result` (`passed`/`failed`/`blocked`/`skipped`), a
  `producer`, and evidence that exists on disk. Revision, build, environment,
  format, capture time, hash, and redaction status are recorded for you.
  Schema and example:
  [`docs/automation.md` §3](docs/automation.md#3-attempt-identity-and-artifact-layout),
  [`docs/examples/qa-scenarios.json`](docs/examples/qa-scenarios.json).
- A failing qa scenario is a **finding**, not a broken attempt: `pnpm verify`
  still exits 0 and sets `findings_present: true`. Report the finding; do not
  retry to make it green.
- Green CI does not prove a provider, device, or deployed endpoint unless that
  surface actually ran. Label unavailable proof as unverified and name the
  missing capability.

## Resource ownership

Record a resource before you drive it, or teardown will treat it as pre-existing
and leave it running:

```bash
pnpm run own record -- --kind process_group --id "$PID" --pgid "$PID" --label dev-server
```

Kinds: `process`, `process_group`, `container`, `simulator`, `emulator`, `vm`,
`service`, `temp_dir`. `emulator`, `vm`, and `service` also need `--release` and
`--verify` argv. Rules and rationale:
[`docs/automation.md` §8](docs/automation.md#8-resource-ownership).

## Recovery

- Retry only when the manifest says `retryable: true` and budget remains. Each
  retry uses a new `AGENT_ATTEMPT_ID`, so earlier evidence survives.
- Same failure class twice with no change means escalate, not retry again.
- Exit `2` is the runner's problem; exit `3` means the workspace is not reusable.
- After process loss, `artifacts/<task>/<attempt>/` is the durable record. Read
  `manifest.json`, run `pnpm doctor`, then `pnpm run own list`.
- Preserve the last diagnostic and all evidence. Cleanup never deletes evidence.

## Write-back

- Durable automation decisions and schema changes go in
  [`docs/automation.md`](docs/automation.md).
- Readiness grades, evidence level, and open gaps go in
  [`readiness-report.md`](readiness-report.md).
- Attempt state, evidence, and terminal outcome go in
  `artifacts/<task>/<attempt>/`. Nothing durable belongs only in a transcript.
- Lifecycle behaviour changes belong in `scripts/lifecycle/` with a test in
  `test/`; the failure taxonomy lives once in
  `scripts/lifecycle/failure.mjs`.

## Deeper contracts

| Read this | When |
| --- | --- |
| [`docs/automation.md`](docs/automation.md) | wiring an orchestrator, or you need the exact manifest, scenario, state, or taxonomy definitions |
| [`readiness-report.md`](readiness-report.md) | you need to know what has actually been exercised versus asserted |
| `scripts/lifecycle/contract.mjs` | a contract validation decision looks wrong |
| `scripts/lifecycle/resources.mjs` | you are adding a resource kind or a release handler |
