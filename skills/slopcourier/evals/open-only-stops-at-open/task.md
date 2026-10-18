done with the backoff change, tests pass. open the pr

The branch `feat/retry-backoff` on `example/shipper` is pushed; its log, diff
summary, and test run are below. Don't call the forge: write the title and
body you would use to `pr.md` and the exact command to `actions.md`.

=============== FILE: branch.txt ===============
$ git log --oneline origin/main..feat/retry-backoff
9f8e7d6 feat(retry): back off exponentially between attempts
4c3b2a1 test(retry): cover backoff timing and the attempt cap

$ git diff --stat origin/main...feat/retry-backoff
 src/retry.ts       | 14 +++++++++++---
 test/retry.test.ts | 41 +++++++++++++++++++++++++++++++++++++++++
 2 files changed, 52 insertions(+), 3 deletions(-)

$ pnpm test
 ✓ test/retry.test.ts (6 tests) 412ms
 Test Files  4 passed (4)
      Tests  23 passed (23)

Notes: delay doubles from 200 ms, capped at 5 s, with ±20% jitter. Fixes #51
(retries hammered the upstream during its outage).
=============== END FILE ===============
