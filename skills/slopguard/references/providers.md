# Provider selection

- Select one provider for the entire review.
- Honor the user's provider, model, and reasoning-effort choices first, then
  trusted repository or account configuration. Pass explicit user model and
  effort choices as CLI flags.
- If no source chooses a provider, use Codex with medium reasoning.
- Do not run multiple providers, fall back after failure, or claim consensus.

Engines, defaults, and per-engine constraints: `slopguard review --help`,
`slopguard config`, and the
[engine docs](https://github.com/uinaf/ffss/blob/main/cli/slopguard/docs/engines/README.md).

- When the user authorizes an alternative for an unavailable configuration,
  keep the engine's default model and step the effort down one level.
- A capability or authentication failure is an operational result, not
  permission to switch engines.

Web-access policy lives in [configuration.md](configuration.md).
