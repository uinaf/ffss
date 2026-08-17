# Unattended automation contract

This repository can be driven by any orchestrator that satisfies the contract
below. Nothing here names or requires a particular orchestrator implementation.

Split of ownership, in one line: **the runner owns identity, workspace,
queueing, and external submission mechanics; this repository owns lifecycle
commands, evidence schemas, and terminal classification.**

## 1. Task input

The runner supplies task input as environment variables. `pnpm bootstrap`,
`pnpm doctor`, `pnpm verify`, `pnpm teardown`, and `pnpm run own` all parse them
through one validator (`scripts/lifecycle/contract.mjs`) and refuse to start on a
contract violation.

| Variable | Required | Meaning |
| --- | --- | --- |
| `AGENT_TASK_ID` | yes | stable task identity; `[A-Za-z0-9][A-Za-z0-9._-]{0,127}` |
| `AGENT_ATTEMPT_ID` | yes | identity of this attempt at the task; same charset |
| `AGENT_RESULT_TYPE` | yes | `implementation` or `qa` |
| `AGENT_ATTEMPT_NUMBER` | no (`1`) | 1-based attempt counter |
| `AGENT_RETRY_BUDGET` | no (`2`) | retries the orchestrator still allows |
| `AGENT_SUBMISSION_TARGET` | no (per result type) | `pull_request`, `qa_report`, or `none` |
| `AGENT_MERGE_AUTHORITY` | no (`none`) | `merge` only on `implementation` |
| `AGENT_BASE_REF` | no (`main`) | implementation base branch |
| `AGENT_TARGET_REVISION` | qa only | the revision under test |
| `AGENT_TARGET_BUILD` | no | prebuilt artifact identifier under test |
| `AGENT_ENVIRONMENT` | no (`devbox`) | environment label recorded in evidence |
| `AGENT_RUNNER` | no (`unknown`) | runner or image label recorded in evidence |
| `AGENT_ARTIFACT_ROOT` | no (`artifacts`) | root of the attempt artifact tree |
| `AGENT_VERIFY_TIMEOUT_SECONDS` | no (`900`) | bound on the verification gate |
| `INFISICAL_TOKEN` | yes | short-lived machine-identity token; presence only, never read or logged |
| `INFISICAL_API_URL` | yes | must be `https`; non-secret |
| `INFISICAL_PROJECT_ID` | yes | non-secret project identifier |

Optional switches, all default-off:

| Variable | Effect |
| --- | --- |
| `AGENT_IDENTITY_PROBE=1` | bootstrap makes one bounded authenticated request to prove the identity works |
| `AGENT_QA_RUN_REPOSITORY_GATE=1` | a qa attempt also runs the repository gate |
| `AGENT_QA_FAIL_ON_SCENARIO_FAILURE=1` | a failing qa scenario fails the command instead of being reported as a finding |

There is no interactive login, no profile switching, no `.env.local`, and no
secret file. Provisioning, rotation, revocation, and emergency recovery of the
machine identity are human-owned and happen outside task execution.

## 2. Result types

| | `implementation` | `qa` |
| --- | --- | --- |
| produces a diff | yes | no |
| allowed branch | `agent/<task>/<attempt>` | none |
| allowed submission target | `pull_request`, `none` | `qa_report`, `none` |
| merge authority grantable | yes, via `AGENT_MERGE_AUTHORITY=merge` | never |
| mandatory gate | `pnpm test` via `pnpm verify` | `scenarios.json` plus its referenced evidence |
| reconciles | PR + CI + independent review | report acceptance and rerun requests |

A qa attempt must not create a branch, commit, or pull request. The validator
rejects `AGENT_SUBMISSION_TARGET=pull_request` on a qa task with
`runner/invalid_submission_target`, and `manifest.allowed_branch` is `null`, so
the mistake fails before any external side effect.

## 3. Attempt identity and artifact layout

Every artifact is keyed by task and attempt:

```text
artifacts/<AGENT_TASK_ID>/<AGENT_ATTEMPT_ID>/
├── bootstrap.json               environment, toolchain, install, identity presence
├── doctor.json                  last read-only health report
├── manifest.json                the machine-readable result record (schema below)
├── scenarios.json               qa only: declared scenarios and their evidence
├── owned-resources.jsonl        append-only ownership ledger
├── preexisting-resources.json   snapshot taken before the attempt acquired anything
├── teardown.json                one record per teardown run
├── logs/
│   ├── verify.log               gate stdout and stderr
│   └── install.json             install exit code and tails, when an install ran
└── evidence/                    qa captures: screenshots, recordings, traces, JSON
```

Artifacts survive teardown. Cleanup releases runtime resources and never deletes
evidence.

### `manifest.json` — `agent-artifact-manifest/1`

Written by `pnpm verify` on success and on failure. Key fields:

