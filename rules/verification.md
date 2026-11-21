## Verification

The repository guide owns proof. Without one, run what the user will use (the
app, CLI, or endpoint the change touches), choosing the cheapest run that
would fail if the change were wrong. Put lasting checks at the highest layer
the gate affords: real surface, owned boundary, properties, then units for
pure logic. Prove costly invariants tests rarely reach, such as races,
retries, and state machines, with a model checker or bounded proof. Replay
its counterexample on the real code: remove one guard in the model and in the
code, and both must fail.

- Keep gates and hooks enabled; fix change-caused failures without asking.
  A gate counts only after you've seen it refuse a bad change, including an
  attempt to skip it.
- Local runs and read-only live checks need no scope; paid calls, shared or
  production writes, and shared devices do.
- A check proves a claim only if it fails on a negative control, such as the
  fix reverted; otherwise it's an observation.
- Take expected values from a spec, fixture, captured real I/O, or property,
  never from the code under test. Mock only process, network, clock, or
  randomness boundaries, never the behavior asserted.
- Add a test only for a regression existing coverage would miss, once, at its
  strongest boundary. When a change breaks a test, fix the code unless the
  contract changed on purpose or the test pins implementation; then update
  the test and say so.
- For a performance claim, alternate baseline and candidate runs, count
  completed work and errors inside the timed region, and report the spread
  and the user-visible effect.
- Validate review findings before acting on them.
- Clean up only the processes you started.
- When reporting a change, name what ran, failed, was skipped, or was
  unavailable, with each proof's revision and command. Never claim an
  unexecuted check passed.
