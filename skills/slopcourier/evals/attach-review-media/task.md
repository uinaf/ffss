# Open the settings retry PR with the recording

Branch `fix/settings-save-retry` in `tallowmere/console` (origin
`git@github.com:tallowmere/console.git`) is done and its checks passed on the
head. The fix: when saving settings fails, the panel now keeps the unsaved
input and shows a keyboard-reachable Retry button. Previously the form reset
and the input was lost. CI can't show this interaction.

Evidence I captured, all in `evidence/` in this checkout:

- `before.png`, `after.png`: mobile screenshots, 1170x2532, of the failed-save
  state on main and on the branch.
- `save-retry.webm`: the raw Playwright video of the whole test run.
- `steps.log`: the test's step timestamps, below.

Local tooling: `gh version 2.99.2`, logged in to github.com with my OAuth
token; `ffmpeg` and `ffprobe` installed.

Don't push, open, or upload anything yet. Write `delivery.md` with the exact
commands you would run from here, including any processing of the evidence,
and the PR body as it should end up.

=============== FILE: evidence/steps.log ===============
00:00.0 launch chromium
00:03.2 goto /login
00:09.8 login ok
00:14.1 goto /settings
00:18.6 fill "Display name" = "Ada L."
00:19.4 click Save (settings API stubbed to 500)
00:20.1 error toast visible; "Display name" still "Ada L."
00:26.8 click Retry (stub restored)
00:27.5 "Saved" toast visible
00:41.0 teardown
=============== END FILE ===============
