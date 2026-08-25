# Agent-First Documentation

Write internal and operational docs for the agent that retrieves them; humans
get the same precision for free.

Apply the deletion test to every line: would removing it cause a capable agent
to make a material mistake? If not, delete it or move it behind a focused
link. Instructions that restate capable-model defaults spend attention without
changing behavior; delete them.

## Budget Retrieval

Agent-facing guidance spends two resources:

- **Context cost**: always-loaded rules and pointers tax every task.
- **Discovery cost**: material moved out of context must still be found.

Reduce context cost with focused links, but never hide required guidance
behind an unnamed or weakly described target. Spend always-loaded words on
routing that reliably changes what the agent reads next.

## Select Information

Keep facts an agent cannot safely infer: canonical commands, owned paths and
source-of-truth files, architectural and security boundaries, repo-specific
conventions, required prerequisites, known failure recovery, exact
verification gates.

Remove or route elsewhere: generic engineering advice, directory tours file
search already answers, narrative history in current-state docs, repeated
explanations of linked material, exhaustive lists of removed capabilities,
speculative guidance, and introductions or recaps carrying no operational
fact.

The repository is the source of truth. Documentation that repeats an easy
file search, script listing, config value, or `--help` lookup is a cache and
must justify its drift risk. Document the lookup only when it is expensive,
ambiguous, or missing the rationale an agent needs.

For deterministic setup, policy, or workflow shapes, point to the maintained
script, config, test, or reference implementation and name the contract it
demonstrates. Keep rationale and adaptation boundaries in prose; do not make
an agent reconstruct code from prose or copy the implementation into another
doc.

## State Capabilities

Apply the [negative-state rule](../SKILL.md#negative-state-rule). Lead with
what works, where it lives, and how to use it.

```diff
- Redis, RabbitMQ, and Kafka are not provisioned.
+ Jobs use PostgreSQL through `DATABASE_URL`.
```

When an actionable limitation stays, show the supported path:

```md
Webhook delivery is unsupported. Poll `GET /v1/jobs/:id` for status.
```

Do not rehome deleted names under `Exclusions`, `Unavailable`,
`Not supported`, or similar headings.

## Match Structure to Information

- Put the answer or action directly below its heading.
- No bibliography before the task; cite a source beside the claim it
  supports.
- Short sentences; one independent fact per bullet.
- Numbered lists only when order matters.
- Tables for exact mappings and repeated-field comparisons; paragraphs for
  rationale and trade-offs, one idea each.
- Minimal, normative examples; code blocks only for copyable commands, data,
  or exact syntax.
- Preserve literal commands, paths, identifiers, and error text for search.
- One stable term per recurring concept, defined once.

Prefer task-shaped headings (`Start`, `Verify`, `Deploy`, `Recover`) over
headings that mirror the file tree. End each runbook step with an observable
completion condition; "every modified command resolves" beats "docs
understood".

## Design Pointers

A pointer is the short text that decides whether an agent opens another
source. Its wording is behavior, not navigation.

- Front-load the task or domain that triggers the lookup.
- State what the target contains and when it is needed.
- One trigger per distinct branch; collapse synonyms.
- Describe the question answered, not a bare filename.
- Inline a mandatory instruction only when no pointer can make retrieval
  reliable.

## Progressive Disclosure

Keep the root entrypoint a map; link one level down to task- or
domain-specific sources; put specialized rules near the files they govern.
Split a document when readers routinely need only one section; do not
fragment when the same task always needs every fragment. Each link costs a
tool call, so split by retrieval boundary, not size. Use imports, symlinks,
or generated views when multiple harnesses need identical guidance from one
authored source.

## Capture Proven Workflows

When a session's workflow earns reuse (a prompt sequence, an instrument setup,
a refinement loop), distill it from the conversation into a named skill or
runbook: the operator's prompts plus the approach that worked. Capture after
the workflow has proven itself, not on first use; premature formalization
freezes guesswork into guidance.

## AGENTS.md

Use `AGENTS.md` for requirements that apply to agent work in its scope, at
the right layer:

- **Global or owner guidance** routes identities, workspaces, repositories,
  and harness policy; project mechanics stay in their owning repository.
- **Repository guidance** gives a compact working model for cross-cutting
  work: what the system does, who relies on it, what must not regress,
  repository-wide hazards and their safe alternatives, domain terms whose
  everyday meanings would mislead, architecture that changes code placement,
  exact lifecycle commands, and the surfaces a change may need to cover.
- **Scoped guidance** carries package, language, or subsystem rules needed
  only inside that scope.

A repository guide can be more than a table of contents: inline a fact when
it changes many tasks and a missed lookup would cause a material error; route
deep architecture and local-only conventions behind task-shaped pointers.

Translate values into decisions. Pair each abstract preference with an
observable consequence, failure mode, or small example; `protect performance`
becomes useful when it names the regressions to watch and the proof expected.
A bounded matrix (clients, providers, entry points, connection modes,
contracts, reverse states) earns its context cost when omissions across those
dimensions are a recurring defect.

Keep at the root: brief system orientation and non-negotiable outcomes; exact
setup, run, and verification commands; repository-wide hazards and
completeness checks; what local checks cannot prove; conventions that differ
from model defaults; ownership and safety boundaries; where durable
decisions, operational state, and handoffs get written back; links to deeper
docs and scoped guidance.

Move closer to the code: language- or package-specific rules, commands unique
to a service, conventions for one directory or file pattern, per-directory
`AGENTS.md` / `AGENTS.override.md`, and Cursor path-scoped
`.cursor/rules/*.mdc` with explicit globs such as `*.test.ts`.

Codex mechanics: global guidance loads first, then project guidance from the
root toward the working directory; closer files load later and override
broader rules; within one directory Codex loads `AGENTS.override.md` if
present, else `AGENTS.md`, at most one per directory. Verify against the
current [Codex `AGENTS.md` guide](https://developers.openai.com/codex/guides/agents-md)
when changing hierarchy or filenames.

Avoid generated repository tours and generic rule dumps; repository-context
evaluations found irrelevant or excessive guidance can raise cost without
improving completion ([Gloaguen et al.](https://arxiv.org/abs/2602.11988),
[Lulla et al.](https://arxiv.org/abs/2601.20404)). For a concrete shape,
inspect [T3 Code's `AGENTS.md`](https://github.com/pingdotgg/t3code/blob/main/AGENTS.md):
reuse its contract-oriented structure, not its product rules.

Treat roughly 100 lines as a warning threshold, not a target; relevance and
the common task path decide what stays.

When a repo already uses `AGENTS.md`, add a `CLAUDE.md` symlink or the
literal `@AGENTS.md` import when the harness supports it. Keep one authored
source and verify import behavior against current
[Claude Code memory guidance](https://code.claude.com/docs/en/memory).
