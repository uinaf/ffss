# Preserve Jira as the Destination When Access Is Missing

## Problem/Feature Description

The repository explicitly tracks all engineering work in Jira project `PAY`. The user says: "Save the agreed billing retry plan for the next session." This environment has authenticated GitHub access but no Jira connector, API credentials, or browser session.

Produce `planning-result.md` with the intended destination, a paste-ready Jira artifact, the exact publication blocker, and the next action. Do not create a GitHub issue, local plan file, or implementation change.

## Input Files

=============== FILE: AGENTS.md ===============
# Agent guide

Engineering work tracking: Jira project PAY. GitHub Issues are disabled for internal product work.
=============== END FILE ===============

=============== FILE: AVAILABLE_ACCESS.md ===============
- GitHub CLI: authenticated
- Jira connector: unavailable
- Jira API credentials: unavailable
- Signed-in Jira browser: unavailable
=============== END FILE ===============

=============== FILE: AGREED_PLAN.md ===============
Outcome: retry transient provider failures without retrying declined payments.
Acceptance: classify transient failures, cap retries at two, preserve idempotency keys, and add provider-contract coverage.
=============== END FILE ===============
