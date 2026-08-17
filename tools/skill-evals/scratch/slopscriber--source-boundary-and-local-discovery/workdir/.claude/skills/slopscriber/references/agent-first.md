# Agent-First Documentation

Optimize internal and operational docs for agent retrieval. Human readers benefit from the same precision and scanability.

Use this deletion test: would removing the line cause a capable agent to make a material mistake? If not, remove it or move it behind a focused link. Delete instructions that merely restate capable-model defaults; they spend attention without changing behavior.

## Budget Retrieval

Agent-facing guidance spends two different resources:

- **Context cost**: always-loaded rules and pointers consume attention on every task.
- **Discovery cost**: material moved out of context must still be found by an agent or remembered by a human.

Reduce context cost with focused links, but do not hide required guidance behind an unnamed or weakly described target. Spend always-loaded words on routing that reliably changes what the agent reads next.

## Select Information

Keep facts an agent cannot safely infer:

- canonical commands
- owned paths and source-of-truth files
- architectural and security boundaries
- repo-specific conventions
- required prerequisites
- known failure recovery
- exact verification gates

Remove or route elsewhere:

- generic engineering advice
- directory tours available through file search
- narrative history in current-state docs
- repeated explanations of linked material
- exhaustive lists of removed or unavailable capabilities
- speculative guidance and future possibilities
- introductions, transitions, and recaps without operational facts

Treat the repository as a source of truth. Documentation that repeats an easy file search, script listing, config value, or `--help` lookup is a cache and must justify its drift risk. Document the lookup only when it is expensive, ambiguous, or missing the rationale an agent needs.

For deterministic setup, policy, or workflow shapes, point to the maintained
script, config, test, or reference implementation and name the contract it
demonstrates. Keep rationale and adaptation boundaries in prose; do not make an
agent reconstruct code from prose or copy the implementation into another doc.

## State Capabilities

Apply the [negative-state rule](../SKILL.md#negative-state-rule). Lead with what works, where it lives, and how to use it.

```diff
- Redis, RabbitMQ, and Kafka are not provisioned.
+ Jobs use PostgreSQL through `DATABASE_URL`.
```

When an actionable limitation stays, show the supported path:

```md
Webhook delivery is unsupported. Poll `GET /v1/jobs/:id` for status.
```

Do not rehome deleted names under `Exclusions`, `Unavailable`, `Not supported`, or similar headings.

## Match Structure to Information

- Put the answer or action directly below its heading.
- Do not put a bibliography before the task. Cite a source beside the claim it
  supports; keep attribution-only material in a narrow upstream notice.
- Use short sentences.
- Put one independent fact in each bullet.
- Use numbered lists only when order matters.
- Use tables for exact mappings and repeated-field comparisons.
- Use paragraphs for rationale, trade-offs, and causal explanation.
- Keep paragraphs to one idea; two or three sentences usually suffice.
- Keep examples minimal and normative.
- Use code blocks only for copyable commands, data, or exact syntax.
- Preserve literal commands, paths, identifiers, and error text for search.
- Use one stable, familiar term for each recurring concept; define it once instead of restating its meaning under several labels.

Prefer task-shaped headings such as `Start`, `Verify`, `Deploy`, and `Recover`. Avoid headings that merely mirror the file tree.

For procedures and runbooks, end each ordered step with an observable completion condition. Prefer exhaustive bounds such as "every modified command resolves" over vague outcomes such as "docs understood" or "review complete."

## Design Pointers

A pointer is the short text that decides whether an agent opens another source. Its wording is part of the behavior, not decorative navigation.

- Front-load the task or domain that should trigger the lookup.
- State both what the target contains and when it is needed.
- Give each distinct branch one trigger; collapse synonyms that describe the same branch.
- Describe the question answered instead of listing a bare filename.
- Keep mandatory instructions inline only when a focused pointer cannot make retrieval reliable.

## Progressive Disclosure

- Keep the root entrypoint a map.
- Link one level down to task- or domain-specific sources.
- Apply the pointer rules above to every link.
- Put specialized rules near the files they govern.
- Split a document when readers routinely need only one section.
- Avoid fragmentation when the same task always requires every fragment.
- Use imports, symlinks, or generated views when multiple harnesses need identical guidance from one authored source.

Each link costs a tool call. Split by retrieval boundary, not by arbitrary size.

## AGENTS.md

Use `AGENTS.md` for requirements that apply to agent work in its scope.

Match the content to the guidance layer:

- **Global or owner guidance** routes identities, workspaces, repositories, and
  harness policy. Keep project mechanics in their owning repository.
- **Repository guidance** gives a compact working model for cross-cutting work:
  what the product or system does, who relies on it, what must not regress,
  repository-wide hazards, architecture or data flow that changes code
  placement, exact lifecycle commands, and the surfaces a change may need to
  cover.
- **Scoped guidance** carries package, language, or subsystem rules needed only
  after an agent enters that scope.

A repository guide can be more than a table of contents. Inline a fact when it
changes many tasks and a missed lookup would cause a material error. Route
deep architecture, complete references, and local-only conventions behind
task-shaped pointers.

Translate values and taste into decisions. Pair each abstract preference with
an observable consequence, known failure mode, or small example. For example,
`protect performance` becomes useful when it names the regressions to watch and
the proof expected. A bounded matrix such as clients, providers, entry points,
connection modes, or reverse states earns its context cost when omissions
across those dimensions are a recurring defect.

Keep at the root:

- a brief product or system orientation and its non-negotiable outcomes
- exact setup, run, and verification commands
- repository-wide hazards and cross-cutting completeness checks
- conventions that differ from model defaults
- repository-wide ownership or safety boundaries
- links to deeper task and architecture docs
- pointers to scoped guidance

Move closer to the code:

- language- or package-specific rules
- commands unique to a service
- conventions that apply to one directory or file pattern
- per-directory `AGENTS.md` or `AGENTS.override.md` files
- Cursor path-scoped files under `.cursor/rules/*.mdc`, with explicit globs such as `*.test.ts`

Codex loads global guidance, then project guidance from the root toward the
working directory. Guidance closer to the working directory appears later and
overrides broader rules. Within one directory, Codex loads
`AGENTS.override.md` when present; otherwise it loads `AGENTS.md`. It loads at
most one guidance file per directory. Verify these mechanics against the
current [Codex `AGENTS.md` guide](https://developers.openai.com/codex/guides/agents-md)
when changing hierarchy or filenames.

Avoid generated repository tours and generic rule dumps. Repository-context
evaluations found that irrelevant or excessive guidance can increase cost
without improving completion ([Gloaguen et al.](https://arxiv.org/abs/2602.11988),
[Lulla et al.](https://arxiv.org/abs/2601.20404)).

For a concrete repository guide, inspect [T3 Code's
`AGENTS.md`](https://github.com/pingdotgg/t3code/blob/main/AGENTS.md): reuse its
contract-oriented shape, not its product rules.

Treat roughly 100 lines as a warning threshold, not a target. Relevance and the
common task path decide what stays.

When a repo already uses `AGENTS.md`, use a `CLAUDE.md` symlink or the literal
`@AGENTS.md` import when the harness supports it. Keep one authored source and
verify the import behavior against current [Claude Code memory
guidance](https://code.claude.com/docs/en/memory).
