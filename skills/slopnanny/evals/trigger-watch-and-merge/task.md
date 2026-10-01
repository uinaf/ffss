57's up. keep an eye on it, handle whatever comes in, merge when it's green

This is what polling `example/shipper` just returned; treat it as current
instead of querying live. Make code changes in the working directory, and
write every forge command and any reply you would post to `actions.md`
instead of running them. Anything that happens after your push is unknown:
say what you would wait for.

=============== FILE: forge-state.txt ===============
$ gh pr view 57 --json state,mergeStateStatus,reviewDecision,headRefOid,author
{"state": "OPEN", "mergeStateStatus": "CLEAN", "reviewDecision": "", "headRefOid": "a1b2c3d", "author": {"login": "you"}}

$ gh pr checks 57
build  pass  58s
test   pass  2m11s

$ gh api graphql (reviewThreads for #57, unresolved)
{"reviewThreads": {"nodes": [{"id": "PRRT_1", "isResolved": false, "path": "src/retry.ts", "line": 3,
  "comments": {"nodes": [{"author": {"login": "chatgpt-codex-connector[bot]"},
  "body": "P2: `attempt <= maxAttempts` runs one extra attempt; with maxAttempts=3 the loop tries 4 times. Use `<`."}]}}]}}

$ gh api repos/example/shipper --jq '{allow_squash_merge, allow_merge_commit, allow_rebase_merge}'
{"allow_squash_merge": true, "allow_merge_commit": false, "allow_rebase_merge": false}
=============== END FILE ===============

=============== FILE: src/retry.ts ===============
export async function withRetry<T>(fn: () => Promise<T>, maxAttempts = 3): Promise<T> {
  let lastError: unknown;
  for (let attempt = 0; attempt <= maxAttempts; attempt++) {
    try {
      return await fn();
    } catch (error) {
      lastError = error;
    }
  }
  throw lastError;
}
=============== END FILE ===============
