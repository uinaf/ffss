---
name: slopmachine
description: "Run a governed, deterministic implementation workflow via the slopmachine CLI: intake, user-authorized release, build, verify, independent review, delivery. Use for /slopmachine, running a plan, or governed multi-step implementation; not ad-hoc edits or planning."
---

# Slopmachine

Execute the agreed plan through the CLI's durable state and evidence gates.
The machine owns transitions; the agent implements and supplies real evidence.

## Require the binary

```bash
command -v slopmachine
slopmachine version
```

If missing, ask for installation through the approved host workflow. Do not
download installers or invent a second runtime.

Read [protocol.md](references/protocol.md) before the first mutation.

## Bind the repo profile

```bash
slopmachine repo show --json
```

If unregistered, register the repo with its actual forge, verification command,
delivery policy, and reviewers. For example:

```bash
slopmachine repo register --forge github --trust low \
  --verify-cmd "mise run verify" --delivery pr-hold \
  --bind review=slopguard
```

Use `--forge gitlab` for GitLab.com or self-hosted GitLab repositories on
standard HTTPS; `glab` host selection does not support custom-port URLs.

- Map forge-resident reviewers (bots that review on the change request) with
  `--forge-reviewer identity=login` so the machine corroborates their evidence
  against the live change request.
- Low trust requires machine-executed `verify --cmd` and resolvable local
  review artifacts. Registration enables forge-verified (`observed`) evidence.

## Bootstrap the run

```bash
slopmachine status --json --fields state,run_id,next_action,allowed_commands,required_evidence,intake_revision,required_reviewers,completed_reviewers,delivered_units,delivery_mode,blocker,decision_question,evidence_verification,route_ready,routing_policy_version
```

- `UNINITIALIZED` → obey its `slopmachine init` next action.
- Multiple open runs → show their IDs and ask which one to resume. Do not
  replace a blocked run.
- `RUN_DONE` plus a request for new work → create a new run; otherwise report
  the completed run.

After init, submit bounded, dependency-ordered units with verifiable acceptance
criteria and a risk tier. Select required reviewers once from registered
identities (`slopmachine reviewers`), honoring the agreed review requirement.

```bash
slopmachine intake --file - --run run-id-from-status --dry-run --json <<'JSON'
{
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

Put only the intake document in the `--file` payload; pass the run through
`--run`, exactly as `next_action` prints it. (The raw `--input` shape embeds
`run` inside the payload instead; do not mix the two.)

Release authority comes from the user's request:

- `run`, `start`, `execute`, `continue`, or `resume` authorizes release of the
  intake that matches the agreed plan. Validate the exact
  `slopmachine release --revision N` command from `next_action`, apply it, and
  keep moving without asking again.
- `prepare`, `inspect`, `draft`, `intake`, or `dry-run` does not authorize
  release. Stop at `AWAITING_RELEASE` with a compact summary and the exact next
  action.
- If the intake materially differs from the agreed plan, show the delta and
  obtain approval before release. Do not stretch an execution request into new
  scope.

Report units, delivery mode, reviewers, and revision briefly; no second approval
prompt for the matching authorized intake.

## Status leash

- Obey `next_action`, `allowed_commands`, and `required_evidence` from the
  current status. Fill placeholders with real values.
- Validate each mutation with `--dry-run --json`, then apply the accepted
  projection without `--dry-run`. A projection is not persisted status.
- Use the resulting JSON status directly after success; do not immediately
  poll the same state again. Re-read after errors, plain output, or external
  changes. [status.md](references/status.md) defines fields and observation.
- Send payloads through stdin, never repository evidence files. Discover
  unfamiliar fields/enums with `slopmachine schema --command <name>`;
  [protocol.md](references/protocol.md) owns input and storage rules.

## Machine loop

After release: build → verify → review → deliver, driven by status.
Iterate with focused checks during BUILD; run the required verification gate
when the unit is ready, then invoke its required reviewers. Do not add review
after each edit or repeat an accepted verification/review on unchanged state.
New changes or machine invalidation require refreshed evidence at the next
gate; never substitute an earlier result for a required new attempt.

When status reports `route_ready: true`, `slopmachine route --json --run ID`
previews the deterministic route for the current or sole ready unit. Treat it
as declared execution guidance only: the command never launches a worker, and
missing policy or budget coverage is a real fail-closed blocker.

```bash
# after verify succeeds and status asks for review
slopmachine review --evidence - --run run-id-from-status --dry-run --json <<'JSON'
{"reviewer":"slopguard","verdict":"clean","artifact_ref":"file:///tmp/slopguard-result.json"}
JSON
```

Use the required reviewer's installed tool, never a simulated verdict. For
local review, save its real result outside the repository and reference it;
the machine refuses dangling or opaque refs on a forge-bound repo.

Deliver only after every required reviewer is present in `completed_reviewers`.
Use stdin evidence and match the intake delivery mode:

```bash
slopmachine deliver --evidence - --run run-id-from-status --dry-run --json <<'JSON'
{"delivery_mode":"pr-hold","pr_url":"https://github.com/example/repo/pull/1"}
JSON
```

Deliver from the built checkout: the machine anchors the change request's
head to the local head (or an explicit `commit_sha`) and refuses a change
request that is already merged or closed.

When status shows `evidence_verification: observed`, the binary checks
deliver and review evidence against the live forge before accepting it.

- Give real change-request URLs and the actual delivered head.
- Exit 3: the forge disagrees with the claim. Fix the claim, never retry with
  altered evidence.
- Exit 7: the forge was unreachable. Retry, or ask the human before recording
  a bypass with `--unverified --reason`.

## Error recovery

- Failed verification records `BLOCKED` and exits 6: show the compact
  failure, re-read status, surface `blocker` verbatim, and ask how to recover.
- After the human confirms the recovery, record it with
  `slopmachine retry --reason "…"`; never retry silently.
- For a known external blocker before verification, use
  `slopmachine block --reason "…"` and follow the same recovery rule.
- Malformed input exits 2; illegal transitions or unmet guards exit 3. Use
  `error.kind`, `error.message`, and `error.exit_code` to correct input or
  re-read status, never bypass the gate.
- Record decisions through `slopmachine ask --question …` and
  `slopmachine decide --answer …` after the human answers.
- An empty next action means the run is done or needs human inspection.

## Post-delivery babysit

Use `slopmachine watch --once` to observe delivered change requests. Delivery
does not settle the unit; later ready units may build while it waits.
[status.md](references/status.md#delivered-units) owns signals, observation
limits, and manual observation. Keep waiting at `AWAITING_SIGNALS`; it is not
completion.

## Post-review flow

- `clean` records that reviewer.
- `findings` moves directly to `REWORK`; summarize findings, then obey the next
  build command.
- `ambiguous` moves to `NEEDS_DECISION`; ask the human and record the answer.

## Done

Stop when `RUN_DONE` (every unit settled), blocked pending human recovery,
or waiting for release authorization or a decision. Report the outcome or
blocker in short prose, with waiting change-request URLs where relevant.
SQLite holds the canonical event log.
