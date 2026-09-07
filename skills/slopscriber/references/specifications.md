# Specifications and decisions

Use a durable spec when acceptance is ambiguous or spans owners and the
existing work item or executable contract cannot express it. Small fixes and
dependency changes rarely need one.

Follow the repository's layout and templates. Otherwise use
`docs/specs/<feature-slug>.md` for behavior and
`docs/decisions/<NNNN>-<slug>.md` for consequential choices.

A spec needs the problem, observable requirements and acceptance, constraints,
and material non-goals. A decision record needs context, the choice, and its
consequences. Omit empty sections. Tactical implementation and status stay in
the tracker.

Prefer existing integration or contract tests for executable acceptance. New
checks derive from requirements, accept equivalent implementations, and cover
meaningful failure paths through the normal verification lanes. Do not invent
a fixture framework to support a spec.

When implementation reveals an ambiguity, reconcile the requirement, decision,
and affected acceptance coverage. Record reversals as superseding decisions;
do not rewrite the rationale of a historical choice.

Add a task-shaped pointer when agents need to discover the contract. Retire
documents whose maintenance cost exceeds the decisions they support.
