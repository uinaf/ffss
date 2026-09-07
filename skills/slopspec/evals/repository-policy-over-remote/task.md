# Update Existing Linear Work Despite a GitHub Remote

## Problem/Feature Description

The repository is hosted on GitHub, but its agent guidance tracks product work in Linear team `SDK`. The conversation began from Linear issue `SDK-41`, whose URL and body are below. The user agreed on two additional acceptance criteria and says: "save this plan so another agent can pick it up."

Produce `planning-result.md` describing the durable destination, whether to update or create artifacts, the content that should change, and the verified resume point. Do not implement the feature.

## Input Files

=============== FILE: .git/config ===============
[remote "origin"]
  url = git@github.com:acme/sdk.git
=============== END FILE ===============

=============== FILE: AGENTS.md ===============
# Agent guide

Work tracking: Linear team SDK is canonical for planned product work. GitHub issues are used only for external bug reports.
=============== END FILE ===============

=============== FILE: CURRENT_ISSUE.md ===============
URL: https://linear.app/acme/issue/SDK-41/retry-policy
Title: Add configurable retry policy

## Outcome
SDK consumers can configure retry limits.

## Acceptance criteria
- [ ] The default remains three attempts.
=============== END FILE ===============

=============== FILE: AGREED_CHANGES.md ===============
- Reject negative retry counts at configuration parsing.
- Document zero retries as disabling retry behavior.
=============== END FILE ===============
