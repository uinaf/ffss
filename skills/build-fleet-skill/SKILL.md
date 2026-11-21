---
name: build-fleet-skill
description: "Build or maintain a private `fleet` skill: where builds, tests, untrusted code, and device runs go across one user's machines, plus how to maintain them. Use when asked to set up fleet routing or offload work to other machines, or when an existing fleet skill sends work to a dead host or misses a new one."
---

# Build fleet skill

Write a `fleet` skill for one user's machines, from what this machine can
reach. The generated skill names their hosts, addresses, and accounts, so it
lives only in their workspace. This skill stays generic; so does anything you
write outside that workspace.

| Ask | Mode |
| --- | --- |
| no fleet skill yet, or "start over" | [Build](#build) |
| a fleet skill exists | [Maintain](#maintain) |

## Build

1. **Locate.** Find the workspace and its skills directory from its agent
   guide, else the harness's project skills directory. Target
   `<skills>/fleet/`. If the workspace is pushed anywhere public, pick a
   gitignored or untracked path and say which.
2. **Inventory.** Read what this machine shows before asking: its OS, cores,
   and memory; SSH config hosts; tailnet peers; remote-build or lease CLIs on
   `PATH`; attached devices (`adb devices`, `xcrun simctl list`). Each
   question's options come from this inventory.
3. **Interview.** One question at a time, one decision per question: the
   decision, options from the inventory, and a recommendation. Lead with the
   question; the inventory follows in a line or a small table. Ask the first
   now and hold the rest until it's answered; write no routing on hosts
   guessed from their names. Skip what the inventory already settles and
   state it instead. In order:
   1. Hosts: which machines count. OS and reach come from the probe.
   2. Roles: which jobs each host takes (edit loop, heavy build, full suite,
      another OS, simulators, devices).
   3. Rules: who may start what where; hosts that sleep, are shared, or are
      off limits at times.
   4. Untrusted code: where a stranger's repo may run, and where never.
   5. Limits: concurrent builds, simulators, or emulators per host.
   6. Devices: phones, TVs, and emulators, and which host each hangs off.
   7. Needs the user: reboots, OS updates, deletes, waking shared machines.
4. **Probe.** Reach each host and device read-only: `uname` or `ver`, core
   count, free disk, toolchain versions, `adb devices`. Wake a sleeping host
   only if the user allowed it. Done when every host has a probe result, or
   the user decided on its failure: fix now, drop, or keep marked
   unverified. A host checked only by the user counts when they pasted its
   output.
5. **Write** the [generated skill](#generated-skill).
6. **Report**: the path, each host with its probe result, and what stays
   unverified.

## Maintain

1. Read the generated skill, then redo Inventory and Probe for every host
   and device, including the ones nobody complained about.
2. Classify each difference as **drift**:
   - host offline, renamed, or gone; a long offline stretch differs from a
     machine that sleeps;
   - a new host or device;
   - capacity change: cores, memory, disk pressure, toolchain versions;
   - a device moved to another host.
3. Fix drift in place and keep the user's own edits:
   - Record what the probe shows: a dead host marked unavailable, a device
     on its new host, new capacity.
   - Route a dead host's jobs to remaining hosts that already hold that
     role.
   - A new host's role goes to the user as one [Interview](#build) question;
     route to it only as pending that answer.
   - A capacity change proposes a limit scaled from the probe, as a starting
     point the user can tune.
4. Report one row per host: state, drift, and what you changed or left for
   the user. Done when every host in the skill and the inventory has a fresh
   probe result or a pending question.

## Generated skill

`SKILL.md` and `references/maintain.md`, shaped so an agent routes the same
way every time:

- Frontmatter: `name: fleet` and a `description` that leads with what it
  does, then its triggers (a heavy build, another OS, a device, more builds
  than the local limit, `fleet maintain`).
- **Route**: a table or decision list from job kind to host, with how to
  reach it: edit loop, heavy build, full suite, each OS, simulators, each
  device, untrusted code.
- **Limits**: per host, as starting points to tune against live load; record
  a change that sticks.
- **Rules**: wake steps, quiet hours, shared machines, and who starts
  sessions where. Untrusted code runs only where the user allowed it.
- **Cleanup**: stop what you started and remove your own worktrees,
  emulators, and scratch on every host. A lease, VM, or remote job is
  stopped only when a readback (list or status) no longer shows it; a stop
  request alone is not done. Keep an unconfirmed one in the report as open,
  re-check it a bounded number of times, then hand it to the user.
- **Upkeep**: one line saying that a dead host, a new machine, or a moved
  device means re-running this generator in Maintain mode.

`references/maintain.md` runs `fleet maintain`, one section per step:

1. **Health**: every host up, reachable, and not alerting.
2. **Drift**: config and toolchains against what the user declared.
3. **Updates**: apply only what interrupts nobody; list the rest.
4. **Disk**: hosts past a threshold; delete only stale, regenerable caches.
5. **Checkouts**: stale worktrees and clones with no uncommitted work, no
   unpushed branch, and no process using them.
6. **Needs the user**: reboots, OS upgrades, deletes beyond caches, waking
   shared machines.

Its report is one row per host, and done means every red cell has a fix, an
issue, or a pending item for the user.
