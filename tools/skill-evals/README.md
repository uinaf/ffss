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
need model auth (`ANTHROPIC_BASE_URL` + `ANTHROPIC_AUTH_TOKEN` for a gateway,
or the local Claude Code session) and stay operator-run. The judge always
grades through that Anthropic selection; `--harness codex` or
`--harness cursor` additionally needs the matching CLI login (or
`OPENAI_API_KEY` / `CURSOR_API_KEY`) for the agent leg, e.g.
`npm run sweep -- --harness cursor --agent composer-2.5`. Details in the
[skillcheck docs](https://github.com/uinaf/skillcheck/tree/main/docs).
