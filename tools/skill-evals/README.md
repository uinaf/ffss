# skill-evals

The lint/eval harness is [uinaf/skillcheck](https://github.com/uinaf/skillcheck),
pinned as the npm devDependency `@uinaf/skillcheck`. This directory is only
the npm surface that installs and invokes it.

In this repo:

- Scenarios, next to the skills they grade: `skills/<skill>/evals/<scenario>/`
  with `task.md` + `criteria.json`.
- Committed scorecards: `.skillcheck/scorecards/` at the repo root.
  `.skillcheck/results/` and `.skillcheck/scratch/` are disposable and
  gitignored.

## Running

From this directory:

```sh
npm ci
npm run lint                                  # CI lane; keyless and offline
npm run run -- ../../skills/<skill>/evals/<scenario>
npm run sweep                                 # resumes; only scenarios without results
npm run summarize                             # writes .skillcheck/scorecards/<UTC-date>.json
```

`lint` needs no credentials and runs in CI. Sweeps need model auth and are
operator-run. A bare `--judge` model grades through the Anthropic selection
(`ANTHROPIC_BASE_URL` + `ANTHROPIC_AUTH_TOKEN` for a gateway, or the local
Claude Code session); a provider-qualified judge uses that provider's env.
`--harness codex` or `--harness cursor` needs the matching CLI login (or
`OPENAI_API_KEY` / `CURSOR_API_KEY`) for the agent leg. A fully non-Anthropic
sweep:

```sh
OPENAI_API_KEY=… OPENAI_BASE_URL=… npm run sweep -- --harness cursor \
  --agent composer-2.5 --judge openai:chat:gpt-5.6-sol --judge-effort high
```

Details: [skillcheck docs](https://github.com/uinaf/skillcheck/tree/main/docs).
