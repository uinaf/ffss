# Code patterns

The diff is the unit. Clean only what the change added; leave the surrounding
code alone, even when it has the same tells, unless asked.

- TODO with no owner or ticket: file it or delete it. Keep ticketed TODOs.
- Unreachable branches kept "for completeness": delete, with the reasoning in
  the commit message.
- An exported option or accepted input is a public contract even when this
  module ignores it; keep it and report removal as separate scope. Rename lying
  internal names; report lying public names.
- A pattern that hides a real defect (a swallow masking a reachable failure) is
  a finding, not a cleanup; report it.
