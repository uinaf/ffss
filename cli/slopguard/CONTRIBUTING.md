# Contributing

## Setup

```bash
mise install
go build ./cmd/slopguard
```

## Validation

Deterministic core checks, before opening or updating a pull request:

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

Freezing the current checkout is separate because it depends on local changes.
Opt in when target-collection behavior needs the extra smoke check:

```bash
mise run test:current-checkout
```

For command-surface changes, also exercise the built binary directly. For skill
changes, run the pinned hosted quality gate:

```bash
mise run skill:lint
```

Authenticated provider regression checks are committed but separate from
deterministic verification and CI. They build the current CLI, materialize
public synthetic clean and defective commits, run builder tests, and review both
controls through each selected provider with native configuration:

```bash
mise run verify:live
SLOPGUARD_LIVE_PROVIDERS=codex,grok mise run verify:live
SLOPGUARD_LIVE_PROVIDERS=grok SLOPGUARD_LIVE_REPEAT=3 mise run verify:live
```

- Default: Codex, Claude, Cursor, and Grok sequentially.
- The run removes the selected provider's direct API-key variables and
  preserves normal provider state, XDG configuration, and helper configuration.
  Use an isolated session or gateway/helper profile when those routes need
  separate proof.
- Cursor runs with web access because its harness cannot guarantee per-run web
  disablement; the other providers run with web access off.
- `SLOPGUARD_LIVE_REPEAT` is bounded from 1 through 10.
- Selected providers x 2 controls x repeat count may request at most 80
  reviews, keeping the worst-case retry path inside the 8h30m test timeout.
- These checks consume provider quota and require every selected harness on
  `PATH`.
- At the maximum repeat value, the default four-provider route can exceed eight
  hours when every review consumes its protocol retry, so the full two-route
  matrix caps the repeat count at 5.

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
