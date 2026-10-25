# Lane families

What each part of the generated skill must answer. Write the answer, or a
link to the script or task that gives it. A family the repo lacks gets no
file.

## Gates and proof map (`SKILL.md`)

- The single merge gate and the faster affected command, if one exists.
- One proof-map row per kind of change the repo sees: docs, pure logic, app
  or service code, UI, device behavior, live API, release. Each row names
  the exact command or lane.
- A **negative control** per row: how that proof goes red when the change
  is wrong. Typical controls: revert the fix and watch the check fail; an
  injected-failure marker the script requires; a wrong fixture or account
  that fails closed. A row with no way to go red is an observation; say so.

## Heavy work (`SKILL.md`, "Where it runs")

- Which work leaves this machine: slow builds, full suites, emulators,
  Docker, other operating systems, physical TVs or phones.
- Name the routing skill or runbook (for example a `fleet` skill) and stop
  there. Copy no hosts, leases, or credentials. Put private routing in a
  shared repo only when its owner adopts it.
- The same repo commands run there; no lane has a remote-only variant.

## Devices (`references/devices.md`)

Emulators, simulators, and physical devices.

- Boot, reuse, and stop through the repo's script. Link its `--help`.
- **Ownership:** stop only the device you booted, by its exact serial or
  UDID. Reuse running ones. Kill nothing by process name.
- Which tasks uninstall the app or clear its data, and so must not run
  between authenticated steps.
- The opt-in lane pattern: build and install commands, the opt-in flag
  shape, the pass marker to require, and a timeout.
- Device-wide settings a lane changes, and how to record and restore them.
- Lanes that act on the whole device (launcher, system settings) need a
  device no other proof uses.

## Live account (`references/live.md`)

- **Identity:** the sanctioned test profile and the command that signs it
  in: OAuth in a browser, or a device code a person approves. Then the
  status command to check before relying on it. Passwords, personal
  profiles, and tokens in arguments, logs, or the repo are never the path;
  tokens stay in the CLI's store or the device's secure storage.
- **Fixtures:** a uniquely named root per run. Record each created id in a
  ledger (ignored path) before each write. Remove only ledger ids, leaves
  before parents, then confirm each is gone.
- **Unknown outcome:** keep the ledger, reconcile read-only, and do not
  repeat the mutation.
- Shared account settings: read before, restore after.
- Proof that made no live calls is synthetic proof; the report says so.

## Evidence (`references/evidence.md`)

- The capture command and where captures land (an ignored directory).
- What the script quarantines (corrupt, blank, static) and that quarantined
  files are never attached.
- **Look at each capture** before attaching it. A capture of the wrong
  screen, a loading state, or an error is not proof.
- The attach command for the forge. Evidence is never committed.
- Attribution or redaction rules for captured content.
