The `AGENTS.md` in this repo is two years old and agents working here are
unreliable. Rewrite it for the Claude, Codex, and Grok agents we run today,
and keep it short. Put a few lines on what you changed and why in
`guide-notes.md`. Don't touch anything else, and don't run deploy.

=============== FILE: AGENTS.md ===============
# Agent Instructions

You are a world-class senior staff engineer.

IMPORTANT: You MUST ALWAYS ask before running ANY command.
NEVER modify files outside src/ and docs/. NEVER EVER.
For Claude: put every answer inside XML tags.
For GPT: restate the task before acting.

Before finishing ANY change, run `npm test` AND `npm run e2e`.

Code review: run `npm run review` and ask the reviewer to report only
high-severity issues. Max 2 review rounds per change; stop after 3 fix
commits.

You have a limited context window. Once you have used 60% of your tokens,
wrap up and summarize.

Always ask the user if unsure. Always ask before committing.
=============== END FILE ===============

=============== FILE: package.json ===============
{
  "name": "tideline-web",
  "private": true,
  "scripts": {
    "dev": "vite",
    "build": "vite build",
    "test": "vitest run",
    "test:visual": "storybook test --ci",
    "e2e": "playwright test",
    "review": "slopguard review",
    "deploy": "wrangler deploy --env production"
  }
}
=============== END FILE ===============

=============== FILE: docs/layout.md ===============
# Layout

- `src/lib/` pure booking and pricing logic (unit tests beside it)
- `src/components/` React components with Storybook stories
- `src/routes/` pages; checkout and search flows are covered by Playwright in `e2e/`
- `docs/` product and developer docs
- Production deploy is `npm run deploy`, run only by the on-call maintainer.
=============== END FILE ===============
