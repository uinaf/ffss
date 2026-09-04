# Review cadence

An authorized implementation uses slopguard for final independent review.
Write `cadence.md` with the next action for each snapshot:

1. One of three planned edits is complete. Its focused test passed; two edits
   remain. No one requested an interim review.
2. All edits are complete. Required checks and runtime proof passed. There is
   no independent result for this target yet.
3. A valid clean review exists for that exact frozen target, base, task
   contract, and review requirements. Another agent turn begins; nothing changed.
4. After that review, two accepted fixes change the target. The first fix's
   focused test passed, while the second is still being edited.

State when checks and review should run again and what evidence is sufficient
for final handoff. Do not invoke a model or modify a repository.
