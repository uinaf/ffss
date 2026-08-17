# Observability and Worktree Isolation

Three agents run this API concurrently, one per git worktree. Nothing is shared
between them except the source tree, and nothing is discovered by reading console
output.

## Health query

`GET /healthz` answers whether an instance is alive. Query it with
`curl -s "$(python3 -c 'import json;print(json.load(open(".dev/service.json"))["health_url"])')"`,
or read `health_url` out of the state file below. Response shape:

```json
{"status":"ok","service":"task-manager-api","pid":40122,"port":54031,"uptime_seconds":3.114,"tasks":0}
```

The `pid` and `port` in the body are the point: a caller compares them against
the process it started, which proves the port it allocated is served by *its*
service and not a neighbour's. `/healthz` is read-only and has no side effects.

`python3 tools/dev_service.py doctor` is the packaged form of that question —
"is this instance worth driving?" It checks process ownership, health, that the
port is owned by the expected pid, and that the running revision still matches
the checkout. It exits `0` only when all three required checks pass, never
repairs anything, and prints one JSON object.

## Machine-readable request output

Every request emits exactly one JSON line on stderr, captured to `.dev/service.log`:

```json
{"duration_ms":1.42,"event":"http_request","level":"info","method":"POST","operation":"POST /tasks","outcome":"success","path":"/tasks","pid":40122,"request_id":"9f2c...","route":"/tasks","service":"task-manager-api","status":201,"ts":"2026-08-17T09:14:22.481913+00:00"}
```

Stable fields: `event`, `operation`, `outcome`, `status`, `duration_ms`,
`request_id`, `error_kind`. `outcome` is one of `success`, `client_error`,
`server_error`. The `print()` calls are gone — agents filter the log by field
instead of parsing prose. Error responses are JSON too, with a stable
`error.kind` (`validation`, `not_found`, `method_not_allowed`, `internal`)
rather than an HTML page.

The `X-Request-ID` response header echoes the id, so a client correlates its own
call with the server line. Pass `X-Request-ID` in to supply your own.

## Resource allocation owner

`tools/dev_service.py start` owns port allocation. Nobody hardcodes 5000.

| Precedence | Source | Use |
| --- | --- | --- |
| 1 | `--port N` | pinning a port deliberately |
| 2 | `PORT` in the environment | runner-injected port |
| 3 | `PORT` in `.env.local` | per-worktree local default |
| 4 | OS-allocated | the normal case |

With no explicit request, `start` binds port `0`, takes whatever the kernel
hands back, and launches the service there — so two worktrees cannot negotiate
over the same number. If the port is lost between allocation and bind, the child
exits early and `start` retries on a fresh port up to 5 times. An explicitly
requested port is never retried; if it is occupied, `start` fails with
`conflict/port_in_use` and names the recovery.

Readiness is bounded, not assumed: `start` polls `/healthz` every 100 ms for up
to 20 s (`--timeout`) and also watches for early child exit. On failure it tears
the child down, verifies it is gone, drops the state file, prints
`failure_class` plus the last 20 log lines, and exits non-zero.

## Persisted lifecycle state

`.dev/service.json` in the current worktree, written atomically before readiness
is confirmed so a crash mid-startup still leaves a releasable owner:

```json
{
  "state_version": 1,
  "service": "task-manager-api",
  "worktree": "/Users/agent/wt/feature-a",
  "revision": "5891c1e",
  "pid": 40122,
  "pgid": 40122,
  "proc_start": "Mon Aug 17 12:14:19 2026",
  "port": 54031,
  "port_source": "allocated",
  "health_url": "http://127.0.0.1:54031/healthz",
  "log_path": "/Users/agent/wt/feature-a/.dev/service.log",
  "attempt": 1,
  "started_at": "2026-08-17T12:14:19+0300"
}
```

Two properties make it safe to act on:

- **`proc_start` pairs with `pid`.** After a reboot or pid wraparound the same
  number can belong to an unrelated process, so ownership is only claimed when
  the kernel-reported start time still matches. A mismatch is reported as
  `stale_state_cleared` and the foreign process is left alone.
- **`pgid` is the release boundary.** The service is started in a new session, so
  signalling its process group covers any descendant it spawned. Teardown never
  matches on process *name*, so it cannot hit a sibling worktree's instance.

`start` is idempotent: if this worktree's recorded service is already healthy it
reports `already_running` with the existing port instead of launching a second
one. `.dev/` is gitignored.

## Managed-worktree config copying

`.env.local` holds local-only development configuration, is gitignored, and is
therefore missing from a fresh worktree. `.worktreeinclude` names that one file
so managed Codex/Claude worktrees copy it in. The list is deliberately narrow —
no `.env*` glob, no secret directories, no caches, dependencies, build output, or
machine-global config.

Manual `git worktree add` and custom hooks do **not** run that mechanism. Copy
`.env.local` yourself there. Its absence is not fatal: `start` treats a missing
file as empty local config.

## Teardown

```
python3 tools/dev_service.py stop
```

Releases only the service recorded in the current worktree's
`.dev/service.json`: `SIGTERM` to the owned process group, bounded wait,
`SIGKILL` if needed, then it *verifies* the process is gone and the port no
longer answers as ours before reporting success. Cleanup that did not take
exits non-zero with `teardown/incomplete` and keeps the state file for
diagnosis. With nothing running it reports `not_running` and exits `0`.

`.dev/service.log` survives teardown — cleanup releases resources without
destroying the evidence.

## Exit contract

Every command prints one JSON object on stdout and nothing else. `exit_code` is
included in the payload. `0` means the declared outcome was reached and verified;
non-zero carries a `failure_class` and, where an operator can act, a `recovery`
string.
