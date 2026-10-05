# Repository documentation

Each document serves one reader need. Preserve the repository's established
layout; use these defaults when redistributing overloaded docs.

| File | Content |
| --- | --- |
| README.md | purpose, install/start, first successful use, links to deeper docs |
| CONTRIBUTING.md | contributor setup, local run, validation, repo-specific workflow |
| SECURITY.md | existing private reporting route, scope, and disclosure boundaries; skip when an owner `.github` default covers the repository |
| LICENSE | legal terms |

Coordination workspaces may lead with ownership, quick start, and their
source-of-truth registry.

READMEs name entrypoint commands but not runners, runner variables, or the
repository's own CI and infrastructure config, such as triggers, jobs, tool
pins, or Renovate and ruleset settings; link the owning file where a reader
needs it.

Move established contributor or security policy out of an overloaded README,
but never invent contacts, support promises, or governance. Security guidance
directs reporters to the verified private route, not public issues.

Deep docs cover architecture, API contracts, operations, deployment, and
recovery. Consequential rationale goes in decision records, stable behavior in
specs, tactical status in the tracker. Link these owners without copying their
contents or duplicating navigation lists.
