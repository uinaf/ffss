# Results and failures

- Terminal and JSON output represent the same locally validated report.
- Review results go to stdout; progress and diagnostics go to stderr.
- When `review --output json` is selected unambiguously, representable
  argument, flag, and configuration failures also use the canonical failure
  report instead of prose.
- Help remains a separate human-readable surface.

| Exit | Status | Meaning |
| ---: | --- | --- |
| 0 | `clean` | Valid review with no findings |
| 1 | `findings` | Valid review with one or more findings |
| 2 | `failure` | No trustworthy review result |

- Every valid finding is reported regardless of confidence.
- Findings include priority `P0` through `P3`, confidence, category, and a
  repository-relative location.
- The local validator rejects paths or line ranges outside the frozen target.

- Failure classes are `config`, `target`, `secret_scan`, `capability`,
  `authentication`, `timeout`, `cancelled`, `provider`, `protocol`,
  `source_changed`, and `internal`.
- Do not reinterpret an operational failure as a clean review.

- Slopguard performs at most one configured retry, only for a malformed
  protocol response, using the same frozen bundle and provider.
- Authentication, capability, timeout, cancellation, and provider failures
  are not retried.
- When the malformed response can be classified safely, the retry receives a
  rule-specific correction without provider output or repository content.
- Every finding location must fit within one individual reviewed line range;
  a concern that spans discontiguous hunks must be narrowed or split.

- Cursor first requires one canonical review object.
- Its only recovery accepts plain prose followed by exactly one canonical
  trailing object.
- Fences, ambiguous braces, JSON-value prefixes, multiple objects, and suffix
  prose fail closed.
- Successful local recovery is recorded as `cursor_trailing_object`.

- After provider execution metadata is resolved, operational failures preserve
  the provider, model, harness version, isolation mode, and effective
  web-access state plus any applicable protocol recovery in the canonical
  report.
- Earlier failures keep nullable metadata unset.

After any provider return, slopguard recollects the target. A changed snapshot
produces `source_changed`; discard the findings and rerun from a new freeze.
