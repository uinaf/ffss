# Results and failures

- Terminal and JSON output represent the same locally validated report.
- Review results go to stdout; progress and diagnostics go to stderr.
- With an unambiguous `review --output json`, representable argument, flag,
  and configuration failures also use the canonical failure report instead of
  prose. Help stays human-readable.

| Exit | Status | Meaning |
| ---: | --- | --- |
| 0 | `clean` | Valid review with no findings |
| 1 | `findings` | Valid review with one or more findings |
| 2 | `failure` | No trustworthy review result |

- Every valid finding is reported regardless of confidence, with a priority,
  confidence, category, and a location inside the frozen target.
- Exit 2 carries a stable failure class, such as `authentication`,
  `capability`, `timeout`, or `source_changed`. Never reinterpret an
  operational failure as a clean review.
- Field and enum details:
  [result contract](https://github.com/uinaf/ffss/blob/main/cli/slopguard/docs/RESULT_SCHEMA.md).

- Slopguard performs at most one configured retry, only for a malformed
  protocol response, using the same frozen bundle and provider.
  Authentication, capability, timeout, cancellation, and provider failures
  are not retried. A Claude refusal (`stop_reason: refusal`) is a capability
  failure carrying the refusal category. Report it; another engine needs
  explicit scope, as [providers.md](providers.md) states.
- Once provider execution metadata is resolved, failure reports preserve the
  provider, model, harness version, and web-access state.
- A `protocol` failure carries a reason such as `invalid_envelope`; envelope
  failures also name the violated rule, such as `event after turn.completed`.
  Neither contains provider output.

After any provider return, slopguard recollects the target. A changed snapshot
produces `source_changed`: discard the findings and rerun from a new freeze.
