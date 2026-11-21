# Agents Keep Sending iOS Builds to a Dead Box

Agents keep sending iOS builds to wren and failing. Refresh the fleet skill.
This sandbox can't reach my machines, so I ran checks from my laptop today;
the output is in `probes-today.md`.

Write what you'd say to me in `response.md`. Don't touch the machines.

======= FILE: AGENTS.md =======
# Workspace

Where my agent sessions start on this machine. Workspace skills live in
`skills/<name>/SKILL.md`; the harness links them in. `skills/` is
gitignored.
======= END FILE =======

======= FILE: skills/fleet/SKILL.md =======
---
name: fleet
description: "Route builds, tests and device runs across Sam's machines: this laptop, kestrel, wren, heron-pc. Use when a job is heavy, needs another OS or a device, or more than 3 builds would run here. `fleet maintain` checks health and cleans up."
---

# fleet

| Job | Goes to |
| --- | --- |
| edit loop, lint, one test file | this laptop |
| heavy build, full suite, Docker | kestrel (`ssh kestrel`) |
| untrusted code | throwaway container on kestrel |
| iOS simulator, Xcode build | wren (`ssh wren`) |
| Android device run | Pixel 8 on wren USB |
| Windows check | heron-pc, after `wakeonlan 9c:6b:00:12:34:56` |

## Limits

Starting points; tune to load.

- This laptop: 3 builds at once.
- kestrel: 4 builds at once (16 cores).
- wren: 2 simulators at once.

Maintenance: [maintain](references/maintain.md).
======= END FILE =======

======= FILE: skills/fleet/references/maintain.md =======
# fleet maintain

1. Health: `tailscale status`; `ssh <host> uptime` for each host.
2. Disk: `df -h` per host; prune Docker build cache on kestrel.
3. Updates: list pending OS updates; ask Sam before applying.

Report one row per host.
======= END FILE =======

======= FILE: probes-today.md =======
$ tailscale status
100.64.0.2   laptop     sam@  macOS   -
100.64.0.5   kestrel    sam@  linux   active
100.64.0.7   wren       sam@  macOS   offline, last seen 9d ago
100.64.0.9   osprey     sam@  macOS   active
100.64.0.3   heron-pc   sam@  windows offline, last seen 14h ago

$ ssh kestrel 'nproc; df -h / | tail -1'
64
/dev/nvme0n1p2  1.8T  1.7T   96G  95% /

$ ssh osprey 'sw_vers -productVersion; sysctl -n hw.ncpu; xcodebuild -version | head -1; adb devices'
16.1
24
Xcode 26.1
List of devices attached
39121FDJH00ABC	device
======= END FILE =======
