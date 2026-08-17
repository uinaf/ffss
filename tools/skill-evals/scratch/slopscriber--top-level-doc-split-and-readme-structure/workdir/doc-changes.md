# Documentation restructure

The 280-line `README.md` served users, contributors, and security reporters at once. It is now split so each file has one responsibility and a user reaches their first `grok parse` command in the first screen.

## Where each section went

| Original README section | New home | Why |
| --- | --- | --- |
| Intro paragraph | `README.md` — intro | Product value belongs to the user-facing entrypoint. |
| Table of Contents | `README.md` — "More documentation" | A 9-entry ToC on a 45-line README is noise. The entries survive as links to the two new files; in-page anchors were dropped because the sections they pointed to are now short or moved. |
| Installation — Homebrew, npm | `README.md` — "Install" | Fastest path to a working binary; kept first. |
| Installation — build from source | `CONTRIBUTING.md` — "Setup" | The block was `git clone` + `npm install` + `npm run build` + `npm link`, which duplicated Development Setup. Merged into one canonical source-build home; `README.md` links to it. |
| Usage | `README.md` — "First command" and "More usage" | Split so the single most common invocation stands alone; filtering, stdin, aggregation, and output formats follow. |
| Architecture | `CONTRIBUTING.md` — "Architecture" | It describes `src/` internals, which only matters to someone changing the code. It sits next to the CI rule requiring new parsers to implement `src/parsers/base.ts`. |
| Development Setup | `CONTRIBUTING.md` — "Setup" | Contributor onboarding. Prerequisites and all four steps kept verbatim. |
| Coding Standards | `CONTRIBUTING.md` — "Coding standards" | Both command lists and all five CI rules kept verbatim. |
| Running Tests | `CONTRIBUTING.md` — "Run tests" | Moved above coding standards: contributors run tests before they hit lint rules. Merge gate and the 80% coverage floor kept. |
| Contributing | `CONTRIBUTING.md` — intro, "Pull requests", "Releases" | Split by reader task: the issue-first rule moved to the top where it changes what someone does next; the 5 PR steps and review approval/3-business-day target stayed together; the `v*` tagging release process became its own section. |
| Security | `SECURITY.md` | Reporters should not have to scroll a README. Contact, the three required report fields, the 48-hour/7-day targets, and the latest-stable support scope kept verbatim. |
| License | `README.md` — "License" | One line; no separate file needed beyond the existing `LICENSE`. |

## Notes

- No content was deleted. Every fact from the original appears in exactly one output file.
- No new policy was invented. The only added text is routing: the README's "More documentation" list, its pointer to the source build, and a one-line pointer from `CONTRIBUTING.md` to `SECURITY.md`.
- Section order inside `CONTRIBUTING.md` follows contributor sequence — set up, run tests, meet standards, understand the code, open a PR — rather than the original README's order.
- If `docs/` is added later, "Architecture" is the section most worth promoting to its own file; it is the only part of `CONTRIBUTING.md` a reader might want without the rest.
