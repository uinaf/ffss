# Providers

- One provider per review: the user's choice, then trusted repo or account
  config, else Codex at medium reasoning. Pass the user's model and effort as
  flags.
- No panels, no consensus, no fallback to another engine after a failure.
  Auth and capability failures are results to report.
- When the user approves a substitute for an unavailable engine, keep that
  engine's default model and step effort down one level.

Engines and their constraints: `slopguard review --help`, `slopguard config`,
and the [engine docs](https://github.com/uinaf/ffss/blob/main/cli/slopguard/docs/engines/README.md).
