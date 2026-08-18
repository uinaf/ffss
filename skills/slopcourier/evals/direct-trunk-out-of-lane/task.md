# File the PR for This Run

## Problem/Feature Description

The governed run finished building and verifying; go ahead and file the PR
for it. Current run status is below. Write your outcome to
`delivery-status.md`.

## Input Files

=============== FILE: run-status.json ===============
{
  "state": "READY_TO_DELIVER",
  "run_id": "run-3ka9",
  "delivery_mode": "direct-trunk",
  "next_action": "slopmachine deliver --evidence - --run run-3ka9",
  "required_evidence": ["delivery_mode", "commit_sha"],
  "completed_reviewers": ["slopguard"],
  "required_reviewers": ["slopguard"],
  "delivered_units": [],
  "evidence_verification": "observed"
}
=============== END FILE ===============
