---
name: slopmachine
description: "Execute an agreed plan through the slopmachine CLI when its governed workflow is requested. Not for ordinary edits or planning."
---

# Slopmachine

The CLI owns state and transitions; you implement and supply real evidence.

Use the installed binary (`command -v slopmachine`, `slopmachine version`).
If missing, report it and use the approved host installation; never invent a
second runtime.

## Select the work

`slopmachine repo show --json` for the profile, then:

```bash
slopmachine status --json --fields state,run_id,next_action,allowed_commands,required_evidence,intake_revision,required_reviewers,completed_reviewers,delivered_units,delivery_mode,blocker,decision_question,evidence_verification,route_ready,routing_policy_version
```

- Unregistered repo, new run, or intake awaiting release:
  [setup.md](references/setup.md). Several open runs and no task match: ask
  which to resume; never replace a blocked run.
- Before the first mutation: [protocol.md](references/protocol.md) for
  dry-run validation, stdin payloads, and storage rules.
- After release or on resume: [execution.md](references/execution.md) for
  verification, review, delivery, recovery; [status.md](references/status.md)
  for field meanings and delivered-unit signals.

## Authority and continuation

Run, start, execute, continue, or resume releases the intake matching the
agreed plan. Preparation or inspection stops at `AWAITING_RELEASE`. Ask before
releasing a materially different intake; never re-ask for an authorized one.

Obey `next_action`, `allowed_commands`, and `required_evidence`. Validate each
mutation with `--dry-run --json`, then apply. Use the returned status; re-read
after errors, plain output, or external changes. A dry-run is not persisted.

Continue build, verification, review, rework, and delivery as status permits.
Reuse accepted evidence on unchanged state; refresh when invalidated.
`AWAITING_SIGNALS` is active: observe the forge and work other ready units.

Finish at `RUN_DONE`. Stop earlier only for missing release authority, a
pending decision, or a blocker needing human recovery. Report the outcome or
exact blocker with waiting change-request URLs.
