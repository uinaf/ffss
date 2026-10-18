# Code

Clean only what the diff added, unless asked for more.

- **TODO without an owner or ticket:** delete it and list it in the report
  as a possible follow-up. Keep ticketed TODOs.
- **Unreachable branches "for completeness":** delete; explain in the commit
  message.
- **Exported options and accepted inputs** are public contracts even when
  ignored here: keep them and report removal as separate scope. Rename lying
  internal names; report lying public names.
- **A pattern hiding a real defect** (a swallow masking a reachable failure)
  is a finding to report.
