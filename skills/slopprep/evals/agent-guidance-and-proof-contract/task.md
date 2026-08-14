# Repair a Private Workspace Agent Contract

## Problem

A private engineering workspace supports Codex, Claude Code, and Grok-based
agents. Its current `AGENTS.md` is generic, model-specific in places, and does
not tell an agent who it is helping, where task context lives, what it may do,
or how to prove work. The repository already has working lifecycle scripts, but
agents regularly rediscover them and sometimes claim completion from a unit test
or screenshot alone.

Audit the workspace for agent readiness and replace `AGENTS.md` with the
smallest useful operating contract. Also produce `readiness-report.md` with the
repository grade, runner grade, evidence level, gaps, and exercised checks.

Do not change product code or invent credentials. Treat the provided runner as
local-only; no CI or live deployment access is available.

## Input Files

=============== FILE: AGENTS.md ===============
# Instructions

You are an expert coding assistant. Think step by step and be concise.
For Claude, use XML tags. For GPT, restate the request before acting.
Read every file before making changes and always ask before using tools.
Write clean code and test your work.

=============== FILE: OWNER.md ===============
# Maya Chen

Maya builds developer tools for small teams. She values simple composable
systems, direct evidence, and finishing the smallest complete useful result.
She often explores too many abstractions before shipping. Private identity and
financial details live under `private/` and should be opened only when a task
requires them.

=============== FILE: docs/workspace.md ===============
# Workspace

Product implementation belongs under `packages/`. Cross-package decisions are
recorded in `docs/decisions/`. Runtime incidents go to `ops/log.md`.

Use `./scripts/bootstrap.sh` for reproducible setup, `./scripts/boot.sh` to
start the local service, `./scripts/verify.sh` for the canonical local gate, and
`./scripts/teardown.sh` for cleanup. CI uses the same verify script.

The API smoke surface is `http://127.0.0.1:4317/health`. Invalid configuration
must exit non-zero with a redacted `config/missing_value` diagnostic.

=============== FILE: scripts/bootstrap.sh ===============
#!/usr/bin/env bash
set -euo pipefail
test -f package.json

=============== FILE: scripts/boot.sh ===============
#!/usr/bin/env bash
set -euo pipefail
node server.js

=============== FILE: scripts/verify.sh ===============
#!/usr/bin/env bash
set -euo pipefail
npm run typecheck
npm test
curl --fail --silent http://127.0.0.1:4317/health

=============== FILE: scripts/teardown.sh ===============
#!/usr/bin/env bash
set -euo pipefail
pkill -f 'node server.js' 2>/dev/null || true

## Output

Produce:

- `AGENTS.md`
- `readiness-report.md`
