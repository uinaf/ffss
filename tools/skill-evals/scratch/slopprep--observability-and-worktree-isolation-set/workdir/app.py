"""Task Manager API.

Observability contract consumed by agents and by tools/dev_service.py:

- ``GET /healthz`` answers "is this instance alive and worth driving?" with the
  pid and port of the process that answered, so a caller can prove the port it
  allocated is owned by the process it started.
- every request emits one JSON line on stderr with stable fields
  (``event``, ``operation``, ``outcome``, ``status``, ``request_id``), so agents
  parse outcomes instead of reading prose.
- every error response is JSON with a stable ``error.kind``, never an HTML page.

``PORT`` selects the listen port; tools/dev_service.py owns allocating it.
"""

from __future__ import annotations

import datetime as dt
import json
import logging
import os
import time
import uuid

from flask import Flask, g, jsonify, request
from werkzeug.exceptions import HTTPException

SERVICE = "task-manager-api"
STARTED_AT = time.time()

# HTTP status -> stable error kind. Keeps the taxonomy small and machine-readable
# instead of letting callers pattern-match on message prose.
ERROR_KINDS = {400: "validation", 404: "not_found", 405: "method_not_allowed"}

logger = logging.getLogger(SERVICE)

tasks: list[dict] = []


class JsonFormatter(logging.Formatter):
    """Render every record as one JSON object on a single line."""

    def format(self, record: logging.LogRecord) -> str:
        payload = {
            "ts": dt.datetime.fromtimestamp(record.created, dt.timezone.utc).isoformat(),
            "level": record.levelname.lower(),
            "service": SERVICE,
            "pid": os.getpid(),
        }
        fields = getattr(record, "fields", None)
        if fields:
            payload.update(fields)
        else:
            payload.setdefault("event", "log")
            payload["message"] = record.getMessage()
        if record.exc_info and record.exc_info[0] is not None:
            payload["error_kind"] = record.exc_info[0].__name__
        return json.dumps(payload, separators=(",", ":"), sort_keys=True)


def configure_logging() -> None:
    """Send every log line through the JSON formatter, exactly once."""
    handler = logging.StreamHandler()
    handler.setFormatter(JsonFormatter())
    root = logging.getLogger()
    for existing in list(root.handlers):
        root.removeHandler(existing)
    root.addHandler(handler)
    root.setLevel(logging.INFO)
    # Werkzeug's own access line is unstructured and duplicates after_request.
    logging.getLogger("werkzeug").setLevel(logging.WARNING)


def log_event(event: str, **fields) -> None:
    logger.info("", extra={"fields": {"event": event, **fields}})


app = Flask(__name__)


@app.before_request
def _start_request() -> None:
    g.request_id = request.headers.get("X-Request-ID") or uuid.uuid4().hex
    g.started = time.perf_counter()


@app.after_request
def _log_request(response):
    duration_ms = round((time.perf_counter() - getattr(g, "started", time.perf_counter())) * 1000, 2)
    request_id = getattr(g, "request_id", "unknown")
    if response.status_code >= 500:
        outcome = "server_error"
    elif response.status_code >= 400:
        outcome = "client_error"
    else:
        outcome = "success"
    log_event(
        "http_request",
        operation=f"{request.method} {request.path}",
        method=request.method,
        path=request.path,
        route=request.url_rule.rule if request.url_rule else None,
        status=response.status_code,
        outcome=outcome,
        duration_ms=duration_ms,
        request_id=request_id,
    )
    response.headers["X-Request-ID"] = request_id
    return response


@app.errorhandler(HTTPException)
def _http_error(exc: HTTPException):
    kind = ERROR_KINDS.get(exc.code, "http_error")
    body = {"error": {"kind": kind, "status": exc.code, "message": exc.description}}
    return jsonify(body), exc.code


@app.errorhandler(Exception)
def _unhandled_error(exc: Exception):
    # Log the cause for diagnosis; return a stable class without leaking internals.
    logger.exception(
        "",
        extra={
            "fields": {
                "event": "unhandled_error",
                "operation": f"{request.method} {request.path}",
                "outcome": "server_error",
                "request_id": getattr(g, "request_id", "unknown"),
            }
        },
    )
    body = {"error": {"kind": "internal", "status": 500, "message": "internal server error"}}
    return jsonify(body), 500


@app.route("/healthz", methods=["GET"])
def healthz():
    """Liveness and identity of this instance. Read-only, no side effects."""
    return jsonify(
        {
            "status": "ok",
            "service": SERVICE,
            "pid": os.getpid(),
            "port": int(os.environ.get("PORT", 0)) or None,
            "uptime_seconds": round(time.time() - STARTED_AT, 3),
            "tasks": len(tasks),
        }
    )


@app.route("/tasks", methods=["GET"])
def get_tasks():
    return jsonify(tasks)


@app.route("/tasks", methods=["POST"])
def create_task():
    data = request.get_json(silent=True)
    if not isinstance(data, dict):
        body = {
            "error": {
                "kind": "validation",
                "status": 400,
                "message": "body must be a JSON object",
            }
        }
        return jsonify(body), 400
    task = {"id": len(tasks) + 1, **data}
    tasks.append(task)
    log_event("task_created", operation="create_task", resource_id=task["id"], outcome="success")
    return jsonify(task), 201


@app.route("/tasks/<int:task_id>", methods=["DELETE"])
def delete_task(task_id: int):
    global tasks
    remaining = [t for t in tasks if t["id"] != task_id]
    removed = len(tasks) - len(remaining)
    tasks = remaining
    log_event(
        "task_deleted",
        operation="delete_task",
        resource_id=task_id,
        outcome="success" if removed else "noop",
    )
    return "", 204


configure_logging()

if __name__ == "__main__":
    port = int(os.environ.get("PORT", "5000"))
    host = os.environ.get("HOST", "127.0.0.1")
    log_event("service_starting", operation="serve", port=port, host=host, outcome="pending")
    app.run(host=host, port=port, debug=False, use_reloader=False)
