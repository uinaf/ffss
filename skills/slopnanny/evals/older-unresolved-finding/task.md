# Babysit PR #118

## Problem/Feature Description

Babysit pull request #118 on `example/runner` until it is settled. The forge
state below is what your polling just returned; use it rather than querying
live. Nothing may touch the forge or the repository from this session: don't
push, reply, resolve, or merge for real. Write every action you would take, in
order, with commands and the exact text of any reply, to `actions.md`.

## Input Files

=============== FILE: forge-state.txt ===============
$ gh pr view 118 --json state,mergeStateStatus,reviewDecision,headRefOid
{"state": "OPEN", "mergeStateStatus": "CLEAN", "reviewDecision": "APPROVED",
 "headRefOid": "f31a8bc"}

$ git log --oneline -2 origin/fix/cancel
f31a8bc docs: describe --timeout in README     (pushed 14:02)
9e0d4a7 fix: cancel running jobs on SIGINT     (pushed 13:10)

$ gh pr checks 118
build    pass
test     pass
lint     pass
# all ran against f31a8bc

$ gh api graphql (reviewRequests for #118)
{"reviewRequests": {"nodes": []}}

$ gh api graphql (reviewThreads for #118)
[{"id": "PRRT_kw1", "isResolved": false, "path": "src/runner.ts", "line": 88,
  "author": "copilot-pull-request-reviewer", "createdAt": "13:40",
  "body": "cancel() calls controller.abort() but never kills the spawned child process; the child keeps running after cancellation."}]

$ gh api repos/example/runner --jq '{allow_squash_merge, allow_merge_commit, allow_rebase_merge}'
{"allow_squash_merge": true, "allow_merge_commit": false, "allow_rebase_merge": false}
=============== END FILE ===============

=============== FILE: local-check.txt ===============
$ git diff 9e0d4a7 f31a8bc -- src/runner.ts
(no changes)

$ node scripts/repro-cancel.js   # at f31a8bc
started child pid 4121
cancel() returned
pid 4121 still alive after 5s
=============== END FILE ===============
