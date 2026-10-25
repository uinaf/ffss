# Configuration

Precedence, highest first:

1. CLI flags
2. `SLOPGUARD_*` environment variables (every file key except `telemetry`)
3. `.slopguard.yaml` at the Git root
4. `$XDG_CONFIG_HOME/slopguard/config.yaml`, else
   `~/.config/slopguard/config.yaml`
5. built-in defaults

`slopguard config --engine <engine> [--json]` prints each effective value and
its source. Keys, defaults, and validation live in
[`internal/config`](../internal/config/); default models are the
`Default*Model` constants in [provider/types.go](../internal/provider/types.go).

- The engine has no default. Pick `codex` or `claude` with `--engine` or a
  config source; slopguard never picks one from `PATH`.
- A repository file may set `engine` only when the user file does not; a
  conflicting value is an error that `--engine` or `SLOPGUARD_ENGINE` settles.
- The account-home config file must be a regular file of at most 64 KiB,
  owned by the current user, and not group- or world-writable.

## Web access

On by default: reviewers can search and fetch references. Repository content
under review can then steer the reviewer to attacker-chosen URLs, so turn it
off (`--web-access=false` or `web_access: false`) for targets that must stay
off the network. The repository file, environment variables, and an
`XDG_CONFIG_HOME` file can turn it off but never back on.

With web access on, Claude's only web tools are `WebSearch` and `WebFetch`,
and Codex gets `--search`. With it off, neither engine gets a web tool.

## Repository access

Flag only. On by default for codex, off for claude. The reviewer then also
sees a snapshot of the reviewed revision (or the frozen working tree for
`local`), so it can read callers and tests. Toggle it with
`--repository-access[=false]`.

- The snapshot leaves out sensitive paths, redacts environment files
  ([Targets](TARGETS.md)), skips symlinks and special files, and is capped at
  512 MiB.
- Neither engine loads the snapshot's `AGENTS.md` or `CLAUDE.md`.
- Claude reads only inside the snapshot. Codex runs in its read-only sandbox,
  which can read the whole disk: reviewed content can steer it to any file the
  user can read.
- Keep `TMPDIR` outside the reviewed repository and outside Codex-trusted
  projects.

## Result cache

`review` reuses a clean or findings result when the snapshot, engine, model,
effort, retries, web access, repository access, and slopguard version all
match, and says so on stderr. `--cache=false` forces a fresh review. Failures
are never cached. Entries live in `$XDG_CACHE_HOME/slopguard` or
`~/.cache/slopguard`; a cache directory inside the reviewed repository
disables the cache.

## Telemetry

Off by default. Enable one run with `--telemetry`, or set `telemetry: true`
in the account-home config file; no other source can enable it. Events stay
in `~/.local/state/slopguard/telemetry.jsonl` (newest 256 events, 1 MiB) and
never change review output, status, exit code, or latency.

Events hold only versions, provider, target mode, web access, outcome,
failure class, attempt outcomes, and coarse size, count, and duration buckets.
They hold no paths, revisions, hashes, prompts, findings, provider output,
environment values, credentials, custom model ids, or user or repository
identifiers.

Reviews never upload telemetry. Export is explicit and leaves the local events
in place:

```bash
slopguard telemetry export > telemetry.json
```
