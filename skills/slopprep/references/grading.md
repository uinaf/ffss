# Agent-Readiness Grading

Grade repository capability, runner capability, and proof strength separately.
One letter cannot honestly describe all three.

## Required Output

```text
repository: B
runner: B
evidence: E4
task classes: implementation B, scripted QA B, exploratory QA C
profile: legibility B, executability B, feedback B, safety A, durability B, scale C
first gap: scale; concurrent result reconciliation has not been exercised
```

- **Repository grade**: the versioned checkout and the contracts it exposes.
- **Runner grade**: the declared devbox, CI worker, or orchestrator
  environment.
- **Evidence level**: how strongly you exercised those grades:

| Level | Strongest evidence |
| --- | --- |
| E0 | static inspection only |
| E1 | one declared command exercised |
| E2 | real success and actionable failure paths |
| E3 | repeated representative tasks with outcome graders |
| E4 | longitudinal or parallel operation with exercised recovery |

Read [autonomy-evidence.md](autonomy-evidence.md) only when designing E3/E4
trials, graders, or reliability reporting.

The lowest applicable capability is the headline grade; show the full profile
so the minimum does not hide progress elsewhere. Mark a capability `N/A` only
with a concrete reason tied to the intended task classes.

## Common Grade Ladder

- **F, unavailable:** the capability does not exist or the agent cannot reach
  it.
- **D, human-dependent:** a path exists but leans on undocumented state,
  interactive setup, copied secrets, dashboard operation, a developer's live
  session, or manual recovery.
- **C, functional:** a documented, noninteractive path works once in the
  declared environment and surfaces useful failure context. Progress is
  possible; reliability, coverage, or recovery is not. C is a checkpoint, not
  completion.
- **B, dependable:** reproducible, bounded, enforced, exercised on
  representative real surfaces. The agent completes the intended task class
  unattended and leaves inspectable evidence. Unqualified readiness work
  targets at least B.
- **A, operational:** stays dependable across long-running, concurrent,
  failure-prone operation, with durable state, scoped authority, recovery,
  empirical reliability evidence, and a maintenance loop that turns failures
  into stronger contracts and evals.

## Capability Matrix

### Legibility

- **F:** ownership, commands, or system intent cannot be discovered.
- **D:** essential context lives in chat, a wiki, or someone's memory.
- **C:** a short repository entrypoint states purpose and ownership, then maps
  task-shaped routes to current commands and contracts.
- **B:** orientation, task acceptance, authority, lifecycle, verification,
  ownership, and write-back are versioned, progressively disclosed,
  cross-linked, and checked for drift.
- **A:** recurring maintenance turns production failures, review feedback, and
  architectural drift into updated contracts or mechanical enforcement.

### Executability

- **F:** the target cannot install or boot.
- **D:** setup needs undocumented commands, mutable human profiles, manual env
  files, or a human mid-run.
- **C:** a clean workspace can bootstrap, boot, become ready, and tear down
  through documented commands under an explicit runner contract.
- **B:** setup is pinned or owned, idempotent, cold-start tested, seedable,
  and errors name the missing repository or runner prerequisite.
- **A:** ephemeral and heterogeneous supported runners self-provision or
  self-heal without a named workstation or live user session.

### Feedback

- **F:** the agent cannot observe whether its work functions.
- **D:** only mocked checks, compilation, screenshots without interaction, or
  dashboard-only signals.
- **C:** a smoke or consumer check exercises a real process or shipped
  artifact.
- **B:** canonical local and CI gates cover key real flows, grade final state,
  and emit inspectable logs or artifacts with useful failure diagnostics.
- **A:** invariants, error paths, production-like or shadow evidence,
  performance bounds, and continuous telemetry close the loop.

### Safety

- **F:** required authority is unavailable, or the normal path exposes broad
  secrets.
- **D:** pasted credentials, shared human sessions, prompt-only restrictions,
  open-ended network access, or unclear destructive boundaries.
