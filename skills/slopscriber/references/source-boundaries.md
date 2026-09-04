# Source boundaries

Checked-in docs describe contracts their repository owns.

| Evidence | Treatment |
| --- | --- |
| Target code, config, or scripts | Verify and document the owned contract |
| Upstream API or product | Cite its maintained source |
| Another repository | Link the owner or describe the dependency generically |
| Private workspace, local helper, account, host, or one-off observation | Keep out of repo policy unless explicitly adopted by the owner |

Do not reproduce sensitive identifiers in reports merely to explain their
exclusion. Describe the category. Owner approval to adopt a recurring contract
does not authorize publishing its private values.

## Durable homes

Use existing repository conventions. Common homes are:

- Agent guidance: recurring operating behavior.
- Specs: long-lived requirements and acceptance.
- Decisions: consequential choices and rationale.
- Tracker: tactical work, dependencies, and resumable status.
- Local plan directory: only when established as the tracker.
- Work report: transient evidence safe for its audience.

Before saving a fact, check ownership, lifetime, audience, and maintenance path.
