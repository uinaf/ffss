# Preparing a Repository for Unattended Triage-to-Result Automation

## Problem/Feature Description

The team plans to connect this repository to a Symphony-style orchestrator.
Requests will be triaged into typed tasks and dispatched to isolated devbox
workspaces. The same system must support two result paths without babysitting:

- `implementation`: set up the environment, implement and verify a change, run
  independent review, reconcile the pull request, CI, and review state, and
  merge only when the task grants that authority
- `qa`: set up the environment, test an existing revision or build, capture real
  proof such as screenshots, images, recordings, logs, traces, JSON, and scenario
  outcomes, then submit and reconcile the QA report without manufacturing a PR

The devbox platform already provisions a scoped Infisical machine identity and
exports a short-lived `INFISICAL_TOKEN`, `INFISICAL_API_URL`, and the non-secret
`INFISICAL_PROJECT_ID`. It also supplies `AGENT_TASK_ID`, `AGENT_ATTEMPT_ID`, an
isolated workspace, and provider credentials authorized for the declared result
target. Humans own identity provisioning, rotation, revocation,
and emergency recovery. The repository must consume those capabilities without
interactive login, profile switching, copying secret files, or printing tokens.

The current repository incorrectly asks every agent to run `infisical login`,
create `.env.local`, choose a branch name even for QA, and watch external state
manually. Prepare the repository-side contract for the future orchestrator. Do
not build or call a real orchestrator, authenticate to external services, push a
branch, open a PR, upload artifacts, or submit a report.

## Output Specification

Produce:

1. `readiness-report.md` — Separate repository grade, runner grade, capability
   profile, and E0-through-E4 evidence level. Identify gaps and their owner.
2. `docs/agent-automation.md` — Define task input, result type, attempt identity,
   artifact and manifest layout, triage-to-result states, allowed submission
   target, terminal condition, retry and escalation rules, recovery behavior,
   and repository/runner ownership. Cover implementation and QA result paths.
3. `scripts/agent-bootstrap.sh` — A noninteractive, strict-mode bootstrap that
   validates the runner contract and machine identity without exposing secrets,
   installs dependencies reproducibly, and fails with actionable ownership.
4. `scripts/agent-verify.sh` — The canonical bounded verification entrypoint. It
   records inspectable outcome evidence and a machine-readable artifact manifest
   under a task-and-attempt-specific directory, and exits non-zero with a useful
   diagnostic when verification fails. QA evidence must identify the tested revision, build, environment,
   scenario, producer, capture time, format, and redaction status.
5. `scripts/agent-teardown.sh` — Idempotent cleanup suitable for success,
   failure, cancellation, timeout, and retry.

Document stable contracts and accept compatible orchestrator implementations;
do not prescribe an internal Symphony implementation.

## Input Files

=============== FILE: package.json ===============
{
  "name": "triage-worker-example",
  "private": true,
  "packageManager": "pnpm@11.15.0",
  "scripts": {
    "test": "node --test",
    "verify": "pnpm test"
  }
}

=============== FILE: scripts/current-agent-setup.sh ===============
#!/usr/bin/env bash
infisical login
echo "Copy the project secrets into .env.local, then run pnpm install"
echo "Create any branch you want and watch CI in the browser"

=============== FILE: README.md ===============
# Triage Worker Example

Run `scripts/current-agent-setup.sh`, then ask a maintainer what to do next.
