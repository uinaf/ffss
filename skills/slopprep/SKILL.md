---
name: agent-readiness
description: "Audit and improve repository and runner infrastructure for dependable autonomous implementation, QA, and unattended task execution. Covers reproducible bootstrap, noninteractive machine identities, real-surface verification, artifacts, CI, observability, isolation, recovery, and result submission. Use when making a repo agent-ready, agents cannot boot or verify, setup still needs a human, credentials or worktrees block automation, AGENTS.md lacks a cold-start path, runner setup is broken, automation still needs babysitting, or a devbox/orchestrator must finish tasks unsupervised. Do not use for reviewing an existing diff or documentation-only cleanup."
disable-model-invocation: true
---

# Agent-Readiness

Make a repository and its declared runner dependable for autonomous work.
Default target: B. Treat C as a checkpoint only ([references/grading.md](references/grading.md)).

## Boundaries

- In scope: readiness infrastructure and repeatable application QA.
- Out of scope: source-diff review, unrelated documentation cleanup, ship
  decisions, and unauthorized external actions.
- Do not invent an orchestrator. Grade platform tools, network access, and
  machine authentication as runner capabilities.
- Require task-relevant guidance and empirical proof; mock-only tests,
  documentation claims, and builder self-evaluation do not prove readiness.

## Readiness Model

Grade with [references/grading.md](references/grading.md). Report repository
grade, runner grade, and evidence level separately. The lowest applicable
capability sets each grade; never average away a blocker.

## Automation Path

For unattended work, walk
**Triage → Dispatch → Provision → Execute → Prove → Submit → Reconcile → Complete**,
with **Recover → Retry, Escalate, or Fail** from any nonterminal stage. Record
each stage's input, output, owner, and terminal condition. For no-diff tasks,
also name the result type, evidence, target, and allowed side effects.

## Workflow

### 1. Audit the declared execution boundary

Lock the request first: target grade (default B), task classes
(`implementation` / `qa` / both), and runner (`local` / `ci` / named devbox).
Then:

1. Read `AGENTS.md` (or the repo entrypoint) and follow its linked contracts.
2. Discover lifecycle commands and exercise what exists; record exit codes:

   ```bash
   rg -n 'bootstrap|verify|teardown|boot|smoke' AGENTS.md package.json Makefile Justfile scripts 2>/dev/null
   # run each discovered cold-start, boot, smoke, verify, and teardown command
   ```

   Static file presence alone is weak evidence.
3. Fill the [Required Output](references/grading.md#required-output) profile; for
   every applicable capability record its grade, evidence, gap, and owner.
4. For each automation-path stage, write `input / output / owner / terminal` or
   mark the stage missing.
5. Assign an evidence level using [references/autonomy-evidence.md](references/autonomy-evidence.md).

Use [references/setup-patterns.md](references/setup-patterns.md) for machine
identity ownership and managed-worktree inputs. Interactive human login,
profile switching, copied secrets, and printed tokens are runner gaps.

For React or existing Effect repos, apply
[references/react-enforcement.md](references/react-enforcement.md) and
[references/effect-readiness.md](references/effect-readiness.md). Effect
implementation task classes need repository-local Effect guidance; do not add
Effect solely for readiness.

### 2. Build the missing contract

Work in this order: **Legibility → Runner contract → Cold start → Real-surface
feedback → Enforcement → Isolation → Recovery and result submission → Repeated
trials**.

Reuse the repository's ordinary bootstrap, verify, and teardown commands — the
same surface humans and CI already use. Do not invent a parallel `agent-*`
script layer. If entrypoints are missing, add plain ones and wire them into
`package.json`, Make/`just`, or CI:

```bash
set -euo pipefail
trap './scripts/teardown.sh' EXIT
./scripts/bootstrap.sh
./scripts/verify.sh
```

Bootstrap validates prerequisites; verify is the CI-reused gate; teardown covers
success, failure, timeout, and cancellation. Name the declared commands in
`AGENTS.md`. Key automation artifacts by task and attempt.

| Enforce mechanically | Leave to agent judgment |
| --- | --- |
| workspace and branch setup, allowed targets, tool install, secret injection | task interpretation and implementation |
| boot, test, teardown, artifact manifests, upload and push mechanics | exploratory QA, diagnosis, evidence selection, and recovery strategy |

Keep `AGENTS.md` as the canonical agent guide; symlink `CLAUDE.md` → `AGENTS.md`.

### 3. Prove outcomes, not recipes

- Run the lifecycle once on the happy path and once with a safe failure
  (for example a missing required runner input); preserve both statuses and artifacts.
- Record each trial as machine-readable JSON:

```json
{"task_class":"qa","scenario":"missing-runner-identity","result":"expected_failure","human_interventions":0,"duration_seconds":12,"retries":0,"failure_class":"runner/missing_identity","artifacts":"artifacts/task-123/attempt-1/"}
```

- Grade final environment or repository state, not the agent's completion claim.
- Accept equivalent implementations that satisfy the contract.
- For B or A claims, repeat representative trials and aggregate success,
  intervention, duration, retry, resource, and failure metrics.
- Test parallel isolation and crash or stall recovery for unattended claims.
- Inspect transcripts and artifacts for false success, grader defects, secret
  exposure, and ambiguous requirements.

### 4. Finish at the requested outcome

Finish at the requested target or an evidenced blocker. Report the path to A
and relevant documentation drift without unrelated cleanup.

## Output

```text
- grades: repository and runner, before → after
- evidence: level plus the strongest exercised outcomes
- automation path: first missing or newly proven transition
- files changed: readiness infrastructure only
- remaining gaps: highest-impact gaps with owner, or none
- next: next capability or none
```

Name exact commands only for failures, reproduction, or when asked.

## References

- [references/grading.md](references/grading.md) — repository and runner grades, capability matrix, ceilings, and blockers
- [references/autonomy-evidence.md](references/autonomy-evidence.md) — evidence levels, representative trials, outcome graders, and reliability metrics
- [references/setup-patterns.md](references/setup-patterns.md) — bootstrap, gates, credentials, observability, isolation, artifacts, recovery, and result patterns
- [references/react-enforcement.md](references/react-enforcement.md) — React-specific lint, local-gate, and CI adoption
- [references/effect-readiness.md](references/effect-readiness.md) — Effect-specific source, runtime, test, observability, and cleanup proof
- [references/industry-examples.md](references/industry-examples.md) — current harness, eval, and orchestration patterns
