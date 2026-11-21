# Pause the Lanes

Hold everything. I need buildbox for a customer demo in ten minutes and I
don't want anything moving on the repo until it's over. I'll tell you when to
go again.

The lanes' latest state is below. Write the message you'd send to the lanes
now, and the one you'd send when I say go, to `hold.md`. Don't post anywhere.

======= FILE: lanes-now.md =======
## Lane sync (epic #170)
- PR #161 (#102) is green with approval; the lane said at 13:55 "merging #161
  in a minute".
- Running on buildbox: an iOS simulator `sync-ios-1` and the `e2e-ios` suite
  for #102.
- Branch `sync/103-conflict-banner` has two local commits, not pushed.

## Lane dark (epic #171)
- Rebasing `dark/111-code-blocks` onto main after #160 merged.
- Waiting on `flock /tmp/quillnote-signin.lock` on buildbox to sign the test
  account into an Android emulator `dark-android-1`, which is booted.

## Lane export (epic #172)
- Idle since its last message; PR #157 (#115) open at head 4d5e6f7.
======= END FILE =======
