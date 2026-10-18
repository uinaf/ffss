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
- **Skips** keep an owned reason, or get reported.
- **Negative cases** fail for the reason they name.
- **A test that breaks after a code change:** fix the code, unless the
  contract changed on purpose or the test pins implementation; then update
  it and say so.
- **New tests** name the contract and the regression that turns them red. One
  per contract, at the real boundary. A regression test fails on the pre-fix
  code.

## Pruning a suite

1. **Target.** Set it before editing, e.g. drop the least useful 20% with line
   coverage within 2 points. Stop short rather than cut an uncertain test.
2. **Baseline** at a pinned revision: counts, test and support lines,
   coverage from the repo's command, per-file results. Failing tests are
   defect candidates, not deletions.
3. **Read by owner.** Split along production owners; read each test with its
   owner.
4. **Justify each cut:** what it detects, and the keeper that still proves
   the contract. Missing either, keep it.
5. **Remove dead seams:** test-only exports and flags a removed test kept
   alive. Report published symbols instead.
6. **Prove keepers:** break the owner once per moved contract, confirm the
   keeper fails, restore.

Keep tests that guard a public API, protocol, config, migration, storage,
security, platform, or release contract, even if they look like source
checks. Static or slow is not a reason.

Report baseline and final counts, lines (production and test apart), and
coverage; what was removed; look-alikes kept and why; seams removed;
mutations caught; defects found.
