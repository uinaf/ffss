# Task Manager API — agent guide

A small Flask JSON API for tasks (`app.py`). Task state is in-process and
in-memory: restarting the service empties it, and two instances share nothing.

Several agents work this repo concurrently, one git worktree each. The rule that
follows from that: **never start the service by hand and never assume a port.**
`tools/dev_service.py` owns ports, process lifetime, and readiness.

## Routing

| When you need to | Read |
| --- | --- |
| query health, parse request logs, or understand isolation | [observability-notes.md](observability-notes.md) |
| change lifecycle, port allocation, or teardown | `tools/dev_service.py` |
| change endpoints or log fields | `app.py` |

## Lifecycle

```bash
python3 -m pip install -r requirements.txt   # bootstrap
python3 tools/dev_service.py start           # allocate port, launch, confirm ready
python3 tools/dev_service.py doctor          # read-only: worth driving?
python3 tools/dev_service.py stop            # release this worktree's service only
```

Each command prints one JSON object on stdout and exits non-zero on failure with
a `failure_class`. `start` is idempotent. Run `doctor` before driving the service
and again after anything surprising.

Never leave a service running at the end of a task: `stop`, or hand off the port
and pid from `.dev/service.json` together with the teardown command.

## Authority

- Safe: edit source, run the lifecycle, drive `127.0.0.1` endpoints, read `.dev/`.
- Do not commit `.env.local` or `.dev/` (both gitignored), and do not widen
  `.worktreeinclude` beyond the single local config file it names.
- Do not kill processes by name or clear ports globally — that hits other
  worktrees. Release only via `stop`.

## Proof

No test suite exists yet. Prove an API change on the real surface:

1. `start`, then `doctor` — exit `0`.
2. Exercise the changed endpoint plus one failure (for example `POST /tasks`
   with a non-object body, expecting `400` and `error.kind == "validation"`).
3. Read the matching `http_request` lines in `.dev/service.log` and check
   `status` and `outcome`.
4. `stop` — exit `0` with `released: true`.

Local health does not prove deployment; there is no deploy path here.

## Write-back

Contract changes — health fields, log fields, state schema, port precedence —
belong in [observability-notes.md](observability-notes.md) in the same change.
Bump `STATE_VERSION` in `tools/dev_service.py` when `.dev/service.json` changes
shape; older state files are then ignored rather than misread.
