#!/usr/bin/env python3
"""Owned lifecycle for the Task Manager API: one service per worktree.

    python3 tools/dev_service.py start    # allocate a port, launch, confirm ready
    python3 tools/dev_service.py doctor   # read-only: is this instance worth driving?
    python3 tools/dev_service.py stop     # release only what this worktree started

Every command prints one JSON object on stdout and nothing else, so a caller can
parse the result instead of reading logs. Exit status is 0 only when the declared
outcome was reached and verified.

Ownership rules that keep three concurrent worktrees from breaking each other:

- the port comes from the OS (bind to port 0) unless explicitly requested, so two
  worktrees never negotiate over 5000;
- state lives in ``.dev/service.json`` inside *this* worktree, so a command can
  only ever see and release its own service;
- the recorded pid is paired with that process's start time, so a recycled pid is
  treated as stale state rather than killed;
- teardown signals the child's own process group -- never a process name -- and
  then verifies absence instead of trusting the signal.
"""

from __future__ import annotations

import argparse
import json
import os
import signal
import socket
import subprocess
import sys
import time
import urllib.error
import urllib.request
from pathlib import Path
from typing import Any

REPO_ROOT = Path(__file__).resolve().parent.parent
RUNTIME_DIR = REPO_ROOT / ".dev"
STATE_PATH = RUNTIME_DIR / "service.json"
LOG_PATH = RUNTIME_DIR / "service.log"
ENV_FILE = REPO_ROOT / ".env.local"

STATE_VERSION = 1
HOST = "127.0.0.1"
READY_TIMEOUT = 20.0
READY_POLL_INTERVAL = 0.1
STOP_TIMEOUT = 10.0
PORT_ATTEMPTS = 5
LOG_TAIL_LINES = 20


# --------------------------------------------------------------------------
# result contract
# --------------------------------------------------------------------------


def emit(payload: dict[str, Any], *, code: int) -> int:
    """Print the single machine-readable result line and return the exit code."""
    json.dump({**payload, "exit_code": code}, sys.stdout, sort_keys=True)
    sys.stdout.write("\n")
    sys.stdout.flush()
    return code


def log_tail() -> list[str]:
    """Last lines of the service log, for unattended diagnosis of a failed start."""
    try:
        lines = LOG_PATH.read_text(encoding="utf-8", errors="replace").splitlines()
    except OSError:
        return []
    return lines[-LOG_TAIL_LINES:]


# --------------------------------------------------------------------------
# process identity
# --------------------------------------------------------------------------


def process_start_time(pid: int) -> str | None:
    """Kernel-reported start time of ``pid``, or None if no such process runs.

    Pairing pid with start time is what makes ownership safe: after a reboot or
    pid wraparound the same number can belong to an unrelated process.
    """
    try:
        os.kill(pid, 0)
    except ProcessLookupError:
        return None
    except PermissionError:
        # Alive, but owned by another user -- never ours to signal.
        return None
    try:
        out = subprocess.run(
            ["ps", "-o", "lstart=", "-p", str(pid)],
            capture_output=True,
            text=True,
            timeout=5,
        )
    except (OSError, subprocess.SubprocessError):
        return None
    return out.stdout.strip() or None


def state_is_owned(state: dict[str, Any]) -> bool:
    """True when the recorded process is still the process this worktree started."""
    pid = state.get("pid")
    if not isinstance(pid, int):
        return False
    current = process_start_time(pid)
    return current is not None and current == state.get("proc_start")


# --------------------------------------------------------------------------
# persisted lifecycle state
# --------------------------------------------------------------------------


def read_state() -> dict[str, Any] | None:
    try:
        raw = STATE_PATH.read_text(encoding="utf-8")
    except FileNotFoundError:
        return None
    except OSError:
        return None
    try:
        state = json.loads(raw)
    except json.JSONDecodeError:
        return None
    if not isinstance(state, dict) or state.get("state_version") != STATE_VERSION:
        return None
    # Treat a truncated or hand-edited record as absent rather than crashing a
    # caller that expects these keys.
    if not {"pid", "pgid", "port", "health_url"} <= state.keys():
        return None
    return state


