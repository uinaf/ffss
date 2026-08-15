---
name: agent-readiness
description: "Audit and improve repository guidance, lifecycle commands, and runner infrastructure for dependable autonomous work. Use when making a repo agent-ready, agent guidance lacks orientation or proof paths, agents cannot boot or verify, setup still needs a human, or a runner must finish tasks unsupervised. Do not use for reviewing an existing diff, an ordinary builder self-check in a healthy repo, or prose-only documentation cleanup."
---

# Agent-Readiness

Make a repository, its agent operating guide, and its declared runner dependable
for autonomous work.
Default target: B. Treat C as a checkpoint only ([references/grading.md](references/grading.md)).

## Boundaries

- In scope: agent operating contracts, readiness infrastructure, and repeatable
  application QA.
- Out of scope: source-diff review, ordinary post-change self-checks when the
  repository already has a usable proof path, prose-only documentation cleanup,
  ship decisions, and unauthorized external actions.
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
   Audit it with [references/agent-guidance.md](references/agent-guidance.md):
   orientation, progressive routing, authority, lifecycle, proof, recovery, and
   write-back must be discoverable without turning the entrypoint into a wiki.
2. Discover lifecycle commands and exercise what exists; record exit codes:

   ```bash
   rg -n 'bootstrap|verify|teardown|boot|smoke' AGENTS.md package.json Makefile Justfile scripts 2>/dev/null
   # run each discovered cold-start, boot, smoke, verify, and teardown command
   ```

   Static file presence alone is weak evidence.
3. Compare the guide's commands, paths, ownership, and restrictions with the
   checkout and declared runner. Treat stale or unexecutable guidance as a
   readiness failure, not merely a prose defect.
4. Fill the [Required Output](references/grading.md#required-output) profile; for
   every applicable capability record its grade, evidence, gap, and owner.
5. For each automation-path stage, write `input / output / owner / terminal` or
   mark the stage missing.
6. Assign the evidence level defined in
   [references/grading.md](references/grading.md). Read
   [references/autonomy-evidence.md](references/autonomy-evidence.md) only for
   representative trials, reliability claims, or A-grade operation.

Read [references/setup-patterns.md](references/setup-patterns.md) only for a
missing lifecycle, machine identity, observability, isolation, or unattended
runner contract. Interactive login, profile switching, copied secrets, and
printed tokens are runner gaps. Do not introduce a framework or tool solely to
raise readiness; verify the repository-selected stack instead.

### 2. Build the missing contract

Work in this order: **Legibility → Runner contract → Cold start → Real-surface
feedback → Enforcement → Isolation → Recovery and result submission → Repeated
trials**.

Reuse the repository's ordinary bootstrap, verify, and teardown commands — the
same surface humans and CI already use. Do not invent a parallel `agent-*`
script layer. If entrypoints are missing, extend the existing build manifest,
task graph, compiler/linter/test framework, or typed project CLI. A shell file
is not required merely to give the command a name.

Bootstrap validates prerequisites; verify is the CI-reused gate; teardown covers
success, failure, timeout, and cancellation. Name the declared commands and
their proof boundary in `AGENTS.md`. Key automation artifacts by task and
attempt.

Treat heavyweight runtime resources as owned lifecycle state: simulators,
emulators, virtual machines, containers, browsers, services, databases, and
similar runners. Snapshot pre-existing state, record exact resource IDs, and
release only resources raised by the current task or attempt on every exit path.
Treat a launcher and its descendants as one owned process tree when applicable.
Preserve resources that were already running. Verify the final state after a
successful run and an injected failure. If ordinary work succeeded but teardown
or absence verification fails, return non-zero; otherwise preserve the primary
failure or signal status while reporting the cleanup failure. A persistent
`run` or development task must hand off its resource IDs and explicit teardown
command instead of silently leaking them.

| Enforce mechanically | Leave to agent judgment |
| --- | --- |
| workspace and branch setup, allowed targets, tool install, secret injection | task interpretation and implementation |
| boot, test, teardown, artifact manifests, upload and push mechanics | exploratory QA, diagnosis, evidence selection, and recovery strategy |

Keep `AGENTS.md` as the canonical shared guide; symlink `CLAUDE.md` →
`AGENTS.md`. Keep shared guidance model-neutral. Put private human context in an
owner-controlled layer when the workspace supports one, and keep model or
harness tuning in its owning configuration.

### 3. Prove outcomes, not recipes

- Use [references/verification-contract.md](references/verification-contract.md)
  to distinguish deterministic guardrails, focused regression checks,
  real-surface runtime proof, CI, and live or deploy evidence. One layer never
  implies another.
- Run the lifecycle once on the happy path and once with a safe failure
  (for example a missing required runner input); preserve both statuses and artifacts.
- Confirm the failure path exits non-zero and surfaces a stable classification,
  useful context, and a recovery action when the operator can act.
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
- [references/agent-guidance.md](references/agent-guidance.md) — model-neutral AGENTS.md orientation, routing, authority, lifecycle, and human-context checks
- [references/verification-contract.md](references/verification-contract.md) — canonical gates, proof layers, real-surface evidence, and failure quality
- [references/autonomy-evidence.md](references/autonomy-evidence.md) — evidence levels, representative trials, outcome graders, and reliability metrics
- [references/setup-patterns.md](references/setup-patterns.md) — lifecycle, credentials, observability, isolation, unattended execution, and recovery patterns
