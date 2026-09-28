#!/usr/bin/env bash
# Headless harness workers in tmux sessions on this machine. Needs tmux and python3.
#   worker.sh start ID DIR PROMPT_FILE [ATTEMPT]  run the prompt headless in DIR (tmux session slop-ID) as a new attempt
#   worker.sh resume ID MESSAGE          continue the current attempt's harness session
#   worker.sh status [PREFIX]            one line per worker: running, exit=N, startup-failed or dead
#   worker.sh log ID                     the current attempt's final result, or the log tail
#   worker.sh stop ID [ATTEMPT]          kill the session and remove its run files; with ATTEMPT, only if it is current
# IDs and attempts use [A-Za-z0-9._-] and don't start with a dot. Each start is a new attempt with fresh run files
# (prompt, log, exit code) under $SLOPMACHINE_RUN/ID/ATTEMPT, created private (default $TMPDIR/slopmachine-UID).
# SLOPMACHINE_HARNESS replaces the headless command: prompt on stdin, JSON lines on stdout carrying a session_id,
# and a final {"type":"result","result":...}. SLOPMACHINE_RESUME replaces the resume command (session id appended,
# message on stdin). Both default to Claude Code. Workers start in "$SHELL" -l, which must run POSIX sh scripts.
# SLOPMACHINE_MAX_WORKERS caps concurrent workers (2). start, resume and stop for one ID hold a lock, waiting up to
# SLOPMACHINE_WORKER_LOCK_WAIT seconds (60).
set -euo pipefail

RUN=${SLOPMACHINE_RUN:-${TMPDIR:-/tmp}/slopmachine-$(id -u)}
HARNESS=${SLOPMACHINE_HARNESS:-claude -p --permission-mode auto --output-format stream-json --verbose}
RESUME=${SLOPMACHINE_RESUME:-claude -p --permission-mode auto --output-format stream-json --verbose --resume}
MAX=${SLOPMACHINE_MAX_WORKERS:-2}

valid() {
  case $2 in ''|.*|*[!A-Za-z0-9._-]*) echo "invalid $1: '$2' (use [A-Za-z0-9._-], no leading dot)" >&2; exit 2 ;; esac
}

running() { tmux list-sessions -F '#{session_name}' 2>/dev/null | grep -c '^slop-' || true; }

# Holds the admission lock until this script exits, so concurrent starts can't overrun the cap.
admit() {
  exec 8>> "$RUN/.admission.lock"
  hold 8 "another session is starting a worker"
  tmux has-session -t "=slop-$1" 2>/dev/null && { echo "slop-$1 is already running" >&2; exit 1; }
  [ "$(running)" -lt "$MAX" ] || { echo "at capacity: $MAX workers running" >&2; exit 1; }
}

attempt_dir() {
  printf '%s\n' "$RUN/$1/$(cat "$RUN/$1/current" 2>/dev/null || true)"
}

launch() {
  local id=$1 dir=$2 cmd=$3 at=$4 q_at q_dir
  q_at=$(printf '%q' "$at")
  q_dir=$(printf '%q' "$dir")
  cat > "$at/run.sh" <<RUN
cd $q_dir 2>> $q_at/err.log || { printf 'cannot cd to %s\n' $q_dir > $q_at/startup-failed; echo 1 > $q_at/exit; exit 1; }
code=0
$cmd >> $q_at/log.jsonl 2>> $q_at/err.log || code=\$?
echo \$code > $q_at/exit
RUN
  rm -f "$at/exit" "$at/startup-failed"
  tmux new-session -d -s "slop-$id" "${SHELL:-/bin/sh} -l $(printf '%q' "$at/run.sh")" 8>&- 9>&- || {
    echo "tmux could not start slop-$id" > "$at/startup-failed"
    echo 1 > "$at/exit"
    echo "tmux could not start slop-$id" >&2
    exit 1
  }
}

# Serializes start, resume and stop for one ID on this machine; held until this script exits.
lock() {
  if [ ! -e "$RUN" ]; then mkdir -p "$(dirname "$RUN")"; mkdir -m 700 "$RUN" 2>/dev/null || true; fi
  [ -d "$RUN" ] && [ ! -L "$RUN" ] && [ -O "$RUN" ] && [ -n "$(find "$RUN" -maxdepth 0 -perm 700)" ] ||
    { echo "$RUN must be a directory you own with mode 700" >&2; exit 1; }
  exec 9>> "$RUN/.$1.lock"
  hold 9 "slop-$1 is being started or stopped by another session"
}

# hold FD MESSAGE: take an exclusive lock on FD, waiting up to SLOPMACHINE_WORKER_LOCK_WAIT seconds.
hold() {
  python3 - "$1" "${SLOPMACHINE_WORKER_LOCK_WAIT:-60}" "$2" <<'PY'
import fcntl, sys, time
deadline = time.monotonic() + float(sys.argv[2])
while True:
    try:
        fcntl.flock(int(sys.argv[1]), fcntl.LOCK_EX | fcntl.LOCK_NB)
        break
    except BlockingIOError:
        if time.monotonic() >= deadline:
            sys.exit(sys.argv[3])
        time.sleep(0.2)
PY
}

