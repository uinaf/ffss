# Start the Shared Notebooks Lanes

The shared notebooks epics are approved. Get the lanes going: one agent
session per epic, in parallel.

You can't start sessions or post from here. Write what you'd do now, and what
you'd send to each lane, to `kickoff.md`.

======= FILE: epics.md =======
# quillnote/app: shared notebooks

Goal: a user can invite others to a notebook; invitees accept or decline;
the owner can revoke access; unanswered invites expire after 14 days.

## #180 Epic: Invites API client
- #181 Create and list invites (M)
- #182 Accept or decline an invite (S)
- #183 Revoke access and expire invites (M)

## #184 Epic: Sharing UI
- #185 Invite sheet with email entry (M)
- #186 Pending invites screen (S)
- #187 Shared badge and member list on notebooks (M)

## #188 Epic: Notifications
- #189 Push and email when invited (M)
- #190 Notify the owner on accept, decline, or expiry (S)
======= END FILE =======

======= FILE: api-notes.md =======
Backend team's draft for the invites endpoints. States and transitions are
"roughly" as follows; nobody has written them down. An invite can be
accepted after it was revoked in the current backend; the backend team asked
whether clients should treat that as an error.
======= END FILE =======
