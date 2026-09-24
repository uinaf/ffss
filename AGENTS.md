# Agent Guide

`uinaf/ffss` ships the `slopguard` CLI, the ffss agent skills, and the shared
agent rules. [README](README.md) covers what each piece does and how users
install it.

## Layout

| Path | Owns |
|---|---|
| [skills/](skills/) | Skill packages and their eval scenarios under `evals/` |
| [cli/slopguard/](cli/slopguard/) | Go review CLI; its own [agent guide](cli/slopguard/AGENTS.md) and [contributing](cli/slopguard/CONTRIBUTING.md) |
| [cli/lib/](cli/lib/) | Shared Go module for the CLIs, tagged `cli/lib/vX.Y.Z` ([README](cli/lib/README.md)) |
| [rules/](rules/) | Global agent rules fetched raw from `main` by [dotfiles](https://github.com/uinaf/dotfiles/blob/main/agents/rules.json) |
| [plugin.json](plugin.json), [.claude-plugin/](.claude-plugin/) | Portable and Claude-compatible plugin manifests and marketplace |
| [tools/skill-evals/](tools/skill-evals/) | npm surface for the skillcheck lint and eval harness |
| `.skillcheck/scorecards/` | Committed eval scorecards; `results/` and `scratch/` are disposable |

## Commands

| Change | Gate |
|---|---|
| `skills/`, `cli/*/skills/` | `npm ci && npm run lint && npm run audit` in `tools/skill-evals` ([evals](tools/skill-evals/README.md)) |
| `cli/slopguard/` | `mise run verify` in `cli/slopguard` |
| `cli/lib/` | `mise run verify` in `cli/lib` |

[Verify](.github/workflows/verify.yml) runs only the lanes a change touches and
requires them through its `verify` job.

## Release

- Skills and plugin: merging to `main` publishes; installs pin the commit SHA.
- `slopguard`: every push to `main` evaluates a Conventional Commits release
  ([releases](cli/slopguard/docs/RELEASES.md)).
- `cli/lib`: tag a module version before a consumer depends on it.
- `rules/`: consumers read `main` directly, so a merge is live on their next sync.
