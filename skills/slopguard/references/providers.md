# Provider selection

- Select one provider for the entire review.
- Honor the user's provider, model, and reasoning-effort choices first, then
  trusted repository or account configuration.
- Pass explicit user model and effort choices as CLI flags.
- If no source chooses a provider, select one installed harness that satisfies
  the task and say why you chose it.
- Do not run multiple providers, fall back after failure, or claim consensus.

| Engine | Default model | Strict authentication | Important constraint |
| --- | --- | --- | --- |
| `codex` | `gpt-5.6-sol` | `CODEX_API_KEY` or `OPENAI_API_KEY` | Native mode preserves Codex provider and session authentication; when web access is authorized, only the Codex search surface is enabled |
| `claude` | `claude-opus-5` | `ANTHROPIC_API_KEY` | Effort supports `low`, `medium`, `high`, `xhigh`, or `max`; when web access is enabled, only WebSearch is exposed |
| `cursor` | `cursor-grok-4.6-high-fast` | `CURSOR_API_KEY` | Native mode preserves helper/session auth; no documented per-run web disable, so explicit CLI selection implies web when unset, and explicit `web_access: false` fails capability preflight |
| `grok` | `grok-4.6` | `XAI_API_KEY` | Native mode preserves configured provider or session authentication; tools, memory, plans, and subagents stay disabled; when web access is enabled, only WebSearch and WebFetch are exposed |

- For Cursor, pass a requested compatible model with `--model` and never add
  `--reasoning-effort`.
- If the user requests Cursor plus a separate effort value without a
  compatible model ID, report the unsupported combination instead of guessing
  a model.

- All adapters capability-probe the installed executable and use one explicit
  model with no model fallback.
- A capability or authentication failure is an operational result, not
  permission to switch engines.

Isolation and web-access policy live in [configuration.md](configuration.md).
