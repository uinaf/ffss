# Agent-Readiness Grading

Grade repository capability, runner capability, and proof strength separately.
A single letter cannot honestly describe all three.

## Required Output

```text
repository: B
runner: B
evidence: E4
task classes: implementation B, scripted QA B, exploratory QA C
profile: legibility B, executability B, feedback B, safety A, durability B, scale C
first gap: scale — concurrent result reconciliation has not been exercised
```

- **Repository grade** covers the versioned checkout and the contracts it exposes.
- **Runner grade** covers the declared devbox, CI worker, or orchestrator environment.
- **Evidence level** records how strongly those grades were exercised. Use
  [autonomy-evidence.md](autonomy-evidence.md).

Use the lowest applicable capability as each headline grade. Show the full
profile so the minimum does not hide progress elsewhere. Mark a capability
`N/A` only with a concrete reason tied to the intended task classes.

## Common Grade Ladder

### F — Unavailable

The capability does not exist or the agent cannot access it.

### D — Human-dependent

A path exists, but it relies on undocumented state, interactive setup, copied
secrets, dashboard operation, a developer's live session, or manual recovery.

### C — Functional

A documented, noninteractive path works once in the declared environment and
surfaces useful failure context. The agent can make progress, but reliability,
coverage, or recovery remains limited. C is a checkpoint, not completion.

### B — Dependable

The path is reproducible, bounded, enforced, and exercised on representative
real surfaces. The agent can complete the intended task class unattended in the
declared environment and produce inspectable evidence. Unqualified readiness
work targets at least B.

### A — Operational

The path remains dependable across long-running, concurrent, and failure-prone
operation. It has durable state, scoped authority, recovery, empirical
reliability evidence, and a maintenance loop that turns failures into stronger
contracts and evals.

## Capability Matrix

### Legibility

- **F:** ownership, commands, or system intent cannot be discovered.
- **D:** essential context lives in chat, a wiki, or a person's memory.
- **C:** a short repository entrypoint maps agents to current commands and contracts.
- **B:** architecture, product intent, task acceptance, verification, and ownership
  are versioned, progressively disclosed, cross-linked, and checked for drift.
- **A:** recurring maintenance turns production failures, review feedback, and
  architectural drift into updated contracts or mechanical enforcement.

### Executability

- **F:** the target cannot install or boot.
- **D:** setup requires undocumented commands, mutable human profiles, manual env
  files, or interactive intervention during normal runs.
- **C:** a clean workspace can bootstrap, boot, become ready, and tear down through
  documented commands under an explicit runner contract.
- **B:** setup is pinned or owned, idempotent, cold-start tested, seedable, and
  produces actionable errors for missing repository or runner prerequisites.
- **A:** ephemeral and heterogeneous supported runners can self-provision or
  self-heal without depending on a named workstation or live user session.

### Feedback

- **F:** the agent cannot observe whether its work functions.
- **D:** only mocked checks, compilation, screenshots without interaction, or
  dashboard-only signals exist.
- **C:** a smoke or consumer check exercises a real process or shipped artifact.
- **B:** canonical local and CI gates cover key real flows, grade final state, and
  emit inspectable logs or artifacts with useful failure diagnostics.
- **A:** invariants, error paths, production-like or shadow evidence, performance
  bounds, and continuous telemetry close the verification loop.

### Safety

- **F:** required authority is unavailable or the normal path exposes broad secrets.
- **D:** agents depend on pasted credentials, shared human sessions, prompt-only
  restrictions, open-ended network access, or unclear destructive boundaries.
- **C:** required authority and destructive operations are documented and scoped;
  secrets are neither printed nor committed, and denied access fails usefully.
- **B:** the runner injects a scoped machine or workload identity noninteractively;
  sandbox, network, and approval policy enforce the declared task boundary.
- **A:** identities are least-privilege per workload or environment, auditable,
  revocable and rotatable, isolated across concurrent runs, and continuously checked.

### Durability

- **F:** failure loses work or leaves the environment unsafe.
- **D:** progress depends on a terminal staying open or a human noticing stalls.
- **C:** commands have bounded waits, cleanup, explicit artifact paths, and
  actionable terminal outcomes.
- **B:** task and attempt identity, idempotent setup, durable handoffs, retry caps,
  cancellation, and failure classification support unattended completion.
- **A:** crash and stall recovery is exercised; sessions, state, and evidence can
  be reconstructed without nursing a specific process or container.

### Scale

- **F:** even one agent collides with existing development or shared resources.
- **D:** concurrency requires humans to allocate ports, databases, branches, or accounts.
- **C:** one task runs safely in an owned workspace with explicit resource bounds.
- **B:** concurrent tasks isolate workspaces, process resources, test state, and
  result refs; CI, review, QA, and acceptance feedback can be consumed without babysitting.
- **A:** an orchestrator can route triage through dispatch, retries, result
  submission and reconciliation, back-pressure, terminal states, and cleanup
  with measured cost across implementation and evidence-producing task classes.

## Repository and Runner Ownership

Grade the owner of a failure, not whichever checkout the audit started in.

| Concern | Repository owns | Runner or platform owns |
| --- | --- | --- |
| Toolchain | version contract and bootstrap command | compatible OS, compute, base tools, caches |
| Credentials | required scopes and noninteractive consumption | machine authentication and secret injection |
| Workspace | setup, verification, teardown, artifact conventions | isolated filesystem and resource allocation |
| Network | declared destinations and useful denied-access behavior | network policy and egress enforcement |
| Result submission | artifact/report schemas and branch, PR, CI, review, or acceptance contract | provider credentials, queueing, upload, retry, reconciliation |

A pre-provisioned Infisical machine identity, OIDC workload identity, or scoped
CI token is positive runner evidence. It is not manual repository setup merely
because credentials are required. Human login or profile switching during each
run is a runner autonomy gap.

Examples:

- `repository B / runner D`: bootstrap is correct, but this workstation lacks
  the promised machine identity.
- `repository B / runner B`: the devbox injects a scoped identity and the repo's
  bootstrap consumes it without prompts or secret persistence.
- `repository D / runner B`: the runner is ready, but the repo still asks the
  agent to hand-create `.env` files or follow a wiki.

## Evidence Ceilings and Blockers

- Static inspection only (`E0`) cannot justify more than D.
- A single exercised command (`E1`) cannot justify more than C.
- Real success and failure paths (`E2`) can justify B repository readiness, but
  repeated autonomy claims remain provisional.
- A requires repeated representative trials (`E3`) and operational recovery or
  longitudinal evidence (`E4`) for the claimed task classes.
- No real process, shipped-artifact consumer, or final-state grader means
  Feedback cannot exceed C.
- Unsafe credential exposure, unbounded destructive authority, or an execution
  boundary that cannot be established blocks unattended readiness; do not hide
  it inside an average.
- A missing runner prerequisite in an ad hoc shell is a runner mismatch when the
  declared runner contract supplies it. Prove that contract on the real runner
  before claiming B or A end to end.

## Grading Rules

- Grade what agents can actually use, not what files suggest should work.
- Prefer cold-start execution over warm developer-machine evidence.
- Grade final state and side effects, not an agent's success message.
- Accept equivalent mechanisms that satisfy the capability; do not mandate Git
  hooks, a particular dead-code tool, worktrees, containers, or port algorithm
  without a repository-owned reason.
- Keep task classes explicit. A repo may be B for dependency updates, C for UI
  changes, and B for scripted QA while remaining D for exploratory device QA.
- Record the model, harness, runner, toolchain revision, and evidence date for
  empirical claims because capability and scaffolding drift over time.
