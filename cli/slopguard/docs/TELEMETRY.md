# Optional telemetry

Slopguard telemetry is disabled by default. It performs no filesystem or
network work unless enabled explicitly for a review.

Enable one command with `--telemetry`, or set `telemetry: true` in the
ownership-checked account file at `$HOME/.config/slopguard/config.yaml`.
Repository configuration, environment variables, and an XDG path selected by
`XDG_CONFIG_HOME` cannot enable collection.

After the review result is written, Slopguard appends one sanitized event to:

```text
$HOME/.local/state/slopguard/telemetry.jsonl
```

The local store is locked across processes, limited to 256 events and 1 MiB,
keeps the newest events, and drops malformed records during recovery. A store
failure does not change review output, duration, status, or exit code.

Events contain only:

- telemetry, CLI release, and result-schema versions;
- provider enum, target mode, isolation, web access, outcome, and failure
  class;
- attempt outcomes and protocol-recovery enum;
- coarse bundle-size, finding-count, and phase-duration buckets.

They contain no paths, revisions, hashes, prompts, findings, prose, raw
provider output, environment values, credentials, custom model identifiers,
or persistent user/repository identifiers.

Export is an explicit local command:

```bash
slopguard telemetry export > telemetry.json
```

The command emits one sanitized JSON object and does not upload or delete the
local events. Transport and retention after export belong to the caller.
