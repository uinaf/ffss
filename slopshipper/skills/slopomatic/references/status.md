# Status contract

Use a field mask as the agent leash:

```bash
slopomatic status --json --fields state,run_id,next_action,allowed_commands,required_evidence,intake_revision,required_reviewers,completed_reviewers,delivery_mode,blocker,decision_question
```

Prefer these fields:

| Field | Use |
| --- | --- |
| `next_action` | Preferred next CLI invocation (e.g. `slopomatic build`) |
| `allowed_commands` | Only run commands from this list |
| `required_evidence` | Evidence keys needed for verify / review / deliver |
| `required_reviewers` | Review identities required by intake consent |
| `completed_reviewers` | Distinct clean reviews already recorded |
| `intake_revision` | Pass to `slopomatic release --revision` |
| `blocker` | Human-facing blocker reason when present |
| `decision_question` | Pending ask; answer via `slopomatic decide` |

Before the repository has a run, status returns `state: "UNINITIALIZED"`,
`allowed_commands: ["init"]`, and `next_action: "slopomatic init"`. Run that
command, then read status again before submitting intake.

## Parse example

```bash
slopomatic status --json
# → {
#   "schema_version": 2,
#   "next_action": "slopomatic build --run='demo'",
#   "allowed_commands": ["intake", "ask", "build"],
#   "required_evidence": [],
#   "intake_revision": 1
# }
slopomatic build --run demo
slopomatic status --json
# → {
#   "next_action": "slopomatic verify --cmd '<verification command>' --run='demo'",
#   "required_evidence": ["verify.command", "verify.exit_code"]
# }
slopomatic verify --cmd 'go test ./...' --run demo
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
When review consent is `both`, record one clean `autoreview` result and one
clean `bugbot` result. Repeating the same reviewer does not satisfy the gate.
