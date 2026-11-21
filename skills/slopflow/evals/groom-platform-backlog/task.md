# Sort Out the Quillnote Backlog

Our backlog is a mess. I want it in a shape where I can hand chunks of it to a
few agents working in parallel. Both repositories' open issues are below,
exported a few minutes ago.

You can't reach GitHub from here, so don't try to change anything. Write what
you would do, and what you need from me, to `response.md`.

======= FILE: issues-app.md =======
# quillnote/app: open issues

Monorepo with the iOS, Android, and web clients. Labels name the platform.

- #101 [iOS] Notes edited offline lose changes after reconnect (label: ios)
  > Edit a note in airplane mode, edit it on the web, reconnect: the iOS edit
  > is gone. Looks like last-writer-wins on the whole note.
- #102 [Android] Offline edits disappear after sync (label: android)
  > Same as iOS: offline edits are overwritten by the server copy when two
  > devices edited the note. Repro steps and a screen recording attached.
- #103 [web] No conflict warning when two tabs edit the same note (label: web)
  > Second tab silently overwrites the first. We never surface a conflict.
- #104 Search ignores text inside attachments (labels: ios, android, web)
- #105 [iOS] Search takes 4s with more than 5,000 notes (label: ios)
- #106 [Android] Search doesn't match accents, "cafe" misses "café" (label: android)
- #107 Search results order feels off (label: web)
  > No repro. Reporter didn't answer the follow-up question in March.
- #108 [Android] Crash when sharing a note with images (label: android)
  > Comment from @dana (June): "Fixed by #140, shipped in 3.2.0." Still open.
- #109 Settings: dark mode toggle label is 1px lower than the others (label: web)
- #110 [iOS] Dark mode: editor background stays white (label: ios)
- #111 [web] Dark mode: code blocks unreadable (label: web)
- #112 Move off the legacy v1 notes endpoints (labels: ios, android, web)
  > Comment from @sam (April): "v1 was shut down in March; all clients call v2
  > since #150."
- #113 [Android] lost my notes help (label: android)
  > "my notes are gone after i was offline". No other detail.
- #114 Sync drains battery after the session expires (labels: ios, android)
  > Comment from @sam: "The app just calls the SDK. Root cause is the SDK's
  > retry loop, which never backs off on 401."
- #115 [iOS] Export to PDF cuts off the last page (label: ios)
- #116 [web] Export to PDF ignores the page size setting (label: web)
  > Comment from @lee (product): "We might drop PDF export in 4.0 for the
  > system print dialog. Not decided."
- #117 Make the app better (no label)
======= END FILE =======

======= FILE: issues-sync-sdk.md =======
# quillnote/sync-sdk: open issues

The sync client every app platform embeds.

- #41 Retry loop ignores 401 and never backs off
- #42 Conflict detection compares device wall-clock timestamps
  > Clock skew between devices picks the wrong winner.
- #43 Publish the SDK's API reference
======= END FILE =======
