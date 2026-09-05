---
name: slopprep
description: "Inspect or improve repository and runner readiness for autonomous work, including boot, verification, and recovery."
disable-model-invocation: true
---

# Slopprep

Make the declared repository and runner dependable for the requested task.
Default improvement target: B; C is a checkpoint. Inspection reports the current
state and gaps without expanding into repairs.

## Establish scope and evidence

Identify the requested task classes (`implementation`, `qa`, or both), runner,
and target grade. Trace the relevant lifecycle through tracked task owners,
including hidden CI and delegated packages: a package `verify` may cover only
tooling while another task graph owns product proof.

Check prerequisites, cost, credentials, and state changes before unfamiliar
commands. Run authorized checks without asking again; inspection alone does
not authorize bootstrap, live or paid checks, repairs, or teardown. Read
[resource ownership](references/setup-patterns.md#runtime-resource-ownership)
before starting or stopping resources.

Grade repository, runner, and exercised evidence separately with
[grading.md](references/grading.md#required-output). Unavailable or unsafe
checks are gaps, not failed executions.

## Choose the relevant guidance

- Agent guide structure, routing, authority, and cross-model behavior:
  [agent-guidance.md](references/agent-guidance.md).
- Verification coverage, real surfaces, and useful failure evidence:
  [verification-contract.md](references/verification-contract.md).
- Slow gates, affected selection, or caching:
  [fast-portable-execution.md](references/fast-portable-execution.md). Measure
  unchanged, relevant-change, warm-full, and cold-full paths before optimizing.
- Boot, doctor, resource ownership, identity, isolation, or recovery:
  [setup-patterns.md](references/setup-patterns.md).
- Unattended workflow proof, repeated trials, or A-grade reliability claims:
  [autonomy-evidence.md](references/autonomy-evidence.md).

## Improve and finish

Extend the existing manifest, compiler, test framework, or typed CLI; a plain
repository script may be sufficient. Keep private human context and harness
settings with their owners; never copy or print secrets.

Continue authorized repairs through proof of the changed lifecycle on success
and a safe failure. Preserve primary status, classified diagnostics, recovery
instructions, and task/attempt artifacts. Use the owner's affected checks and
reuse passing proof until a change, failure, or concrete concern invalidates it.

Report repository/runner grades before and after, evidence level, strongest
outcome, first missing automation transition, and remaining gaps with owners.
Finish at the requested outcome or an evidenced blocker; readiness work alone
does not authorize shipping or submitting changes.
