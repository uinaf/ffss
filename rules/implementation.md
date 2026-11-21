## Implementation

- Extend the closest owner before adding scripts, abstractions, or
  infrastructure.
- Edit generated artifacts at their source and regenerate.
- Shell for short command sequences; parsing, policy, retries, and state in
  the project's typed language.
- Validate external input at the boundary; prefer validated types to casts,
  ignores, and non-null assertions.
- Model a lifecycle whose named states matter (resume, stop, retry, inspect)
  as an explicit state machine, with side effects in its invoked actors, not
  as boolean flags checked in several places.
- Preserve error causes and partial failures. Keep retries bounded,
  cancellable, and limited to idempotent transient work. Budget them across
  layers: a client retry inside a job retry multiplies attempts.
- Run noisy commands with quiet flags, or in a delegate that returns only the
  conclusion.
- Write no code comments: names, types, and tests carry intent, and the
  commit message carries the why. Keep only tool directives, license headers,
  published API docs, and comments a lint rule requires.
- A correction overwrites, never annotates: code, tests, copy, and docs show
  current behavior, never what was removed or rejected.
- A product repository's AGENTS.md opens with what it's for, who it serves,
  and what stays out. Change that only on the user's yes.
- Docs carry only what a reader must do or must not break. Link the code, CI,
  or config that owns a fact; a restated copy is a mirror and goes stale. A
  doc beyond the standard files earns its place or folds into one.
- A code change edits a doc only where it changes what a reader must do. If a
  doc restates the changed behavior, cut that mirror instead of updating it.
