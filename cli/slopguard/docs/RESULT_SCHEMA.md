# Result contract

`--output json` writes one result object to stdout; progress and diagnostics
go to stderr. Two schemas own the fields, and `slopguard schema review|result`
prints them:

- [review-v1.schema.json](../schema/review-v1.schema.json): what a provider
  must return.
- [result-v1.schema.json](../schema/result-v1.schema.json): the CLI's final
  report.

Both reject unknown and missing fields.

## Status and exit code

| Status | Meaning | Exit |
| --- | --- | ---: |
| `clean` | A valid review returned no findings | 0 |
| `findings` | A valid review returned one or more findings | 1 |
| `failure` | No trustworthy review result was obtained | 2 |

An operational failure never becomes `clean`.

## Rules the schemas can't express

The Go decoder also enforces these:

- Status follows the finding count: `clean` has none, `findings` has at least
  one.
- A clean review needs overall confidence of at least `0.5`; otherwise it is
  malformed provider output. No threshold suppresses a valid finding.
- Paths are normalized, repository-relative, and POSIX. Reviewed line ranges
  don't overlap. A finding cites a reviewed file, at any line of it.
- Attempts number from one in sequence, and a successful report's last
  attempt is `valid`.
- Failure classes are the `FailureClass` constants in
  [protocol/types.go](../internal/protocol/types.go). Failure messages are
  sanitized and never carry raw provider output.
- Only a malformed provider review consumes the one protocol retry; a
  provider-reported error is a failed attempt.

## Compatibility

`schema_version` is a string. Any new field needs a new schema version,
because version 1 rejects unknown fields.
