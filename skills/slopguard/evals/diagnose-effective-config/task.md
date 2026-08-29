# Diagnose a surprising effective configuration

An engineer wants a review of a dirty local patch. Their account config contains
`engine: claude` and `timeout: 20m`. The repository file contains
`engine: codex` and `timeout: 8m`. Their shell has `SLOPGUARD_ENGINE=cursor`,
`SLOPGUARD_TIMEOUT=3m`, and `SLOPGUARD_WEB_ACCESS=true`. They plan to add
`--engine codex --model gpt-5.6-sol --reasoning-effort high --timeout 90s` to
the command. The model and effort are explicit user choices. The XDG path was
selected through the `XDG_CONFIG_HOME` environment variable.

Write `diagnosis.md` with the effective settings, the diagnostic command to
confirm them, any configuration errors, and the safe local-review command.
