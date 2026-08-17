# AGENTS.md

Contributor guidance for `slopmachine`.

- North star: plan → `/slopmachine` → clarify → human releases → machine runs.
- Binary owns the state machine, schemas, and sqlite store; the skill is thin
  and drives the CLI. Do not invent a second runtime in markdown or scripts.
- Prefer structured I/O (enums, JSON schemas, fail-closed validation).
- Keep `status` compact; obey `next_action`; do not narrate phase theater.
- Reference companion tools (`slopguard` CLI, Cursor Bugbot) as installed
  programs when needed; do not embed a review engine here.
- Check reality before editing docs or examples; keep commands repo-valid.
- Keep docs current-state only; omit upgrade, migration, compatibility, and
  legacy-install troubleshooting guidance.
