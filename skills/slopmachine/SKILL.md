---
name: slopmachine
description: "Runs a structured, deterministic implementation workflow via the installed slopmachine CLI: clarifies intake, gates human release, runs build and verify, records independent review evidence, and delivers a reviewed artifact. Use when the user says /slopmachine, run this plan, execute the task list, ship this in slices, implement with checkpoints, walk the plan end to end, build it with human gates, or do a governed multi-step implementation. Do not use for ad-hoc edits or planning-only work."
disable-model-invocation: true
---

# Slopmachine

Deterministic and structured approach to slop cannoning.

```text
plan → /slopmachine → clarify → human releases → machine runs
```

## Require the binary

```bash
command -v slopmachine
slopmachine version
```

If missing, stop and ask the user to install the CLI through their approved host
package or release workflow. Do not download or run installers from this skill.
Do not invent a second runtime.

## Bootstrap the run

```bash
slopmachine status --json --fields state,run_id,next_action,allowed_commands,required_evidence,intake_revision,required_reviewers,completed_reviewers,delivered_units,delivery_mode,blocker,decision_question,evidence_verification
```

If status reports `UNINITIALIZED`, obey its `slopmachine init` next action.
If it reports multiple open runs, show their IDs and ask which one to resume.
Do not replace a blocked run. If status shows `RUN_DONE` and the user requested
new work, create a new run; otherwise report the completed run.

Turn the agreed plan into intake immediately after init. Keep units concrete,
bounded, and dependency-ordered; give each unit verifiable
`acceptance_criteria` and declare the run's `risk_tier`; use stdin so the
workflow does not leave evidence scratch files in the repository:

```bash
slopmachine intake --input - --dry-run --json <<'JSON'
{
  "run": "run-id-from-status",
  "delivery_mode": "pr-hold",
  "required_reviewers": ["slopguard"],
  "risk_tier": "low",
  "series_bound": 1,
  "units": [
    {"id": "u1", "title": "Implement and verify the agreed change", "blockers": [],
     "acceptance_criteria": ["the agreed behavior is proven end to end"]}
  ]
}
JSON
```

Inspect the dry-run projection. If it matches the agreed intake, repeat the
same command without `--dry-run`. Do this for every mutation: validate first,
then apply. A dry run never advances canonical state.
`verify --cmd --dry-run` cannot predict an exit code; when it returns
`outcome_undetermined: true`, validate the command and current state instead of
expecting a post-verification transition.

Show the human a compact intake summary: units, delivery mode, required
reviewers, and exact `intake_revision`. Wait for explicit release approval,
then run the exact `slopmachine release --revision N` command printed by
`next_action`.

## Status leash

1. Run the field-masked `slopmachine status --json` command above and obey its
   next step. See
   [status.md](references/status.md) for the full status field contract.
2. Validate every mutation with `--dry-run --json`; apply only when the
   projection matches intent. Successful `--json` output is the resulting
   status document.
3. Advance only through named CLI commands with the evidence status requires.
   Re-read status after errors or whenever output was not JSON.
4. Treat placeholders in `next_action` as fields to fill, not literal text.
   Never materialize intake, review, delivery, or other evidence payloads in
   the repository; send them through stdin.
5. Before guessing a payload field or enum, run
   `slopmachine schema --command <name>`. See
   [protocol.md](references/protocol.md) for raw input and error handling.

## Machine loop

After the human releases: build → verify → review → deliver, always driven by
status.

```bash
# after verify succeeds and status asks for review
slopmachine review --input - --dry-run --json <<'JSON'
{"run":"run-id-from-status","reviewer":"slopguard","verdict":"clean","artifact_ref":"slopguard://local"}
JSON
```

Deliver only after every required reviewer is present in `completed_reviewers`.
Use stdin evidence and match the intake delivery mode:

```bash
slopmachine deliver --input - --dry-run --json <<'JSON'
{"run":"run-id-from-status","delivery_mode":"pr-hold","pr_url":"https://github.com/example/repo/pull/1"}
JSON
```

After each projection is accepted, repeat without `--dry-run`.

When status shows `evidence_verification: observed`, the binary checks
deliver and review evidence against the live forge before accepting it:
give real change-request URLs and the actual delivered head. A rejection
(exit 3) means the forge disagrees with the claim — fix the claim, never
retry with altered evidence. Exit 7 means the forge was unreachable; retry,
or ask the human before recording a bypass with `--unverified --reason`.

## Talk to the human

Collaborator voice: short prose + optional tables. Lead with what changed or
what you need — never a wall of CLI JSON. Plain words over machine dumps.

## Three human moments

1. **Release** — confirm table (what/how/review), wait, then
   `slopmachine release --revision` using `intake_revision` from status JSON.
2. **Required reviewers** — once at intake: pick from the registered
   identities (`slopmachine reviewers` lists them; `slopguard` and `bugbot`
   are built in). Store via intake `required_reviewers`. Do not auto-fire
   reviewers.
3. **Decide** — `slopmachine ask --question …`, then
   `slopmachine decide --answer …`.

## Error recovery

Failed verification records `BLOCKED` and exits 6. Show the compact failure,
re-read status, surface `blocker` verbatim, and ask how to recover. After the
human confirms the recovery, record it with `slopmachine retry --reason "…"`;
never retry silently. For a known external blocker before verification, use
`slopmachine block --reason "…"` and follow the same recovery rule.

With `--json`, failures return `error.kind`, `error.message`, and
`error.exit_code`. Malformed input exits 2; illegal transitions or unmet guards
exit 3. Fix the input or re-read status instead of bypassing the gate. Empty
next action means the run is done or needs human inspection.

## Post-delivery babysit

Delivery opens a change request; the unit is `delivered`, not settled, and
later units already build while it waits. Prefer `slopmachine watch --once`:
the binary observes every delivered unit's change request itself and records
the signals — `merged` settles the unit; `checks_failed`, `review_feedback`,
and `head_moved` pull it back through the build loop with the cause
recorded. Passes are idempotent, so rerun freely; `--interval SECONDS`
polls with bounded iterations. Two narrow feedback-identity limits: a
thread reopened without a new comment is not re-detected (any new
comment is), and a change request with more than ten unresolved threads
may conservatively re-trigger one extra rework when the sample shifts. For signals the binary cannot observe (no
change request URL, foreign forge), record what the forge really shows via
`slopmachine observe`, passing `--unit` when several units are delivered.
`AWAITING_SIGNALS` means every remaining unit waits on external signals.
Never invent a signal.

## Post-review flow

- `clean` records that reviewer. Delivery requires a distinct clean result
  from every identity in `required_reviewers`.
- `findings` moves directly to `REWORK`; summarize findings, then obey the next
  build command.
- `ambiguous` moves to `NEEDS_DECISION`; ask the human and record the answer.

## Mindful spend

Strong model for BUILD; cheaper review (Bugbot / lighter slopguard). Never
default "most expensive everywhere."

## Companion tools

Run whichever installed tool matches a required reviewer: the `slopguard`
binary, Cursor `/review-bugbot`, or the registered custom reviewer's own
surface. Never simulate a reviewer.

## Done

Stop when `RUN_DONE` (every unit settled), blocked pending human recovery,
or waiting at release or decision. `AWAITING_SIGNALS` is not done — report
which change requests still wait. SQLite holds the canonical event log.
