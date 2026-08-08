# Status contract

`slopomatic status --json` is the agent leash. Prefer these fields:

| Field | Use |
| --- | --- |
| `next_action` | Preferred next CLI invocation (e.g. `slopomatic build`) |
| `allowed_commands` | Only run commands from this list |
| `required_evidence` | Evidence keys needed for verify / review / deliver |
| `intake_revision` | Pass to `slopomatic release --revision` |
| `blocker` | Human-facing blocker reason when present |
| `decision_question` | Pending ask; answer via `slopomatic decide` |

## Parse example

```bash
slopomatic status --json
# → {
#   "next_action": "slopomatic build",
#   "allowed_commands": ["build", "ask"],
#   "required_evidence": [],
#   "intake_revision": 1
# }
slopomatic build
slopomatic status --json
# → {
#   "next_action": "slopomatic verify",
#   "required_evidence": ["verify.command", "verify.exit_code"]
# }
slopomatic verify --cmd 'go test ./...'
```

After every command, re-read status before choosing the next step.
