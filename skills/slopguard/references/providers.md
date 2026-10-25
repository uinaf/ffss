# Providers

- Pass the user's model and effort as flags.
- No fallback to another engine after a failure. Auth and capability
  failures are results to report.
- When the user approves a substitute for an unavailable engine, keep that
  engine's default model and step effort down one level.

Engines and their constraints: `slopguard review --help` and
`slopguard config`.
