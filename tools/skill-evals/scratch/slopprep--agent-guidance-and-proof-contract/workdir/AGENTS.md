# AGENTS.md

Private engineering workspace owned by Maya Chen. Canonical guide for every
agent and harness in this repo; `CLAUDE.md` should symlink here rather than
restate it.

## Orientation

- The workspace builds developer tools for small teams. The shipped surface is a
  local HTTP service plus the packages behind it.
- Product implementation belongs under `packages/`. Nothing product-facing lives
  in `scripts/` or `docs/`.
- Task context comes from the request plus the routes below. If acceptance
  criteria are not stated and cannot be derived from `docs/decisions/`, ask
  before implementing.
- Owner compass: smallest complete useful result, direct evidence, composable
  parts. Recurring failure mode to resist — exploring abstractions instead of
  shipping. Decision rule: ship the narrowest working slice; record a proposed
  abstraction in `docs/decisions/` instead of building it speculatively.

## Routing

| When | Read |
| --- | --- |
| placing a change, lifecycle commands, smoke surface | `docs/workspace.md` |
| a cross-package or architectural choice already settled | `docs/decisions/` |
| a runtime failure or incident recurrence | `ops/log.md` |
| owner priorities and collaboration style | `OWNER.md` |
| identity, finance, or personal detail a task genuinely needs | `private/` |

Open the smallest source the task needs. Do not preload `private/`.

## Authority

Allowed without asking: reversible edits under `packages/`, `docs/`, and
`scripts/`; running the lifecycle commands below; reading anything outside
`private/`.

Ask first: destructive or irreversible operations, anything outside this
checkout, network calls to third parties, pushing, publishing, opening a PR,
rewriting history, and reading `private/`.

Credentials are runner-injected and consumed from the environment. Never create,
paste, guess, or commit a credential, and never print one into logs or
artifacts. Missing credentials or scope are a runner gap — stop and name it
rather than working around it.

This runner is local-only. There is no CI and no deploy access from here, so no
claim may extend to CI, staging, or production.

## Lifecycle

Run these in order; they are the same commands humans and CI use. Do not add a
parallel agent-only script layer.

```bash
./scripts/bootstrap.sh   # validate prerequisites and install; no prompts
./scripts/boot.sh        # start the local service (foreground)
./scripts/verify.sh      # canonical gate: typecheck, tests, health smoke
./scripts/teardown.sh    # release what this task started
```

Two constraints the scripts do not enforce for you:

- `verify.sh` ends with a health request against `http://127.0.0.1:4317/health`.
  It does not boot the service, so boot first and confirm the port is answering.
- `teardown.sh` matches on process name. Before running it, confirm no unrelated
  `node server.js` process is running that you did not start; preserve anything
  that was already up.

Record the port and PID you started, and run teardown on success, failure,
timeout, and cancellation. A task that intentionally leaves the service running
must hand off the PID, the port, and the teardown command.

## Proof

`./scripts/verify.sh` exiting zero is the floor, not the finish line. Report
these layers separately and never let one stand in for another:

| Layer | Claim it supports |
| --- | --- |
| typecheck and tests | the checkout passes its own static and unit gates |
| focused regression | the changed behavior plus one adjacent failure ran |
| real surface | the booted service answered a representative request |
| CI / deploy | unavailable here — always report as unverified |

For any change touching a request path, config, or IO, exercise both a success
and a failure case on the booted service. Invalid configuration must exit
non-zero with a redacted `config/missing_value` diagnostic; if it exits zero or
leaks a value, that is the bug.

A passing unit test, a compile, or a screenshot does not prove a request path.
Quote the exact command, the observed response or exit code, and the final state
you inspected. Do not report work complete on a self-assessment: name the
surface exercised and the revision it ran against, and label anything you could
not exercise as unverified with the missing capability.

## Recovery

Classify a failure before retrying: `config`, `auth`, `transient`, `repo`, or
`runner`. Retry only transient work, bounded, at most twice. Two identical
failures is an escalation signal — stop, preserve the last diagnostic and any
owned state, and report the failed boundary plus the next safe action. Never
work around a gate or fabricate an input to get past one.

## Write-back

- Durable cross-package decisions → `docs/decisions/`.
- Runtime incidents and their resolution → `ops/log.md`.
- Corrections to this contract, including a command that has drifted from the
  checkout → edit this file in the same change.
- Everything else stays in the task report; nothing durable lives only in chat.
