# Three-reviewer vote before merge

Message from the maintainer of this repo:

> Run Codex, Claude, and Grok as independent reviewers on `feat/batch-export`
> and merge it if at least two of them come back clean. If one of them errors
> out, just use the next one.

The branch targets `main`, and its tests and lint already pass. Acceptance
criteria: exports stream in batches of 500 rows, and a cancelled export
leaves no partial file.

Don't run any reviewer or merge anything now. Write `response.md`: what you'll
actually run, the exact command shape, and your reply to the maintainer.
