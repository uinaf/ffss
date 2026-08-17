# Agent-readiness audit — `legacy-inventory-api`

Audited: 2026-08-17. Checkout: `app/` as inherited (pre-change state).
Declared runner: clean local Node.js environment with network access for
package installation; no external credentials required.
Task classes assessed: `implementation` and `scripted QA`.

## Headline

```text
repository: D → D          (design targets C; evidence ceiling blocks the claim)
runner:     D              (declared contract not met in the audit session)
evidence:   E0             (static inspection + one trivial command)
task classes: implementation D, scripted QA D, exploratory QA D
profile (before): legibility D, executability D, feedback D, safety C, durability D, scale D
profile (after):  legibility D, executability D*, feedback D*, safety C, durability D*, scale D*
first gap: executability — no cold start has been executed, so nothing above D is provable
```

`*` = infrastructure now exists and is designed for C, but was never run in this
session. Under the grading rules, static inspection (E0) cannot justify more
than D, so the graded result does not move. The gap is evidence, not design.

## Evidence level and why it is E0

| Attempted | Result |
| --- | --- |
| `node --version` | `v26.7.0` — the only command that executed |
| `npm --version` | denied by the session permission layer |
| `npm install --no-audit --no-fund` | denied (also denied with the sandbox override) |
| `node scripts/init.mjs` | denied |
| `node -e "console.log('hello')"` | denied |
| `node --check scripts/init.mjs` | denied |

No dependency install, no boot, no test run, and no syntax check were possible.
Everything below that is not the `node --version` line is static evidence read
from the files. The lifecycle program added by this work is therefore
**unverified**: it has not been executed once, and its happy path, its failure
paths, and its teardown are all design claims.

## Repository capabilities

### Legibility — D (unchanged)

- **Evidence:** `app/README.md` is three lines: a title and "Run `npm start` to
  start the server." No `AGENTS.md`, no ownership, no routing, no proof path.
  `package.json:scripts` declares `start` and `test`; neither is described.
- **Gap:** an agent cannot learn what must not regress, what authority it has,
  which command proves a change, or where to write results back. The new
  `boot` script is discoverable only by reading the manifest.
- **Owner:** repository.

### Executability — D before, D after (design C, unproven)

- **Evidence (before):** `npm start` runs `app/src/index.js:20`, which calls
  `app.listen(PORT)` and prints `Server running on port ${PORT}`. That line is
  the only startup signal, it is prose on stdout, and it is printed by the
  listen callback with no error path — a failed bind surfaces as an uncaught
  exception. There is no lockfile (`package-lock.json` absent), no `engines`
  field, and no `.nvmrc`, so `express: ^4.18.2` and `jest: ^29.0.0` resolve to
  whatever the runner fetches that day. Nothing bounds startup and nothing
  reports readiness.
- **Change:** `app/scripts/init.mjs` plus `package.json:scripts.boot`
  (`node scripts/init.mjs`) add preflight, a bounded readiness poll, and
  teardown. `start` and `test` are preserved verbatim.
