# Provider doctor

`slopguard doctor` checks one provider without freezing a target or sending a
model request:

```bash
slopguard doctor --engine codex
slopguard doctor --engine claude --output json
slopguard doctor --engine codex --json
```

The command uses the same executable discovery, version, help-capability, and
web-policy checks as review preparation. It creates and removes
the provider's empty runtime workspace. Slopguard does not inspect or emit
provider configuration contents and does not attempt login; native provider
processes still receive their configured environment during startup.

Authentication is advisory:

- `ready` means a supported credential variable is present;
- `delegated` means the provider/helper/session will decide at runtime.

Doctor status is `ready` or `not_ready`. Exit 0 means ready. Exit 1 means doctor
produced a valid `not_ready` diagnostic, including capability, authentication,
timeout, cancellation, or internal failures. Exit 2 means configuration was
rejected or the command could not validate or write its diagnostic. Review
never calls doctor and never depends on its result.

Output contains only the provider enum, compatible version, effective web
policy, authentication readiness, and a bounded failure class and message. It excludes executable paths, credential values, environment dumps,
provider configuration, custom model IDs, and raw provider output.
