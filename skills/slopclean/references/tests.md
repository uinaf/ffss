# Test patterns

Clean the diff's tests; take the whole suite only when asked. Useful tests fail
when observable contracts break and survive behavior-preserving refactors
([Change-Detector Tests Considered Harmful](https://testing.googleblog.com/2015/01/testing-on-toilet-change-detector-tests.html)).

- Exact strings, ordering, serialization, snapshots, and interaction checks
  stay when they protect an owned contract, such as protocol values, legal
  copy, an audit event, or a notifier; prefer the relevant fields over whole
  objects.
- Source-shape policy belongs in a linter, type system, build, or syntax-aware
  check, not a substring test.
- Mock process, network, clock, or randomness boundaries; other mocks need a
  concrete isolation benefit.
- Preserve intentional skips with an owned reason; report skips without one
  instead of deleting them or inventing coverage.
- Mocks must not implement the behavior under assertion, and fixtures must not
  supply the ordering, receipt, or callback the owner should produce.
- A negative case must fail for the reason it names, not a rejection from a
  different guard.

## New tests

A new test names the contract it protects, the credible regression that turns
it red, and why existing coverage misses it. Give each contract one primary
test at the strongest cheap boundary; another layer needs its own risk. A test
that needs a production seam no production caller uses belongs at the real
boundary. A bug's regression test fails on the pre-fix code; one at the owning
boundary covers it.

## Regression guards

An existing test that fails after a code change is evidence first. Unless the
contract intentionally changed, fix the code. When it did change, or the test
pins implementation rather than behavior, update the assertion to the contract
and report the change.

## Pruning a suite

When asked to prune a suite or area, fix a measurable target before editing,
such as removing the least useful 20% of tests while line coverage stays within
2 points. An open-ended cleanup stops far too early. The target is not a
quota: stop short of it rather than delete uncertain tests.

1. Baseline test count, test and support lines, coverage from the repository's
   own command, and each file's result at a pinned revision. Baseline failures
   are defect candidates, not deletions.
2. Split large surfaces along production owner boundaries, not file names, and
   give each owner one read-only pass that reads every test and its owner.
3. Before editing, record for each candidate the failure it can detect and the
   keeper that still proves the contract, or why no contract exists. Without
   both, keep it.
4. Delete the private test-only exports, flags, and wrappers a removed test
   kept alive, and private code whose only callers were tests. A published
   symbol may have callers outside the repository; report it instead.
5. Coverage counts executed lines, not assertions. For each contract moved to a
   keeper, mutate the production owner once, confirm the keeper fails, then
   restore the source.

Keep a test that independently guards a public API, protocol, config,
migration, storage, security, platform, or release contract, even when it
reads like a source check. Static or slow is not a deletion reason.

Report the baseline and final counts, lines, and coverage, with production and
test lines separate. Also report removed categories, retained look-alikes and
why they stay, seams removed, mutations caught, and defects found.