| Field | Purpose |
| --- | --- |
| `task_id`, `attempt_id`, `attempt_number`, `retry_budget` | attempt identity and remaining budget |
| `result_type`, `submission_target`, `merge_authority`, `allowed_branch` | what may be submitted, and where |
| `revision` | `git_sha`, `dirty`, `base_ref`, and `under_test` (HEAD for implementation, `AGENT_TARGET_REVISION` for qa) |
| `build` | prebuilt artifact identifier and its source, when one was tested |
| `environment` | name, runner, os, arch, node, declared and active package manager, CI flag |
| `started_at`, `finished_at` | attempt bounds |
| `outcome` | `passed` or `failed` — whether the attempt produced a valid result |
| `failure_class`, `retryable`, `recovery` | terminal classification for the orchestrator |
| `gate` | command, exit code, signal, timeout flag, log path |
| `scenario_summary`, `scenarios`, `findings_present` | per-scenario outcomes; `findings_present` distinguishes "qa found a bug" from "qa broke" |
| `redaction` | policy, files scanned, findings, status |
| `artifacts[]` | `path`, `format`, `bytes`, `sha256`, `producer`, `captured_at`, `scenario`, `redaction` |

### `scenarios.json` — `agent-qa-scenarios/1`

A qa attempt writes this before `pnpm verify`. `pnpm verify` validates it,
resolves every evidence path inside the attempt directory, and fails with
`repo/evidence_missing` or `repo/evidence_invalid` rather than submitting a
report that claims untested coverage. See
[`examples/qa-scenarios.json`](examples/qa-scenarios.json).

```json
{
  "schema": "agent-qa-scenarios/1",
  "title": "Playback regression sweep on build 4821",
  "notes": "Optional free text carried into the report.",
  "scenarios": [
    {
      "id": "playback-resumes-after-network-loss",
      "description": "Resume position survives a dropped connection.",
      "result": "failed",
      "producer": "playwright@1.55 chromium 129",
      "started_at": "2026-08-17T09:00:00Z",
      "finished_at": "2026-08-17T09:02:11Z",
      "evidence": [
        { "path": "evidence/playback-resume.webm", "captured_at": "2026-08-17T09:01:40Z" },
        "evidence/console.log"
      ]
    }
  ]
}
```

- `result` is one of `passed`, `failed`, `blocked`, `skipped`.
- `producer` must name what captured the evidence, with version where it matters.
- Evidence paths are relative to the attempt directory; absolute paths and `..`
  are rejected.
- Only a `skipped` scenario may carry no evidence.
- The tested revision, build, and environment come from the contract and are
  written into the manifest, not repeated per scenario.
- `format` and `sha256` are derived per file; `redaction` is `scanned-clean`,
  `unscanned-binary`, or `secret-detected`.

## 4. Triage to result

```text
triage → dispatch → provision → execute → prove → submit → reconcile → complete
                        └──────────── recover → retry | escalate | fail ─────┘
```

| Stage | Input | Output | Owner | Terminal condition |
| --- | --- | --- | --- | --- |
| triage | request | typed task: result type, acceptance criteria, submission target, authority | runner | task has a result type and acceptance criteria |
| dispatch | typed task | attempt identity plus the environment in §1 | runner | `AGENT_TASK_ID` and `AGENT_ATTEMPT_ID` exported |
| provision | attempt identity, machine identity | ready workspace; `bootstrap.json` | runner allocates, repository validates via `pnpm bootstrap` | exit 0, or `runner/*` / `repo/*` failure |
| execute | ready workspace, task | code changes (implementation) or captured evidence (qa) | agent, inside enforced boundaries | work complete, or blocked with a diagnostic |
| prove | changed tree or captured evidence | `manifest.json`, `logs/`, `evidence/` | repository via `pnpm verify` | exit 0 with `outcome: passed`, or non-zero with a `failure_class` |
| submit | passing manifest | PR on `agent/<task>/<attempt>`, or a qa report | runner mechanics, repository schema and target rules | submission acknowledged by the provider |
| reconcile | submission plus external state | merged (only with merge authority), or accepted report | runner | required checks and reviews resolved, or feedback returned to execute |
| complete | reconciled result | terminal state and preserved artifacts | runner | `pnpm teardown` clean and terminal state recorded |
| recover | failure classification | retry, escalation, or failure | runner decides, repository classifies | budget exhausted or the failure repeats unchanged |

`pnpm doctor` is callable before any stage that drives the workspace, and again
after anything surprising. It never repairs; it reports the first gap and its
owner.

## 5. Terminal conditions and exit codes

Every entrypoint uses the same codes:

| Code | Meaning | Orchestrator action |
| --- | --- | --- |
| `0` | stage succeeded and cleanup was clean | continue |
| `1` | repository-owned failure | apply the retry rules below |
| `2` | runner-owned failure | do not retry blindly; fix provisioning or escalate |
| `3` | the work succeeded but cleanup or absence verification did not | quarantine the workspace, do not reuse it |
| `130` | cancelled by signal; teardown already ran | retry if budget remains |

A failure never arrives as prose alone. Both stderr and the manifest carry
`failure_class`, `owner`, `retryable`, and `recovery`.

## 6. Failure taxonomy

Defined once, in `scripts/lifecycle/failure.mjs`.

