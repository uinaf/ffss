# Test patterns

The diff's tests are the unit; take the whole suite only when asked. A slop
test is worse than no test: it adds run time and false confidence while
proving nothing.

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

Fix: use the real collaborator when it is cheap and deterministic; mock only
process, network, clock, and randomness boundaries; assert outputs and state.
Interaction assertions stay only where the call itself is the contract
(a notifier, an audit log).

## Repetition

- Copy-pasted test bodies differing by one literal: table-drive or
  parametrize.
- Setup duplicated per test: one builder or fixture with per-test deltas;
  a `beforeEach` only some tests use moves into those tests.
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
- Assertions on log output or exact error prose the contract does not own.
- Debug leftovers: `console.log`, focused tests (`.only`) that shrink the
  suite.

## Stop conditions

- Never weaken a real assertion, and never delete the only coverage of a
  behavior; replace it with a real test in the same pass or report the gap.
- Whether the code under test is correct is slopguard's lane; a test that
  reveals a real defect is a finding, not a cleanup.
