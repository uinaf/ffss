# Visual evidence ladder

Give a non-trivial change one clear review aid, chosen by what the change
shows best: a labeled UI screenshot, a short recording for interaction or
motion, a focused diagram, or sanitized contract input/output as fenced
text. Skip visual evidence for trivial or text-only changes rather than
manufacturing filler.

Pick the FIRST rung whose tool is available; never commit proof assets to
any product repository branch (no `.github/pr-assets` or similar).

## 1. attach-cli (when installed)

If `command -v attach-cli` succeeds, it owns attachment end to end: run it
against the change request URL and the asset per its `--help`, and embed
whatever reference it returns.

## 2. GitLab (`glab`)

Upload through the project uploads API and embed the returned markdown:

```bash
glab api "projects/:id/uploads" --form "file=@evidence.png"
```

The response carries a `markdown` field (`![…](/uploads/…)`); paste it into
the change-request description or a comment. Uploads inherit project
visibility.

## 3. GitHub (`gh` + user-attachments endpoint)

Images and video upload to the same CDN the web drag-drop uses; the asset
inherits repository visibility and needs no browser:

```bash
repo_id=$(gh api repos/{owner}/{repo} -q .id)
curl -s "https://uploads.github.com/user-attachments/assets?name=<file>&content_type=<mime>&repository_id=${repo_id}" \
  -X POST \
  -H @- \
  --data-binary @<file> <<EOF
Authorization: Bearer $(gh auth token)
Accept: application/json
EOF
```

The heredoc keeps the token out of the process argument list, where other
local processes could read it. Embed the returned `.url` as markdown. Failure modes: 422 = unsupported
content type; 404 = bad repository id or no push permission.

Video: same endpoint with `content_type` `video/mp4` or `video/webm`, and
embed the returned URL on its own bare line — GitHub renders a player there,
while `![]()` image syntax does not. Transcode Playwright's webm for broad
playback first:

```bash
ffmpeg -i in.webm -c:v libx264 -pix_fmt yuv420p out.mp4
```

## 4. Non-media artifacts or endpoint failure

Do not force non-media artifacts through a media endpoint. Fall back to
the change request itself: paste the decisive excerpt as fenced, sanitized
text (contract input/output, log lines), or link an artifact the forge
already hosts, such as a CI job artifact or check run. If no honest
attachment path exists, deliver without the aid and say so instead of
inventing one.
