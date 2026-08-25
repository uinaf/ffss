# Ready the Repo for an Optimization Effort

## Problem/Feature Description

We plan to have an agent make `parseEvents` in this log tool faster,
unsupervised. Prepare the repository so that work can happen without a human
in the loop. Do not optimize the parser itself; that is the follow-up task.
Record what you changed and any measurements in `readiness.md`.

## Input Files

=============== FILE: package.json ===============
{
  "name": "logsift",
  "version": "0.1.0",
  "type": "module",
  "scripts": {
    "test": "node --test"
  }
}
=============== END FILE ===============

=============== FILE: AGENTS.md ===============
# logsift

Parses exported log archives into deduplicated event lists.

- Run `npm test` before pushing.
- For performance-sensitive changes, ask a maintainer to confirm the CLI
  still feels fast on their logs before merging.
=============== END FILE ===============

=============== FILE: src/parse.js ===============
export function parseEvents(lines) {
  const events = [];
  for (const line of lines) {
    const [ts, id, ...rest] = line.split(" ");
    if (!ts || !id) continue;
    const event = { ts, id, message: rest.join(" ") };
    if (!events.some((seen) => seen.id === event.id)) {
      events.push(event);
    }
  }
  return events;
}
=============== END FILE ===============

=============== FILE: test/parse.test.js ===============
import { test } from "node:test";
import assert from "node:assert/strict";
import { parseEvents } from "../src/parse.js";

test("keeps first occurrence per id and skips malformed lines", () => {
  const lines = [
    "2026-08-01T10:00:00Z a1 boot ok",
    "malformed",
    "2026-08-01T10:00:01Z a1 boot repeated",
    "2026-08-01T10:00:02Z b2 login ok",
  ];
  assert.deepEqual(parseEvents(lines), [
    { ts: "2026-08-01T10:00:00Z", id: "a1", message: "boot ok" },
    { ts: "2026-08-01T10:00:02Z", id: "b2", message: "login ok" },
  ]);
});
=============== END FILE ===============
