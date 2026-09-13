# Remove duplicated deployment facts

Clean up `docs/agents.md` so routine host or default changes have one owner.
Preserve information operators need. Write `doc-report.md` describing the
changes and checks you actually performed. Only those two Markdown files may
change. This is a synthetic repository exercise; do not deploy or run host
commands.

All supplied files are checked into the same repository, readable by the
intended operators, and authoritative for their own contents. There are no
generated documentation requirements. The CLI accepts an inventory host name
as the argument to `--host`; the command syntax below remains current.

=============== FILE: inventory/agents.yaml ===============
hosts:
  cedar:
    address: 192.0.2.10
  juniper:
    address: 192.0.2.11
  spruce:
    address: 192.0.2.12
=============== END FILE ===============

=============== FILE: config/agent-defaults.yaml ===============
heartbeat_seconds: 45
retry_limit: 4
=============== END FILE ===============

=============== FILE: docs/agents.md ===============
# Agent operations

## Hosts

| Host | Address |
| --- | --- |
| cedar | 192.0.2.10 |
| juniper | 192.0.2.11 |

## Defaults

Heartbeat interval: 30 seconds. Retry limit: 3.

The heartbeat interval balances outage detection against traffic on metered
links; lower it only after checking the fleet's bandwidth budget.

## Restart

Run `fleetctl restart --host cedar` for cedar.
Run `fleetctl restart --host juniper` for juniper.

## Recovery and retirement

If an agent stops reporting after a rollout, restore its previous image with
`fleetctl rollback --host <host>` and confirm heartbeats resume before retrying.
Removing a host from inventory does not erase its remote data. Export retained
logs before retiring its disk, then remove its monitoring entry to avoid stale
alerts.
=============== END FILE ===============
