# Configuration

Slopguard loads one flat, typed configuration in descending precedence:

1. explicit CLI flags
2. `SLOPGUARD_*` environment variables
3. `.slopguard.yaml` at the resolved Git root
4. `$XDG_CONFIG_HOME/slopguard/config.yaml`, or
   `$HOME/.config/slopguard/config.yaml` when `XDG_CONFIG_HOME` is unset
5. built-in operational defaults

The engine has no default. Select exactly one of `codex`, `claude`, `cursor`, or
`grok` through a flag or configuration source. Slopguard never chooses an
engine by examining `PATH`.

## File schema

Both YAML files use the same strict schema:

```yaml
engine: codex
model: gpt-5.6
reasoning_effort: medium
timeout: 15m
retries: 1
max_bytes: 1048576
web_access: false
telemetry: false
```

- Unknown keys, multiple YAML documents, invalid types, and retry counts above
  one are errors.
- Configuration cannot contain commands, provider argument strings, or named
  profiles.
- There is no `.slopguard.local.yaml`.

`max_bytes` defaults to 1 MiB and must be between 1 byte and 128 MiB
(`134217728`).

Default reasoning effort depends on the selected provider: `medium` for Codex
and Claude, and `high` for Cursor and Grok. Any explicit configuration source
overrides the provider default.

- Each configuration file must be a regular file no larger than 64 KiB, and
  symbolic links are rejected.
- Boolean fields accept only `true` or `false`; `yes`, `no`, `on`, and `off`
  are invalid.
- A trusted account-home XDG file must also be owned by the current user and
  must not be group- or world-writable.

The corresponding environment variables are `SLOPGUARD_ENGINE`,
`SLOPGUARD_MODEL`, `SLOPGUARD_REASONING_EFFORT`, `SLOPGUARD_TIMEOUT`,
`SLOPGUARD_RETRIES`, `SLOPGUARD_MAX_BYTES`, and `SLOPGUARD_WEB_ACCESS`.

Telemetry has no environment-variable source. It defaults off and can be
enabled only with `--telemetry` or `telemetry: true` in the ownership-checked
account-home XDG file. Repository configuration and an XDG path selected by
`XDG_CONFIG_HOME` cannot enable it. See [Optional telemetry](TELEMETRY.md).

## Security controls

Web access defaults to off for Codex, Claude, and Grok.

- An explicit CLI `--engine cursor` changes an otherwise unset default to on
  because Cursor Agent has no documented per-run web-disable control.
- Cursor selected by repository, environment, or XDG engine configuration does
  not receive that implicit grant; pass `--web-access` with the explicit
  command, or set `web_access: true` in an ownership-checked account XDG
  config.
- An explicit `web_access: false` from any source remains authoritative and
  makes Cursor fail capability preflight.
- A path selected through `XDG_CONFIG_HOME`, repository configuration, and
  environment variables cannot enable web access. Their `web_access: true`
  values are rejected even when an explicit CLI Cursor selection would
  otherwise imply web; omit the untrusted restatement and let the flag-derived
  value apply.

Providers run from a new empty temporary workspace, receive only the already
frozen review bundle, and inherit the normal provider environment, user
configuration, and configured provider or session authentication.

## Effective configuration

Inspect resolved values and their source without printing credentials or the
full environment:

```bash
slopguard config --engine codex
slopguard config --engine codex --json
```

The diagnostic supports the same typed overrides: `--model`,
`--reasoning-effort`, `--timeout`, `--retries`, `--max-bytes`, `--web-access`,
and `--telemetry`. Use `--repository` to inspect another
checkout.
