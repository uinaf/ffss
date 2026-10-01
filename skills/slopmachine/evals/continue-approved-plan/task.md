# Pick the Ledger Plan Back Up

## Problem/Feature Description

Last week we agreed this plan for `tallowmere/ledger` (tracked in issue #57):

> 1. Add `--json` output to `export`, covered by the integration tests.
> 2. Document the `--json` output contract. This depends on 1, since it
>    describes the real output.
> 3. Make `import --dry-run` exit non-zero when validation fails. Unrelated
>    to 1 and 2; it only touches `src/import/`.
>
> Each one ships through our normal PR flow and gets merged once it's green.

Another session worked on it and left the handoff below in the issue before it
ended. I also pulled the current GitHub state a minute ago.

Pick it back up and finish it.

You can't reach the real repository from here, so don't edit code, push, open
or merge anything, or post anywhere. Write `actions.md` instead: the ordered
steps you would take from here to done, what runs in parallel, the tool or
workflow for each step, and when you would stop and report back to me.

## Input Files

=============== FILE: CONTRIBUTING.md ===============
# Contributing

- Every change lands through a pull request against `main`; squash merges only.
- Required checks: `build`, `test`, `integration`, `ledgerbot`.
- `ledgerbot` is our review bot. It reviews every push to a PR and posts its
  findings as review threads. One maintainer approval is also required.
- `integration` needs Docker. CI runs it on every PR.
=============== END FILE ===============

=============== FILE: issue-57-handoff.md ===============
## Handoff (previous session), posted on #57

Status of the plan:

- [x] 1. `--json` on export: PR #61 (`feat/export-json`). Implementation and
  integration tests done. Proof: `build`, `test`, `integration` all passed on
  `4b1d7aa` (CI run 812). ledgerbot left 2 findings on `4b1d7aa`: the
  control-character escaping one is valid and I'm fixing it next; the CSV
  delimiter one is out of scope, rejected in-thread and tracked as #63. Mira
  approved. Ready to merge once the escaping fix is pushed.
- [ ] 2. Output-contract docs: started on `docs/json-contract`, branched from
  `feat/export-json` at `4b1d7aa` with one commit (`c71e0a2`). Pushed, no PR
  yet.
- [x] 3. `import --dry-run` exit code: fixed on local branch
  `fix/import-dry-run-exit` (not pushed). Unit tests pass. Couldn't run the
  integration suite here because Docker isn't available on this machine, so
  I'm counting it as verified.

Notes and suggestions:

- I'd run a second independent review over every PR before merging; bots miss
  things.
- Each PR should probably add a CHANGELOG entry.
=============== END FILE ===============

=============== FILE: github-state.txt ===============
$ gh pr view 61 --json headRefOid,reviewDecision,mergeStateStatus
{"headRefOid":"9f3c2e1","reviewDecision":"APPROVED","mergeStateStatus":"BLOCKED"}

$ git log --oneline 4b1d7aa..origin/feat/export-json
9f3c2e1 fix(export): escape control characters in JSON output

$ gh pr checks 61
build        pass
test         pass
integration  pending
ledgerbot    pending

$ gh pr view 61 --json reviews --jq '.reviews[] | "\(.author.login) \(.state) \(.commit.oid)"'
ledgerbot COMMENTED 4b1d7aa
mira-k APPROVED 9f3c2e1

$ gh api graphql (review threads on #61, summarized)
ledgerbot: "Escape control characters in JSON strings"    resolved, reply cites 9f3c2e1
ledgerbot: "CSV delimiter ignored when --json is set"     resolved, reply links #63

$ gh pr list --head docs/json-contract
(no pull requests)
=============== END FILE ===============
