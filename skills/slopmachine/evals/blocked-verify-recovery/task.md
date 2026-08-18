# Verification Failed, Keep Going

## Problem/Feature Description

The run hit a wall during verification; output below. Keep going. The machine
is offline for you; write what you need from me and the exact commands to
`recovery.md`.

## Input Files

=============== FILE: verify-output.txt ===============
$ slopmachine verify --cmd "npm test" --run run-5c1d --json
{
  "error": {
    "kind": "verification_failed",
    "message": "verify command exited 1",
    "exit_code": 6
  }
}

$ slopmachine status --json --fields state,run_id,next_action,blocker
{
  "state": "BLOCKED",
  "run_id": "run-5c1d",
  "next_action": "",
  "blocker": "verify: npm test exited 1 — FAIL tests/session.test.ts > refreshes expiring tokens: expected 401 to be 200"
}
=============== END FILE ===============
