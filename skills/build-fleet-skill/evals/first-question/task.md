# Where Should Builds Run?

Agents on this laptop keep running everything locally and cooking it. I have
other machines they could use. Set up a fleet skill in this workspace so
agents know where builds, tests, and device runs go.

Write what you'd say to me in `response.md`. Don't connect to or start
anything on other machines.

======= FILE: AGENTS.md =======
# Workspace

Where my agent sessions start on this machine. Workspace skills live in
`skills/<name>/SKILL.md`; the harness links them in.
======= END FILE =======

======= FILE: ssh-config.txt =======
Host kestrel
  HostName kestrel.tail4b2e.ts.net
  User sam
Host wren
  HostName wren.tail4b2e.ts.net
  User sam
Host heron-pc
  HostName 192.168.1.40
  User sam
======= END FILE =======
