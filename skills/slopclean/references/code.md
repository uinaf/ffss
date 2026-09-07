# Code patterns

The diff is the unit. Clean what the change added; leave the neighborhood
alone unless asked.

## Comments

- Narration of the next line ("// increment the counter"): delete.
- Change-history narration ("// updated to use the new API", "// fix for
  review feedback"): delete; git owns history.
- Reviewer-directed justifications ("// this is safe because the caller
  checks"): prefer a precise name; keep comments for constraints the code
  cannot express. Adding an assertion is not cleanup if it changes reachable
  behavior.
- Doc comments that restate the signature: write the contract or delete.

## Structure

- Call count alone does not make an abstraction wasteful. Keep a helper that
  names a domain operation, validates input, or owns resource cleanup; inline
  private indirection that only forwards when removing it simplifies the flow.
- Defensive checks for states the type system or an upstream gate already
  excludes: remove only after proving the state unreachable; preserve runtime
  validation at external input boundaries.
- Pass-through wrappers, needless interfaces, single-variant enums added
  "for flexibility": collapse to the concrete thing.
- Proven-unused private flags, options, and escape hatches: delete. An exported
  option or accepted input is a public contract even when this module ignores
  it; preserve it and report removal as separate scope.
- Abstractions that contradict themselves or the surrounding module (a name,
  type, or doc promising one shape while callers pass another): reconcile the
  contract or report the mismatch.

## Naming and shape

- Hedged names (doSomethingSafely, tryProcessMaybe, helper, util, manager):
  name the actual behavior.
- Names that lie (a `validate` that mutates, a `getUser` that creates, a name
  that needs an "actually..." aside): rename internals to the observed
  behavior; public names are API, so report the lie instead.
- Error messages that apologize or narrate ("something went wrong while
  attempting"): state the operation, the input, and the failure.
- Dead symmetry: unreachable branches or cases kept "for completeness";
  delete with the reasoning in the commit message.

## Docs in the diff

- README/CHANGELOG filler added alongside code ("this powerful new
  feature"): prose mode applies.
- TODO with no owner or ticket: file it or delete it.

## Stop conditions

- A pattern that hides a real defect (a defensive check masking a reachable
  state) is a finding, not a cleanup; report it.
- Behavior and public API never change in a cleaning pass; test changes are
  [test mode](tests.md)'s lane with its own stop conditions.
