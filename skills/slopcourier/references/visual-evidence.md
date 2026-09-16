# Visual evidence ladder

Use an aid when it answers a review question more clearly than the diff and
description already do. Reuse existing evidence; do not create an artifact
just because the change is non-trivial.

## Choose the view

- A focused diff shows an existing behavior's change; show the whole small
  block only when omitted context would hide ownership or order.
- A shallow call tree or Mermaid diagram explains control flow, state, or
  boundaries. Include only the elements needed to understand the change.
- Actual sanitized input/output demonstrates a contract; a screenshot or short
  recording shows observed UI behavior. Label examples and explanatory diagrams
  as such: they do not prove execution or replace required checks.
- Keep the aid beside the claim it supports. If the diff is already clear,
  omit the extra aid. Do not turn delivery into a standalone HTML explainer.

Selection principle adapted from [HumanLayer's show-me skill](https://github.com/humanlayer/skills/blob/main/plugins/show-me/skills/show-me/SKILL.md).

## Attach only when needed

Inline text and Mermaid need no upload. For captured media, use the first
applicable attachment route below.

- Forge rungs apply only to the forge the delivery dispatched to; never upload
  through the other forge's API because its CLI happens to be installed.
- Never commit proof assets to any product repository branch (no
  `.github/pr-assets` or similar).

## Recording content

A recording proves an interaction; the reviewer's time starts at frame one.

- Start at (or within ~2 seconds of) the first relevant action and end when
  the outcome is visible. App launch, setup, and waiting are not evidence.
- Review the artifact before attaching: check the duration with `ffprobe` and
  confirm the opening frames show relevant state, not an idle screen.
- Trim dead time instead of re-recording:
  `ffmpeg -ss <start> -to <end> -i in.mp4 -c copy out.mp4`.
- When the wait itself is the behavior (a progress or loading state), compress
  it: a before/after screenshot pair or a short clip of the transition, never
  real-time idle footage.
- Prefer roughly 15 seconds or less; beyond that, a labeled screenshot
  sequence usually reads better.

## 1. github.com deliveries (`gh --attach`)

`gh` 2.99+ uploads media natively; check `gh --version` first (older `gh`
uses the fallback below). `gh pr create`, `gh pr edit`, `gh pr comment`, and
the `issue` equivalents take a repeatable `--attach <file>` flag, at most 50
per command: png, jpg, jpeg, gif, webp, svg, mp4, mov, webm. `gh pr review`
does not; use `gh pr comment` for review media. The asset inherits repository
visibility.

```bash
gh pr create --attach './after.png#Login error state'   # alt text after '#', quoted
gh pr comment 13 --attach ./before.png --attach ./after.png
gh pr edit 13 --attach ./flow.mp4                       # video takes no alt text
```

- If the body already references the local path (`![alt](./after.png)`), `gh`
  rewrites that reference to the uploaded URL; otherwise it appends the asset.
- Video is embedded as a bare URL on its own line, where GitHub renders a
  player. Never wrap it in `![]()` yourself. `#alt` on a video fails with
  "cannot set alt text on video".
- Transcode Playwright's webm for broad playback first:
  `ffmpeg -i in.webm -c:v libx264 -pix_fmt yuv420p out.mp4`.
- `--attach` does not combine with `--web` or `--dry-run`.
- Partial upload failure on create still creates the pull request and reports
  the failed files; retry them with `gh pr edit --attach`.
- Requires GitHub.com or a GHE.com tenant, an OAuth token, classic PAT, or
  fine-grained PAT, and WRITE or higher on the repository. GitHub Enterprise
  Server and GitHub App installation tokens are unsupported
  ([cli/cli#14309](https://github.com/cli/cli/issues/14309)), so unattended
  runtimes on App tokens fall through to the fallback below, then to rung 3.

<details><summary>Fallback: gh below 2.99, GitHub Enterprise Server, or an App token</summary>

Upload to the CDN the web drag-drop uses, then embed the returned `.url`
(images as markdown, video as a bare line):

```bash
repo_id=$(gh api repos/{owner}/{repo} -q .id)
name=$(jq -rn --arg v "<file basename>" '$v|@uri')
curl -s "https://uploads.github.com/user-attachments/assets?name=${name}&content_type=<mime>&repository_id=${repo_id}" \
  -X POST \
  -H @- \
  --data-binary @<file> <<EOF
Authorization: Bearer $(GH_HOST=github.com gh auth token)
Accept: application/json
EOF
```

The heredoc keeps the token out of the process argument list. 422 =
unsupported content type; 404 = bad repository id or no push permission.

</details>

## 2. GitLab deliveries (`glab`)

Upload through the project uploads API and embed the returned markdown:

```bash
glab api "projects/:id/uploads" --form "file=@evidence.png"
```

- The response carries a `markdown` field (`![…](/uploads/…)`); paste it into
  the change-request description or a comment.
- Uploads inherit project visibility.

## Embedding

Structure the evidence; never paste bare images that take over the page:

- Constrain size with an HTML img tag, especially mobile screenshots
  (`<img src="…" width="300">`).
- Put comparisons side by side in a table (before/after, light/dark,
  breakpoints), one labeled column each.
- Collapse anything long or secondary in `<details><summary>…</summary>`.
- One primary aid inline; everything else collapsed or linked.

## 3. Non-media artifacts or endpoint failure

- Do not force non-media artifacts through a media endpoint.
- Fall back to the change request itself: paste the decisive excerpt as
  fenced, sanitized text (contract input/output, log lines), or link an
  artifact the forge already hosts, such as a CI job artifact or check run.
- If no honest attachment path exists, deliver without the aid and say so.
