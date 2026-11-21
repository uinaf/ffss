# Brief the Three Quillnote Lanes

These three epics are approved. I'll start one agent session per epic and
paste your brief into each, so they run in parallel. They should get their
issues to merged PRs without me unless something needs my call.

Write the three briefs to `briefs.md`. Don't start anything or post anywhere.

======= FILE: epics.md =======
# quillnote/app: approved epics

## #170 Epic: Offline sync conflicts
- #101 [iOS] Offline edits lost after reconnect (M)
- #102 [Android] Offline edits overwritten by the server copy (M)
- #103 [web] No conflict warning when two tabs edit one note (S)
Files a quick scan says these touch: `src/sync/`, `src/editor/Editor.tsx`
(conflict banner), `ios/Sync/`, `android/sync/`.

## #171 Epic: Dark mode
- #110 [iOS] Editor background stays white (S)
- #111 [web] Code blocks unreadable (S)
Files: `src/theme/`, `src/editor/Editor.tsx` (theme props), `ios/Theme/`.
#109 (toggle label 1px off) is parked; don't touch it.

## #172 Epic: PDF export
- #115 [iOS] Export cuts off the last page (S)
- #116 [web] Export ignores the page size setting (M)
Files: `src/export/`, `src/settings/PageSize.ts`, `ios/Export/`.
======= END FILE =======

======= FILE: CONTRIBUTING.md =======
# Contributing

- One PR per issue against `main`, squash merge. Required checks: `lint`,
  `unit`, `e2e-ios`, `e2e-android`, `e2e-web`. `quillbot` reviews every push.
- UI changes need a screenshot from each platform they touch.
======= END FILE =======

======= FILE: environment.md =======
# Where agents run

- Agents edit on their own machine. iOS simulators, Android emulators, and
  the e2e suites run on `buildbox`, a shared 16-core Mac that the rest of the
  team also builds on. Last month six Android emulators at once made it
  unusable for an afternoon.
- The e2e test account allows 5 device sign-ins per 15 minutes; a sixth locks
  it for an hour.
======= END FILE =======
