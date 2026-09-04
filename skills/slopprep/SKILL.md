---
name: slopprep
description: "Make a repository, its agent guide, and its declared runner dependable for autonomous work. Use when agents cannot boot, verify, or finish unsupervised, or gates are slow or ad hoc; not for diff review or ordinary self-checks."
disable-model-invocation: true
---

# Slopprep

Make the declared repository and runner dependable for the requested task.
Default improvement target: B; C is a checkpoint. Inspection reports the current
state and gaps without expanding into repairs.

## Inspect before executing

1. Establish inspection or improvement authority, task classes (`implementation`,
   `qa`, or both), runner, and target grade. Read the repository guide and its
   task-relevant contracts; use [agent-guidance.md](references/agent-guidance.md)
   to check orientation, routing, authority, proof, and recovery.
2. Discover tracked task owners, including hidden CI configuration, and trace
   delegation through manifests and nested packages. A package `verify` may
   check only tooling while mise, Vite+, or another graph owns product proof.
3. Inspect applicable commands' coverage, prerequisites, credentials, cost, and
   state changes before execution. Read
   [setup-patterns.md](references/setup-patterns.md#runtime-resource-ownership)
   before starting or stopping resources. Discovery alone never authorizes
   bootstrap, live or paid checks, repairs, or broad teardown.
4. Run authorized cheap checks without asking again. For changed-code proof,
   use the owner's affected lanes; broaden when shared inputs, uncertain
   coverage, or repository policy require it. Do not repeat passing checks
   without new changes, failures, or unresolved concerns.
5. Record exercised outcomes and exit codes. Unsafe or unavailable checks are
   gaps, not failed executions. Grade repository, runner, and evidence separately
   with [grading.md](references/grading.md#required-output); the lowest applicable
   capability sets the grade.

## Improve within scope

Work from legibility and runner prerequisites through cold start, real-surface
feedback, enforcement, isolation, and recovery. Extend the existing manifest,
compiler, test framework, or typed CLI; no parallel agent-only wrapper or new
orchestrator. A plain repository script may be the complete solution.

Use the relevant contract:

- [verification-contract.md](references/verification-contract.md): proof matched
  to the change, real surfaces, failure quality, and honest reporting.
- [fast-portable-execution.md](references/fast-portable-execution.md): task graphs,
  affected selection, and cache correctness. Read before changing verification
  performance or CI selection; measure unchanged, relevant-change, warm-full,
  and cold-full paths.
- [setup-patterns.md](references/setup-patterns.md): missing lifecycle, doctor,
  resource ownership, machine identity, isolation, and recovery. Authentication
  and network access are runner capabilities; never copy or print secrets.
- [autonomy-evidence.md](references/autonomy-evidence.md): representative repeated
  trials, graders, and reliability claims. Read for E3/E4 or A-grade work.

Keep `AGENTS.md` the model-neutral guide to commands and proof boundaries;
normalize `CLAUDE.md` through agent-guidance's symlink-or-import rule. Keep
private human context and harness settings with their owners.

## Prove the claimed outcome

Exercise the changed lifecycle on success and a safe failure. Preserve primary
status, classified diagnostics, recovery instructions, and task/attempt artifacts.
Grade final state and side effects, not the agent's completion claim.

For unattended work, trace:
`triage → dispatch → provision → execute → prove → submit → reconcile → complete`.
Record input, output, owner, and terminal condition at each applicable stage,
including recovery to retry, escalation, or failure. No-diff QA declares its
result, evidence, target, and allowed side effects; it need not create a branch.

For repeated trials, record task class, scenario, result, human interventions,
duration, retries, failure class, and artifacts as JSON. Aggregate success,
intervention, duration, resource, retry, and failure metrics for autonomy claims;
exercise parallel isolation and crash/stall recovery where claimed.

## Finish

Report repository/runner grades before and after, evidence level and strongest
outcome, first missing automation transition, changed files, and remaining gaps
with owners. Give exact commands for reproduction or on request.

Stop at the requested result or an evidenced blocker. Source-diff review,
ordinary self-checks of an already-ready repo, prose-only cleanup, ship decisions,
and unauthorized submissions remain outside this skill.
