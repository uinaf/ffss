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

Authenticated provider regression checks are committed but separate from
deterministic verification and CI. They build the current CLI, materialize the
public [synthetic fixture](testdata/v0.1-fixture/README.md) as clean and
defective commits, run builder tests, and review both controls through each
selected provider at its default model with high effort and web access off:

```bash
mise run verify:live
SLOPGUARD_LIVE_PROVIDERS=claude mise run verify:live
SLOPGUARD_LIVE_PROVIDERS=codex SLOPGUARD_LIVE_REPEAT=3 mise run verify:live
```

[`TestBinaryLiveProviderMatrix`](cmd/slopguard/live_e2e_test.go) owns the
provider list, repeat bounds, per-review timeout, and review cap.

- The run removes the selected provider's direct API-key variables and
  preserves normal provider state, XDG configuration, and helper configuration.
  Use an isolated session or gateway/helper profile when those routes need
  separate proof.
- The review cap keeps a worst case where every review uses its retry inside
  the `verify:live` test timeout in [mise.toml](mise.toml); raising either
  side needs the other.
- These checks consume provider quota and require every selected harness on
  `PATH`.

The planted-bug eval compares review configurations on recall and false
positives. [Cases](evals/testdata/cases) are small Go, TypeScript, and Python
changes, each a base tree plus a change; most plant one bug, some need code
outside the diff to spot it, and a few are clean controls. The
[runner](evals/main.go) reviews each change as one commit with the cache off,
web access off, and an isolated config directory, so engine defaults apply:

```bash
mise run eval
mise run eval -- -configs codex-diff,codex-snapshot -runs 1
mise run eval -- -summary
```

- A finding catches a bug when it cites the planted file within three lines
  of the planted line or range; other findings count as unrelated, and a
  failed review of a bug case counts as a miss.
- Rows land in `eval-results.jsonl` with a fingerprint of the binary, effort,
  timeout, parallelism, and cases. A rerun with the same fingerprint skips
  finished rows, so an interrupted run resumes; any other fingerprint is
  refused, so pass a new `-out` or delete the file.
- It consumes provider quota: every case runs once per configuration per run.

## Pull Requests

- Start from a GitHub issue and use Conventional Commits.
- Update tests and user-facing documentation with contract changes.
- Validate AI-review findings before fixing them, then close their threads.

## Releases

Successful pushes to protected `main` evaluate Conventional Commits after the
verification and snapshot jobs pass. See [Releases](docs/RELEASES.md) for
version selection, artifact signing, publication, and recovery.
