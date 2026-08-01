# Agent-First Documentation

Optimize internal and operational docs for agent retrieval. Human readers benefit from the same precision and scanability.

## Sources

- OpenAI Codex `AGENTS.md` guide: https://developers.openai.com/codex/guides/agents-md
- OpenAI harness engineering: https://openai.com/index/harness-engineering/
- Claude Code memory guidance: https://code.claude.com/docs/en/memory
- Anthropic context engineering: https://www.anthropic.com/engineering/effective-context-engineering-for-ai-agents
- Anthropic Agent Skills architecture: https://www.anthropic.com/engineering/equipping-agents-for-the-real-world-with-agent-skills
- Anthropic Agent Skills best practices: https://platform.claude.com/docs/en/agents-and-tools/agent-skills/best-practices
- Anthropic prompting guidance: https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/prompting-claude-opus-5
- GitHub instruction-file guidance: https://github.blog/ai-and-ml/github-copilot/unlocking-the-full-power-of-copilot-code-review-master-your-instructions-files/
- Stripe scoped-rule practice: https://stripe.dev/blog/minions-stripes-one-shot-end-to-end-coding-agents
- Gloaguen et al., repository context evaluation: https://arxiv.org/abs/2602.11988
- Lulla et al., `AGENTS.md` efficiency study: https://arxiv.org/abs/2601.20404

The empirical results are mixed. Context files can help or hurt depending on their relevance and delivery. The stable recommendation is minimal, specific requirements plus just-in-time retrieval.

Use this deletion test: would removing the line cause a capable agent to make a material mistake? If not, remove it or move it behind a focused link.

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
- Use short sentences.
- Put one independent fact in each bullet.
- Use numbered lists only when order matters.
- Use tables for exact mappings and repeated-field comparisons.
- Use paragraphs for rationale, trade-offs, and causal explanation.
- Keep paragraphs to one idea; two or three sentences usually suffice.
- Keep examples minimal and normative.
- Use code blocks only for copyable commands, data, or exact syntax.
- Preserve literal commands, paths, identifiers, and error text for search.

Prefer task-shaped headings such as `Start`, `Verify`, `Deploy`, and `Recover`. Avoid headings that merely mirror the file tree.

## Progressive Disclosure

- Keep the root entrypoint a map.
- Link one level down to task- or domain-specific sources.
- Describe what each link answers.
- Put specialized rules near the files they govern.
- Split a document when readers routinely need only one section.
- Avoid fragmentation when the same task always requires every fragment.
- Use imports, symlinks, or generated views when multiple harnesses need identical guidance from one authored source.

Each link costs a tool call. Split by retrieval boundary, not by arbitrary size.

## AGENTS.md

Use `AGENTS.md` for requirements that apply to agent work in its scope.

Keep at the root:

- exact setup, run, and verification commands
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

Codex loads global guidance, then project guidance from the root toward the working directory. Guidance closer to the working directory appears later and overrides broader rules. Within one directory, Codex loads `AGENTS.override.md` when present; otherwise it loads `AGENTS.md`. It loads at most one guidance file per directory.

Avoid generated repository tours and generic rule dumps. Gloaguen et al. found no significant completion gain from context files overall, higher inference cost, and recommended minimal, specific requirements.

OpenAI reports success with a root file of roughly 100 lines; Claude recommends keeping `CLAUDE.md` under 200 lines. Treat these as warning thresholds, not content targets. Relevance decides what stays.

When a repo already uses `AGENTS.md`, use a `CLAUDE.md` symlink or the literal `@AGENTS.md` import when the harness supports it. Keep one authored source.
