# Tests

Clean the diff's tests; the whole suite only when asked. A good test fails
when a contract breaks and survives refactors
([Change-Detector Tests Considered Harmful](https://testing.googleblog.com/2015/01/testing-on-toilet-change-detector-tests.html)).

- **Exact strings, snapshots, interaction checks** stay when they protect an
  owned contract (protocol values, legal copy, audit events). Assert the
  fields that matter.
- **Source-shape policy** goes in a linter, type, or build check.
- **Mocks** sit at process, network, clock, or randomness boundaries and
  never implement the behavior under test.
- **Expected values** come from a spec, fixture, captured real I/O, or a
  property. Values copied from the subject's output, or snapshots accepted on
  first run, assert nothing.
- **Skips** keep an owned reason, or get reported.
- **Negative cases** fail for the reason they name.
- **A test that breaks after a code change:** fix the code, unless the
  contract changed on purpose or the test pins implementation; then update
  it and say so.
- **New tests** name the contract and the regression that turns them red. One
  per contract, at the highest layer the gate can afford: real surface, owned
  boundary, property, then unit for pure logic. A regression test fails on the
  pre-fix code.

## Pruning a suite

1. **Target.** Name the owners in scope and a cost goal (CI seconds, test
   lines), never a deletion count. Report the coverage change but don't gate
   on it: tests that mirror the code inflate coverage. Stop short rather than
   cut an uncertain test.
2. **Baseline** at a pinned revision: counts, test and support lines,
   coverage from the repo's command, per-file results. Failing tests are
   defect candidates, not deletions.
3. **Read by owner.** Split along production owners; read each test with its
   owner.
4. **Mark each test:** R (retain: the contract and the bug it catches), F (fix
   a vacuous assertion), C (consolidate: name the absorbing keeper), D (delete:
   name the remaining proof, or why no contract exists). Missing evidence, keep
   it. Then cut redundant layers: one contract replayed through mocks around a
   stronger boundary suite.
5. **Remove dead seams:** test-only exports, flags, and support files a removed
   test kept alive; run the repo's dead-code check. Report published symbols
   instead.
6. **Prove keepers:** break the owner once per moved contract, confirm the
   keeper fails, restore. Then have a different model review deleted coverage
   against keepers (slopguard).

Keep tests that guard a public API, protocol, config, migration, storage,
security, platform, or release contract, even if they look like source
checks. Static or slow is not a reason.

Report baseline and final counts, lines (production and test apart), and
coverage; what was removed; look-alikes kept and why; seams removed;
mutations caught; defects found.
