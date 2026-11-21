# How Are the Lanes Doing?

How are the three lanes doing, and what needs me? Their latest messages and
the epic checklists are below; it's Thursday 14:00.

Write your reply to me in `response.md`, and anything you'd send to the lanes
in `messages.md`. Don't post anywhere.

======= FILE: lane-messages.md =======
## Lane sync (epic #170), Thursday 13:40
#101 merged as #160 (merge commit a1b2c3d on main). #102: PR #161 is green,
head 9e8f7a6, proven on the Android emulator; iOS not run yet because buildbox
was busy. #103 is stuck: do we show a conflict banner and let the user pick,
or merge both edits automatically and keep a history entry? That's a product
call.

## Lane dark (epic #171), Thursday 12:15
#110 merged as #158. #111 needs the editor theme hook from the sync lane's
Editor.tsx change in #160. Waiting until that lands, then I'll rebase.

## Lane export (epic #172), Tuesday 16:05
#115: PR #157 open, head 4d5e6f7. quillbot flagged that page size is read
twice; fixing. #116 might be pointless if PDF export goes away in 4.0. Need a
decision.
======= END FILE =======

======= FILE: epic-checklists.md =======
## #170 Lane sync
- [x] #101 merged #160
- [ ] #102 PR #161
- [ ] #103 needs-user: conflict UX

## #171 Lane dark
- [x] #110 merged #158
- [ ] #111 blocked on #160

## #172 Lane export
- [ ] #115 PR #157
- [ ] #116 needs-user: keep PDF export?
======= END FILE =======
