# Specifications and decisions

Use a durable spec when acceptance is ambiguous or spans owners and cannot be
expressed clearly in the existing work item or executable contract. Small fixes
and dependency changes rarely need another document.

Follow the repository's layout and templates. Otherwise use
`docs/specs/<feature-slug>.md` for behavior and
`docs/decisions/<NNNN>-<slug>.md` for consequential choices.

A spec needs the problem, observable requirements and acceptance, constraints,
and material non-goals. A decision record needs context, the choice, and its
consequences. Omit empty sections. Tactical implementation and status stay in
the tracker.

Prefer existing integration or contract tests for executable acceptance. New
checks should derive from requirements, accept equivalent implementations,
and cover meaningful failure paths through the normal verification lanes.
Do not invent a fixture framework merely to support a spec.

When implementation reveals an ambiguity, reconcile the requirement, decision,
and affected acceptance coverage. Record reversals as superseding decisions;
do not silently rewrite the rationale of a historical choice.

Add a task-shaped pointer when agents need to discover the contract. Retire or
reduce documents whose maintenance cost exceeds the decisions they support.
