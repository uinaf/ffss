# Test patterns

Clean the diff's tests; take the whole suite only when asked. Useful tests fail
when observable contracts break and survive behavior-preserving refactors.

## Change detectors

Replace assertions about private source literals, selectors, helper names,
incidental copy, or mock choreography with the smallest complete set of public
outcomes. Rename a private helper or reorder independent work mentally: should
that make this test fail? Could behavior break while the test still passes?

Exact strings, ordering, serialization, snapshots, and interaction checks stay
when they protect an owned contract, such as protocol values, legal copy, an
audit event, or a notifier. Prefer relevant fields over whole objects where the
rest is incidental. Source-shape policy belongs in a linter, type system, build,
or syntax-aware check; substring checks are a last resort, not runtime proof.

See [Change-Detector Tests Considered Harmful](https://testing.googleblog.com/2015/01/testing-on-toilet-change-detector-tests.html).

## Tautologies and mocks

- Replace expected values computed by the implementation, configured mock return
  values asserted against themselves, and snapshots of test-constructed values
  with independent expectations.
- Existence or “does not throw” assertions are weak when behavior requires a
  particular result; strengthen them rather than treating every such assertion
  as inherently useless.
- Exercise real collaborators or small in-memory fakes when cheap and
  deterministic. Mock process, network, clock, or randomness boundaries as needed;
  other mocks need a concrete isolation benefit, not a blanket ban.
- Assert call details only when the interaction itself is the contract.

## Repetition and failure handling

Collapse cosmetic happy-path clones and large repeated fixtures when doing so
makes the cases clearer. Keep distinct boundaries and failure cases explicit;
shared builders and parameterization are not goals themselves.

Remove accidental focused tests and debug leftovers. Fix wait conditions behind
flaky sleeps; don't widen timeouts to hide failures. Conditional assertions and
catch blocks must not silently pass on an unasserted path. Preserve intentional
skips with an owned reason; report stale ones instead of inventing coverage.

## Boundaries

Never weaken a real assertion or delete a behavior's only coverage. Replace it
in the same pass or report the gap. Before removing a pinned detail, identify
the contract it protects. Tests that expose real defects are findings, not
cleanup opportunities. Keep additions scoped to changed behavior and existing
coverage gaps; elaborate new test infrastructure is a signal to shrink the pass.
