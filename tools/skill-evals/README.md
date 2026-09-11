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
npm run audit                                 # CI lane; fails on high or critical
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

## Security overrides

`package.json` lifts two transitive pins past their advisories. Each key carries
the vulnerable range, so the override stops applying once the dependent that
holds the version back declares a patched range of its own:

- `adm-zip@<0.6.1`, held by `onnxruntime-node`:
  [GHSA-xcpc-8h2w-3j85](https://github.com/advisories/GHSA-xcpc-8h2w-3j85),
  [GHSA-vwc7-r8mq-g2x9](https://github.com/advisories/GHSA-vwc7-r8mq-g2x9).
- `sharp@<0.35.4`, held by `@huggingface/transformers`:
  [GHSA-rgj7-g3m4-5g8c](https://github.com/advisories/GHSA-rgj7-g3m4-5g8c).

`npm run audit` fails on a high or critical advisory and runs in CI beside
`npm run lint`, so a regression cannot land unnoticed.

Details: [skillcheck docs](https://github.com/uinaf/skillcheck/tree/main/docs).
