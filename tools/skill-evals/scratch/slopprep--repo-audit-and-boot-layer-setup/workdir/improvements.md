# What changed, what is still missing

Target is B. What this change builds reaches for C, and C is a checkpoint — the
point where an agent can make progress, not where the repository is done.

## Addressed

**Executability — a bounded, noninteractive boot.** `npm run boot`
(`app/scripts/init.mjs`) validates prerequisites, starts the app through the
package's own `start` script, and returns a terminal answer within a deadline.
It does not replace `npm start`; it drives it.

**Feedback — a real readiness signal.** Readiness is `GET /items` answering
HTTP 200 with a JSON array, polled every 250 ms until `READY_TIMEOUT_MS`
(default 30 s). No health endpoint was added: the shipped route is real
evidence, whereas an endpoint written for the checker proves only that the
checker's endpoint works. `Server running on port …` on stdout is not treated
as readiness — it fires from the `listen` callback and says nothing about
whether a request can be served.

**Durability — teardown that is verified, not assumed.** Cleanup is registered
before the first failure point and runs on success, failure, timeout, `SIGINT`,
and `SIGTERM`. Only the process group this attempt started is signaled, by
recorded ID, never by process name. After `SIGTERM` → `SIGKILL` escalation the
script checks final state (process gone *and* port not accepting) instead of
trusting the kill's exit status. A leak after an otherwise successful boot still
exits non-zero (`app/teardown_failed`); a signal preserves `128 + signum`.

**Durability — failures that can be acted on.** Every failure carries a class
from a fixed taxonomy (`repo/manifest_unreadable`, `repo/entrypoint_missing`,
`repo/dependencies_missing`, `runner/port_unavailable`,
`runner/start_command_unavailable`, `app/exited_early`,
`app/readiness_timeout`, `app/readiness_contract`, `app/teardown_failed`) plus a
recovery line, on JSON lines an orchestrator can parse. The `repo/*`
vs. `runner/*` split tells an agent whether to fix the checkout or escalate to
the machine.

**Scale — no port collision.** With `PORT` unset the script reserves a free
ephemeral port and hands it to the child, so parallel boots and a developer's
own copy coexist. An explicit `PORT` that is already busy fails as
`runner/port_unavailable` rather than fighting whoever owns it.

**Long-running work — an explicit handoff.** `npm run boot -- --keep` leaves the
app running and prints its pid, pgid, URL, log path, and the exact teardown
command, so a persistent instance is owned rather than leaked.

## Not addressed — and honest about it

**None of it has been executed.** The audit session could not run npm or node
scripts, so evidence stands at E0. Until one cold start and one injected failure
are run, C is a design claim. This is the single highest-priority item; nothing
below matters until it is done.

**`npm test` is broken.** `app/test/items.test.js` imports `../src/items`, which
does not exist. The suite cannot collect, and both tests only assert that a mock
returns its own configured value. There is currently no gate that would notice a
regression in `GET /items` or `POST /items`.

**No repository guidance.** There is no `AGENTS.md`, and `README.md` still says
only "Run `npm start`". Legibility stays at D: an agent cannot discover the
`boot` command, the authority boundary, or the proof path without reading the
manifest. This was left out deliberately — the requested output was the audit,
the lifecycle program, and the manifest change — but it is a real gap, owned by
the repository.

## Toward B

1. **Execute the lifecycle, both paths.** Happy path and an injected failure
   (remove `node_modules/express`); keep both transcripts. E0 → E2.
2. **Pin the toolchain.** Commit `package-lock.json`, add `engines.node`, so
   `npm ci` is reproducible instead of resolving `^` ranges per run.
3. **Make `npm test` real.** Delete the mock-only suite that references a
   missing module; replace it with an integration test that boots the app and
   exercises `POST /items` then `GET /items` round trip against the running
   process. Mock-only checks cannot lift Feedback past D.
4. **One verify entrypoint, reused by CI.** `npm run verify` = lint/type gate +
   tests + `boot` smoke. Add a CI workflow that runs exactly that command, so
   local and remote proof cannot drift.
5. **Write `AGENTS.md`** with orientation, routing, authority, the
   bootstrap/boot/verify/teardown commands, what proof each one does and does
   not give, and where results are written back. Symlink `CLAUDE.md` to it.
6. **Task and attempt identity.** Write logs and evidence to
   `artifacts/<task>/<attempt>/` so an unattended run's evidence survives the
   process.
7. **Add a `doctor` command** — read-only "is this instance worth driving?"
   (process up, port owned by it, route answering) for drivers to call before
   driving and after anything surprising.
8. **Give the app a shutdown handler.** `app/src/index.js` ignores `SIGTERM`;
   the server currently dies rather than closing its listener, which will matter
   once state outlives a single process.

## Toward A

- Repeated representative trials (E3) with an outcome grader that reads final
  state, not the agent's completion message; then parallel and long-running
  operation with exercised crash/stall recovery (E4).
- Retry budgets, escalation ownership, and back-pressure — the automation path
  currently has no Submit, Reconcile, or Complete stage at all.
- Workspace and result-ref isolation, so concurrent tasks share nothing but the
  remote.
- A maintenance loop that turns each production failure and review comment into
  a stronger contract or a mechanical check, rather than more prose.

## Do this next

```bash
cd app && npm ci && npm run boot
```

Expect `"outcome":"ready"` and exit 0. If it fails, the `error_kind` names the
owner. Then item 2 above.
