---
name: slopinator
description: >-
  Runs a deterministic, evidence-gated loop×graph implementation workflow via
  the installed slopinator CLI: clarify intake, human release, build, verify,
  independent review, deliver. Use only when the user explicitly invokes
  /slopinator for structured task/plan execution. Do not use for ad-hoc edits
  or planning-only work.
disable-model-invocation: true
---

# Slopinator

Deterministic and structured approach to slop cannoning.

```text
plan → /slopinator → clarify → human releases → machine runs
```

## Require the binary

```bash
command -v slopinator
slopinator version
```

If missing, stop and ask the user to install it (`go install github.com/uinaf/slopinator/cmd/slopinator@latest` or their release path). Do not invent a second runtime.

## Leash

1. Run `slopinator status` (add `--json` when parsing).
2. Obey `next_action` / `allowed_commands`. Do not narrate state-machine theater.
3. Advance only through named CLI commands with structured evidence.

## Three human moments

1. **Release** — after intake, show the confirm-table and wait for the human; then `slopinator release --revision "$INTAKE_REVISION"`.
2. **Review consent** — at intake, confirm approach once: `autoreview` CLI (portable), Cursor `/review-bugbot` (cheap local), both, or human. Store via intake `review_consent`. Do not auto-fire reviewers.
3. **Decide** — park with `slopinator ask --question …`; when status is `NEEDS_DECISION`, `slopinator decide --answer …`.

## Mindful spend

Strong/capable model for BUILD. Prefer cheaper review (Bugbot / lighter autoreview models). Never default “most expensive everywhere.”

## Companion tools

- Preferred portable review: installed `autoreview` binary (ask harness/model if unclear).
- On Cursor: `/review-bugbot` is cheap — still confirm.
- Record review with `slopinator review --evidence PATH.json` (`reviewer`, `verdict`, `artifact_ref`).

## Done

Stop at `RUN_DONE` / `BLOCKED` / awaiting human. Keep chat updates compact; sqlite holds the event log.
