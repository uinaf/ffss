Agents working on the scheduler keep burning time in `docs/runtime.md` before
they find what they need. Clean it up. Everything in it is accurate as of
today.

=============== FILE: docs/runtime.md ===============
# Runtime Notes and Background

The runtime changed several times. In 2023 the team used CloudWatch for run diagnostics, then experimented with Splunk, and moved run records and artifacts onto the host during a 2025 cost-control project. Old tickets and screenshots contain references to each design.

The scheduler is started with `pnpm run scheduler`. The process entrypoint is `src/scheduler/main.ts`, while run persistence is implemented in `src/runs/repository.ts`. Run records are stored in the SQLite file named by `RUN_STORE_PATH`. Generated artifacts are written under `ARTIFACT_DIR`, and completion or failure notices are sent through the SMTP relay in `SMTP_URL`.

The host inventory also records that `SENTRY_DSN`, `PAGERDUTY_TOKEN`, `DATADOG_API_KEY`, `PROMETHEUS_URL`, `OTEL_EXPORTER_OTLP_ENDPOINT`, and `LOKI_URL` are unset. Sentry, PagerDuty, Datadog, Prometheus, OpenTelemetry, and Loki were removed, rejected, or never deployed. The runtime does not include hosted log search, metrics scraping, or a telemetry collector.

Calling `curl http://127.0.0.1:6120/status` returns scheduler and run-store status. When logs contain `artifact write failed`, check permissions on the configured artifact directory and retry the run with `pnpm run scheduler:retry -- <run-id>`.

There is no live log-streaming endpoint. Clients that need new output must page `GET /v2/runs/:id/logs?after=<cursor>` every 5 seconds.

A 2025 retrospective describes the host migration as the point when the runtime became smaller than earlier plans.
=============== END FILE ===============
