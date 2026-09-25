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
