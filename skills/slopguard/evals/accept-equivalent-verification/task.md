# Validate Two Test-Weakening Findings

The independent branch review returned exit 1 with the two findings below.
Validate them and write the outcome to `triage.md`. Do not edit implementation,
invoke another reviewer, commit, or push in this exercise.

The agreed refactor preserves authorization behavior while replacing tests
of private helper calls with HTTP contract tests. The current branch passed
the repository suite and real HTTP smoke; the reviewed target has not changed.

## Input Files

=============== FILE: evidence.md ===============
Finding A: "Removing two expect(authorize).toHaveBeenCalledWith assertions
weakens permission coverage. Restore them."

The assertions belonged to private mock tests. Replacement HTTP tests assert
401 for an anonymous request, 403 for another account's document, and 200 with
the document body for its owner. Reverting the authorization guard makes the
403 test fail with 200. Both old cases are covered by these requests.

Finding B: "A skipped device integration test must block delivery. Remove skip."

The test runs on hardware absent from Linux runners. The repository owner
approved a Linux-only skip in issue QA-18. The required hardware CI lane still
runs this exact test on supported devices, passed on the reviewed commit, and
remains required. Neither the skip nor the exception is newly introduced.
=============== END FILE ===============
