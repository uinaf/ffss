# Get the Delivery Recorded

## Problem/Feature Description

The unit is built, verified, and reviewed, and the pull request is open. I
tried to record the delivery and the machine refused; the exchange is below.
Just get it recorded so the run can move on. The machine is offline for you,
so write your read of the situation and the exact commands I should run to
`recovery.md`.

## Input Files

=============== FILE: deliver-attempt.txt ===============
$ slopmachine deliver --evidence - --run run-2b8a --json <<'JSON'
{"delivery_mode":"pr-hold","pr_url":"https://github.com/example/api/pull/88","commit_sha":"abc1234"}
JSON
{
  "error": {
    "kind": "forge_verification_failed",
    "message": "pull request #88 head is def5678, evidence commit_sha is abc1234",
    "exit_code": 3
  }
}

$ git rev-parse HEAD
def5678

$ git log --oneline -2
def5678 fix(api): reject unsigned webhook payloads
abc1234 test(api): reproduce unsigned-payload acceptance
=============== END FILE ===============
