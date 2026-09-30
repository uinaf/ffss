# Configuration

Keys, sources, and precedence:
[configuration docs](https://github.com/uinaf/ffss/blob/main/cli/slopguard/docs/CONFIG.md).
The engine has no built-in default.

Inspect resolved values and their source before a paid run:

```bash
slopguard config --repository . --engine "$engine" --json
slopguard doctor --repository . --engine "$engine" --json
```

`doctor` checks the executable, capabilities, and web policy offline and
reports authentication as `ready` (a supported credential variable is set) or
`delegated` (the provider session decides at runtime).

## Runtime

Reviews preserve configured provider or session authentication and run in an
empty temporary workspace holding only the frozen bundle.

## Web access

- Defaults on, so reviewers can search for and fetch references. Pass
  `--web-access=false` when the target must not steer the reviewer to the
  network or the user disallows web access.
- Which sources may re-enable it:
  [configuration docs](https://github.com/uinaf/ffss/blob/main/cli/slopguard/docs/CONFIG.md).
