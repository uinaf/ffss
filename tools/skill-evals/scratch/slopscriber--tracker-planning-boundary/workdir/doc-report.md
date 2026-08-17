# Doc report

## Changed

- `README.md` — replaced the stale `scripts/bootstrap.sh` instruction with `npm run setup`, the setup command declared in `package.json`.

`package.json` stays the single home for the command string; `README.md` invokes it by script name rather than copying `node scripts/setup.mjs`.

## Verified

- `test -e scripts/bootstrap.sh` → absent; the removed command referenced a dead path.
- `rg -n "setup" package.json` → `"setup": "node scripts/setup.mjs"` is the only declared setup entrypoint.
- `rg -n "bootstrap" README.md AGENTS.md` → no remaining references.
- `rg -n "npm run setup" README.md AGENTS.md package.json` → one home, no conflicting copy.

Not verified: `npm run setup` was not executed. `scripts/setup.mjs` is not present in this worktree, so the command is documented from `package.json` as declared, not from a successful run. Confirming the script exists is code work, not docs work.

## Gaps

- `AGENTS.md` and `README.md` are current and non-duplicating; no further doc drift found.

## Next action (not done here)

The agreed rollout plan in `AGREED_ROLLOUT.txt` is still conversation-only. It is a tactical work plan — three independently scheduled stages that must be resumable by another agent — and `AGENTS.md` states that engineering work and rollout plans are tracked in Linear team APP, while checked-in docs describe current behavior and operating procedures.

So its durable home is Linear, not this repo. Persisting it is a work-tracking operation, deliberately excluded from this documentation change:

- File the three stages in Linear team APP, one issue per independently scheduled stage, with enough per-stage state for another agent to resume.
- Do not add a checked-in plan file; that would create a second backlog alongside Linear.
- If the rollout later changes how the system behaves or how it is operated, document that outcome here as current state — separately from the plan itself.
