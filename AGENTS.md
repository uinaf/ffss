# Agent Guide

`uinaf/ffss` ships the ffss agent skills and the shared agent rules. The
[README](README.md) covers install.

## Boundaries

- `rules/` holds one standalone file per topic. Consumers fetch them raw from
  `main` and combine them in their own order; skills must work without them.

## Gates

| Change | Gate |
|---|---|
| `skills/` | `npm ci && npm run lint && npm run audit` in `tools/skill-evals` |
| `skills/slopcourier/scripts/bodycheck/`, `.../prwatch/` | `gofmt -l .`, `go vet ./...`, and `go test ./...` in each |
| Root docs, `rules/` | none |

Evals run outside this repo, in skillcheck's
[isolated image](https://github.com/uinaf/skillcheck/blob/main/docs/usage.md#isolated-runs);
eval engines, results, and scorecards stay out.

## Release

- Skills and plugin: merging to `main` publishes; installs pin the commit SHA.
- `rules/`: consumers read `main` directly, so a merge is live on their next
  sync.
