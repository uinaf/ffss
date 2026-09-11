# Provider doctor

`slopguard doctor` checks one provider without freezing a target or sending a
model request:

```bash
slopguard doctor --engine codex
slopguard doctor --engine claude --output json
slopguard doctor --engine codex --json
```

It runs the same executable discovery, version, help-capability, and web-policy
checks as review preparation, and creates and removes the provider's empty
runtime workspace. It omits the Cursor authentication probe that review
preparation runs, so it stays offline and spends no provider call. It does not
inspect or emit provider configuration contents or attempt login; native
provider processes still receive their configured environment during startup.

Authentication is advisory:

- `ready`: a supported credential variable is present;
- `delegated`: the provider/helper/session decides at runtime.

Status is `ready` or `not_ready`. Exit 0: ready. Exit 1: a valid `not_ready`
diagnostic, including capability, authentication, timeout, cancellation, or
internal failures. Exit 2: configuration rejected, or the diagnostic could not
be validated or written. Review never calls doctor or depends on its result.

Output contains only the provider enum, compatible version, effective web
policy, authentication readiness, and a bounded failure class and message. It
excludes executable paths, credential values, environment dumps, provider
configuration, custom model IDs, and raw provider output.