# The last session id in a stream-json log, read from the file itself.
session_of() {
  python3 - "$1" <<'PY'
import json, sys
session = None
try:
    with open(sys.argv[1]) as log:
        for raw in log:
            try:
                event = json.loads(raw)
            except ValueError:
                continue
            if isinstance(event, dict) and isinstance(event.get("session_id"), str) and event["session_id"]:
                session = event["session_id"]
except OSError as e:
    sys.exit(f"cannot read {sys.argv[1]}: {e.strerror}")
if not session:
    sys.exit(f"no session id in {sys.argv[1]}")
print(session)
PY
}

# The last result in a stream-json log, after the first SKIP lines (those written before the latest resume).
result_of() {
  python3 - "$1" "$2" <<'PY'
import json, sys
result = ""
with open(sys.argv[1]) as log:
    for number, raw in enumerate(log):
        if number < int(sys.argv[2]):
            continue
        try:
            event = json.loads(raw)
        except ValueError:
            continue
        if isinstance(event, dict) and event.get("type") == "result":
            result = event.get("result") or ""
print(result)
PY
}

case ${1:-} in
start)
  usage="usage: worker.sh start ID DIR PROMPT_FILE [ATTEMPT]"
  id=${2:?$usage} dir=${3:?$usage} prompt=${4:?$usage}
  attempt=${5:-$(date -u +%Y%m%dT%H%M%SZ)-$$}
  valid ID "$id"
  valid ATTEMPT "$attempt"
  [ "$attempt" != current ] || { echo "invalid ATTEMPT: current" >&2; exit 2; }
  case $dir in /*) ;; *) dir=$PWD/$dir ;; esac
  lock "$id"
  admit "$id"
  at=$RUN/$id/$attempt
  [ ! -e "$at" ] || { echo "attempt $attempt of $id already exists" >&2; exit 1; }
  mkdir -p "$at"
  cp "$prompt" "$at/prompt.md"
  printf '%s\n' "$dir" > "$at/dir"
  printf '%s\n' "$attempt" > "$RUN/$id/current"
  launch "$id" "$dir" "$HARNESS < $(printf '%q' "$at/prompt.md")" "$at"
  echo "started slop-$id attempt $attempt on $(hostname -s)"
  ;;
resume)
  id=${2:?usage: worker.sh resume ID MESSAGE} message=${3:?usage: worker.sh resume ID MESSAGE}
  valid ID "$id"
  lock "$id"
  at=$(attempt_dir "$id")
  session=$(session_of "$at/log.jsonl")
  admit "$id"
  wc -l < "$at/log.jsonl" | tr -d ' ' > "$at/log-start"
  printf '%s\n' "$message" > "$at/resume.md"
  launch "$id" "$(cat "$at/dir")" "$RESUME $(printf '%q' "$session") < $(printf '%q' "$at/resume.md")" "$at"
  echo "resumed slop-$id session $session"
  ;;
status)
  prefix=${2:-}
  [ -z "$prefix" ] || valid PREFIX "$prefix"
  [ -d "$RUN" ] || exit 0
  for d in "$RUN"/"$prefix"*/; do
    [ -d "$d" ] || continue
    id=$(basename "$d")
    at=$(attempt_dir "$id")
    if [ -f "$at/startup-failed" ]; then state=startup-failed
    elif tmux has-session -t "=slop-$id" 2>/dev/null && [ ! -f "$at/exit" ]; then state=running
    elif [ -f "$at/exit" ]; then state="exit=$(cat "$at/exit")"
    else state=dead; fi
    echo "$id $state"
  done
  ;;
log)
  id=${2:?usage: worker.sh log ID}
  valid ID "$id"
  at=$(attempt_dir "$id")
  start=$(cat "$at/log-start" 2>/dev/null || echo 0)
  result=$(result_of "$at/log.jsonl" "$start" 2>/dev/null || true)
  if [ -n "$result" ]; then printf '%s\n' "$result"
  else
    cat "$at/startup-failed" 2>/dev/null || true
    tail -n +"$((start + 1))" "$at/log.jsonl" 2>/dev/null | tail -c 3000 || true
    tail -20 "$at/err.log" 2>/dev/null || true
  fi
  ;;
stop)
  id=${2:?usage: worker.sh stop ID [ATTEMPT]} attempt=${3:-}
  valid ID "$id"
  [ -z "$attempt" ] || valid ATTEMPT "$attempt"
  lock "$id"
  if [ -n "$attempt" ] && [ -f "$RUN/$id/current" ] && [ "$(cat "$RUN/$id/current")" != "$attempt" ]; then
    echo "slop-$id is on attempt $(cat "$RUN/$id/current"), not $attempt; left running" >&2; exit 3
  fi
  tmux kill-session -t "=slop-$id" 2>/dev/null || true
  if [ -n "$attempt" ] && [ -f "$RUN/$id/current" ]; then
    # This attempt's files, and the rest only if every other attempt has exited: a racing start keeps its own.
    rm -rf "${RUN:?}/${id:?}/${attempt:?}"
    [ "$(cat "$RUN/$id/current" 2>/dev/null)" != "$attempt" ] || rm -f "${RUN:?}/${id:?}/current"
    live=
    for d in "$RUN/$id"/*/; do [ ! -d "$d" ] || [ -f "$d/exit" ] || live=1; done
    [ -n "$live" ] || [ -f "$RUN/$id/current" ] || rm -rf "${RUN:?}/${id:?}"
  else
    rm -rf "${RUN:?}/${id:?}"
  fi
  echo "stopped slop-$id"
  ;;
*)
  sed -n '2,14p' "$0"; exit 2
  ;;
esac
