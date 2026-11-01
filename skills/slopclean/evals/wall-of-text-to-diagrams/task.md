`docs/design.md` is 900 lines in the real repo and goes stale every time
someone touches the UI. This is a trimmed copy. Clean it up so it stops
rotting.

=============== FILE: package.json ===============
{
  "private": true,
  "workspaces": ["packages/app", "packages/sync", "packages/store", "packages/api"]
}
=============== END FILE ===============

=============== FILE: packages/app/package.json ===============
{ "name": "@relay/app", "dependencies": { "@relay/sync": "workspace:*", "@relay/store": "workspace:*" } }
=============== END FILE ===============

=============== FILE: packages/sync/package.json ===============
{ "name": "@relay/sync", "dependencies": { "@relay/api": "workspace:*", "@relay/store": "workspace:*" } }
=============== END FILE ===============

=============== FILE: packages/store/package.json ===============
{ "name": "@relay/store" }
=============== END FILE ===============

=============== FILE: packages/api/package.json ===============
{ "name": "@relay/api" }
=============== END FILE ===============

=============== FILE: packages/sync/src/session.ts ===============
export type SessionState = "signedOut" | "pairing" | "active" | "expired";
export type SessionEvent = "pairStarted" | "paired" | "codeExpired" | "tokenRejected" | "signedOut";

export function next(state: SessionState, event: SessionEvent): SessionState {
  switch (state) {
    case "signedOut":
      return event === "pairStarted" ? "pairing" : state;
    case "pairing":
      if (event === "paired") return "active";
      return event === "codeExpired" ? "signedOut" : state;
    case "active":
      if (event === "tokenRejected") return "expired";
      return event === "signedOut" ? "signedOut" : state;
    case "expired":
      return event === "pairStarted" ? "pairing" : state;
  }
}
=============== END FILE ===============

=============== FILE: docs/design.md ===============
# Relay design

Relay is split into four packages. The app package is the entry point and it talks to the sync package, which in turn uses the api package to reach the server, and both the app and the sync package read and write through the store package, which owns the local database. The api package does not depend on anything else in the repo, and neither does the store package, which keeps them easy to test on their own.

## Session

When the user first opens the app they are signed out. Starting pairing moves them to the pairing state, where a code is shown. If the code expires before it is used, the app goes back to signed out. Once the code is paired the session becomes active. While active, a rejected token moves the session to expired, and signing out returns to signed out. From expired, the app refreshes the token and goes straight back to active without asking the user. Starting pairing again from expired also works.

## Screens

### Pairing screen

The pairing code is shown in 48sp monospace, centred, with 24dp padding above and 16dp below. The QR code sits 32dp under the code. Updated in 2.3: the QR code moved from the right column to below the code after feedback.

### Home screen

Rows use 72dp height with 16dp horizontal padding. The settings toggle was reduced from 40dp to 32dp in 2.4 because it looked huge next to the list item. The empty state shows the illustration at 160dp with the headline 12dp below it.

### Settings screen

Section headers are 14sp medium with 8dp bottom margin. Sign out is the last row, styled as a destructive button with white text since 2.5.
=============== END FILE ===============
