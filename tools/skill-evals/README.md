# skill-evals

Promptfoo harness for the `skills/*/evals/` fixture corpus (uinaf/ffsstack#10).
Each scenario is `task.md` (problem + embedded input files) + `criteria.json`
(weighted checklist). A run materializes the inputs and the skill under test
into a scratch workdir, drives an agent against it, and grades the written
deliverables with one `llm-rubric` per checklist item (weight = `max_score`,
assert-set threshold 0.7) plus a mandatory `skill-used` check.

## Commands

```sh
npm install            # once
npm run typecheck

# One scenario (default: claude harness, claude-opus-5 agent + judge)
node evals.ts run ../../skills/slopspec/evals/single-item-minimality
node evals.ts run <scenario-dir> --agent MODEL --judge MODEL --harness claude|codex

# Every scenario under skills/*/evals and cli/*/skills/*/evals,
# sequentially, skipping ones with existing results (rerun with --all)
node evals.ts sweep [--all]

# Reduce results/*.json into a committed scorecards/<UTC-date>.json
node evals.ts summarize [--allow-mixed]
```

`results/` and `scratch/` are disposable and gitignored; `scorecards/` is
committed.

## Provenance

Each run writes a `results/<name>.meta.json` sidecar recording the
`skills_tree_sha` (repo HEAD at run time). `summarize` reads per-result
provenance and refuses to mix revisions unless `--allow-mixed` (the scorecard's
top-level `skills_tree_sha` then becomes `mixed`; per-entry shas remain).
Results predating the sidecar mechanism are labeled `unattested-799dab4`: they
all ran against 799dab4's skills tree, but that is operator attestation, not
machine-recorded.

## Auth

- **Agent (claude harness) + judge:** with `ANTHROPIC_API_KEY` set, the judge
  uses `anthropic:messages:<model>`; without it, both legs run through
  `anthropic:claude-agent-sdk` with `apiKeyRequired: false` (local Claude Code
  session / subscription auth).
- **Agent (codex harness):** `openai:codex-sdk` reuses the local `codex` CLI
  login (`CODEX_HOME`, default `~/.codex`) — ChatGPT subscription auth — or
  `OPENAI_API_KEY` if set. Omitting `--agent` uses the Codex CLI's current
  default model. The judge stays on the Anthropic selection above.
