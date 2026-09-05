---
name: slopmachine
description: "Execute an agreed plan through the slopmachine CLI when its governed workflow is requested. Not for ordinary edits or planning."
---

# Slopmachine

Execute the agreed plan through the CLI's durable state and evidence gates.
The machine owns transitions; the agent implements and supplies real evidence.

Use the installed `slopmachine` binary (`command -v slopmachine` and
`slopmachine version`). If missing, report the prerequisite and use the
approved host installation workflow; do not invent a second runtime.

## Select the current work

Check the profile with `slopmachine repo show --json`, then read status to find
the run and its next action:

```bash
slopmachine status --json --fields state,run_id,next_action,allowed_commands,required_evidence,intake_revision,required_reviewers,completed_reviewers,delivered_units,delivery_mode,blocker,decision_question,evidence_verification,route_ready,routing_policy_version
```

- For an unregistered repo, new run, or intake awaiting release, read
  [setup.md](references/setup.md). If several runs are open and the task does
  not identify one, ask which to resume; do not replace a blocked run.
- Before the first mutation, read [protocol.md](references/protocol.md) for
  dry-run validation, stdin payloads, and storage rules.
- After release or when resuming execution, read
  [execution.md](references/execution.md) for verification, review, delivery,
  and recovery. Use [status.md](references/status.md) when interpreting fields
  or delivered-unit signals.

## Authority and continuation

A request to run, start, execute, continue, or resume releases the intake that
matches the agreed plan. Preparation or inspection stops at `AWAITING_RELEASE`.
Ask before releasing a materially different intake; do not ask again for an
already authorized one.

Obey `next_action`, `allowed_commands`, and `required_evidence`. Validate each
mutation with `--dry-run --json`, then apply the accepted command without
`--dry-run`. Use the returned status directly; re-read after errors, plain
output, or external changes. A dry-run projection is not persisted state.

Continue build, verification, review, rework, and delivery as status permits.
Reuse accepted evidence on unchanged state; refresh it when changes or the
machine invalidate it. `AWAITING_SIGNALS` is still active work: observe the
forge and work on other ready units while waiting.

Finish at `RUN_DONE`, when every unit is settled. Stop earlier only for missing
release authority, a pending decision, or a blocker requiring human recovery.
Report the outcome or exact blocker, with waiting change-request URLs.
