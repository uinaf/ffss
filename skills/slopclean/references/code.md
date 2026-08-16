# Code patterns

The diff is the unit. Clean what the change added; leave the neighborhood
alone unless asked.

## Comments

- Narration of the next line ("// increment the counter"): delete.
- Change-history narration ("// updated to use the new API", "// fix for
  review feedback"): delete — git owns history.
- Reviewer-directed justifications ("// this is safe because the caller
  checks"): move the invariant into an assertion or a name; keep a comment
  only for constraints the code cannot express.
- Doc comments that restate the signature: write the contract or delete.

## Structure

- Abstraction with one caller and no second use in sight: inline it.
- Defensive checks for states the type system or an upstream gate already
  excludes: delete, or turn into an explicit invariant failure.
- Pass-through wrappers, needless interfaces, single-variant enums added
  "for flexibility": collapse to the concrete thing.
- Config flags, options, and escape hatches nothing reads: delete.

## Naming and shape

- Hedged names (doSomethingSafely, tryProcessMaybe, helper, util, manager):
  name the actual behavior.
- Error messages that apologize or narrate ("something went wrong while
  attempting"): state the operation, the input, and the failure.
- Dead symmetry: branches or cases kept "for completeness" that are
  unreachable — delete with the reasoning in the commit message.

## Docs in the diff

- README/CHANGELOG filler added alongside code ("this powerful new
  feature"): prose mode applies.
- TODO(comment) with no owner or ticket: file it or delete it.

## Stop conditions

- A pattern that hides a real defect (a defensive check masking an actual
  reachable state) is a finding, not a cleanup — report it.
- Behavior, public API, and test semantics never change in a cleaning
  pass.
