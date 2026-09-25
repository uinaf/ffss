# Babysit PR #42 to a Settled Outcome

## Problem/Feature Description

Babysit pull request #42 on `example/api` until it is settled. The forge state
below is what your polling just returned; treat it as current rather than
querying live. Write the actions you would take — every forge command and any
replies you would post — to `actions.md` instead of running them.

## Input Files

=============== FILE: forge-state.txt ===============
$ gh pr view 42 --json state,mergeStateStatus,reviewDecision,headRefOid
{
  "state": "OPEN",
  "mergeStateStatus": "CLEAN",
  "reviewDecision": "APPROVED",
  "headRefOid": "e4f5a6b"
}

$ gh pr checks 42
build    pass  1m2s
test     pass  3m40s
lint     pass  22s

# all checks above ran against head e4f5a6b (latest push, 14:02)

$ gh api graphql (reviewThreads for #42)
{"reviewThreads": {"nodes": []}}

$ gh api graphql (reviewRequests for #42)
{"reviewRequests": {"nodes": []}}

$ gh api repos/example/api --jq '{allow_squash_merge, allow_merge_commit, allow_rebase_merge}'
{"allow_squash_merge": true, "allow_merge_commit": false, "allow_rebase_merge": false}
=============== END FILE ===============
