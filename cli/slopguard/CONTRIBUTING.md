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

The test tasks cap Go parallelism because Git-backed tests spawn many
subprocesses; the race task runs serially because macOS TSan can crash when
Git subprocess tests run concurrently.

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

For command-surface changes, also exercise the built binary directly. For
changes to the [agent skill](../../skills/slopguard/SKILL.md), run the
[skillcheck lint](../../tools/skill-evals/README.md) that CI runs:

```bash
cd ../../tools/skill-evals && npm ci && npm run lint
```

Authenticated provider regression checks are committed but separate from
deterministic verification and CI. They build the current CLI, materialize the
public [synthetic fixture](testdata/v0.1-fixture/README.md) as clean and
defective commits, run builder tests, and review both controls through each
selected provider at its default model with high effort and web access off:

```bash
mise run verify:live
SLOPGUARD_LIVE_PROVIDERS=codex,grok mise run verify:live
SLOPGUARD_LIVE_PROVIDERS=grok SLOPGUARD_LIVE_REPEAT=3 mise run verify:live
```

[`TestBinaryLiveProviderMatrix`](cmd/slopguard/live_e2e_test.go) owns the
provider list, repeat bounds, per-review timeout, and review cap.

- The run removes the selected provider's direct API-key variables and
  preserves normal provider state, XDG configuration, and helper configuration.
  Use an isolated session or gateway/helper profile when those routes need
  separate proof.
- The per-review timeout covers Grok at high effort, whose clean review
  exceeds 3m.
- The review cap keeps a worst case where every review uses its retry inside
  the `verify:live` test timeout in [mise.toml](mise.toml); raising either
  side needs the other.
- These checks consume provider quota and require every selected harness on
  `PATH`.

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
