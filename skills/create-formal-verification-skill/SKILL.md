---
name: create-formal-verification-skill
description: "Generate a repo-local formal-verification skill and its first models: bounded model checks or formal specs for races, retries, state machines, and protocols that tests rarely reach. Use when setting one up, or when a concurrency, retry, or protocol bug escaped the tests."
---

# Create formal verification skill

Add model-checked proof for the invariants a test suite rarely reaches, and
a repo-local `formal-verification` skill that keeps it alive. Each model
ships with a mapping to code, stated bounds, a CI job, and a **negative
control**: a known bug the checker catches. Ordinary unit, property, and
device tests are out of scope.

## Steps

1. **Find candidates.** Read fix commits and incidents, flaky tests, retry
   and backoff code, locks, coroutines and actors, reducers and lifecycle
   enums, and cross-service or device-server protocols. Done when each
   candidate has: the invariant in one sentence, its code owner (file and
   symbol), why tests miss it, and a real or plausible bug that breaks it.
   Pick one to three by cost of failure. Ask the user only when two
   rankings would change the pick.
2. **Pick the checker.** Prefer one that runs on the real code; use a spec
   when the invariant spans processes, services, or time, or the language
   has no checker. Choices by stack, with run commands:
   [tools.md](references/tools.md). Done when each candidate has one checker
   and a reason.
3. **Model.** Follow [model-contract.md](references/model-contract.md):
   location, mapping table, bounds, and invariants. Done when every model
   variable and action maps to a code symbol.
4. **Negative control.** Reinsert the known bug as a variant the checker
   must reject with a counterexample on the named invariant. A parse error,
   timeout, or out-of-memory exit is not red. Done when the clean model
   passes and the variant fails for the stated reason.
5. **Wire CI.** One repo task runs every model and every variant, with the
   checker version pinned, a time bound, and a path filter on the models
   and mapped code ([model-contract.md](references/model-contract.md#ci)).
   Runs longer than a few minutes go to a scheduled job; pull requests run
   the small bound.
6. **Write the skill.** Template: [skill-template.md](references/skill-template.md).
   Add one proof-map row to the repo's verification skill or agent guide
   that points at it.
7. **Prove it.** Run the task locally, or where the repo routes heavy work.
   Record per model: checker version, bound, distinct states or depth,
   wall time, and the variant's counterexample. Done when the CI job runs
   both on the change request.

## Report

Per model: invariant, code owner, checker and version, bound, states and
time, negative-control result, CI job link. Then the skill tree and any
candidate left unmodeled, with its reason.
