# Skill quality

What lint and evals cannot judge: whether a skill is worth routing to and
cheap to load. Run `npx -y @uinaf/skillcheck lint --root <dir>` first on each
dir holding `skills/`.

## Metadata and discovery

- `name` is concrete and easy to say out loud. `helper`, `tools`, and `utils`
  are discovery smells.
- `description` is third person and says what the skill does and when to use
  it. It is always loaded: front-load the concrete action or domain that
  should activate it.
- One trigger per materially distinct request branch. Collapse synonyms that
  rename one branch.
- State the main overlap boundary. Name another skill only when it ships in
  this plugin or one this plugin depends on.

## Body shape

- `SKILL.md` holds workflow, principles, boundaries, and routing. Lead with the
  task, not a bibliography.
- Say what to do. Keep a prohibition only for a likely, costly mistake,
  beside the action that replaces it.
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
- A plugin stays independently usable. Its skills may link and invoke each
  other and skills of a plugin it depends on; they never assume a skill
  outside those.

## Grade

Grade each dimension strong, mixed, or weak:

| Dimension | Question |
| --- | --- |
| Discovery | Does metadata alone route a realistic request here? |
| Workflow | Does the body say how to begin, what evidence to gather, when to stop? |
| Progressive disclosure | Is detail in the right file? |
| Repo fit | Are links, commands, and conventions current? |
| Verification | Is the strongest mechanical check named, plus a real evidence loop? |
| Boundaries | Are limits and next steps stated without leaning on a skill outside the plugin and its dependencies? |

Blockers: a description that fails discovery; stale commands or paths; a
workflow with no start, evidence loop, or completion; conflicts with the
repo's guidance; a dependency on a skill outside the plugin and its
dependencies.

Majors: vague name; synonym-stuffed description; bloated `SKILL.md`; missing
or muddy boundaries; prose re-inventing a deterministic tool; abstract
examples.

## Fix

Fix blockers first, then the highest-leverage majors, each with the smallest
change that improves activation, decision quality, or proof. When pruning,
measure common-path context for representative requests. After edits, rerun
`skillcheck lint` and the repo's gate, and rerun evals when behavior was the
thing changed. For a disputed rewrite, one judge grades both versions'
outputs in one pass, under anonymous labels, against one rubric.
