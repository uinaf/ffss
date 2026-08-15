# Vision: the factory spine

Status: agreed direction, 2026-08-14. This document describes the target
state. [AGENT_INTERFACE.md](AGENT_INTERFACE.md) and the installed binary
describe what exists; when they disagree with this document, they win until
the tracker — [epic #25](https://github.com/uinaf/slopshipper/issues/25) and
its children — says otherwise.

## North star

`slopshipper` becomes the spine of an agentic software factory: a
harness-independent work ledger, state machine, and router that turns
approved task contracts into delivered, independently reviewed, babysat,
QA-gated changes — with humans holding release, recovery, and merge
authority. It is not tied to any vendor's agents, reviewers, or
infrastructure: environments bind their own tools into its roles.

```text
plan source (issue, spec, planning session)
      ▼
intake      task contract: units, acceptance criteria, complexity,
            risk tier, budget · optional plan-review gate
      ▼
release     human latch — a terminal today, an async approval tomorrow
      ▼
dispatch    route → venue: current session, local worktree, remote lease,
            provider cloud agent, sandbox
      ▼
build → verify (binary-executed) → review (bound reviewers)
      ▼
deliver     change request on the repo's forge — template, risk statement,
            verification evidence, media — via bound delivery conventions
      ▼
babysit     spine observes CI, review threads, head moves → events → rework
      ▼
qa          bound QA providers; classed evidence; never a synthetic pass
      ▼
learn       telemetry, route calibration, optional memory export
```

The current machine implements the middle of this pipeline for one
interactive session. The conversion grows the edges: where work runs, what
happens after delivery, and how claims are checked.

## Thesis

**From narrated evidence to observed evidence.** Today `verify` is the only
transition the binary proves itself; review and delivery evidence are
assertions by the driving agent. The factory version observes forges, CI,
and reviewers directly and records what it saw. Every gate moves toward
evidence the binary fetched or executed, away from evidence it was told.

**Trust before scale.** More parallel coding only grows the queue of
untrusted work. The stages that manufacture trust — observed evidence,
babysitting, QA gates, risk-tiered review — land before parallel dispatch.

## Principles

1. **The binary owns the runtime.** State machine, schemas, store, registry,
   routing policy, and observation live in Go with tests. Skills and drivers
   stay thin. No second runtime in markdown, ever.
2. **The protocol is the product.** Any shell-capable agent — regardless of
   harness or vendor — must drive a full run using `status --json`,
   `schema`, and stdin payloads alone. `next_action` contains executable
   commands, never harness idioms. Per-harness drivers are generated
   packagings of one source; a repo `AGENTS.md` block is the most portable
   driver of all. Harness conformance is documented and tested, not assumed.
3. **Roles, not vendors.** The machine defines gates, roles, and evidence
   shapes. Per-repo profiles bind roles to implementations: reviewers, QA
   providers, venues, memory sinks, forge. A human plus recorded evidence is
   the universal binding of last resort. A required role with no binding
   fails closed. Detection may propose bindings; a human confirms them.
4. **Forge independence.** The state machine and schemas never assume one
   forge. Observation goes through a small adapter seam: change-request URL,
   head SHA, checks state, review threads, mergeability. GitHub is the first
   adapter; others are added when a repo that needs them registers.
5. **Deterministic orchestration.** Queues, polling, policy, routing, and
   escalation are code. Models run only at reasoning steps: building,
   reviewing, judging, clarifying.
6. **Escalation-ladder economics.** Routes start cheap by default, escalate
   on rework evidence, and demote when a task class keeps passing. Parallel
   attempts are a deliberate spend for high-uncertainty work behind judge
   gates, not a default.
7. **Risk-tiered autonomy.** Lights-out flow is earned per tier and per
   repo, expanded by accumulated evidence. High-risk work always ends at an
   evidence-rich human moment. There is no unconditional autopilot.

## Nouns

| Noun | Meaning |
| --- | --- |
| Task contract | Released intake: units with acceptance criteria and complexity, run-level risk tier and budget |
| Run / unit / attempt | Ledger records. Units latch independently — one builds while another awaits checks. Attempts are parallel tries at one unit, resolved by a judge gate |
| Gate | An abstract requirement on a transition: plan review, verification, independent code review, QA, delivery verification |
| Profile | A repo's bindings from roles to implementations, plus policy: trust tier, canonical verify command, delivery policy, readiness verdict |
| Venue | Where a unit executes: current session, local worktree, remote lease, provider cloud agent, sandbox |
| Route | (venue, harness, role→model map, parallelism, review depth, budget), resolved per unit from tier and profile |
| Ledger | The SQLite event log, extended with duration, cost, and route outcome per transition |

## Gates

- **Plan review** — an independent reviewer challenges the task contract
  before release: outcome, acceptance criteria, slicing. Optional per
  profile; the cheapest defect removal in the pipeline.
- **Verification** — binary-executed commands with digested output. The
  canonical verify command comes from the profile, so agents stop guessing
  how a repo proves itself.
- **Independent code review** — one or more bound reviewers with normalized
  verdicts: a hosted review bot, a forge- or CI-resident reviewer, a
  second-model review CLI, a human.
- **QA** — bound providers exercise the product. Evidence is classed:
  automated check, browser, simulator/emulator, physical device, manual. A
  required surface that is unavailable yields `blocked` or `incomplete`,
  never a synthetic pass.
- **Delivery verification** — the change request is observed on the forge
  (exists, head SHA matches the delivered work) before the ledger accepts
  it.

## Human moments

Release, decisions, recovery, and merge stay human. They are CLI verbs, so
any notifier can front them; the target is asynchronous approval — a
notification with structured choices — so runs progress without an open
terminal. Rising autonomy tiers reduce how many human moments a run has,
never whether a required one happens.

## Delivery

Change requests carry the repository's template, a risk statement,
verification evidence, and attached media, and may land as stacked
revisions. These conventions live in bound driver skills and companion
tools, not in the binary: the spine records the outcome and verifies it
against the forge. Babysitting then owns the tail: observed check failures,
review feedback, and head moves become events that pull the unit back
through rework until the change request is genuinely merge-ready.

## The orchestrator

Routing is task-level, not request-level. Each released unit resolves to a
route: venue, harness, role→model map, parallelism, review depth, budget.

**Venues** sit behind one executor seam. Worker adapters are
(harness, headless invocation, model configuration) tuples, so the same
contract runs under an interactive session, a local worktree worker, a
remote lease, a provider's cloud agent, or a sandbox — whichever the
profile allows and the route selects.

**Parallelism has three shapes.** Independent units proceed concurrently
under their own latches. Parallel attempts race on one unit and end at a
judge gate that selects on evidence. Cross-harness best-of-N attempts the
same contract under different harness and model stacks — comparative
evidence no single-harness workflow can produce.

**Sub-agents stay harness-internal.** The route sets a budget envelope; the
driver translates it into its harness's idiom. The binary orchestrates
units and attempts; it never instructs a harness to spawn anything.

**Policy is a table, not a vibe.** A versioned, per-repo-overridable table
maps tier to route and spends no tokens itself. Rework escalates the route;
sustained clean passes demote a task class; the ledger and an eval harness
calibrate the table over time. A cheap classifier may eventually propose
tiers at intake — the table still decides, and the release moment corrects
mistakes for free.

slopshipper is not an LLM gateway: it never proxies model traffic and never
holds model keys. Harness adapters keep their own model configuration;
request-level gateways can sit under a harness without the spine knowing.

## Telemetry and learning

Every transition can carry duration, cost, and the route actually used.
Babysit observation records time-to-green, so CI speed stops being
folklore: slow pipelines and flaky checks become named, measured drag on
the factory. Run outcomes — blockers, decisions, review findings, route
results — export to a bound memory sink when the profile has one, so later
runs inherit what earlier runs learned. `serve` grows from a single-run
projector into a read-only fleet dashboard across repos and runs; the
projector rule is unchanged — workflow changes go through the CLI.

## Example profile

Roles bind to whatever an environment has: a hosted review bot or a
CI-resident reviewer, a device lab or a plain e2e suite, a memory service
or nothing at all, one laptop or a fleet of leased runners. The
maintainer's own profile doubles as the reference integration:

| Role | Binding |
| --- | --- |
| Spine | slopshipper |
| Review | [slopzapper](https://slopzapper.uinaf.dev), [slopguard](https://github.com/uinaf/slopguard) |
| QA | unbound today; slopscouter is the planned binding |
| Remote execution | [crabbox](https://github.com/openclaw/crabbox) |
| Readiness audits | agent-readiness skill |
| Delivery media | attach CLI |
| Memory | [Hindsight](https://github.com/uinaf/hindsight) (optional sink) |
| Evals / calibration | [slopbench](https://github.com/uinaf/slopbench) |

A different environment might bind Copilot code review or GitLab Duo as the
reviewer, its existing e2e suite as QA, and no memory sink — the spine does
not change.

## Non-goals

- Embedding a review engine, QA engine, or memory store in the binary.
- Embedding delivery conventions — templates, media hosting, stack
  management stay bound tools.
- An LLM gateway, model proxy, or key custody of any kind.
- A resident platform daemon, webhook receiver, or hosted control plane in
  this arc; the local-first CLI and its contracts come first, and any hosted
  front is separate later work on the same protocol.
- Unconditional autonomy or synthetic passes for unavailable surfaces.
- Porting harness-specific playbook libraries; behavior that matters becomes
  binary contract instead.

## Sequence

Detail lives in [epic #25](https://github.com/uinaf/slopshipper/issues/25);
this is the shape.

- **M0 — credibility.** Fix first-run initialization and install-path
  defects. The front door works before anything is built on it.
- **M1 — contract v2.** Task-contract intake, generalized gates with
  registered reviewer identities (plan review and QA ride the same
  registration), profiles and bindings, post-delivery unit states with
  independent unit latches, harness conformance contract.
- **M2 — observed evidence.** Transition telemetry, the forge adapter seam
  with a GitHub implementation, a `watch` verb turning forge observations
  into babysit events, verified review and delivery evidence, delivery
  conventions in bound driver skills.
- **M3 — dispatch and routing.** Venue and worker adapters from local
  worktrees outward to remote leases and provider cloud agents, router v1
  with escalation, attempts and judge gates for best-of-N, the fleet view
  in `serve`.
- **M4 — learn.** Route calibration from the ledger, memory export,
  eval-driven policy changes, asynchronous human moments, optional intake
  classifier.
