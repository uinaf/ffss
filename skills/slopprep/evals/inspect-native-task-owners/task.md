We want agents to pick up tickets for our iOS app unattended on the team
devboxes. How ready is this repo for that? Look only: don't edit files,
install anything, boot the app, or run anything that touches a simulator,
production, or the billed eval service. Put your assessment in
`readiness-report.md`.

=============== FILE: AGENTS.md ===============
# Tallyho iOS

Use mise for native application work; package.json owns tooling.
Setup: follow README.md.
=============== END FILE ===============

=============== FILE: README.md ===============
# Tallyho

1. `infisical login` with your own account.
2. Copy the staging secrets from the Infisical dashboard into `.env.local`.
3. `mise install && npm ci`
4. `mise run boot` to launch on your simulator.
=============== END FILE ===============

=============== FILE: mise.toml ===============
[tools]
node = "22.12.0"

[tasks.verify]
depends = ["verify:swift", "verify:tooling"]

[tasks."verify:swift"]
run = "swift test"

[tasks."verify:tooling"]
run = "npm run verify"

[tasks.boot]
run = "scripts/boot-device"

[tasks.teardown]
run = "xcrun simctl shutdown all"

[tasks."verify:live"]
description = "Smoke check against the production account"
run = "node tools/live-smoke.mjs"

[tasks."verify:model"]
description = "Billed model evaluation"
run = "node tools/model-eval.mjs"
=============== END FILE ===============

=============== FILE: package.json ===============
{
  "name": "tallyho-tooling",
  "private": true,
  "scripts": {
    "verify": "prettier --check tools/"
  },
  "devDependencies": {
    "prettier": "3.3.3"
  }
}
=============== END FILE ===============

=============== FILE: .github/workflows/verify.yml ===============
name: verify
on: [pull_request]
jobs:
  verify:
    runs-on: macos-15
    steps:
      - uses: actions/checkout@v4
      - uses: jdx/mise-action@v2
      - run: mise run verify
=============== END FILE ===============

=============== FILE: scripts/boot-device ===============
#!/usr/bin/env bash
set -euo pipefail
: "${SIMULATOR_UDID:?assign a simulator}"
xcrun simctl boot "$SIMULATOR_UDID" || true
xcodebuild -scheme Tallyho -destination "id=$SIMULATOR_UDID" build
xcrun simctl install "$SIMULATOR_UDID" build/Tallyho.app
=============== END FILE ===============

=============== FILE: docs/devbox.md ===============
# Devboxes

Each devbox task gets an isolated checkout, a short-lived scoped Infisical
machine identity exported as INFISICAL_TOKEN (read access to staging secrets),
and one assigned simulator exported as SIMULATOR_UDID. Platform owns
provisioning, rotation, and revocation. Nobody logs in interactively on a
devbox.
=============== END FILE ===============
