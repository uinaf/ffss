# Configuration

Slopguard loads one flat, typed configuration in descending precedence:

1. explicit CLI flags
2. `SLOPGUARD_*` environment variables
3. `.slopguard.yaml` at the resolved Git root
4. `$XDG_CONFIG_HOME/slopguard/config.yaml`, or
   `$HOME/.config/slopguard/config.yaml` when `XDG_CONFIG_HOME` is unset
5. built-in operational defaults

The engine has no default. Select exactly one of `codex` or `claude`
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
web_access: true
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

Web access defaults to on so reviewers can search for and fetch references.
Pass `--web-access=false` or set `web_access: false` to turn it off.

- A path selected through `XDG_CONFIG_HOME`, repository configuration, and
  environment variables may disable web access but cannot enable it. Their
  `web_access: true` is ignored while web access is already on and rejected
  when a lower-precedence source turned it off.
- With web access on, repository content under review can steer the reviewer
  toward attacker-chosen URLs. Turn it off for targets that must not reach the
  network.

Providers run from a new empty temporary workspace, receive only the frozen
review bundle, and inherit the normal provider environment, user configuration,
and configured provider or session authentication.

Repository access has only a flag form and is on by default for codex, off for
claude: the reviewer runs inside a snapshot of the repository instead of the
empty workspace, so it can read callers and tests. Pass
`--repository-access=false` to keep codex to the bundle, or
`--repository-access` to give claude the snapshot. In the
[planted-bug eval](../CONTRIBUTING.md), codex reviewing only the diff missed
most bugs that need surrounding code; the snapshot caught them for about two
seconds more, while claude caught them without it.

- The snapshot holds the reviewed revision's files, read straight from its
  tree so `export-ignore` and `export-subst` cannot hide or rewrite them, or the
  frozen working tree for `local`, without Git metadata. Sensitive paths are left out, environment
  files are [redacted](TARGETS.md), and symlinks and special files are skipped.
  It is capped at 512 MiB, counting 512 bytes plus the path for every file.
- Neither engine loads the snapshot's own instruction files. Claude keeps the
  empty workspace and gets the snapshot through `--add-dir`, with
  `CLAUDE_CODE_ADDITIONAL_DIRECTORIES_CLAUDE_MD=0`, so its `CLAUDE.md` stays
  data; codex runs in the snapshot with project `AGENTS.md` loading turned
  off.
- Claude gets `Read`, `Grep`, and `Glob` without pre-approval, so it can read
  only inside the snapshot. Codex runs in its read-only sandbox, which still
  reads the whole disk: repository content under review can steer it to any
  file the user can read.
- The snapshot directory must be outside the reviewed repository; point
  `TMPDIR` elsewhere if it is not. Keep `TMPDIR` outside Codex-trusted
  projects too, so Codex cannot apply a reviewed project configuration.

## Result cache

`review` reuses a completed clean or findings result when the snapshot hash,
engine, model, reasoning effort, retries, web access, repository access, and
slopguard version all match, the source still passes the unchanged check, and
prints `reusing the cached review of this snapshot` to stderr. The snapshot hash already covers the diff, prompt, and context files.
Pass `--cache=false` to review again.

- Entries live under `$XDG_CACHE_HOME/slopguard`, or `~/.cache/slopguard`,
  as files only the current user can read; a shared or symlinked entry is
  ignored, and any cache error falls back to a fresh, unstored review.
- Failures are never cached. A cached result keeps its original attempts and
  duration. The newest 512 results are kept.
- A cache directory inside the reviewed repository disables the cache.
- A cache hit records no [telemetry](TELEMETRY.md) event, since no provider ran.
- Local snapshot hashes use a per-user `state-key` kept in the same directory,
  so an unchanged local target reproduces its hash across runs.

## Effective configuration

Inspect resolved values and their source without printing credentials or the
full environment:

```bash
slopguard config --engine codex
slopguard config --engine codex --json
```

It accepts the same typed overrides as `review`; `--repository` inspects
another checkout.
