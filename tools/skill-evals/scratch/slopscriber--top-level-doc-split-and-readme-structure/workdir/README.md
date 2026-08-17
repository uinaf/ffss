# grok-cli

grok-cli parses structured log files (JSON, logfmt, CEF) and pretty-prints them with filtering, highlighting, and aggregation support. It works with stdin or file inputs.

## Install

```bash
brew install grok-cli
```

```bash
npm install -g grok-cli
```

To build from source, see [Contributing](CONTRIBUTING.md#setup).

## First command

Parse a JSON log file:

```bash
grok parse app.log
```

## More usage

Filter by level:

```bash
grok parse app.log --level error
```

Pipe from stdin:

```bash
cat app.log | grok parse -
```

Aggregate by field:

```bash
grok stats app.log --group-by service
```

Output formats: `--format pretty` (default), `--format json`, `--format csv`

## More documentation

- [Contributing](CONTRIBUTING.md) — development setup, architecture, coding standards, tests, pull requests
- [Security](SECURITY.md) — reporting a vulnerability

## License

MIT License — see [LICENSE](LICENSE) for details.
