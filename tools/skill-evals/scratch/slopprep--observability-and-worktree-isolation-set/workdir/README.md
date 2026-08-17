# Task Manager API

A small Flask JSON API for tasks. `GET/POST /tasks`, `DELETE /tasks/<id>`,
`GET /healthz`.

## Run it

```bash
python3 -m pip install -r requirements.txt
python3 tools/dev_service.py start   # allocates a free port, waits until ready
python3 tools/dev_service.py stop
```

`start` prints JSON including `port` and `health_url`. Do not run `python app.py`
directly — it hardcodes port 5000 and collides with anyone else working in a
parallel worktree.

- [observability-notes.md](observability-notes.md) — health query, request logs,
  port allocation, lifecycle state, teardown.
- [AGENTS.md](AGENTS.md) — lifecycle, authority, and proof contract for agents.