- **C:** authority and destructive operations are documented and scoped;
  secrets are neither printed nor committed; denied access fails usefully.
- **B:** the runner injects a scoped machine or workload identity
  noninteractively; sandbox, network, and approval policy enforce the declared
  task boundary.
- **A:** identities are least-privilege per workload, auditable, revocable and
  rotatable, isolated across concurrent runs, continuously checked.

### Durability

- **F:** failure loses work or leaves the environment unsafe.
- **D:** progress depends on a terminal staying open or a human noticing
  stalls.
- **C:** commands have bounded waits, cleanup, explicit artifact paths, and
  actionable terminal outcomes.
- **B:** task and attempt identity, idempotent setup, durable handoffs, retry
  caps, cancellation, and failure classification support unattended
  completion.
- **A:** crash and stall recovery is exercised; sessions, state, and evidence
  reconstruct without nursing a specific process or container.

### Scale

- **F:** even one agent collides with existing development or shared
  resources.
- **D:** concurrency needs humans allocating ports, databases, branches, or
  accounts.
- **C:** one task runs safely in an owned workspace with explicit resource
  bounds.
- **B:** concurrent tasks isolate workspaces, process resources, test state,
  and result refs; CI, review, QA, and acceptance feedback flow without
  babysitting.
- **A:** an orchestrator routes triage through dispatch, retries, result
  submission and reconciliation, back-pressure, terminal states, and cleanup
  with measured cost across implementation and evidence-producing task
  classes.

## Repository and Runner Ownership

Grade the owner of a failure, not whichever checkout you started the audit in.

| Concern | Repository owns | Runner or platform owns |
| --- | --- | --- |
| Toolchain | version contract and bootstrap command | compatible OS, compute, base tools, caches |
| Credentials | required scopes and noninteractive consumption | machine authentication and secret injection |
| Workspace | setup, verification, teardown, artifact conventions | isolated filesystem and resource allocation |
| Network | declared destinations and useful denied-access behavior | network policy and egress enforcement |
| Result submission | artifact/report schemas and branch, PR, CI, review, or acceptance contract | provider credentials, queueing, upload, retry, reconciliation |

A pre-provisioned Infisical machine identity, OIDC workload identity, or
scoped CI token is positive runner evidence, not manual repository setup.
Human login or profile switching during each run is a runner autonomy gap.

Examples: `repository B / runner D`: bootstrap is correct, this workstation
lacks the promised machine identity. `repository B / runner B`: the devbox
injects a scoped identity and bootstrap consumes it without prompts.
`repository D / runner B`: the runner is ready, but the repo still asks the
agent to hand-create `.env` files or follow a wiki.

## Evidence Ceilings and Blockers

- E0 justifies at most D; E1 at most C.
- E2 can justify B repository readiness; repeated-autonomy claims stay
  provisional.
- A requires E3 trials plus E4 recovery or longitudinal evidence for the
  claimed task classes.
- No real process, shipped-artifact consumer, or final-state grader caps
  Feedback at C.
- Unsafe credential exposure, unbounded destructive authority, or an
  execution boundary that cannot be established blocks unattended readiness;
  never hide it inside an average.
- A missing prerequisite in an ad hoc shell is a runner mismatch when the
  declared runner contract supplies it. Prove the contract on the real runner
  before claiming B or A end to end.

## Grading Rules

- Grade what you can actually run, not what the files claim works.
- Prefer cold-start execution over warm developer-machine evidence.
- Grade final state and side effects, not the agent's success message.
- Accept equivalent mechanisms; don't mandate Git hooks, a particular
  dead-code tool, worktrees, containers, or a port algorithm without a
  repository-owned reason.
- Keep task classes explicit: a repo may be B for dependency updates, C for UI
  changes, B for scripted QA, and D for exploratory device QA all at once.
- Record model, harness, runner, toolchain revision, and evidence date for
  empirical claims; capability and scaffolding drift.
