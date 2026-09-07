# Prepare two separate delivery decisions

For each case below, prepare a short title/body or state the concrete blocker in
`delivery.md`. Do not push, open, or combine change requests. The supplied
outputs are verified observations from this session. Both repos use github.com,
have working host-specific gh auth, authorize preparation only, and have no
template.

A. Branch `refactor/retry-helper`: a private forwarding helper was inlined. The
exported API, error behavior, and timing are unchanged. The repository's
affected gate ran before and after: typecheck and the same 12 retry contract
tests pass. The checkout has no unrelated changes. Reverting this refactor also
passes all 12 tests, as expected. Recent title:
`refactor(client): simplify request retries`.

B. Branch `docs/verify-command`: README said `npm test`; the existing tracked
`mise.toml` and CI both invoke `mise run verify`. Only that command text
changed. Documentation link/render checks passed this session; source code is
untouched. Repository policy requires docs checks for docs-only changes. The
checkout has no unrelated changes. Recent title:
`docs: correct local setup command`.

Explain each proof boundary without inventing new test requirements or running
unrelated gates.
