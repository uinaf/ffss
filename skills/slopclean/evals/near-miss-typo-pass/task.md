quick one: fix the typos in docs/setup.md and the link that points nowhere. don't touch anything else

=============== FILE: docs/setup.md ===============
# Setup

Install the CLI with Homebrew, then run `streamlog init` once per host. The
init step writes `/etc/streamlog/config.toml` and registers a systemd unit; it
does not start the service.

Start it with `systemctl start streamlog`. Logs from the shipper itself go to
the jounral, so `journalctl -u streamlog` is the first place to look when
nothing arrives downstream.

For non-Homebrew platforms see [installing from source](./instal.md). The
default port is 5140; change it in the config, not with a flag, because the
flag is ignored once a config file exists. Seperate hosts can share one config
through your configuration managment tool.
=============== END FILE ===============

=============== FILE: docs/install.md ===============
# Installing from source

Build with `make release` and copy `bin/streamlog` onto your PATH.
=============== END FILE ===============
