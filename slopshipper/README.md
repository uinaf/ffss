# slopinator

Deterministic and structured approach to slop cannoning.

```text
make a plan  →  /slopinator  →  clarify with the human  →  human releases  →  machine runs
```

`slopinator` is a Go CLI and thin agent skill: an evidence-gated loop×graph state
machine so coding agents build, verify, and deliver work one auditable
transition at a time — mindful token spend, without sacrificing quality.

## Status

Bootstrap in progress. Track work on [GitHub Issues](https://github.com/uinaf/slopinator/issues).

## Install (planned)

```bash
go install github.com/uinaf/slopinator/cmd/slopinator@latest
slopinator --version
```

## Companion tools

- [`autoreview`](https://github.com/uinaf/autoreview) — preferred portable independent review CLI
- On Cursor: `/review-bugbot` as a cheap local review option (confirm before use)

## License

[MIT](LICENSE)
