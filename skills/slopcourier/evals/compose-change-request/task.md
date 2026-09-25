# PR for the websocket compression fix

Branch `fix/ws-compression` in `tallowmere/dashboard` (origin
`git@github.com:tallowmere/dashboard.git`) is finished. `npm test` passed on
the branch head a few minutes ago (214 passed), and CI runs the same suite on
every PR. The repository itself has no pull request template anywhere in its
tree.

I want to read everything before it goes up. Don't push, open, or edit
anything on GitHub. Write `delivery.md` with the exact commands you would run
from this checkout to open the PR, followed by the title and the complete body
you would submit.

What changed: dashboard clients on slow links were dropping live updates
because every websocket frame shipped uncompressed. The server now negotiates
permessage-deflate and echoes the negotiated `Sec-WebSocket-Extensions`
header. Staging, busiest feed: median frame size went from 41 KB to 11 KB.
Clients that don't offer the extension still get uncompressed frames.

Files touched: `server/ws.ts`, `server/handshake.ts`, `server/ws.test.ts`.

An independent review ran twice on this branch. The first pass caught the
missing header echo, fixed in `a1f2c3d`; the second pass was clean.

Last three merged PR titles in this repo:

- `fix(api): stop dropping auth renewals under clock skew`
- `perf(worker): cut cold-start p95 from 900ms to 320ms`
- `feat(alerts): page on-call before the queue backs up`
