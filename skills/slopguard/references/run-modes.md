# Run modes

Open when composing a `slopguard review` command.

Staged, unstaged, and non-ignored untracked changes:

```bash
printf '%s' "$task_contract" |
  slopguard review --mode local --engine "$engine" --output json --prompt-file -
```

Branch or PR: `--mode branch --base "$base"` with the PR's real base. One
non-merge commit: `--mode commit --commit "$commit"`.

`--context-file` (repeatable) takes only existing repository-relative evidence.
Keep `--output json` for the canonical report, failures included.
`--prompt-file -` is trusted instruction input; pipe the distilled prompt on
stdin rather than writing a file into the reviewed tree.
