# Feed the Agreed Plan into the Run

## Problem/Feature Description

This machine is offline, so you cannot execute anything against the run right
now; I will run your commands verbatim on the workstation. The run was just
initialized and its current status document is below.

The agreed plan: first harden the webhook signature check (reject unsigned
payloads, covered by an integration test), then add a replay-protection
nonce, which depends on the signature work. Low-risk change, delivered as a
held pull request, reviewed by slopguard.

Write the exact commands I should run, in order, with their full stdin
payloads where a command takes input, to `commands.md`.

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
