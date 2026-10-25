# Contributing

## Setup

```bash
mise install
go build ./cmd/slopguard
```

## Validation

Deterministic checks, before opening or updating a pull request:

```bash
mise run verify
```

Before release-related changes, run the release gate and release configuration
checks. The release gate needs network access for the current Go vulnerability
database.

```bash
mise run verify:release
mise run release:check
mise run release:snapshot
```

When target collection changes, also freeze the current checkout (it depends
on local changes, so it stays out of `verify`):

```bash
mise run test:current-checkout
```

For command-surface changes, also exercise the built binary directly. For
changes to the [agent skill](../../skills/slopguard/SKILL.md), run the
[skillcheck lint](../../tools/skill-evals/README.md) that CI runs:

```bash
cd ../../tools/skill-evals && npm ci && npm run lint
```

Live provider checks and the planted-bug eval spend provider quota and need
each selected harness on `PATH`; they stay out of `verify` and CI:

```bash
SLOPGUARD_LIVE_PROVIDERS=claude mise run verify:live
mise run eval -- -configs codex-diff,codex-snapshot -runs 1
```

Overhead benchmark:

```bash
go test ./cmd/slopguard -run '^$' \
  -bench '^BenchmarkReviewOverhead$' -benchtime=10x -count=1
```

## Pull Requests

- Start from a GitHub issue and use Conventional Commits.
- Update tests and user-facing documentation with contract changes.
- Validate AI-review findings before fixing them, then close their threads.

## Releases

Pushes to `main` that touch `cli/slopguard/` release from their Conventional
Commits ([`release-slopguard.yml`](../../.github/workflows/release-slopguard.yml)).
If publication fails, rerun the failed workflow; never cut a version by hand.
