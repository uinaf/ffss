`docs/agents.md` has drifted again: spruce is missing from the host table and
the defaults are wrong. Fix the doc so this stops happening. Don't run
`fleetctl` or touch the hosts; the doc and a short note of what you changed
and checked are all I need.

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