| Class | Owner | Retryable |
| --- | --- | --- |
| `runner/missing_task_identity`, `runner/missing_identity`, `runner/invalid_identity` | runner | no |
| `runner/invalid_task_input`, `runner/invalid_result_type` | runner | no |
| `runner/invalid_submission_target`, `runner/invalid_authority` | runner | no |
| `runner/missing_target_revision`, `runner/toolchain_missing` | runner | no |
| `runner/workspace_not_writable`, `runner/network_denied` | runner | yes |
| `repo/contract_violation`, `repo/package_manager_mismatch`, `repo/missing_lockfile` | repository | no |
| `repo/dependency_install_failed`, `repo/verification_timeout`, `repo/teardown_failed` | repository | yes |
| `repo/verification_failed`, `repo/evidence_missing`, `repo/evidence_invalid` | repository | no |
| `repo/secret_exposure`, `repo/unreleasable_resource` | repository | no |
| `internal/unknown` | internal | yes |

## 7. Retry, escalation, recovery

- Retry only when `manifest.retryable` (or the stderr `retryable`) is true and
  `AGENT_ATTEMPT_NUMBER` is below `AGENT_RETRY_BUDGET`.
- Each retry gets a **new** `AGENT_ATTEMPT_ID`, so evidence from the failed
  attempt is never overwritten.
- `pnpm bootstrap` and `pnpm teardown` are idempotent; a retry re-runs both
  safely. `teardown.json` accumulates one record per run.
- Escalate to a human when the same `failure_class` repeats unchanged across
  attempts, when the class is non-retryable, or on exit code `2` or `3`.
- `repo/secret_exposure` escalates immediately and the affected artifacts must
  not be uploaded.
- After process loss, the artifact directory is the durable record: read
  `manifest.json` for the last known outcome, `owned-resources.jsonl` for
  outstanding resources, and run `pnpm doctor` to decide whether the workspace is
  still drivable. `pnpm run own list` prints anything still held.
- Escalation preserves the last diagnostic, the owned-resource ledger, the
  teardown record, and all evidence.

## 8. Resource ownership

Anything the attempt raises — process trees, containers, simulators, emulators,
virtual machines, services, temp directories — is recorded before it is driven:

```bash
pnpm run own record -- --kind process_group --id "$PID" --pgid "$PID" --label dev-server
pnpm run own record -- --kind container --id "$CID" --runtime docker
pnpm run own record -- --kind vm --id builder-1 \
  --release '["vmctl","stop","builder-1"]' --verify '["vmctl","is-stopped","builder-1"]'
```

Rules the implementation enforces:

- Resources absent from the ledger are treated as pre-existing and preserved.
- `preexisting-resources.json` is snapshotted before acquisition; anything in it
  is never released, even if it also appears in the ledger.
- Release happens by recorded id or owned process group, never by process name.
  There is no `killall`, no "stop all containers", no `simctl shutdown all`.
- Absence is verified after release; the release command's exit status is not
  trusted.
- Kinds without a built-in handler must carry `--release` and `--verify` argv,
  otherwise recording fails with `repo/unreleasable_resource`.
- A persistent resource that is intentionally handed off must be reported with
  its ids and `pnpm teardown` as the release command.

`pnpm verify` releases the process tree it raised on success, failure, timeout,
and signal. `pnpm teardown` releases everything the attempt still holds:

```bash
pnpm run teardown -- --reason success
pnpm run teardown -- --reason failure   --primary-status 1
pnpm run teardown -- --reason cancelled --primary-status 130
pnpm run teardown -- --reason timeout   --primary-status 124
pnpm run teardown -- --reason retry
```

With `--primary-status` non-zero, the primary status is preserved and the cleanup
failure is reported separately. With a clean primary status, an incomplete
cleanup exits `3`.

## 9. Repository versus runner ownership

| Concern | Repository owns | Runner owns |
| --- | --- | --- |
| Toolchain | `packageManager` pin, `engines`, `pnpm bootstrap` | OS, node, corepack, git, caches |
| Credentials | required variables, noninteractive consumption, no persistence | machine-identity authentication, injection, rotation, revocation |
| Workspace | artifact layout, ledger, teardown, absence verification | isolated filesystem, compute, resource allocation |
| Network | declared destinations, useful denied-access classification | egress policy and enforcement |
| Result submission | manifest and scenario schemas, allowed target and branch rules | provider credentials, queueing, upload, retry, reconciliation |
| Isolation | task-and-attempt-keyed paths, refs, and ledgers | one workspace per attempt, no shared mutable state |

Concurrency: two attempts collide only through shared runtime resources the
runner allocates. Everything this repository writes — artifact directory, ledger,
snapshot, teardown record, allowed branch — is keyed by task and attempt.

## 10. What this repository deliberately does not do

- It does not implement, embed, or call an orchestrator.
- It does not authenticate to a provider, push a branch, open a pull request,
  upload an artifact, or submit a report. It produces the manifest and evidence
  the runner submits.
- It does not print, persist, or copy a secret value.
