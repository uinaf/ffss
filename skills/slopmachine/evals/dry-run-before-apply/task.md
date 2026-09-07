# Feed the Agreed Plan into the Run

## Problem/Feature Description

The machine is offline; I will run your commands verbatim on the workstation.
The run was just initialized; its status document is below.

The agreed plan: first harden the webhook signature check (reject unsigned
payloads, covered by an integration test), then add a replay-protection nonce,
which depends on the signature work. Low-risk change, delivered as a held pull
request, reviewed by slopguard. Prepare the intake only; do not release it or
start execution yet.

Write the exact commands I should run, in order, with full stdin payloads where
a command takes input, to `commands.md`.

## Input Files

=============== FILE: status.json ===============
{
  "state": "INTAKE_REQUIRED",
  "run_id": "run-7f2c",
  "next_action": "slopmachine intake --file - --run run-7f2c",
  "allowed_commands": ["intake", "status", "schema"],
  "required_evidence": ["units", "delivery_mode", "risk_tier"],
  "intake_revision": 0,
  "required_reviewers": [],
  "completed_reviewers": [],
  "delivered_units": [],
  "delivery_mode": null,
  "blocker": null,
  "decision_question": null,
  "evidence_verification": "observed"
}
=============== END FILE ===============