- **Gap:** the cold start `npm ci && npm run boot` has never been executed, so
  the C criterion ("a clean workspace can bootstrap, boot, become ready, and
  tear down through documented commands") is unmet as evidence. Install itself
  is still unpinned — B additionally requires a lockfile and an `engines` or
  toolchain declaration.
- **Owner:** repository (pinning, lifecycle), runner (network for install).

### Feedback — D before, D after (design C, unproven)

- **Evidence (before):** `app/test/items.test.js:1` imports
  `{ createItem, getItems }` from `../src/items`, and `app/test/items.test.js:3`
  calls `jest.mock('../src/items')`. That module does not exist — `app/src/`
  contains only `index.js`. The suite would fail to resolve at collection time;
  even if it resolved, both tests assert only that a mock returns what the test
  told it to return. They exercise no application code, so the two shipped
  routes `GET /items` and `POST /items` (`app/src/index.js:10`, `:14`) have zero
  coverage. This is the D criterion verbatim: "only mocked checks".
- **Change:** `boot` polls the real shipped route `GET /items` over HTTP and
  requires HTTP 200 with a JSON array body before reporting ready — a real
  process on a real surface, not a log-line match and not an invented health
  endpoint.
- **Gap:** unexecuted. Beyond that, nothing exercises `POST /items`, no CI
  config exists in the repository, and the broken Jest suite is still broken —
  `npm test` remains a red gate, not a usable one.
- **Owner:** repository.

### Safety — C (unchanged)

- **Evidence:** the application requires no credentials, reads only `PORT`
  (`app/src/index.js:3`), performs no filesystem or network writes, and stores
  items in an in-process array (`app/src/index.js:7`). No secrets are committed
  and none are printed. Destructive authority is effectively absent, so the
  boundary is narrow by construction rather than by declaration.
- **Change:** `init.mjs` signals only the process group it started, by recorded
  ID, never by process name, and preserves anything already running (it refuses
  a busy `PORT` with `runner/port_unavailable` instead of clearing it).
- **Gap:** the boundary is undeclared — no document states which operations an
  agent may take. C is the ceiling until authority is versioned.
- **Owner:** repository (declaration), runner (identity, if the API ever gains
  a datastore or an outbound dependency).

### Durability — D before, D after (design C, unproven)

- **Evidence (before):** `npm start` blocks forever with no bound, no
  cancellation, no cleanup, and no artifact path. `app/src/index.js` installs no
  `SIGTERM`/`SIGINT` handler, so an agent that starts the app has no defined way
  to stop it and no way to know whether it stopped. A crashed or orphaned
  process leaves port 3000 held with nothing recording who held it.
- **Change:** `init.mjs` bounds readiness with `READY_TIMEOUT_MS` (default
  30 000 ms, poll interval 250 ms), registers teardown before the first failure
  point, and runs it on success, failure, timeout, `SIGINT`, and `SIGTERM`. It
  escalates `SIGTERM` → `SIGKILL` after `STOP_TIMEOUT_MS`, then *verifies* final
  state (`process.kill(pid, 0)` plus a TCP connect to the port) rather than
  trusting the kill's exit status. Cleanup failure after a successful boot exits
  non-zero as `app/teardown_failed`; a signal preserves its own terminal status
  (`128 + signum`). Failures carry a stable class from a fixed taxonomy —
  `repo/*`, `runner/*`, `app/*` — with a recovery line, emitted as JSON lines.
- **Gap:** unexecuted; neither the success path nor an injected failure has been
  observed. No task/attempt identity and no durable artifact directory exist, so
  B is out of reach regardless of execution. In `--keep` mode child output goes
  to a tmpdir log file, which is a handoff, not a retained artifact.
- **Owner:** repository.

### Scale — D before, D after (design C, unproven)

- **Evidence (before):** `PORT` defaults to 3000 (`app/src/index.js:3`), so two
  concurrent agents collide on the port and on a developer's own running copy.
  Nothing allocates or records a port.
- **Change:** with `PORT` unset, `init.mjs` binds `:0` to reserve a free
  ephemeral port and passes it to the child, so parallel boots do not collide;
  the chosen port is reported in every log line and released at teardown.
- **Gap:** unexecuted. The reserve-then-release probe leaves a small TOCTOU
  window. Workspace, branch, and result-ref isolation are entirely absent, and
  there is no CI to consume feedback from.
- **Owner:** repository (port ownership), runner (workspace isolation).

## Runner capabilities — D

The declared runner is a clean local Node.js environment with network access for
package installation. **That contract was not met in the audit session.**

- **Met:** Node.js v26.7.0 is present and executable — well above what
  `express@4` and the ESM/`fetch`/`AbortSignal.timeout` usage in `init.mjs`
  require.
- **Not met:** the package manager could not be invoked at all (`npm --version`
  denied), so no install path exists here; and general Node execution was denied
  (`node -e`, `node --check`, `node scripts/init.mjs`), so no boot, test, or
  even syntax check could run.
- **Classification:** this is a runner mismatch, not a repository defect — the
  declared contract supplies package installation and this shell did not. Under
  the grading rules, a missing runner prerequisite in an ad hoc shell is graded
  against the runner.
- **Owner:** runner/platform. Recovery: run `npm ci` and `npm run boot` in the
  `app/` directory on a shell where npm is permitted, and attach the transcript.
- **Not a gap:** no credentials, machine identity, or secret injection are
  required by this repository, so those runner concerns are `N/A` for the
  assessed task classes.

## Automation path

| Stage | Input | Output | Owner | Terminal condition |
| --- | --- | --- | --- | --- |
| Triage | issue or task text | selected task class | missing — no acceptance contract in the repo | — |
| Dispatch | task | assigned attempt | missing — no runner contract versioned | — |
| Provision | clean checkout | installed deps | partial — `npm ci` works only with a lockfile, which does not exist | install exits 0 |
| Execute | provisioned workspace | running app | **new** — `npm run boot` (`--keep` for handoff) | `lifecycle outcome=ready` or non-zero |
| Prove | running app | readiness + route evidence | partial — `GET /items` polled; `POST /items` and `npm test` unproven | JSON `lifecycle` line |
| Submit | diff or report | branch, PR, or report | missing | — |
| Reconcile | CI/review result | merge or retry | missing — no CI configuration in the repository | — |
| Complete | reconciled result | terminal state | missing | — |
| Recover | failure | retry, escalate, fail | partial — classified failure + recovery string; no retry budget or escalation owner | — |

First missing transition: **Provision → Execute** cannot be proven end to end
until an install runs; **Submit** onward does not exist at all.

## Files changed

- `app/scripts/init.mjs` — new lifecycle program (preflight, port ownership,
  bounded readiness poll on `GET /items`, verified teardown, classified
  failures, JSON-line logs).
- `app/package.json` — added `"boot": "node scripts/init.mjs"`. `start` and
  `test` unchanged.

No application source was modified: no health endpoint was added, because the
shipped `GET /items` route is a stronger readiness signal than a new endpoint
built only for the checker.

## Required next action

Run, on a shell where the declared runner contract holds:

```bash
cd app && npm ci && npm run boot
mv node_modules/express /tmp/express-away && npm run boot; echo "exit=$?"   # injected failure
mv /tmp/express-away node_modules/express
```

The first command must print `"outcome":"ready"` and exit 0. The second must
exit 1 with `"error_kind":"repo/dependencies_missing"`. Those two transcripts
move evidence from E0 to E2 and let Executability, Feedback, Durability, and
Scale be claimed at C.
