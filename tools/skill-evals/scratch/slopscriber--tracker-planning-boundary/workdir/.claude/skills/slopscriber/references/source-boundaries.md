# Source Boundaries

Documentation captures contracts the target repo owns.

## Evidence Routing

| Source | Durable action |
|---|---|
| Target repo file, config, or script | Write the current contract; verify the path or command |
| Upstream product or API docs | Link or cite the upstream source |
| Another workspace repo | Link the owner or state the dependency generically |
| Local machine, private workspace, credential, account, host, or one-off tool | Keep it in the work report unless the user makes it repo policy |
| User-approved recurring rule, prompt, specification, or decision | Write it in the owning durable documentation surface |
| Tactical plan, backlog item, epic, or ticket | Use the repository's preferred tracker or the user's selected destination |

## Durable Homes

- `docs/decisions/`: why a choice was made
- `docs/specs/`: long-lived behavioral contracts
- `AGENTS.md` or scoped guidance: behavior future agents must repeat
- repository work tracker: tactical execution, ownership, dependencies, and resumable status
- an existing local plan directory: only when the repository explicitly uses it as the work tracker
- work report: transient, machine-local, private, or one-off evidence

Before promotion, confirm:

- the target repo owns the fact
- the fact is stable enough to maintain
- the wording exposes no private workspace or machine detail
- the destination matches the fact's lifetime
