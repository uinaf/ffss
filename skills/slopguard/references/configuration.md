# Configuration

Slopguard resolves one flat typed configuration: CLI flags, then
`SLOPGUARD_*` environment variables, then `.slopguard.yaml` at the Git root,
then the account config file, then built-in defaults. The engine has no
built-in default. Keys, validation, and precedence:
[configuration docs](https://github.com/uinaf/ffss/blob/main/cli/slopguard/docs/CONFIG.md).

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

- Defaults off.
- Only an explicit flag or the ownership-checked `~/.config/slopguard/config.yaml`
  may enable web access. A file selected through `XDG_CONFIG_HOME`, repository
  configuration, and environment variables cannot.
