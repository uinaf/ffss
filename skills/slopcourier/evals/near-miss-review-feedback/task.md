codex left a comment on 63, fix it and get it merged

This is what polling `example/shipper` just returned; treat it as current
instead of querying live. Make code changes in the working directory, and
write every forge command and any reply you would post to `actions.md`
instead of running them. Anything that happens after your push is unknown:
say what you would wait for.

=============== FILE: forge-state.txt ===============
$ gh pr view 63 --json state,mergeStateStatus,headRefOid,title,body
{"state": "OPEN", "mergeStateStatus": "CLEAN", "headRefOid": "d4e5f6a",
 "title": "feat(queue): pause reads above the high-water mark",
 "body": "Reads pause when the outbound queue passes 80% of queue.max instead of dropping batches.\n\nVerified: pnpm test (31 passing)."}

$ gh pr checks 63
build  pass  1m01s
test   pass  2m30s

$ gh api graphql (reviewThreads for #63, unresolved)
{"reviewThreads": {"nodes": [{"id": "PRRT_9", "isResolved": false, "path": "src/queue.ts", "line": 4,
  "comments": {"nodes": [{"author": {"login": "chatgpt-codex-connector[bot]"},
  "body": "P1: the high-water mark compares against a hardcoded 100 instead of queue.max, so a non-default queue.max never pauses."}]}}]}}

$ gh api repos/example/shipper --jq '{allow_squash_merge, allow_merge_commit, allow_rebase_merge}'
{"allow_squash_merge": true, "allow_merge_commit": false, "allow_rebase_merge": false}
=============== END FILE ===============

=============== FILE: src/queue.ts ===============
export function shouldPause(queued: number, max: number): boolean {
  const ratio = 0.8;
  // pause above 80% of the configured max
  return queued > 100 * ratio;
}
=============== END FILE ===============
