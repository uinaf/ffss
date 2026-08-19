# skill-evals

The lint/eval harness lives in [uinaf/skillcheck](https://github.com/uinaf/skillcheck),
pinned here as an npm devDependency (`@uinaf/skillcheck`). This
directory is only the npm surface that installs and invokes it.

What stays in this repo:

- Scenarios, next to the skills they grade: `skills/<skill>/evals/<scenario>/`
  with `task.md` + `criteria.json`.
- Committed scorecards: `.skillcheck/scorecards/` at the repo root.
  `.skillcheck/results/` and `.skillcheck/scratch/` are disposable and
  gitignored.

## Running

From this directory:

```sh
npm ci
npm run lint                                  # keyless, no network — the CI half
npm run run -- ../../skills/<skill>/evals/<scenario>
npm run sweep                                 # resumes; only scenarios without results
npm run summarize                             # writes .skillcheck/scorecards/<UTC-date>.json
```

Auth is split on purpose: `lint` needs no credentials and runs in CI. Sweeps
need model auth and stay operator-run. A bare `--judge` model grades through
the Anthropic selection (`ANTHROPIC_BASE_URL` + `ANTHROPIC_AUTH_TOKEN` for a
gateway, or the local Claude Code session); a provider-qualified judge uses
that provider's env instead. `--harness codex` or `--harness cursor` needs the
matching CLI login (or `OPENAI_API_KEY` / `CURSOR_API_KEY`) for the agent leg.
A fully non-Anthropic sweep:

```sh
OPENAI_API_KEY=… OPENAI_BASE_URL=… npm run sweep -- --harness cursor \
  --agent composer-2.5 --judge openai:chat:gpt-5.6-sol --judge-effort high
```

Details in the
[skillcheck docs](https://github.com/uinaf/skillcheck/tree/main/docs).
