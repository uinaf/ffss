Time for my monthly agent setup retro. I copied my Claude Code setup and last
month's prompt history into this folder (`home/` is my home directory). Tell
me what to prune, fix, or add. Don't change anything yet; we'll discuss first.
An old note from a previous audit is in there too.

=============== FILE: home/.claude/CLAUDE.md ===============
# Rules

- Keep replies short. Lead with the outcome.
- Write clean, readable code and handle errors properly.
- Open a pull request for every change; never push to main.
- After a pull request is open, wait for review bots, then merge when green.
=============== END FILE ===============

=============== FILE: home/.claude/skills/shipit/SKILL.md ===============
---
name: shipit
description: "Deliver finished work: commit, push, and merge."
---

# Shipit

1. Commit with a Conventional Commits message.
2. Push straight to main; small changes don't need a pull request.
3. Keep the final message short and lead with the outcome.
=============== END FILE ===============

=============== FILE: home/.claude/skills/diagram/SKILL.md ===============
---
name: diagram
description: "Draw architecture diagrams as Mermaid."
---

# Diagram

Render the requested architecture as a Mermaid diagram.
=============== END FILE ===============

=============== FILE: home/.claude/skills/changelog/SKILL.md ===============
---
name: changelog
description: "Write release notes from merged pull requests."
---

# Changelog

Group merged pull requests by type and write release notes.
=============== END FILE ===============

=============== FILE: home/.claude/history.jsonl ===============
{"display":"fix the flaky login test","project":"/home/me/app","timestamp":1756710000000}
{"display":"/changelog for v2.3","project":"/home/me/app","timestamp":1756720000000}
{"display":"too long, make it shorter","project":"/home/me/app","timestamp":1756730000000}
{"display":"ship it","project":"/home/me/app","timestamp":1756800000000}
{"display":"why did you push to main?? I said PR","project":"/home/me/app","timestamp":1756800100000}
{"display":"too long. shorter please","project":"/home/me/api","timestamp":1756900000000}
{"display":"/changelog for v2.4","project":"/home/me/api","timestamp":1757000000000}
{"display":"add retries to the webhook client","project":"/home/me/api","timestamp":1757100000000}
{"display":"wall of text again, shorter","project":"/home/me/api","timestamp":1757200000000}
{"display":"ship it","project":"/home/me/api","timestamp":1757300000000}
{"display":"stop pushing to main, open a PR","project":"/home/me/api","timestamp":1757300100000}
{"display":"too long","project":"/home/me/app","timestamp":1757400000000}
=============== END FILE ===============

=============== FILE: notes/previous-audit.md ===============
# Audit (last quarter)

- `changelog` is never used; delete it.
- `diagram` is used weekly; keep it.
=============== END FILE ===============
