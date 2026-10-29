## Implementation

- Extend the closest owner before adding scripts, abstractions, or
  infrastructure.
- Edit generated artifacts at their source and regenerate.
- Shell for short command sequences; parsing, policy, retries, and state in
  the project's typed language.
- Validate external input at the boundary; prefer validated types to casts,
  ignores, and non-null assertions.
- Preserve error causes and partial failures. Keep retries bounded,
  cancellable, and limited to idempotent transient work.
- Run noisy commands with quiet flags, or in a delegate that returns only the
  conclusion.
- Comment only invariants and external constraints the code can't express.
- When behavior changes, update the owning doc to state current behavior,
  without citing issues or change requests.
- Docs carry only what a reader must do or must not break. Link the code, CI,
  or config that owns a fact; a restated copy is a mirror and goes stale. A
  doc beyond the standard files earns its place or folds into one.
