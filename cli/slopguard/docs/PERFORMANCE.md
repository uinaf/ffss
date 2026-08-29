# Performance measurement

Slopguard records internal spans for the work around a model call. These spans
are not part of result schema v1 and are not emitted unless an in-process
observer is configured.

| Phase | Work |
| --- | --- |
| `config` | argument, prompt, and effective-config resolution |
| `dependency_probes` | Git and provider executable discovery and capability probes |
| `target_freeze` | target resolution, collection, and bundle construction |
| `provider_preparation` | isolated runtime, prompt, schema, and invocation preparation |
| `provider_process` | selected model CLI process only |
| `protocol_decode` | provider envelope and canonical review decoding |
| `source_revalidation` | post-attempt frozen-source check |
| `report_write` | terminal or JSON rendering and output |

The clock and observer are injectable. Deterministic tests use a stepped clock;
production uses Go's monotonic `time.Time` component. Total `duration_ms` starts
before config loading, so config and dependency failures retain elapsed work.

The hermetic benchmark uses controlled Git and Codex wrappers and
reports p50/p95 total time, p50/p95 non-provider overhead, each phase's p50, and
the median subprocess count:

```bash
go test ./cmd/slopguard -run '^$' \
  -bench '^BenchmarkReviewOverhead$' -benchtime=10x -count=1
```

Shared CI enforces subprocess budgets for cold local, branch, commit, and
malformed-retry paths. It does not enforce host-specific millisecond limits.
