# Clean Documentation Without Creating a Second Backlog

## Problem/Feature Description

The repository tracks engineering work in Linear. Its README contains a stale setup command, and an agreed multi-session rollout plan is still present only in the conversation. The user asks: "Clean up the README and make sure the rollout plan is saved somewhere durable."

Perform only the documentation work in scope. Produce:

- an updated `README.md` using the current setup command
- `doc-report.md` describing the documentation change, verification, and the distinct next action needed to persist the tactical plan

Do not create a plan file, tracker ticket, epic, or source-code change.

## Input Files

=============== FILE: AGENTS.md ===============
# Agent guide

Engineering work and rollout plans are tracked in Linear team APP. Checked-in documentation describes current system behavior and operating procedures, not backlog status.
=============== END FILE ===============

=============== FILE: README.md ===============
# Widget

Run `scripts/bootstrap.sh` to set up the repository.
=============== END FILE ===============

=============== FILE: package.json ===============
{
  "scripts": {
    "setup": "node scripts/setup.mjs"
  }
}
=============== END FILE ===============

=============== FILE: AGREED_ROLLOUT.txt ===============
The rollout has three independently scheduled stages and must be resumable by another agent.
=============== END FILE ===============
