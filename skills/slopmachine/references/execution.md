# Execution and recovery

Read after release, or when resuming a run in verification, review, delivery,
or recovery. Apply the [protocol](protocol.md) to every mutation.

## Machine loop

After release: build → verify → review → deliver, driven by status. Iterate
with focused checks during BUILD; run the required verification gate when the
unit is ready, then invoke its required reviewers. Do not review after each
edit or repeat an accepted verification/review on unchanged state. New changes
or machine invalidation require refreshed evidence at the next gate; never
substitute an earlier result for a required new attempt.

When status reports `route_ready: true`, `slopmachine route --json --run ID`
previews the deterministic route for the current or sole ready unit. It is
declared guidance only: it never launches a worker, and missing policy or
budget coverage is a real fail-closed blocker.

```bash
# after verify succeeds and status asks for review
slopmachine review --evidence - --run run-id-from-status --dry-run --json <<'JSON'
{"reviewer":"slopguard","verdict":"clean","artifact_ref":"file:///tmp/slopguard-result.json"}
JSON
```

Use the required reviewer's installed tool, never a simulated verdict. Save a
local review result outside the repository and reference it; the machine
refuses dangling or opaque refs on a forge-bound repo.

Deliver only after every required reviewer is in `completed_reviewers`, with
stdin evidence matching the intake delivery mode:

```bash
slopmachine deliver --evidence - --run run-id-from-status --dry-run --json <<'JSON'
{"delivery_mode":"pr-hold","pr_url":"https://github.com/example/repo/pull/1"}
JSON
```

Deliver from the built checkout: the machine anchors the change request's head
to the local head (or an explicit `commit_sha`) and refuses a merged or closed
change request.

With `evidence_verification: observed`, the binary checks deliver and review
evidence against the live forge.

- Give real change-request URLs and the actual delivered head.
- Exit 3: the forge disagrees with the claim. Fix the claim; never retry with
  altered evidence.
- Exit 7: the forge was unreachable. Retry, or ask the human before recording
  a bypass with `--unverified --reason`.

## Error recovery

- Failed verification records `BLOCKED` and exits 6: show the compact failure,
  re-read status, surface `blocker` verbatim, and ask how to recover.
- After the human confirms the recovery, record it with
  `slopmachine retry --reason "…"`; never retry silently.
- For a known external blocker before verification, use
  `slopmachine block --reason "…"` and follow the same recovery rule.
- Malformed input exits 2; illegal transitions or unmet guards exit 3. Use
  `error.kind`, `error.message`, and `error.exit_code` to correct input or
  re-read status; never bypass the gate.
- Record decisions with `slopmachine ask --question …` and
  `slopmachine decide --answer …` after the human answers.
- An empty next action means the run is done or needs human inspection.

## Post-delivery babysit

`slopmachine watch --once` observes delivered change requests. Delivery does
not settle the unit; later ready units may build while it waits.
[status.md](status.md#delivered-units) owns signals, observation limits, and
manual observation. `AWAITING_SIGNALS` is not completion.

## Post-review flow

- `clean` records that reviewer.
- `findings` moves to `REWORK`; summarize findings, then obey the next build
  command.
- `ambiguous` moves to `NEEDS_DECISION`; ask the human and record the answer.
