# Checkers

Pick the first row that fits. Code-level checkers run the real code, so the
model cannot drift. Specs cover what spans processes, services, or time.
Pin the checker version in the repo; read flags from its own docs.

| Invariant shape | Stack | Checker | Kind |
| --- | --- | --- | --- |
| Pure state machine with finite states and events (reducer, lifecycle enum) | Any | Exhaustive enumeration of every event sequence up to length N, in the existing test framework | Bounded, in-language |
| Concurrent data structure or coroutine-level component | Kotlin, Java | [Lincheck](https://github.com/JetBrains/lincheck) in model-checking mode | Bounded interleavings, real code |
| Memory safety, arithmetic, index bounds, parser and state logic | Rust | [Kani](https://model-checking.github.io/kani/) | Bounded model checking, real code |
| Thread interleavings over atomics, mutexes, channels | Rust | [loom](https://github.com/tokio-rs/loom) | Exhaustive up to a preemption bound, real code |
| Bounds, overflow, assertions | C, C++ | [CBMC](https://www.cprover.org/cbmc/) | Bounded model checking, real code |
| Protocol, retry, lease, token refresh, sync, or lifecycle across components | Any (Kotlin and Android, Swift, Go, TypeScript, services) | [TLA+](https://lamport.azurewebsites.net/tla/tla.html), written as PlusCal when the algorithm reads like code, checked with TLC; [Apalache](https://apalache-mc.org/) when integer domains or depth make TLC's state space too large | Spec, exhaustive within bounds |
| Message-passing systems with many event handlers | Any | [P](https://p-org.github.io/P/) | Spec, systematic bounded exploration (not exhaustive) |
| Structural or relational rules: permissions, config, schema, ownership graphs | Any | [Alloy](https://alloytools.org/) | Spec, exhaustive within scope |

A race detector, fuzzer, or property-based test is a test, not a proof.
Keep it, and label it as such in the report.

## Running and negative controls

- **Enumeration:** generate all sequences up to N, assert the invariant
  after each step, and log the sequence count. The variant reinserts the bug
  behind a test-only flag; a test expects the assertion to fail.
- **Lincheck:** `@Operation` methods plus `ModelCheckingOptions().check(...)`
  in a JVM test. The variant is a buggy implementation; a test expects
  Lincheck's failure with an interleaving.
- **Kani:** `#[kani::proof]` harnesses with `kani::any()` inputs and
  `#[kani::unwind(N)]`, run by `cargo kani`. The variant harness drives the
  buggy path and carries `#[kani::should_panic]`.
- **loom:** tests inside `loom::model`, run with `RUSTFLAGS="--cfg loom"`
  in release mode. The variant is a `cfg`-gated buggy ordering whose test
  expects the panic.
- **TLC:** translate PlusCal with `pcal.trans`, then run `tlc2.TLC` with a
  `.cfg` that names constants and invariants. Add a `BUGGY` constant that
  enables the bad step; a second `.cfg` sets it `TRUE`. The variant passes
  only when the output reports the named invariant violated.
- **Apalache:** `apalache-mc check --inv=<Inv> --length=<N>`. Same `BUGGY`
  constant pattern. Type annotations are required.
- **P:** `p compile`, then `p check` with a test case and a schedule count.
  The variant is a test case wiring the buggy machine; it must report the
  spec monitor's assertion.
- **Alloy:** `check <Assertion> for <scope>` in the pinned CLI. The variant
  is a `check` on the buggy predicate that must find a counterexample.

In every case, match the invariant or assertion name in the output, not
only a non-zero exit.
