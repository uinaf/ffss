We settled the scope for team invitations (SCOPE.md). Break it into Jira work
in TEAM so several agents can pick pieces up in parallel. Jira isn't reachable
from here, so give me the epic, the tickets, and their links to create by hand.
Don't start implementing.

=============== FILE: AGENTS.md ===============
# Agent guide

Planned engineering work lives in Jira project TEAM. Multi-session features
use an Epic with child Tasks and `blocks` links for hard dependencies.
=============== END FILE ===============

=============== FILE: SCOPE.md ===============
Team invitations

- An admin creates an invitation in the admin UI; the API stores it and emails
  the invite link.
- The invitee opens the link, accepts, and joins the team.
- An admin can revoke a pending invitation from the admin UI.

Stack: React admin UI, Node API, Postgres. Every change ships with its tests.
=============== END FILE ===============
