# Test patterns

The diff's tests are the unit; take the whole suite only when asked. A slop
test is worse than no test: it adds run time and false confidence while
proving nothing. A useful test fails when an observable contract breaks and
survives a refactor that preserves that contract.

## Change detectors

A change detector restates the implementation in test syntax. It fails on
harmless edits without gaining a better chance of catching a defect:

- Reading source, CSS, templates, or generated files and matching private
  literals, selectors, helper names, or line structure instead of exercising
  the supported interface.
- Pinning exact UI copy, error prose, class names, test IDs, serialized order,
  or an entire object when those details are not the contract.
- Mocking every collaborator and replaying the function body as call counts,
  arguments, and ordering. The test proves today's wiring, not the result.
- Large snapshots whose meaningful fields would fit in a direct assertion.
  Any unrelated field or formatting change becomes test maintenance.

Use the counterfactual: rename a private helper, extract a function, reorder
independent work, or change non-contract copy and styling. If behavior stays
the same but the test fails, replace the test. If behavior can break while it
still passes, delete or strengthen it.

Fix: drive the public seam and assert the smallest complete set of observable
outcomes. Prefer error kinds to prose, roles and state to CSS classes, action
intent to button copy, and relevant fields to whole-value snapshots. Use the
real collaborator or a small in-memory fake when it is cheap and deterministic.
Keep exact strings, ordering, serialization, or interaction assertions only
when that detail is itself the owned contract, such as a protocol value, legal
copy, audit event, or notifier call.

When source shape really is policy, enforce it with the repository's linter,
type system, build, or an abstract syntax tree check. Raw substring checks are
a last resort, not a behavior test.

This is the failure mode described in Alex Eagle's
[Change-Detector Tests Considered Harmful](https://testing.googleblog.com/2015/01/testing-on-toilet-change-detector-tests.html).

## Tautology

- Expected value computed by the code under test: `expect(f(x)).toEqual(f(x))`,
  or the expected literal copy-pasted from the function's own output.
- Asserting a mock returns what the test configured it to return.
- Vacuous assertions as the only check: `toBeDefined`, `not.toBeNull`,
  "does not throw", `expect(true).toBe(true)`.
- Snapshots of values the test itself constructs; they lock in whatever the
  code did and prove nothing.
- Testing the language or framework: setter/getter round-trips,
  `JSON.parse`, default struct values.

Fix: state the expected value independently of the implementation and assert
the observable contract. When no independent expectation exists, the test is
decoration; delete it.

## Overmocking

- Every collaborator mocked, so no real code path runs; the test proves
  wiring, not behavior.
- Mock call counts or "called with correct arguments" as the primary
  outcome when the interaction is not the contract.
- Mocks of pure functions, value objects, or the standard library.
- Mock return values that mirror the assertion: tautology in disguise.

Fix: use the real collaborator or an in-memory fake when it is cheap and
deterministic. Mock only process, network, clock, and randomness boundaries;
assert outputs and state. Interaction assertions stay only where the call
itself is the contract (a notifier, an audit log), and cover only the required
payload or ordering.

## Repetition

- Copy-pasted test bodies differing by one literal: table-drive or
  parametrize.
- Setup duplicated per test: one builder or fixture with per-test deltas;
  move a `beforeEach` only some tests use into those tests.
- Giant inline fixtures re-declared in every test: one named fixture.
- N happy-path variants with cosmetic input changes and no boundary or
  failure case: keep one, add the missing edge or report the gap.

## Other tells

- Names narrating implementation ("calls repo.save with mapped dto")
  instead of the behavior the test proves.
- `skip`/`todo`/commented-out tests added "for later": file a tracker item
  or delete.
- Sleeps and arbitrary timeouts calming flakiness: fix the wait condition
  or report the flake; never widen the timeout.
- Conditional assertions (`if (result) expect(...)`) that pass silently on
  the other branch.
- `try/catch` swallowing the failure, then asserting a boolean.
- Assertions on exact UI copy, log output, or error prose the contract does
  not own.
- Debug leftovers: `console.log`, focused tests (`.only`) that shrink the
  suite.
- Test additions notably larger or more complex than the change they verify:
  scope expanded through the suite instead of the source; trim to the changed
  behavior's contract.

## Stop conditions

- Never weaken a real assertion, and never delete the only coverage of a
  behavior; replace it with a real test in the same pass or report the gap.
- Before deleting an exact string, ordering, snapshot, or interaction check,
  state what contract it protects. Preserve it when that detail is the
  contract; otherwise keep the semantic behavior and drop the incidental pin.
- Whether the code under test is correct is slopguard's lane; a test that
  reveals a real defect is a finding, not a cleanup.
