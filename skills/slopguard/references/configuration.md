# Configuration

Keys and precedence: [config docs](https://github.com/uinaf/ffss/blob/main/cli/slopguard/docs/CONFIG.md).
There is no built-in default engine.

Check resolved values before a paid run:

```bash
slopguard config --repository . --engine "$engine" --json
slopguard doctor --repository . --engine "$engine" --json
```

`doctor` runs offline and reports auth as `ready` (a credential variable is
set) or `delegated` (the provider session decides at runtime).

**Web access** is on by default. Pass `--web-access=false` when the target
shouldn't steer the reviewer to the network or the user says so.
