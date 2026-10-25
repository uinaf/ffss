# Targets and disclosure

Slopguard freezes exactly one explicit target before it calls a provider. It
never auto-detects or fetches a revision.

| Mode | Identity | Reviewed material |
| --- | --- | --- |
| `local` | `HEAD`, or the empty tree when unborn | staged and unstaged diffs, deletions, non-ignored untracked files |
| `branch` | merge base of `--base` and `HEAD`, plus `HEAD` | merge-base-to-`HEAD` diff |
| `commit` | one non-merge commit | parent-to-commit diff (empty tree for a root commit) |

Needs Git 2.45 or newer. Git runs with hooks, filters, credentials, global
config, and lazy fetch off, so a missing object in a partial clone fails the
target.

## Bundle

The bundle holds the target identity, the diff, deleted and untracked file
contents, the task prompt, and any `--context-file` files. Providers get the
bundle as input, never a path to the live checkout.

- The prompt is trusted: pass it with `--prompt`, `--prompt-file <path>`, or
  `--prompt-file -` for stdin. Distill and authorize repository text before
  passing it as the prompt.
- The bundle limit is 1 MiB by default and at most 128 MiB (`max_bytes`). An
  oversized target fails before the provider runs, names its largest
  contributors, and is never split.
- Binary files go as Git's summary line (tracked) or size and SHA-256
  (untracked). A binary that looks like code fails closed, as does any binary
  above 128 MiB.
- These fail closed: invalid UTF-8, binary context files, sensitive paths,
  submodules, merge commits, unsafe revisions, symlink escapes, special
  files, split indexes, and incomplete reads.
- Changed environment templates (`.env.*.example`, `.sample`, `.template`) go
  whole. Other changed `.env` and `.env.*` files go redacted: each line
  becomes `KEY=[REDACTED]` when every line is a simple one-word assignment,
  otherwise `[REDACTED]`. `.env.*` files with a code or config extension
  (`.js`, `.py`, `.sh`, `.yml`, `.json`) fail closed. Context files get no
  template or environment exception.
- A source that changes during the review fails the result as
  `source_changed`.

## What leaves the machine

- The whole bundle, deleted bytes and context included, goes to the selected
  provider.
- With [repository access](CONFIG.md#repository-access), any snapshot file the
  reviewer reads also goes, unchanged files and environment key names
  included.
- Environment values never leave. Slopguard does not scan for credentials or
  judge whether template defaults or key names are safe to disclose.
- The operator authorizes the frozen content for that provider. Secret
  scanning belongs in the repository's own gates.
