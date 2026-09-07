# Security and issue reporting

- The CLI freezes one explicit local, branch, or commit target into a bounded
  UTF-8 bundle and labels repository material as untrusted.
- It sends the complete frozen bundle, including deleted bytes and context, to
  the selected provider without credential scanning.
- It invokes provider executables outside the reviewed repository and refuses
  stale source after provider execution.

- Do not bypass a sensitive-path, size, binary-data, symlink, revision,
  capability, or source-change refusal.
- Confirm the frozen target is authorized for disclosure to the selected
  provider; Slopguard does not scan it for credentials.
- Do not split an oversized bundle and claim whole-change cleanliness.

## Execution approval

Slopguard is a local launcher, not a hosted review service. The selected harness
sends the frozen bundle to its configured model provider; disabling web search
does not make that processing offline.

Use the task's existing authorization for that repository, target, and provider.
When execution needs approval, state those facts, what content leaves the
machine, and that the reviewer only reports. Installation, login, or this skill
alone does not authorize disclosure. Do not expose credential values to prove
the route.

If approval is denied, report the reason and continue independent work. Do not
hide the transfer, switch providers, weaken the sandbox, or retry through
another command. An unresolved disclosure boundary needs a scoped user decision;
see [Codex auto-review](https://learn.chatgpt.com/docs/sandboxing/auto-review).

## Public defects

Create a public issue only for a reproducible non-security slopguard defect.
Search open and closed issues first:

```bash
gh issue list --repo uinaf/ffss --state all --search "$sanitized_summary in:title"
```

The issue may contain only:

- slopguard version
- operating system and architecture
- provider name and web-access state
- stable failure class
- sanitized reproduction steps using public or synthetic input
- expected behavior and high-level actual behavior

Never include the frozen bundle, task prompt, reviewed source, diff, repository
identity, private or absolute paths, credentials, environment output, provider
command line, or raw provider stdout/stderr. Write a sanitized body to a
temporary file and use
`gh issue create --repo uinaf/ffss --title "$sanitized_summary" --body-file "$sanitized_body"`.

If GitHub access is missing or any field cannot be safely sanitized, do not
create the issue; tell the user what prevented safe reporting.

## Vulnerabilities

- Never open a public issue for suspected secret exposure, command injection,
  path traversal, unsafe provider execution, sandbox escape, bundle-boundary
  failure, or malformed output accepted as clean.
- Use private vulnerability reporting from the `uinaf/ffss` repository
  Security tab, with synthetic, high-level reproduction details.
- The same exclusion list applies to private reports.
