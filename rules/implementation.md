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
- Keep secrets and sensitive payloads out of logs and artifacts.
- Comment only invariants and external constraints the code can't express.
- When behavior changes, update the owning doc to state current behavior,
  without citing issues or change requests.
