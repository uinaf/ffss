# Contributing

## Setup

Install the toolchain and build the CLI:

```bash
mise install
go build ./cmd/slopguard
```

## Validation

Run the deterministic core checks before opening or updating a pull request:

```bash
mise run verify
```

Before release-related changes, run the complete release gate and release
configuration checks. The release gate requires the installed `trufflehog`
executable and network access: it exercises trufflehog's benign and detection
paths and checks the current Go vulnerability database.

```bash
mise run verify:release
mise run release:check
mise run release:snapshot
```

Freezing the current checkout is intentionally separate because it depends on
local changes. Opt in when target-collection behavior needs that additional
smoke check:

```bash
mise run test:current-checkout
```

For command-surface changes, also exercise the built binary directly.
For skill changes, run the pinned hosted quality gate:

```bash
mise run skill:lint
```

Authenticated provider regression checks are committed but intentionally
separate from deterministic verification and CI. They build the current CLI,
materialize public synthetic clean and defective commits, run builder tests,
and review both controls through each selected provider with native
configuration or strict-key authentication:

```bash
mise run verify:live
SLOPGUARD_LIVE_PROVIDERS=codex,grok mise run verify:live
SLOPGUARD_LIVE_PROVIDERS=codex SLOPGUARD_LIVE_AUTH_ROUTES=strict-key mise run verify:live
SLOPGUARD_LIVE_AUTH_ROUTES=native-config,strict-key mise run verify:live
SLOPGUARD_LIVE_PROVIDERS=grok SLOPGUARD_LIVE_REPEAT=3 mise run verify:live
```

- The default checks Codex, Claude, Cursor, and Grok sequentially through
  `native-config`.
- `native-config` removes the selected provider's direct API-key variables and
  preserves normal provider state, XDG configuration, and helper configuration.
  Run it from an isolated session or gateway/helper profile when those routes
  need separate proof.
- `strict-key` requires one supported direct key for every selected provider,
  then relies on strict isolation to remove provider state and competing auth.
- `SLOPGUARD_LIVE_AUTH_ROUTES` accepts `native-config`, `strict-key`, or both.
- Cursor runs with web access because its harness cannot guarantee per-run
  web disablement; the other providers run with web access off.
- `SLOPGUARD_LIVE_REPEAT` is bounded from 1 through 10.
- These checks consume provider quota and require every selected harness
  plus `trufflehog` on `PATH`.
- At the maximum repeat value, the default four-provider route can take more
  than eight hours when every review consumes its protocol retry. Selecting
  both auth routes doubles the review count.

## Pull Requests

- Start from a GitHub issue.
- Keep changes focused and use conventional commits.
- Update tests and user-facing documentation with contract changes.
- Treat AI-review findings as hypotheses: validate them, fix actionable
  defects, and close the corresponding threads.

## Releases

Successful pushes to protected `main` evaluate Conventional Commits after the
verification and snapshot jobs pass. See [Releases](docs/RELEASES.md) for
version selection, artifact signing, publication, and recovery.
