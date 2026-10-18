# Run modes

```bash
printf '%s' "$task_contract" |
  slopguard review --mode local --engine "$engine" --output json --prompt-file -
```

- `--mode local`: staged, unstaged, and untracked non-ignored changes.
- `--mode branch --base "$base"`: a branch or PR against its real base.
- `--mode commit --commit "$commit"`: one non-merge commit.

Pipe the prompt on stdin with `--prompt-file -` so nothing lands in the
reviewed tree. `--context-file` (repeatable) takes existing repo-relative
files. Keep `--output json`; failures use the same report shape.
