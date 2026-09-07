# Configuration

Slopguard resolves one flat typed configuration in this precedence order:

1. CLI flags
2. `SLOPGUARD_*` environment variables
3. `.slopguard.yaml` at the Git root
4. account XDG (Cross-Desktop Group) config
5. built-in operational defaults

- The engine has no built-in default.
- Valid engines are `codex`, `claude`, `cursor`, and `grok`.

Inspect resolved values and their source with:

```bash
slopguard config --repository . --engine "$engine"
slopguard config --repository . --engine "$engine" --json
```

YAML schema:

```yaml
engine: codex
model: gpt-6-astra
reasoning_effort: medium
timeout: 15m
retries: 1
max_bytes: 1048576
web_access: false
```

Environment variables: `SLOPGUARD_ENGINE`, `SLOPGUARD_MODEL`,
`SLOPGUARD_REASONING_EFFORT`, `SLOPGUARD_TIMEOUT`, `SLOPGUARD_RETRIES`,
`SLOPGUARD_MAX_BYTES`, `SLOPGUARD_WEB_ACCESS`.

- Unknown keys, loose YAML booleans, multiple documents, retry counts outside
  zero or one, and invalid types fail closed.
- No profiles or local override files.
- `max_bytes` defaults to 1 MiB; maximum 128 MiB.
- Reasoning effort defaults to `medium` for Codex and Claude, `high` for Cursor
  and Grok. Explicit configuration overrides it for Codex, Claude, and Grok.
  Cursor encodes effort in the model ID and rejects a separate
  `reasoning_effort`; choose its effort with `--model`.

## Runtime

Reviews preserve configured provider or session authentication and run in an
empty temporary workspace holding only the frozen bundle.

## Web access

- Defaults off for Codex, Claude, and Grok.
- Explicit CLI `--engine cursor` enables otherwise-unset web access implicitly
  because Cursor cannot guarantee a per-run web disable.
- Repository, environment, or XDG engine selection does not grant web access.
- Explicit `web_access: false` stays authoritative and prevents a Cursor run.
- Only an explicit flag or an ownership-checked account-home XDG file may
  otherwise enable web access.
