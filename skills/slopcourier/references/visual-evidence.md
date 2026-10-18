# Visual evidence

Reuse existing evidence first. Pick the smallest view that answers the review
question: a focused diff or code sample, a call tree or Mermaid diagram, a
table, sanitized input/output, a screenshot or short recording. Put it next to
the claim it supports. Label diagrams and examples as explanatory; they don't
prove execution.

Text, tables, code, and Mermaid need no upload. Media goes up through the
PR's own forge. Proof assets are never committed to the repo.

## Recordings

- Start within about 2 seconds of the first relevant action; end when the
  outcome is visible.
- Trim instead of re-recording: `ffmpeg -ss <start> -to <end> -i in.mp4 -c copy out.mp4`.
  Check the result with `ffprobe` before attaching.
- Transcode webm first: `ffmpeg -i in.webm -c:v libx264 -pix_fmt yuv420p out.mp4`.
- Show a long wait as a before/after pair or a short clip of the transition.
- Over about 15 seconds, a labeled screenshot sequence reads better.

## GitHub

`gh` 2.99+ takes a repeatable `--attach` on
`pr create`, `pr edit`, `pr comment`, and the `issue` equivalents.
`gh pr review` doesn't; use `gh pr comment` for review media.

```bash
gh pr create --attach './after.png#Login error state'   # alt text after '#'
gh pr comment 13 --attach ./before.png --attach ./after.png
gh pr edit 13 --attach ./flow.mp4                       # video takes no alt text
```

- Video embeds as a bare URL on its own line. No `![]()`, no alt text.
- `--attach` doesn't work with `--web` or `--dry-run`. Retry a failed upload
  with `gh pr edit --attach`.
- GitHub App installation tokens get a 404 from the upload endpoint
  ([cli/cli#14309](https://github.com/cli/cli/issues/14309)); fall through to
  [No upload route](#no-upload-route).

## GitLab

```bash
glab api "projects/:id/uploads" --form "file=@evidence.png"
```

Paste the response's `markdown` field into the description or a comment.

## Embedding

- Constrain size, especially mobile screenshots: `<img src="…" width="300">`.
- Put comparisons side by side in a table with labeled columns.
- One primary aid inline; collapse the rest in `<details>`.

## No upload route

Paste the decisive excerpt as fenced, sanitized text, or link an artifact the
forge already hosts (CI artifact, check run). If nothing works, deliver
without the aid and say so.

Selection adapted from [HumanLayer's show-me skill](https://github.com/humanlayer/skills/blob/main/plugins/show-me/skills/show-me/SKILL.md).
