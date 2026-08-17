# Triage Worker Example

A Node worker driven by an external orchestrator. A request is triaged into a
typed task, dispatched to an isolated workspace, and finished unattended along one
of two result paths: `implementation` (a proven change, handed off as a pull
request) or `qa` (evidence about an existing revision or build, handed off as a
report).

## Lifecycle

```bash
pnpm bootstrap                        # validate the runner contract, install reproducibly
pnpm doctor                           # read-only: is this workspace worth driving?
pnpm verify                           # canonical gate; writes the result manifest
pnpm run teardown -- --reason success # release only what this attempt raised
```

All four are noninteractive. `pnpm bootstrap` consumes the machine identity the
runner injects (`INFISICAL_TOKEN`, `INFISICAL_API_URL`, `INFISICAL_PROJECT_ID`);
there is no login step, no `.env.local`, and no secret is ever printed.

Results land under `artifacts/<AGENT_TASK_ID>/<AGENT_ATTEMPT_ID>/`, with
`manifest.json` as the machine-readable record.

## Documentation

| Document | Contents |
| --- | --- |
| [`AGENTS.md`](AGENTS.md) | the agent operating guide: commands, authority, proof, recovery |
| [`docs/automation.md`](docs/automation.md) | the orchestrator contract: task input, states, schemas, taxonomy, ownership |
| [`readiness-report.md`](readiness-report.md) | readiness grades, evidence level, and open gaps |

## Requirements

Node >= 22, git, corepack. The pinned package-manager version comes from
`packageManager` in `package.json` and is activated by `pnpm bootstrap`.
