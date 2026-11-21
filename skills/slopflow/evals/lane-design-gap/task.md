# Dark Lane: Next Steps

You are the dark lane on quillnote/app. Your brief and your notes so far are
below. You've just hit a snag on #111.

You can't reach the repository or GitHub from here. Write what you do next,
including any update to the epic and anything you'd send the coordinator, to
`next.md`.

======= FILE: brief.md =======
# Lane dark: epic #171 Dark mode

Goal, in order: #111 code blocks unreadable in dark mode (S), #205 note list
background stays white on Android (S), #206 dark mode app icon (S). Leave
#109 alone.

You own `src/theme/`, `src/editor/CodeBlock.tsx`, `android/theme/`, and
`assets/icons/`. Branches, PRs, and merges in the repo are yours. Keep the
`## Lane dark` checklist on #171 current. Report to the coordinator on
completion or on a blocker only the user can clear.
======= END FILE =======

======= FILE: notes.md =======
#111: code blocks use a hard-coded light palette. Fixing it means picking
their colors in dark mode. Two options found in the codebase and design
files:

1. Follow the app theme: code blocks go dark whenever the app is dark.
2. Follow the note's own "paper" setting: some users keep notes on a light
   paper even in dark mode, and the design file shows light code blocks on
   light paper in dark mode.

Design file says "TBD with product" next to both. The issue doesn't say.
Option 1 is about 20 lines; option 2 needs the paper setting plumbed into
CodeBlock (about 80 lines).

#205 and #206 don't depend on #111.
======= END FILE =======

======= FILE: epic-171-checklist.md =======
## Lane dark
- [ ] #111
- [ ] #205
- [ ] #206
======= END FILE =======
