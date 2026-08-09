# Status contract

`slopomatic status --json` is the agent leash. Prefer these fields:

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

After every command, re-read status before choosing the next step.

Arrays are always present, including when empty. `next_action` contains a
usable command template; replace angle-bracket placeholders with real values.
When review consent is `both`, record one clean `autoreview` result and one
clean `bugbot` result. Repeating the same reviewer does not satisfy the gate.
