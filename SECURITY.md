# Security

Do not open a public issue for a suspected vulnerability. Use GitHub's private
vulnerability reporting from this repository's Security tab (Report a
vulnerability).

- Name the affected member (`cli/slopguard`, `cli/lib`, or `skills`) and version
  or component, the impact, a minimal reproduction, and known mitigations.
- Do not include live credentials or private source code.

In scope:

- secret exposure, command injection, and path traversal
- unsafe provider execution, sandbox escape, and review-bundle boundary
  failures (slopguard)
- malformed provider output accepted as a clean review (slopguard)

Fixes are best-effort, on the latest release and `main`.
