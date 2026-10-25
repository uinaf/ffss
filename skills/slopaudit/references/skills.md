# Skill quality

What lint and evals cannot judge: whether a skill is worth routing to and
cheap to load. Run `skillcheck lint` first; evidence beats stylistic
preference.

## Metadata and discovery

- `name` is concrete and easy to say out loud. `helper`, `tools`, and `utils`
  are discovery smells.
- `description` is third person and says what the skill does and when to use
  it. It is always loaded: front-load the concrete action or domain that
  should activate it.
- One trigger per materially distinct request branch. Collapse synonyms that
  rename one branch.
- State the main overlap boundary without naming another skill.

## Body shape

- `SKILL.md` holds workflow, principles, boundaries, and routing. Lead with the
  task, not a bibliography.
- Spend tokens on repo-specific judgment. Delete any instruction that would
  not change a capable model's behavior.
- Match freedom to risk: high for contextual judgment, medium when a preferred
  pattern exists, low for fragile operations.
- Say what evidence to gather and what a complete result includes. End each
  step on an observable completion condition, not "understood" or "handled".

## Progressive disclosure

- Durable detail, rubrics, and long examples go in `references/`, one hop from
  `SKILL.md`, each with a task-shaped retrieval job. Material every path needs
  stays inline.
- Repeated deterministic work routes to the target's existing framework,
  schema, task graph, or library; otherwise to a tested module in the
  project's primary language, not ad-hoc shell written as prose.
- Executable code another project maintains: link the exact public artifact
  and the contract it demonstrates instead of forking it into prose.
- A package stays independently usable: it states prerequisites and
  out-of-scope next steps locally, and never invokes, imports, or assumes a
  sibling skill.

## Grade

Grade each dimension strong, mixed, or weak:

| Dimension | Question |
| --- | --- |
| Discovery | Does metadata alone route a realistic request here? |
| Workflow | Does the body say how to begin, what evidence to gather, when to stop? |
| Progressive disclosure | Is detail in the right file? |
| Repo fit | Are links, commands, and conventions current? |
| Verification | Is the strongest mechanical check named, plus a real evidence loop? |
| Boundaries | Are limits and next steps stated without leaning on a sibling skill? |

Blockers: invalid frontmatter; a description that fails discovery; stale
commands, paths, or links; a workflow with no start, evidence loop, or
completion; conflicts with the repo's guidance; sibling-skill dependencies.

Majors: vague name; synonym-stuffed description; bloated `SKILL.md`; missing
or muddy boundaries; prose re-inventing a deterministic tool; abstract
examples.

## Fix

Fix blockers first, then the highest-leverage majors, each with the smallest
change that improves activation, decision quality, or proof. When pruning,
measure common-path context for representative requests; line count alone
hides retrieval cost. After edits, rerun `skillcheck lint` and the repo's gate,
and rerun evals when behavior was the thing changed.
