# Assess a Completed Benchmark Comparison

Assess the available proof for this repository's autonomous optimization
workflow and write `readiness.md`. Inspection only; no new runs or edits. Do
not claim production performance or shipping approval from a local benchmark.

## Input Files

=============== FILE: measurements.md ===============
The repository-owned npm run bench:parse command used the same representative
fixture, runner, runtime, warm-cache setup, and 100-iteration budget for each
sample. Baseline and candidate were interleaved; only the parser implementation
changed. All durations below are milliseconds, lower is better.

Baseline: 100, 101, 99, 100, 100
Candidate: 80, 81, 79, 80, 80

The planned acceptance was at least a 10% latency reduction on this workload,
with identical parsed output. The output-equivalence suite passed for both
versions, including malformed input and duplicate-event cases. The measurement
artifacts identify both revisions. Neither revision nor the benchmark contract
has changed since these checks.
=============== END FILE ===============
