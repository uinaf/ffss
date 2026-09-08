# Assess Optimization Evidence

Inspect whether these two benchmark reports are sufficient for an unattended
optimization workflow. Write `readiness.md` with the justified conclusions and
smallest missing proof. Inspection only: do not run benchmarks, modify the
parser, or change verification gates. The repository owns `npm run bench:parse`
and its output-equivalence test; both candidates pass correctness checks.

## Input Files

=============== FILE: measurements.md ===============
Candidate A: baseline 120 ms, candidate 80 ms. The baseline used the production
fixture, a cold cache, and 100 iterations on runner A. The candidate used a
smaller fixture, a warm cache, and 10 iterations on runner B with a newer
runtime. The author claims a one-third latency reduction.

Candidate B: baseline mean 100 ms, candidate mean 97 ms. The command, fixture,
runner, runtime, cache state, and iteration count match. Repeated samples for
both versions span 95–105 ms; the report provides no analysis separating the
3 ms mean difference from run-to-run variation. The author says the lower
mean is enough to call this an improvement.
=============== END FILE ===============
