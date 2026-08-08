---
name: slopomatic
description: "Runs a structured, deterministic implementation workflow via the installed slopomatic CLI: clarifies intake, gates human release, runs build and verify, records independent review evidence, and delivers a reviewed artifact. Use when the user says /slopomatic, run this plan, execute the task list, ship this in slices, implement with checkpoints, walk the plan end to end, build it with human gates, or do a governed multi-step implementation. Do not use for ad-hoc edits or planning-only work."
disable-model-invocation: true
---

# Slopomatic

Deterministic and structured approach to slop cannoning.

```text
plan → /slopomatic → clarify → human releases → machine runs
```

## Require the binary

```bash
command -v slopomatic
slopomatic version
```

If missing, stop and ask the user to install the CLI through their approved host
package or release workflow. Do not download or run installers from this skill.
Do not invent a second runtime.

## Commands

```bash
slopomatic status --json
slopomatic release --revision "$INTAKE_REVISION"
slopomatic build
slopomatic verify --cmd 'go test ./...'
slopomatic review --evidence ./review.evidence.json
slopomatic deliver --evidence ./deliver.evidence.json
slopomatic ask --question "Should we merge or hold the PR?"
slopomatic decide --answer "Merge when CI is green"
slopomatic version
```

## Leash

1. Run `slopomatic status --json` and obey its next step. See
   [status.md](references/status.md) for the full status field contract.
2. Advance only through named CLI commands with the evidence status requires.
3. Re-read status after every advance. Do not invent transitions or narrate
   phase theater.

## Machine loop

After the human releases: build → verify → review → deliver, always driven by
status.

```bash
# after verify succeeds and status asks for review:
cat > ./review.evidence.json <<'EOF'
{"reviewer":"autoreview","verdict":"clean","artifact_ref":"autoreview://local"}
EOF
slopomatic review --evidence ./review.evidence.json
```

Deliver only after a recorded review, with `./deliver.evidence.json` covering
the delivery mode and PR/commit fields status requires.

## Talk to the human

Collaborator voice: short prose + optional tables. Lead with what changed or
what you need — never a wall of CLI JSON. Plain words over machine dumps.

## Three human moments

1. **Release** — confirm table (what/how/review), wait, then
   `slopomatic release --revision` using `intake_revision` from status JSON.
2. **Review consent** — once at intake: `autoreview` CLI, Cursor `/review-bugbot`,
   both, or human. Store via intake `review_consent`. Do not auto-fire reviewers.
3. **Decide** — `slopomatic ask --question …`, then
   `slopomatic decide --answer …`.

## Error recovery

Non-zero exit: show stderr, re-read status, surface the `blocker` field
verbatim when present, stop, and ask how to proceed. Never retry silently.
Empty/illegal next step: tell the human and wait.

## Post-review flow

- `verdict: clean` → summarize, then deliver when status says so.
- Findings / non-clean → summarize, ask whether to `slopomatic rework` (when
  allowed) or decide.
- Ambiguous verdict → **Decide** moment.

## Mindful spend

Strong model for BUILD; cheaper review (Bugbot / lighter autoreview). Never
default "most expensive everywhere."

## Companion tools

Installed `autoreview` binary preferred; Cursor `/review-bugbot` is a cheap
local option with consent.

## Done

Stop when finished, blocked, or waiting on the human. Sqlite holds the event log.
