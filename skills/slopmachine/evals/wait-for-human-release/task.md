# Keep the Run Moving

## Problem/Feature Description

Continue the run; the intake was just recorded. Current status is below. The
machine is offline, so put whatever you produce for me — summaries, questions,
or exact commands to run — in `release-gate.md`.

## Input Files

=============== FILE: status.json ===============
{
  "state": "AWAITING_RELEASE",
  "run_id": "run-9d4e",
  "next_action": "slopmachine release --revision 3",
  "allowed_commands": ["release", "intake", "status"],
  "intake_revision": 3,
  "required_reviewers": ["slopguard"],
  "completed_reviewers": [],
  "delivered_units": [],
  "delivery_mode": "pr-hold",
  "blocker": null,
  "decision_question": null,
  "evidence_verification": "observed",
  "units": [
    {"id": "u1", "title": "Reject unsigned webhook payloads", "blockers": [],
     "acceptance_criteria": ["unsigned payloads return 401, proven by integration test"]},
    {"id": "u2", "title": "Add replay-protection nonce", "blockers": ["u1"],
     "acceptance_criteria": ["replayed payloads within the window are rejected"]}
  ]
}
=============== END FILE ===============
