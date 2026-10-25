# Model contract

## Location

- Specs (TLA+, P, Alloy): `formal/<model>/`, or the repo's existing
  convention. One directory per model: the spec, its configs, and the
  variant config.
- Code-level harnesses (Lincheck, Kani, loom, enumeration): beside the code
  in the test source set, in a file or module named for the invariant.
- No inventory file: the directory listing is the inventory.

## Mapping

Every spec opens with a header comment, in its own comment syntax, that
maps it to code. TLA+ shown:

```text
\* Models: <invariant in one sentence>
\* Code:   <repo path>:<symbol>
\* | Model           | Code                               |
\* | sessions        | SessionStore.tokens                |
\* | Refresh(c)      | TokenRefresher.refresh() on 401    |
\* | Fail(c)         | network error before the response  |
\* Bound: <constants>; <why a bug needs no more>
\* Abstracts away: <what the model leaves out and why that is safe>
```

The mapped code gets a one-line comment naming the model, so an edit there
leads to it. A code-level harness needs no table: it calls the code.

## Bounds

- State each bound and why it suffices: the known bug needs at most that
  many clients, retries, or steps. Two or three of each finds most bugs.
- Record distinct states or depth and wall time for each run in the report.
  A jump between runs means the model or code changed shape; look at why.
- Liveness (something eventually happens) needs fairness assumptions;
  state them beside the property.

## CI

- One repo task (Gradle, npm, `make`, `mise`, Cargo) runs every model and
  every variant. CI calls that task; local runs use the same task.
- Pin the checker: a release version with checksum, a lockfile entry, or a
  toolchain file.
- A variant passes only when the checker reports its named invariant
  violated.
- Path filter: the model directory plus every mapped code path.
- Time bound per job. Pull requests run the small bound; a scheduled job
  runs the larger one.
- Keep checker output as a CI artifact when a run fails, so the
  counterexample trace survives.

## Drift

A spec is a second copy of the design. Keep it honest:

- A change to mapped code updates the model or states in the change
  request why the model still holds.
- When a bug escapes in modeled code, first make the model fail on it (a
  new variant), then fix the code.
- Retire a model whose code is gone; delete the directory and its CI entry.
