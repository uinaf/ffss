# Readiness Setup Patterns

Use only the section matching the missing capability. Adapt contracts to the
repository's existing tools; don't copy generic infrastructure into a repo.

## Lifecycle

Build one ordinary repository-owned surface shared by humans, agents, and CI:

| Stage | Outcome |
| --- | --- |
| bootstrap | validate prerequisites and install reproducibly, no prompts |
| boot | start the real target and expose a bounded readiness signal |
| doctor | read-only check that the running instance is worth driving |
| verify | canonical guardrails plus the strongest cheap real surface |
| teardown | release owned processes, runtimes, and state on success, failure, timeout, and cancellation |

Keep each stage noninteractive, bounded, and idempotent where practical; each
failure says whether it belongs to the repository or the runner. No parallel
`agent-*` wrappers; wire missing entrypoints into package scripts,
Make/`just`, or checked-in scripts CI already uses.

### Doctor

Give each driven target one read-only "is this instance worth driving?" check:
process up, expected build, port owned by the right process, auth valid,
whichever contracts the target has. Run it before driving and again after
anything surprising. Doctor never mutates, repairs, or replaces bootstrap; it
reports whether the instance matches the contract and names the missing
capability when it does not. A plain repo-local script is a complete
implementation.

### Runtime resource ownership

The lifecycle applies to everything a task raises: process trees, ports,
simulators, emulators, virtual machines, containers, browsers, services,
databases, external fixtures. Release only what the attempt raised, by
recorded ID or owned process group; anything already running stays running.

- Persist each exact resource ID with task and attempt ownership, and register
  cleanup before the first later failure point.
- A cleanup or absence-verification failure after primary success fails the
  command; after primary failure, keep the original status and report the
  cleanup failure separately.
- Verify final state: nothing owned remains, nothing pre-existing changed.
  Exercise this once after success and once after an injected safe failure.
- A persistent `run`, preview, or development task hands off its owned IDs and
  teardown command; the lifecycle is not complete until the resource is
  released or ownership accepted.
- Broad cleanup (`simctl shutdown all`, deleting every container, stopping
  shared databases) only when the command's declared scope owns the entire set.
- Cleanup never eats the evidence: artifacts and logs survive teardown.

Own tool versions once: the repository's runtime and package-manager
declarations, lockfile, catalogs, or tool manager. CI consumes those owners
instead of copying literals into workflow files.

Proof selection and failure requirements:
[verification-contract.md](verification-contract.md).

## Mechanical Enforcement

Put deterministic policy in the narrowest existing mechanical surface:
compiler, linter, or type checker; schema validator; test-framework extension;
task graph for ordering; the canonical local gate; CI or branch policy only for
unavoidable merge enforcement. Git hooks are optional adapters to the gate,
never the owner of policy. Adopt an existing linter before adding another;
baseline noisy checks before making them blocking.

For TypeScript repos already linting with Oxlint, offer vendoring the
[dmmulroy/anti-slop](https://github.com/dmmulroy/anti-slop) rules: copy and
own the rule source, let the installed toolchain run it, baseline before
blocking. Don't introduce Oxlint just to carry them.

Shell is the last adapter, not the first implementation: strict process
options plus a few established commands. Once a flow parses JSON/YAML,
branches on domain state, transforms data, retries, manages concurrency, or
needs unit tests, move it to the typed language or an existing library. Never
write a shell wrapper that only duplicates a package script, task-runner
target, or framework command.

## Machine Identity

Credentials fit readiness when ownership is explicit: the runner
authenticates a machine or workload identity and injects short-lived access;
the repository declares required scopes and consumes them noninteractively;
the operator provisions, rotates, revokes, and recovers outside task
execution.

- Separate roles for triage, test fixtures, artifact submission, delivery, and
  production changes.
- A denied or missing scope fails without printing values and names the
  recovery owner.
- Never embed bootstrap secrets, write fetched secrets to artifacts, or switch
  a human profile during an unattended run.

## Observable State

Expose the smallest machine-readable signal the target already supports. Don't
demand a health endpoint or JSON logging where the shipped surface already
gives a better signal; use versioned seed data when empty state cannot
exercise the real contract.

## Isolation

Concurrent tasks must not collide: workspaces, branches, ports, processes,
databases, external fixtures, artifact paths, result refs. Grade collision
freedom and cleanup, not one allocation algorithm.

- Managed Codex or Claude worktrees may use `.worktreeinclude` for a small
  explicit set of ignored files already covered by `.gitignore`; never broad
  `.env*`, secret directories, caches, dependencies, build output, or
  machine-global configuration.
- Manual worktrees and custom hooks need their own copy or bootstrap path; the
  managed-worktree mechanism does not run there.

## Unattended Execution

An unattended path needs: task and attempt identity, bounded runtime and
retry policy, durable logs, artifacts, and terminal state, cleanup on every
exit path, scoped credentials and allowed side effects, and recovery or
escalation ownership.

The repository exposes ordinary lifecycle and artifact commands; a runner or
orchestrator owns workspace allocation, credentials, queueing, retries, and
external submission mechanics.

For triage-to-result work, give each stage input, output, owner, and terminal
condition:

```text
queued -> provisioning -> executing -> proving -> submitting -> reconciling
       -> blocked | retrying | failed | canceled | complete
```

Implementation and no-diff QA are separate result types; a QA task never
manufactures a branch or pull request unless its contract requests changes.
Tie every preserved artifact's task, attempt, revision or build, scenario,
producer, capture time, format, and redaction status to an observed outcome.

## Recovery and Back-Pressure

- Retry only classified recoverable failures while a configured budget
  remains; repeated unchanged failure is an escalation signal.
- Successful hooks stay quiet, failures stay concise; never rerun a command
  just to redisplay its output.
- Long or secret-bearing operations get non-cancellable critical sections or
  durable reconciliation, not optimistic process-exit handling.

## Maintained Examples

Once you know the missing contract, study two public implementations:

- [`uinaf/workspace-kit` lifecycle scripts](https://github.com/uinaf/workspace-kit/blob/main/package.json),
  [installed-package smoke](https://github.com/uinaf/workspace-kit/blob/main/scripts/smoke-package.mjs),
  and [CI gate](https://github.com/uinaf/workspace-kit/blob/main/.github/workflows/verify.yml):
  one package-owned local/CI lifecycle with consumer proof.
- [`uinaf/dotfiles` task graph](https://github.com/uinaf/dotfiles/blob/main/mise.toml),
  [check registry](https://github.com/uinaf/dotfiles/blob/main/scripts/verify/checks.json),
  and [runner](https://github.com/uinaf/dotfiles/blob/main/scripts/verify/run.ts):
  deterministic domain selection, parallel checks, and failure aggregation in
  a heterogeneous repository.

Copy neither wholesale. Inspect current tests and guides, then reuse only the
contract that matches the target repository's existing toolchain.
