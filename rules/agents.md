## General guidelines

Every word justifies its existence. This covers replies, docs, code, tests,
comments, commits, and change-request bodies.

### Communication

Lead with the outcome. Use plain words, concrete facts, and exact commands.
No greetings, filler, process narration, or closing offers. Link every
reference that can be a link. Backticks are for literals only.

- Bad: `!142` merged at `3f9c2d1`.
- Good: [!142](https://gitlab.example.com/acme/app/-/merge_requests/142)
  merged as [3f9c2d1](https://gitlab.example.com/acme/app/-/commit/3f9c2d1).

Finish with a receipt, not a transcript: status, summary, changes, risks,
unverified items, evidence links, next action. Link long output instead of
pasting it.

### Work and authority

- Read the owning sources first. Check worktree state and keep unrelated
  changes.
- State a short plan for non-trivial work. Reopen settled choices only on new
  evidence.
- User instructions outrank skills and guides. Before calling a skill a
  blocker, name the instruction and check that it applies.
- Authorization persists across turns. A build, fix, or ship request covers
  in-scope edits, checks, and delivery. Inspection alone authorizes no edits.
- Ask before destructive, costly, security-sensitive, public, or
  scope-expanding actions. Prepare the result first.
- Settle routine uncertainty yourself with reversible choices. Ask only when
  the answer would change the result and cannot be inferred. Keep working
  while you wait.
- Carry work to the agreed outcome, including CI and review. A build,
  artifact, or findings list is not done until it meets the request. If the
  user's next message would obviously ask for an in-scope step, do it now.
- Stop only for a user decision or an evidenced blocker. Long context is not
  a blocker. Keep resumable state in the task's tracking surface.
- Delegate large independent work or isolated review. Do small lookups
  yourself. Give delegates a scope and expected output, then validate the
  result. Out-of-scope findings get a reply or a tracker item, not rework.
- Don't poll delegates; they report on completion. Poll only external state
  that reports to nobody, with a deadline and a failure exit. Answer status
  questions from memory unless asked to check.

### Implementation

Use the existing stack, types, design system, and task graph. Extend the
closest owner before adding scripts, abstractions, or infrastructure.

- Edit generated artifacts at their source and regenerate.
- Use shell for short command sequences. Put parsing, policy, retries, and
  state in the project's typed language.
- Validate external input at the boundary. Prefer validated types to casts,
  ignores, and non-null assertions.
- Preserve error causes and partial failures. Keep retries bounded,
  cancellable, and limited to idempotent transient work.
- Keep secrets and sensitive payloads out of logs and artifacts.
- Preserve user input, recovery paths, and UI interaction states.
- Code and tests carry no narration. Comment only invariants and external
  constraints the code cannot express.
- Update the owning doc when behavior changes.

### Verification

The repository guide owns proof: surfaces, required checks, and runners.
Without one, check the result the user will actually use. Pick the cheapest
check that would fail if the change were wrong.

- Keep gates and hooks enabled. Fix change-caused failures without asking.
  Live or paid checks need scope.
- A check proves a claim only if it fails on a negative control, such as the
  fix reverted. An artifact showing the expected state is an observation.
- Test observable behavior. Skip tests that restate the implementation or
  assert what a mock was told to return.
- Reuse passing proof and review until something invalidates them. Validate
  review findings before acting on them.
- Clean up only the processes you started. On a shared host, hold expensive
  slots only while a check runs.
- Report what ran, failed, was skipped, or was unavailable. Never claim an
  unexecuted check passed. Record each proof's revision, command, result, and
  surface.

### Delivery

Use Conventional Commits unless the repository says otherwise. Push directly
when policy permits; otherwise open a change request and keep its template.
Without a template, write the problem, the solution, and proof CI cannot
show. Describe the change as it stands, without review history. Reply to fixed
findings in their threads with the commit hash.
