# Workspace

Product implementation belongs under `packages/`. Cross-package decisions are
recorded in `docs/decisions/`. Runtime incidents go to `ops/log.md`.

Use `./scripts/bootstrap.sh` for reproducible setup, `./scripts/boot.sh` to
start the local service, `./scripts/verify.sh` for the canonical local gate, and
`./scripts/teardown.sh` for cleanup. CI uses the same verify script.

The API smoke surface is `http://127.0.0.1:4317/health`. Invalid configuration
must exit non-zero with a redacted `config/missing_value` diagnostic.
