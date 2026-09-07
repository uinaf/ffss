# Repository documentation

Each document serves one reader need. Preserve the repository's established
layout; use these defaults when redistributing overloaded docs.

| File | Content |
| --- | --- |
| README.md | purpose, install/start, first successful use, links to deeper docs |
| CONTRIBUTING.md | contributor setup, local run, validation, repo-specific workflow |
| SECURITY.md | existing private reporting route, scope, and disclosure boundaries |
| LICENSE | legal terms |

Lead a package README with installation and minimal usage. App READMEs serve
users; contributor setup belongs elsewhere. Coordination workspaces may lead
with ownership, quick start, and their source-of-truth registry.

Move established contributor or security policy out of an overloaded README,
but never invent contacts, support promises, or governance. Security guidance
directs reporters to the verified private route, not public issues.

Deep docs cover architecture, API contracts, operations, deployment, and
recovery. Consequential rationale goes in decision records, stable behavior in
specs, tactical status in the tracker. Link these owners without copying their
contents or duplicating navigation lists.

After a rename, removal, or behavior change, search reader and agent docs for
old paths, commands, and claims. Verify replacements against the source; keep
valid links and unrelated content.
