# Configuration

Slopguard loads one flat, typed configuration in descending precedence:

1. explicit CLI flags
2. `SLOPGUARD_*` environment variables
3. `.slopguard.yaml` at the resolved Git root
4. `$XDG_CONFIG_HOME/slopguard/config.yaml`, or
   `$HOME/.config/slopguard/config.yaml` when `XDG_CONFIG_HOME` is unset
5. built-in operational defaults

The engine has no default. Select exactly one of `codex`, `claude`, or `grok`
through a flag or configuration source. Slopguard never chooses an engine by
examining `PATH`.

## File schema

Both YAML files use the same strict schema:

```yaml
engine: claude
model: claude-sonnet-5
reasoning_effort: high
timeout: 30m
retries: 0
max_bytes: 4194304
web_access: false
telemetry: false
```

Omitted keys keep their built-in defaults: [`defaults()`](../internal/config/types.go)
for operational values, [`applyProviderDefaults`](../internal/config/load.go)
for per-engine reasoning effort, and the `Default*Model` constants in
[provider/types.go](../internal/provider/types.go), which the adapter applies
when the model is unset. `slopguard config` prints each value and its source.

- Unknown keys, multiple YAML documents, and invalid types are errors.
- `retries` must be `0` or `1`; `timeout` must be positive and at most `24h`.
- Configuration cannot contain commands, provider argument strings, or named
  profiles.
- There is no `.slopguard.local.yaml`.
- `max_bytes` is bounded by the [target limits](TARGETS.md).
- Each configuration file must be a regular file no larger than 64 KiB;
  symbolic links are rejected.
- Boolean fields accept only `true` or `false`; `yes`, `no`, `on`, and `off`
  are invalid.
- A trusted account-home XDG file must be owned by the current user and must
  not be group- or world-writable.

Each file key except `telemetry` has an upper-case `SLOPGUARD_` environment
variable, such as `SLOPGUARD_REASONING_EFFORT`.

Telemetry has no environment-variable source. It defaults off and can be
enabled only with `--telemetry` or `telemetry: true` in the ownership-checked
account-home XDG file. Repository configuration and an XDG path selected by
`XDG_CONFIG_HOME` cannot enable it. See [Optional telemetry](TELEMETRY.md).

## Security controls

Web access defaults to off.

- A path selected through `XDG_CONFIG_HOME`, repository configuration, and
  environment variables cannot enable web access. Their `web_access: true`
  values are rejected.

Providers run from a new empty temporary workspace, receive only the frozen
review bundle, and inherit the normal provider environment, user configuration,
and configured provider or session authentication.

## Effective configuration

Inspect resolved values and their source without printing credentials or the
full environment:

```bash
slopguard config --engine codex
slopguard config --engine codex --json
```

It accepts the same typed overrides as `review`; `--repository` inspects
another checkout.
