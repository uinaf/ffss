# Security

Do not open a public issue for a suspected vulnerability.

Use GitHub's private vulnerability reporting from this repository's Security
tab (Report a vulnerability).

- Name the affected member (`cli/slopmachine`, `cli/slopguard`) and version
  or component, the impact, a minimal reproduction, and any known
  mitigations.
- Do not include live credentials or private source code.

Useful reports include:

- secret exposure, command injection, and path traversal
- unsafe provider execution, sandbox escape, and review-bundle boundary
  failures (slopguard)
- malformed provider output accepted as a clean review (slopguard)
- unsafe command execution from stored evidence and state-machine bypasses
  that accept empty review or verify evidence (slopmachine)

Security fixes are applied on a best-effort basis to the latest release and
the latest code on `main`.
