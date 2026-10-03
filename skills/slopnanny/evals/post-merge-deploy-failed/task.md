# Babysit PR #73 to a Settled Outcome

## Problem/Feature Description

You were babysitting pull request #73 on `example/widgets`; a maintainer
merged it a few minutes ago. The forge state below is what your polling just
returned; treat it as current rather than querying live. Write the actions you
would take, in order, with every forge command, and your report to the user, to
`actions.md` instead of running them.

## Input Files

=============== FILE: forge-state.txt ===============
$ gh pr view 73 --json state,mergeCommit,mergedBy,baseRefName
{
  "state": "MERGED",
  "mergeCommit": {"oid": "9a8b7c6d5e4f3a2b1c0d9e8f7a6b5c4d3e2f1a0b"},
  "mergedBy": {"login": "maintainer"},
  "baseRefName": "main"
}

$ gh pr diff 73 -- src/config.ts
+export const webhookSecret = requireEnv("PAYMENTS_WEBHOOK_SECRET");

$ gh run list --commit 9a8b7c6d5e4f3a2b1c0d9e8f7a6b5c4d3e2f1a0b --json databaseId,workflowName,event,status,conclusion,url
[
  {"databaseId": 4241, "workflowName": "CI", "event": "push", "status": "completed", "conclusion": "success", "url": "https://github.com/example/widgets/actions/runs/4241"},
  {"databaseId": 4242, "workflowName": "Deploy", "event": "workflow_run", "status": "completed", "conclusion": "failure", "url": "https://github.com/example/widgets/actions/runs/4242"}
]

$ gh run view 4242 --log-failed
deploy  Smoke test  GET https://widgets.example.com/healthz -> 500
deploy  Smoke test  Error: missing required env var PAYMENTS_WEBHOOK_SECRET
deploy  Smoke test      at requireEnv (src/config.ts:4:11)
deploy  Rollback    restored release 2026-10-02.3

$ gh api repos/example/widgets/rules/branches/main --jq '.[].type'
pull_request
required_status_checks
=============== END FILE ===============
