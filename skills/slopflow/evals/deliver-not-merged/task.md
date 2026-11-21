# Wrap Up the Export Epic

The export lane says it's finished. Can we wrap up the export epic? Its report
and what the repo shows are below.

Write the epic's updated lane checklist and your answer to `wrapup.md`. Don't
post anywhere.

======= FILE: lane-export-final.md =======
## Lane export (epic #172), final message

All three issues merged, worktrees and branches cleaned up. Ready to archive.
- #222 Export to PDF: merged #231
- #224 Export to Markdown: merged #234
- #226 Export guide in `docs/`: merged #238
======= END FILE =======

======= FILE: repo-state.md =======
## Deploys
`deploy-web` runs on every push to main and deploys to staging; it skips
changes under `docs/`. Production takes a manual `promote` run. `docs/` is
read in the repository only; nothing publishes it.

| Run | Workflow | Commit | Result |
| --- | --- | --- | --- |
| 881 | deploy-web | #231 merge | success, staging |
| 884 | promote | #231 merge | success, production |
| 890 | deploy-web | #234 merge | success, staging |
| 893 | deploy-web | #238 merge | skipped |

## Smoke tests
- 4411 `e2e-prod` (production), after run 884: PDF export passes.
- No production run since #234.

## Epic #172
## Lane export
- [ ] #222 PR #231
- [ ] #224 PR #234
- [ ] #226 PR #238
======= END FILE =======
