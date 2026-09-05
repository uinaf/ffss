# Provider selection

- Select one provider for the entire review.
- Honor the user's provider, model, and reasoning-effort choices first, then
  trusted repository or account configuration.
- Pass explicit user model and effort choices as CLI flags.
- If no source chooses a provider, use Codex with medium reasoning.
- Do not run multiple providers, fall back after failure, or claim consensus.

| Engine | Default model | Default effort | Important constraint |
| --- | --- | --- | --- |
| `codex` | `gpt-6-astra` | `medium` | Preserves Codex provider and session authentication; when web access is authorized, only the Codex search surface is enabled |
| `claude` | `claude-fable-5-1` | `medium` | Effort supports `low`, `medium`, `high`, `xhigh`, or `max`; when web access is enabled, only WebSearch is exposed |
| `cursor` | `cursor-grok-4.6-high` | `high` in the model ID | Preserves helper/session auth; no documented per-run web disable, so explicit CLI selection implies web when unset, and explicit `web_access: false` fails capability preflight |
| `grok` | `grok-4.6` | `high` | Preserves configured provider or session authentication; tools, memory, plans, and subagents stay disabled; when web access is enabled, only WebSearch and WebFetch are exposed |

- When the user authorizes an alternative for unavailable models, use
  `gpt-5.6-sol` for Codex or `claude-opus-5` for Claude, both at `medium`.
- For Cursor, pass a requested compatible model with `--model` and never add
  `--reasoning-effort`.
- If the user requests Cursor plus a separate effort value without a
  compatible model ID, report the unsupported combination instead of guessing
  a model.

- All adapters capability-probe the installed executable and use one explicit
  model with no model fallback.
- A capability or authentication failure is an operational result, not
  permission to switch engines.

Web-access policy lives in [configuration.md](configuration.md).
