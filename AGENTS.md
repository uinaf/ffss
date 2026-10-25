# Agent Guide

`uinaf/ffss` ships the `slopguard` CLI, the ffss agent skills, and the shared
agent rules. The [README](README.md) covers install.

## Layout

| Path | Owns |
|---|---|
| [skills/](skills/) | Skill packages; eval scenarios in `<skill>/evals/<scenario>/` (`task.md`, `criteria.json`) |
| [cli/slopguard/](cli/slopguard/) | Go review CLI, with its own [agent guide](cli/slopguard/AGENTS.md) and [contributing](cli/slopguard/CONTRIBUTING.md) |
| [cli/lib/](cli/lib/) | Shared Go module for the CLIs; no binary |
| [rules/](rules/) | Global agent rules, one standalone file per topic. Consumers fetch them raw from `main` and combine them in their own order; skills must work without them |
| [plugin.json](plugin.json), [.claude-plugin/](.claude-plugin/), [.cursor-plugin/](.cursor-plugin/) | Plugin manifests and the Claude marketplace |
| [tools/skill-evals/](tools/skill-evals/) | Pinned [skillcheck](https://github.com/uinaf/skillcheck) for the skills lint |

## Gates

| Change | Gate |
|---|---|
| `skills/` | `npm ci && npm run lint && npm run audit` in `tools/skill-evals` |
| `skills/slopcourier/scripts/bodycheck/`, `.../prwatch/` | `gofmt -l .`, `go vet ./...`, and `go test ./...` in each |
| `cli/slopguard/` | `mise run verify` in `cli/slopguard` |
| `cli/lib/` | `mise run verify` in `cli/lib` |
| Root docs, `rules/` | none |

Evals run outside this repo, in skillcheck's
[isolated image](https://github.com/uinaf/skillcheck/blob/main/docs/usage.md#isolated-runs);
eval engines, results, and scorecards stay out.

## Release

- Skills and plugin: merging to `main` publishes; installs pin the commit SHA.
- `slopguard`: see [Releases](cli/slopguard/CONTRIBUTING.md#releases).
- `cli/lib`: tag `cli/lib/vX.Y.Z` before a consumer depends on it. Consumers
  pin a published tag so `GOWORK=off` builds stay reproducible.
- `rules/`: consumers read `main` directly, so a merge is live on their next
  sync.
