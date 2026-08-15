# Status contract

Use a field mask as the agent leash:

```bash
slopmachine status --json --fields state,run_id,next_action,allowed_commands,required_evidence,intake_revision,required_reviewers,completed_reviewers,delivered_units,delivery_mode,blocker,decision_question,evidence_verification
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
| `blocker` | Human-facing blocker reason when present |
| `decision_question` | Pending ask; answer via `slopmachine decide` |

Before the repository has a run, status returns `state: "UNINITIALIZED"`,
`allowed_commands: ["init"]`, and `next_action: "slopmachine init"`. Run that
command, then read status again before submitting intake.

## Parse example

```bash
slopmachine status --json
# → {
#   "schema_version": 3,
#   "next_action": "slopmachine build --run='demo'",
#   "allowed_commands": ["intake", "ask", "build"],
#   "required_evidence": [],
#   "intake_revision": 1
# }
slopmachine build --run demo
slopmachine status --json
# → {
#   "next_action": "slopmachine verify --cmd '<verification command>' --run='demo'",
#   "required_evidence": ["verify.command", "verify.exit_code"]
# }
slopmachine verify --cmd 'go test ./...' --run demo
```

A successful mutation invoked with `--json` already returns its resulting
status document. A dry-run projection additionally includes `dry_run: true`
and `validated_command`; it does not represent persisted state. Re-read status
after plain output or an error before choosing the next step.
`verify --cmd --dry-run` cannot know the command outcome, so it keeps the
current state and adds `outcome_undetermined: true`.

Arrays are always present, including when empty. `next_action` contains a
usable command template; replace angle-bracket placeholders with real values.
Field masks validate every requested name and omit optional fields that are not
present in the canonical status document; they never synthesize `null` values.
Structured intake, review, and delivery actions use stdin through `--file -` or
`--evidence -`; do not create the payload in the repository.
Delivery requires one clean result from every identity in
`required_reviewers`. Repeating the same reviewer does not satisfy the gate.
