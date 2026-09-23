# Provider selection

- Select one provider for the entire review.
- Honor the user's provider, model, and reasoning-effort choices first, then
  trusted repository or account configuration. Pass explicit user model and
  effort choices as CLI flags.
- If no source chooses a provider, use Codex with medium reasoning.
- Do not run multiple providers, fall back after failure, or claim consensus.

| Engine | Default model | Default effort | Important constraint |
| --- | --- | --- | --- |
| `codex` | `gpt-6-sol` | `medium` | Preserves Codex provider and session authentication; when web access is authorized, only the Codex search surface is enabled |
| `claude` | `claude-opus-5-5` | `medium` | Effort supports `low`, `medium`, `high`, `xhigh`, or `max`; when web access is enabled, only WebSearch is exposed |
| `grok` | `grok-4.7` | `high` | Preserves configured provider or session authentication; tools, memory, plans, and subagents stay disabled; when web access is enabled, only WebSearch and WebFetch are exposed |

- When the user authorizes an alternative for an unavailable configuration,
  keep the engine's default model and step the effort down one level.
- All adapters capability-probe the installed executable and use one explicit
  model with no model fallback.
- A capability or authentication failure is an operational result, not
  permission to switch engines.

Web-access policy lives in [configuration.md](configuration.md).
