# Readiness Setup Patterns

Use only the section matching the missing capability. Adapt contracts to the
repository's existing tools; do not copy generic infrastructure into a repo.

## Lifecycle

Prefer one ordinary repository-owned surface used by humans, agents, and CI:

| Stage | Required outcome |
| --- | --- |
| bootstrap | validate prerequisites and install reproducibly without prompts |
| boot | start the real target and expose a bounded readiness signal |
| verify | run canonical guardrails plus the strongest cheap real surface |
| teardown | release owned processes, heavyweight runtimes, and state on success, failure, timeout, and cancellation |

Keep each stage noninteractive, idempotent where practical, bounded, and
explicit about whether a failure belongs to the repository or runner. Do not
invent parallel `agent-*` wrappers; wire missing entrypoints into package
scripts, Make/`just`, or checked-in scripts already used by CI.

### Runtime resource ownership

Apply the lifecycle to every resource a task raises: child processes, ports,
simulators, emulators, virtual machines, containers, browsers, services,
databases, and external test fixtures.

- Snapshot relevant state before acquisition and persist the exact resource ID
  plus task and attempt ownership.
- Release only resources created or acquired by that attempt. Preserve a device,
  container, service, or database that was already running.
- Register cleanup before the first later failure point and run it on success,
  failure, timeout, cancellation, and retry. Preserve the primary command's
  terminal status if cleanup also fails.
- Verify final state: no owned resource remains, and pre-existing resources are
  unchanged. Exercise this once after success and once after an injected safe
  failure.
- For intentionally persistent `run`, preview, or development tasks, emit the
  owned IDs and explicit teardown command as a handoff. Do not report the
  lifecycle complete until the resource is released or ownership is accepted.

Avoid broad cleanup such as `killall`, `simctl shutdown all`, deleting every
container, or stopping shared databases unless the command's declared scope
owns the entire target set.

Own tool versions once. Preserve the repository's runtime and package-manager
declarations, lockfile, catalogs, or tool manager. Make CI consume those owners
instead of copying literals into workflow files.

For proof selection and failure requirements, use
[verification-contract.md](verification-contract.md).

## Mechanical Enforcement

Put deterministic policy in the narrowest existing mechanical surface:

- formatter, linter, type checker, or compiler for source constraints
- schema or config validator for structured contracts
- test-framework extension, matcher, fixture, or reporter for behavioral checks
- build or task graph for dependency ordering and reusable command composition
- canonical local gate for pre-handoff proof
- CI or branch policy for unavoidable merge enforcement
- a tested module or local action in the repository's primary typed language
  when existing tools cannot express the rule

Adopt an existing linter or hook shape before adding another. Baseline noisy
checks before making them blocking. Error output should name the violated rule,
affected boundary, and recovery action when one exists.

Treat a new shell script as the last adapter, not the first implementation. It
may set strict process options and sequence a few established commands. Move to
the repository's typed language or an existing library as soon as the flow
parses JSON/YAML, branches on domain state, transforms data, retries, manages
concurrency, or needs unit tests. Never create a shell wrapper that only
duplicates a package script, task-runner target, or framework command.

## Machine Identity

Credentials are compatible with readiness when ownership is explicit:

- the runner authenticates a machine or workload identity and injects
  short-lived access
- the repository declares required scopes and consumes injected access
  noninteractively
- the operator provisions, rotates, revokes, and recovers identities outside
  normal task execution

Prefer separate roles for triage, test fixtures, artifact submission, delivery,
and production changes. Denied or missing scope must fail without printing
values and identify the recovery owner. Never embed bootstrap secrets, write
fetched secrets to artifacts, or switch a human profile during an unattended
run.

## Observable and Reproducible State

Expose the smallest machine-readable signal appropriate to the target:

- service: readiness plus structured request or job outcomes
- CLI or library: exit status, stdout/stderr contract, and consumer invocation
- UI: interactable runtime surface plus console or application diagnostics
- stateful system: reproducible fixtures and isolated write/read round trips

Use versioned seed data when empty state cannot exercise the real contract.
Keep diagnostics contextual and redacted. Do not require a health endpoint or
JSON logging where the repository's shipped surface provides a better signal.

## Isolation

Concurrent tasks must not collide through workspaces, branches, ports,
processes, databases, external fixtures, artifact paths, or result refs. Grade
collision freedom and cleanup, not one allocation algorithm.

When managed Codex or Claude worktrees need ignored local files,
`.worktreeinclude` may name a small explicit set already covered by
`.gitignore`. Do not include broad `.env*`, secret directories, caches,
dependencies, build output, or machine-global configuration. Manual worktrees
and custom hooks need their own copy or bootstrap path; do not assume the
managed-worktree mechanism runs there.

## Unattended Execution

An unattended path needs:

- task and attempt identity
- bounded runtime and retry policy
- durable logs, artifacts, and terminal state
- cleanup on every exit path
- scoped credentials and allowed side effects
- recovery or escalation ownership

The repository exposes ordinary lifecycle and artifact commands. A runner or
orchestrator owns workspace allocation, credentials, queueing, retries, and
external submission mechanics.

For triage-to-result work, define each stage's input, output, owner, and
terminal condition:

```text
queued -> provisioning -> executing -> proving -> submitting -> reconciling
       -> blocked | retrying | failed | canceled | complete
```

Implementation and no-diff QA are separate result types. A QA task must not
manufacture a branch or pull request unless its contract requests repository
changes. Every preserved artifact ties task, attempt, revision or build,
scenario, producer, capture time, format, and redaction status to an observed
outcome.

## Recovery and Back-Pressure

Retry only classified recoverable failures while a configured budget remains.
Treat repeated unchanged failure as an escalation signal. Preserve the last
diagnostic, owned state, attempted recovery, and next safe action.

Run targeted checks at the earliest reliable boundary. Keep successful hooks
quiet and failures concise; do not rerun commands merely to redisplay output.
Long or secret-bearing operations require non-cancellable critical sections or
durable reconciliation rather than optimistic process-exit handling.

## Maintained Examples

Use these public implementations only after identifying the missing contract:

- [`uinaf/workspace-kit` lifecycle scripts](https://github.com/uinaf/workspace-kit/blob/main/package.json),
  [installed-package smoke](https://github.com/uinaf/workspace-kit/blob/main/scripts/smoke-package.mjs),
  and [CI gate](https://github.com/uinaf/workspace-kit/blob/main/.github/workflows/verify.yml)
  demonstrate one package-owned local/CI lifecycle with consumer proof.
- [`uinaf/dotfiles` task graph](https://github.com/uinaf/dotfiles/blob/main/mise.toml),
  [check registry](https://github.com/uinaf/dotfiles/blob/main/scripts/verify/checks.json),
  and [runner](https://github.com/uinaf/dotfiles/blob/main/scripts/verify/run.ts)
  demonstrate deterministic domain selection, parallel checks, and failure
  aggregation for a heterogeneous repository.

Copy neither implementation wholesale. Inspect its current tests and guide,
then reuse only the contract that matches the target repository's existing
toolchain and lifecycle.
