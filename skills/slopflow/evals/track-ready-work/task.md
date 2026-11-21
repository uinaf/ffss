# Let Lanes Find Their Next Issue

The lanes keep pinging each other and me to ask what they can start next, and
two of them picked up the same issue yesterday. Set up how we track what's
ready, who has what, what's blocked, and what's done, so a lane can find its
next issue on its own.

Our GitHub org has sub-issues and issue dependencies ("blocked by") turned
on. You can't reach GitHub from here. Write the setup, and which issues are
ready right now, to `response.md`.

======= FILE: backlog.md =======
# quillnote/app

## #210 Epic: Shared notebooks (lanes: api, ui)
- #211 Invite model and endpoints client (api lane has it, PR #230 open)
- #212 Accept or decline an invite (needs #211)
- #213 Invite sheet (needs #211)
- #214 Pending invites screen (needs #212)
- #215 Member list on notebooks (no dependencies)

## #220 Epic: Export (lane export)
- #221 PageLayout type (merged #225)
- #222 Page breaks (needs #221)
- #223 Footer on the last page (needs #222; we still need to decide if the
  footer shows page numbers)
- #224 Page size setting (needs #222)

Right now the lanes track this in a shared `PLAN.md` in the repo that each one
edits, and in their own heads.
======= END FILE =======
