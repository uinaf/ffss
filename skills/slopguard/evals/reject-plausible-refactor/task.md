# Validate a Cheap Defensive Refactor

The branch review completed with the finding below. Use slopguard to decide
the disposition and next actions in `triage.md`. This is an offline exercise;
do not execute commands or claim proposed actions ran.

The approved task adds bounded pagination to the item endpoint, then verifies,
reviews, and ships it through the repository's PR flow. General pagination
cleanup across other endpoints is outside scope. Required checks passed on
the reviewed commit a31dc70. Repository policy requires independent review but
does not require another pass for comment-only edits or rejected findings.

## Input Files

=============== FILE: evidence.md ===============
Review: branch mode, frozen target a31dc70, completed with exit 1.

Finding: "An omitted limit could reach selectItems as undefined and return
every item. Introduce a shared defensive pagination normalizer in selectItems
and migrate the other list endpoints to it. This is plausible and cheap to fix."

The reviewed boundary and sole helper call:

```ts
const querySchema = z.object({
  limit: z.coerce.number().int().min(1).max(100).default(20),
});

function selectItems(items: Item[], limit: number): Item[] {
  return items.slice(0, limit);
}

export function listItems(rawQuery: unknown, items: Item[]): Item[] {
  const query = querySchema.parse(rawQuery);
  return selectItems(items, query.limit);
}
```

Caller inspection confirms selectItems is private and has no other calls.
Observed HTTP integration results at a31dc70: omitted limit returns 20 of 150
items; limit 3 returns 3; limits 0, -1, 101, and nonnumeric text return 400.
The schema rejects invalid requests before the helper runs.

Current HEAD b82e104 differs from a31dc70 by this entire diff:

```diff
-// Clients recieve at most 100 items per page.
+// Clients receive at most 100 items per page.
```

The worktree is clean. No behavior, contract, dependency, or configuration has
changed since review. No other finding or acceptance question is outstanding.
The approved delivery work remains to be completed.
=============== END FILE ===============
