# Status contract

Use a field mask as your leash:

```bash
slopmachine status --json --fields state,run_id,next_action,allowed_commands,required_evidence,intake_revision,required_reviewers,completed_reviewers,delivered_units,delivery_mode,blocker,decision_question,evidence_verification,route_ready,routing_policy_version
```

Prefer these fields:

| Field | Use |
| --- | --- |
| `next_action` | Preferred next CLI invocation (e.g. `slopmachine build`) |
| `allowed_commands` | Only run commands from this list |
| `required_evidence` | Evidence keys needed for verify / review / deliver |
| `required_reviewers` | Registered reviewer identities the intake requires |
| `completed_reviewers` | Distinct clean reviews already recorded |
| `intake_revision` | Pass to `slopmachine release --revision` |
| `units` | Per-unit `{id, phase, attempt}`; phases: pending, active, rework, delivered, done |
| `delivered_units` | Units awaiting external signals; targets for `observe --unit` |
| `risk_tier`, `budget_tokens`, `budget_minutes` | Recorded contract signals for routing policy |
| `route_ready`, `routing_policy_version` | Whether the repo profile can resolve a versioned route |
| `blocker` | Human-facing blocker reason when present |
| `decision_question` | Pending ask; answer via `slopmachine decide` |

Before the repository has a run, status returns `state: "UNINITIALIZED"`,
`allowed_commands: ["init"]`, and `next_action: "slopmachine init"`. Run that
command, then read status again before submitting intake.

## Status freshness

- A successful mutation invoked with `--json` already returns its resulting
  status document.
- A dry-run projection additionally includes `dry_run: true` and
  `validated_command`; it does not represent persisted state.
- Re-read status after plain output or an error before choosing the next
  step.

- Arrays are always present, including when empty.
- `next_action` contains a usable command template; replace angle-bracket
  placeholders with real values.
- Field masks validate every requested name and omit optional fields that are
  not present in the canonical status document; they never synthesize `null`
  values.
- Delivery requires one clean result from every identity in
  `required_reviewers`. Repeating the same reviewer does not satisfy the
  gate.

## Delivered units

`slopmachine watch --once` observes delivered change requests and records
signals. Passes are idempotent; `--interval SECONDS` polls with bounded
iterations. An unchanged observation does not call for new verification or
review.

- `merged` settles the unit; `checks_failed`, `review_feedback`, and
  `head_moved` return it to the build loop with the cause recorded.
- A thread reopened without a new comment is not re-detected; a new comment
  is. More than ten unresolved threads may conservatively trigger one extra
  rework when the sample shifts. Honor recorded rework rather than overriding
  it as a duplicate.
- For signals the binary cannot observe (no change request URL, foreign
  forge), use `slopmachine observe` with evidence from the forge. Pass `--unit`
  when several units are delivered.
- `AWAITING_SIGNALS` means remaining units need external signals; never invent
  one or report the run done.
