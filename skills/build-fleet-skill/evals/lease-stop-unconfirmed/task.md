# Fleet Skill With Rented Runners

You asked me the setup questions for routing work across my machines; my
answers are in `answers.md`. This sandbox can't reach them, so I ran the
checks you'd run from my laptop and pasted the output in `probes.md`.

Build the fleet skill in this workspace now. Write what you'd say to me in
`response.md`. Don't touch the machines or start any leases.

======= FILE: AGENTS.md =======
# Workspace

Where my agent sessions start on this machine. Workspace skills live in
`skills/<name>/SKILL.md`; the harness links them in. `skills/` is
gitignored.
======= END FILE =======

======= FILE: answers.md =======
- This laptop: macOS, 10 cores. Edit loops, lint, single test files; at most
  2 builds at once.
- Heavy Linux builds and anything untrusted: rented runners from
  `runnerctl`, one lease per job. Leases bill by the minute, so agents stop
  every lease they start.
- No other machines yet.
======= END FILE =======

======= FILE: probes.md =======
$ runnerctl version
runnerctl 2.4.0

$ runnerctl lease start --size 16cpu --ttl 30m
lease L-7f3a started (16cpu, ttl 30m)

$ runnerctl run L-7f3a -- uname -sm
Linux x86_64

$ runnerctl lease stop L-7f3a
stop requested for L-7f3a

======= END FILE =======
