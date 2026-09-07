# AGENTS.md

Contributor guidance for `slopmachine`.

- North star: plan → `/slopmachine` → clarify → authorized release → machine
  runs. A request to start or continue authorizes release of the matching
  intake; do not ask twice.
- Binary owns the state machine, schemas, and sqlite store; the skill is thin
  and drives the CLI. No second runtime in markdown or scripts.
- Prefer structured I/O (enums, JSON schemas, fail-closed validation).
- Keep `status` compact; obey `next_action`; no phase theater.
- Reference companion tools (`slopguard` CLI, Cursor Bugbot) as installed
  programs; do not embed a review engine here.
- Check reality before editing docs or examples; keep commands repo-valid.
- Docs are current-state only: no upgrade, migration, compatibility, or
  legacy-install troubleshooting guidance.
