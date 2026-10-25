# Configuration

Keys and precedence: [config docs](https://github.com/uinaf/ffss/blob/main/cli/slopguard/docs/CONFIG.md).
There is no built-in default engine.

Check resolved values before a paid run:

```bash
slopguard config --repository . --engine "$engine" --json
slopguard doctor --repository . --engine "$engine" --json
```
