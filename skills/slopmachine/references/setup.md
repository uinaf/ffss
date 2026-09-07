# Run setup

Read when registering a repository, creating a run, or preparing its intake.

## Bind the repo profile

```bash
slopmachine repo show --json
```

If unregistered, register the repo with its actual forge, verification command,
delivery policy, and reviewers:

```bash
slopmachine repo register --forge github --trust low \
  --verify-cmd "mise run verify" --delivery pr-hold \
  --bind review=slopguard
```

Use `--forge gitlab` for GitLab.com or self-hosted GitLab on standard HTTPS;
`glab` host selection does not support custom-port URLs.

- Map forge-resident reviewers (bots that review on the change request) with
  `--forge-reviewer identity=login` so the machine corroborates their evidence
  against the live change request.
- Low trust requires machine-executed `verify --cmd` and resolvable local
  review artifacts. Registration enables forge-verified (`observed`) evidence.

## Bootstrap the run

- `UNINITIALIZED` → obey its `slopmachine init` next action.
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

## Release the matching intake

Use the [entrypoint's authority rules](../SKILL.md#authority-and-continuation).
Report units, delivery mode, reviewers, and revision briefly. When release is
already authorized, validate the exact `slopmachine release --revision N`
command from `next_action`, apply it, and continue. Preparation-only work stops
at `AWAITING_RELEASE` with the summary and exact next action.