def write_state(state: dict[str, Any]) -> None:
    RUNTIME_DIR.mkdir(parents=True, exist_ok=True)
    tmp = STATE_PATH.with_suffix(".json.tmp")
    tmp.write_text(json.dumps(state, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    tmp.replace(STATE_PATH)  # atomic: a reader never sees a half-written owner


def clear_state() -> None:
    STATE_PATH.unlink(missing_ok=True)


# --------------------------------------------------------------------------
# environment and port allocation
# --------------------------------------------------------------------------


def load_env_file(path: Path = ENV_FILE) -> dict[str, str]:
    """Parse ``KEY=VALUE`` lines from the gitignored local config file.

    Managed Codex/Claude worktrees receive this file via .worktreeinclude; a
    manual worktree simply has no file and gets an empty mapping.
    """
    values: dict[str, str] = {}
    try:
        text = path.read_text(encoding="utf-8")
    except (FileNotFoundError, OSError):
        return values
    for line in text.splitlines():
        line = line.strip()
        if not line or line.startswith("#") or "=" not in line:
            continue
        key, _, value = line.partition("=")
        value = value.strip()
        if len(value) >= 2 and value[0] == value[-1] and value[0] in "\"'":
            value = value[1:-1]
        values[key.strip()] = value
    return values


def free_port() -> int:
    """Ask the OS for a currently unused port."""
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as sock:
        sock.bind((HOST, 0))
        return sock.getsockname()[1]


def port_available(port: int) -> bool:
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as sock:
        sock.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
        try:
            sock.bind((HOST, port))
        except OSError:
            return False
    return True


def resolve_requested_port(flag: int | None, env_values: dict[str, str]) -> tuple[int | None, str]:
    """Explicit port request, if any: flag beats process env beats .env.local."""
    for value, source in (
        (flag, "flag"),
        (os.environ.get("PORT"), "env"),
        (env_values.get("PORT"), "env_file"),
    ):
        if value in (None, ""):
            continue
        try:
            port = int(value)
        except (TypeError, ValueError):
            raise ValueError(f"PORT from {source} is not an integer: {value!r}") from None
        if not 1 <= port <= 65535:
            raise ValueError(f"PORT from {source} is out of range: {port}")
        return port, source
    return None, "allocated"


# --------------------------------------------------------------------------
# readiness and teardown
# --------------------------------------------------------------------------


def query_health(port: int, timeout: float = 1.0) -> dict[str, Any] | None:
    """One /healthz query. None when the service did not answer usefully."""
    url = f"http://{HOST}:{port}/healthz"
    try:
        with urllib.request.urlopen(url, timeout=timeout) as response:  # noqa: S310 - fixed localhost URL
            if response.status != 200:
                return None
            return json.loads(response.read().decode("utf-8"))
    except (urllib.error.URLError, OSError, json.JSONDecodeError, ValueError):
        return None


def await_ready(process: subprocess.Popen, port: int, timeout: float) -> tuple[dict[str, Any] | None, str | None]:
    """Poll /healthz until the expected process answers, the child dies, or time runs out.

    Returns ``(health, failure_class)`` -- exactly one is set.
    """
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        if process.poll() is not None:
            return None, "service/exited_early"
        health = query_health(port)
        if health and health.get("pid") == process.pid:
            return health, None
        time.sleep(READY_POLL_INTERVAL)
    if process.poll() is not None:
        return None, "service/exited_early"
    return None, "service/readiness_timeout"


def release(pid: int, pgid: int, port: int, timeout: float = STOP_TIMEOUT) -> dict[str, Any]:
    """Signal the owned process group, then verify the resources are actually gone.

    Never trusts the signal: the returned ``released`` flag reflects an observed
    final state, so a caller can fail the command when cleanup did not happen.
    """
    steps: list[str] = []
    # Refuse to signal a group we do not own -- most importantly our own.
    if pgid <= 1 or pgid == os.getpgrp():
        return {"released": False, "reason": "unsafe_pgid", "pgid": pgid, "steps": steps}

    for sig, label in ((signal.SIGTERM, "sigterm"), (signal.SIGKILL, "sigkill")):
        if process_start_time(pid) is None:
            break
        try:
            os.killpg(pgid, sig)
            steps.append(label)
        except ProcessLookupError:
            break
        except PermissionError:
            return {"released": False, "reason": "permission_denied", "pgid": pgid, "steps": steps}
        deadline = time.monotonic() + (timeout if sig == signal.SIGTERM else 2.0)
        while time.monotonic() < deadline:
            if process_start_time(pid) is None:
                break
            time.sleep(0.1)

    process_gone = process_start_time(pid) is None
    # The port counts as released once *our* process no longer answers on it; a
    # different worktree that has since claimed it is not our resource to fail on.
    successor = query_health(port, timeout=0.5)
    port_free = successor is None or successor.get("pid") != pid
    return {
        "released": process_gone and port_free,
        "process_gone": process_gone,
        "port_released": port_free,
        "steps": steps,
    }


# --------------------------------------------------------------------------
# commands
# --------------------------------------------------------------------------


def git_revision() -> str | None:
    try:
        out = subprocess.run(
            ["git", "-C", str(REPO_ROOT), "rev-parse", "--short", "HEAD"],
            capture_output=True,
            text=True,
            timeout=5,
        )
    except (OSError, subprocess.SubprocessError):
        return None
    return out.stdout.strip() or None


def cmd_start(args: argparse.Namespace) -> int:
    RUNTIME_DIR.mkdir(parents=True, exist_ok=True)
    env_values = load_env_file()

    try:
        requested_port, port_source = resolve_requested_port(args.port, env_values)
    except ValueError as exc:
        return emit(
            {
                "event": "dev_service_start",
                "outcome": "failed",
                "failure_class": "validation/bad_port",
                "message": str(exc),
                "recovery": "pass a port in 1-65535 or unset PORT to let the OS allocate one",
            },
            code=2,
        )

    # Idempotent acquire: a healthy service this worktree already owns is reused.
    state = read_state()
    if state is not None:
        if state_is_owned(state):
            health = query_health(state["port"])
            if health and health.get("pid") == state["pid"]:
                return emit(
                    {
                        "event": "dev_service_start",
                        "outcome": "already_running",
                        "port": state["port"],
                        "pid": state["pid"],
                        "health_url": state["health_url"],
                        "state_path": str(STATE_PATH),
                        "teardown": "python3 tools/dev_service.py stop",
                    },
                    code=0,
                )
            # Ours, but not serving: release it before raising a replacement.
            release(state["pid"], state["pgid"], state["port"])
        clear_state()

    if requested_port is not None and not port_available(requested_port):
        return emit(
            {
                "event": "dev_service_start",
                "outcome": "failed",
                "failure_class": "conflict/port_in_use",
                "port": requested_port,
                "port_source": port_source,
                "message": f"port {requested_port} is held by another process",
                "recovery": "unset PORT to let this worktree allocate its own port",
            },
            code=1,
        )

    attempts = 1 if requested_port is not None else PORT_ATTEMPTS
    failure: dict[str, Any] = {}

    for attempt in range(1, attempts + 1):
        port = requested_port if requested_port is not None else free_port()
        child_env = {**env_values, **os.environ, "PORT": str(port), "PYTHONUNBUFFERED": "1"}

        log_handle = LOG_PATH.open("a", encoding="utf-8")
        try:
            process = subprocess.Popen(
                [sys.executable, "app.py"],
                cwd=str(REPO_ROOT),
                env=child_env,
                stdout=log_handle,
                stderr=subprocess.STDOUT,
                start_new_session=True,  # own process group -> teardown covers descendants
            )
        except OSError as exc:
            log_handle.close()
            return emit(
                {
                    "event": "dev_service_start",
                    "outcome": "failed",
                    "failure_class": "internal/spawn_failed",
                    "message": str(exc),
                },
                code=1,
            )
        finally:
            log_handle.close()

        pgid = os.getpgid(process.pid)
        # Persist ownership before the first failure point, so a crash during
        # readiness still leaves `stop` something it can release.
        state = {
            "state_version": STATE_VERSION,
            "service": "task-manager-api",
            "worktree": str(REPO_ROOT),
            "revision": git_revision(),
            "pid": process.pid,
            "pgid": pgid,
            "proc_start": process_start_time(process.pid),
            "port": port,
            "port_source": port_source,
            "host": HOST,
            "health_url": f"http://{HOST}:{port}/healthz",
            "log_path": str(LOG_PATH),
            "attempt": attempt,
            "started_at": time.strftime("%Y-%m-%dT%H:%M:%S%z"),
        }
        write_state(state)

        health, failure_class = await_ready(process, port, args.timeout)
        if health is not None:
            return emit(
                {
                    "event": "dev_service_start",
                    "outcome": "started",
                    "port": port,
                    "port_source": port_source,
                    "pid": process.pid,
                    "pgid": pgid,
                    "attempt": attempt,
                    "revision": state["revision"],
                    "health_url": state["health_url"],
                    "state_path": str(STATE_PATH),
                    "log_path": str(LOG_PATH),
                    "teardown": "python3 tools/dev_service.py stop",
                },
                code=0,
            )

        # Cleanup runs on every failure path, then state is dropped.
        cleanup = release(process.pid, pgid, port)
        clear_state()
        failure = {
            "failure_class": failure_class,
            "port": port,
            "attempt": attempt,
            "exit_status": process.poll(),
            "cleanup_verified": cleanup["released"],
            "log_tail": log_tail(),
        }
        # A lost port race is the one recoverable case: retry on a fresh port.
        if failure_class == "service/exited_early" and requested_port is None and attempt < attempts:
            continue
        break

    return emit(
        {
            "event": "dev_service_start",
            "outcome": "failed",
            "message": "service did not become ready",
            "recovery": f"inspect {LOG_PATH}; fix the failure it reports, then start again",
            **failure,
        },
        code=1,
    )


def cmd_stop(args: argparse.Namespace) -> int:
    state = read_state()
    if state is None:
        return emit(
            {
                "event": "dev_service_stop",
                "outcome": "not_running",
                "message": "no service is recorded for this worktree",
            },
            code=0,
        )

    if not state_is_owned(state):
        clear_state()
        return emit(
            {
                "event": "dev_service_stop",
                "outcome": "stale_state_cleared",
                "pid": state.get("pid"),
                "message": "recorded process is gone or belongs to someone else; left it untouched",
            },
            code=0,
        )

    cleanup = release(state["pid"], state["pgid"], state["port"], timeout=args.timeout)
    if not cleanup["released"]:
        return emit(
            {
                "event": "dev_service_stop",
                "outcome": "failed",
                "failure_class": "teardown/incomplete",
                "pid": state["pid"],
                "port": state["port"],
                "recovery": f"inspect pid {state['pid']}; state kept at {STATE_PATH}",
                **cleanup,
            },
            code=1,
        )

    clear_state()
    return emit(
        {
            "event": "dev_service_stop",
            "outcome": "stopped",
            "pid": state["pid"],
            "pgid": state["pgid"],
            "port": state["port"],
            **cleanup,
        },
        code=0,
    )


def cmd_doctor(args: argparse.Namespace) -> int:
    """Read-only: is the recorded instance worth driving? Never repairs anything."""
    state = read_state()
    if state is None:
        return emit(
            {
                "event": "dev_service_doctor",
                "outcome": "not_running",
                "worth_driving": False,
                "recovery": "python3 tools/dev_service.py start",
            },
            code=1,
        )

    owned = state_is_owned(state)
    health = query_health(state["port"]) if owned else None
    revision = git_revision()
    checks = {
        "state_readable": True,
        "process_owned": owned,
        "health_ok": bool(health),
        "port_owned_by_process": bool(health and health.get("pid") == state["pid"]),
        "revision_matches": revision == state.get("revision"),
    }
    required = ("process_owned", "health_ok", "port_owned_by_process")
    worth_driving = all(checks[name] for name in required)
    return emit(
        {
            "event": "dev_service_doctor",
            "outcome": "ok" if worth_driving else "unhealthy",
            "worth_driving": worth_driving,
            "checks": checks,
            "port": state["port"],
            "pid": state["pid"],
            "revision": {"recorded": state.get("revision"), "checkout": revision},
            "uptime_seconds": (health or {}).get("uptime_seconds"),
            "recovery": None
            if worth_driving
            else "python3 tools/dev_service.py stop && python3 tools/dev_service.py start",
        },
        code=0 if worth_driving else 1,
    )


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        prog="dev_service",
        description="Start, inspect, and stop this worktree's Task Manager API instance.",
    )
    sub = parser.add_subparsers(dest="command", required=True)

    start = sub.add_parser("start", help="allocate a port, launch the service, confirm readiness")
    start.add_argument("--port", type=int, default=None, help="request a specific port instead of allocating one")
    start.add_argument("--timeout", type=float, default=READY_TIMEOUT, help="readiness budget in seconds")
    start.set_defaults(func=cmd_start)

    doctor = sub.add_parser("doctor", help="read-only health and ownership check")
    doctor.set_defaults(func=cmd_doctor)

    stop = sub.add_parser("stop", help="release the service owned by this worktree")
    stop.add_argument("--timeout", type=float, default=STOP_TIMEOUT, help="teardown budget in seconds")
    stop.set_defaults(func=cmd_stop)

    return parser


def main(argv: list[str] | None = None) -> int:
    args = build_parser().parse_args(argv)
    return args.func(args)


if __name__ == "__main__":
    sys.exit(main())
