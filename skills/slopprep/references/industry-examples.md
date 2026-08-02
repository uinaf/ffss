# Industry Examples

Real-world patterns from teams running agents at scale.

## Sources

- [OpenAI harness engineering](https://openai.com/index/harness-engineering/)
- [OpenAI Symphony](https://openai.com/index/open-source-codex-orchestration-symphony/)
- [OpenAI safety controls](https://openai.com/index/running-codex-safely/)
- [OpenAI coding-eval audit](https://openai.com/index/separating-signal-from-noise-coding-evaluations/)
- [Anthropic long-running agent harness](https://www.anthropic.com/engineering/harness-design-long-running-apps)
- [Anthropic agent evals](https://www.anthropic.com/engineering/demystifying-evals-for-ai-agents)
- [Anthropic managed agents](https://www.anthropic.com/engineering/managed-agents)
- [METR time horizons](https://metr.org/time-horizons/)
- [Stripe Minions](https://stripe.dev/blog/minions-stripes-one-shot-end-to-end-coding-agents)
- [Datadog harness-first agents](https://www.datadoghq.com/blog/ai/harness-first-agents/)
- [Cursor self-driving codebases](https://cursor.com/blog/self-driving-codebases)
- [Uber via Pragmatic Engineer](https://newsletter.pragmaticengineer.com/p/how-uber-uses-ai-for-development)
- [HumanLayer harness engineering](https://www.humanlayer.dev/blog/skill-issue-harness-engineering-for-coding-agents)
- [LangChain agent harness](https://blog.langchain.com/the-anatomy-of-an-agent-harness/)
- [Ramp Inspect](https://engineering.ramp.com/inspect)
- [Background Agents](https://background-agents.com/)
- [Meta REA](https://engineering.fb.com/2026/03/17/developer-tools/ranking-engineer-agent-rea-autonomous-ai-system-accelerating-meta-ads-ranking-innovation/)

## Contents

- [OpenAI — Codex Frontend](#openai--codex-frontend)
- [Anthropic — Evaluator Pattern](#anthropic--evaluator-pattern)
- [Agent Evals — Outcomes and Reliability](#agent-evals--outcomes-and-reliability)
- [Durable Orchestration and Safety](#durable-orchestration-and-safety)
- [Stripe — Minions](#stripe--minions)
- [Datadog — Observability-Driven Verification](#datadog--observability-driven-verification)
- [Cursor — Self-Driving Codebases](#cursor--self-driving-codebases)
- [Convergent Architecture](#convergent-architecture)

## OpenAI — Codex Frontend

3 engineers, ~1,500 PRs, ~1M LOC, 3.5 PRs/engineer/day. Zero lines of manually-written code over 5 months.

**Infrastructure**:
- Per-worktree bootable app — each change gets its own running instance
- CDP wired into agent runtime — DOM snapshots, screenshots, navigation
- Ephemeral observability per worktree — logs, metrics, traces torn down after task
- Custom linters with error messages that inject remediation into agent context

**AGENTS.md**: ~100 lines, table of contents pointing to `docs/`. "We tried the big AGENTS.md. It failed."

**Architecture**: rigid layers mechanically enforced (Types → Config → Repo → Service → Runtime → UI). "Enforce boundaries centrally, allow autonomy locally."

**Review**: agent-to-agent (Ralph Wiggum Loop). Humans may review but aren't required.

**Slop management**: "golden principles" + background Codex tasks scan for deviations, open refactoring PRs. Technical debt as high-interest loan — pay down continuously.

Source: [OpenAI harness engineering](https://openai.com/index/harness-engineering/)

## Anthropic — Evaluator Pattern

GAN-inspired three-agent pattern: Planner → Generator → Evaluator.

**How the evaluator works**:
1. Boots the app
2. Navigates key flows with Playwright MCP
3. Takes screenshots
4. Grades against predefined criteria (design quality, originality, craft, functionality)
5. Returns pass/fail with evidence and specific feedback

**Sprint contracts**: before each sprint, generator and evaluator negotiate testable "done" criteria.

**Key findings**:
- LLMs are terrible self-evaluators — confidently praise mediocre work
- Out of the box, Claude identifies issues then talks itself into approving
- Multiple tuning rounds needed: read evaluator logs → find judgment divergences → update QA prompt
- Opus 4.6 allowed removing sprint construct entirely (model handles coherence natively)

**Communication**: agents coordinate via files, not message passing.

**Cost**: solo $9/20min → full 3-agent setup $200/6hr. Simplified (Opus 4.6): $125/4hr.

Source: [Anthropic long-running agent harness](https://www.anthropic.com/engineering/harness-design-long-running-apps)

## Agent Evals — Outcomes and Reliability

Anthropic distinguishes the agent harness from the evaluation harness and
grades final environment state separately from the transcript. Multiple trials
are required because one agent-task pairing is stochastic; `pass@1` and
`pass^k` answer different reliability questions. Start with representative
manual checks and real failures, then inspect trajectories to distinguish agent
failure from broken tasks or graders.

OpenAI's 2026 SWE-Bench audits found that implementation-specific tests,
underspecified prompts, low coverage, and contaminated tasks can make a
benchmark confidently wrong. Readiness evals should accept equivalent solutions,
ship a known-valid reference path, and audit suspicious failures and saturation.

METR's task-completion horizons report both 50%- and 80%-reliable durations.
This makes task length and reliability explicit instead of treating an
occasional multi-hour success as dependable autonomy.

Sources:

- [Anthropic agent evals](https://www.anthropic.com/engineering/demystifying-evals-for-ai-agents)
- [OpenAI coding-eval audit](https://openai.com/index/separating-signal-from-noise-coding-evaluations/)
- [METR time horizons](https://metr.org/time-horizons/)

## Durable Orchestration and Safety

OpenAI's Symphony treats the issue tracker as a control plane: tasks receive
isolated workspaces, explicit attempt state, bounded turns and retries, stall
detection, reconciliation, and provider result state. A successful process
exit does not by itself mean the task is complete.

Anthropic separates durable session logs, replaceable harness processes, and
sandbox execution. A crashed harness can resume from recorded events instead of
requiring a human to nurse a specific container.

OpenAI's production Codex controls combine sandboxing, network policy, scoped
approval, rules, and agent-native telemetry. Autonomy is productive freedom
inside infrastructure-enforced boundaries, not broad credentials plus prompt
instructions.

Sources:

- [OpenAI Symphony](https://openai.com/index/open-source-codex-orchestration-symphony/)
- [Anthropic managed agents](https://www.anthropic.com/engineering/managed-agents)
- [OpenAI safety controls](https://openai.com/index/running-codex-safely/)

## Stripe — Minions

1,300+ PRs/week. All human-reviewed, zero human-written code. Manages >$1 trillion annual payment volume.

**Devboxes**: cloud machines pre-loaded with codebase, 10-second spin-up via warm pool. QA-isolated. Built for humans years before agents — agents just slotted in.

**Blueprints**: hybrid orchestration mixing deterministic nodes (lint, push, PR template) with agentic nodes (implement, fix CI). "Putting LLMs into contained boxes compounds into reliability."

**Scoped rules**: global rules "very judiciously." Almost all rules scoped to subdirectories/file patterns, auto-attached as agent navigates. Same rules for Minions, Cursor, Claude Code.

**Feedback**: pre-push lint < 5 seconds (background daemon precomputes), selective CI from 3M+ tests with autofixes, max 2 CI rounds.

**Toolshed**: centralized MCP server with ~500 tools. Agents get curated subsets, not kitchen sink.

**Key insight**: "Investments in human developer productivity over time have returned to pay dividends in the world of agents."

Source: [Stripe Minions](https://stripe.dev/blog/minions-stripes-one-shot-end-to-end-coding-agents)

## Datadog — Observability-Driven Verification

The most rigorous verification approach. Infrastructure-first: invest in automated checks, not code review.

**Verification pyramid**:

| Layer | Tool | Time |
|-------|------|------|
| Symbolic | TLA+ specs | 2 min read |
| Primary | Deterministic Simulation Testing (DST) | ~5 seconds |
| Exhaustive | Model checking (Stateright) | 30-60 seconds |
| Bounded | Bounded verification (Kani) | ~60 seconds |
| Empirical | Telemetry + benchmarks | seconds-minutes |

**DST as workhorse**: each run ~5 seconds, exercises production code through randomized scenarios. Target: 500 seeds per component, 10 million across all components. Caught bugs impossible to find in code review.

**Contracts before code**: core invariants defined upfront. Agent is NOT allowed to invent system meaning. "Semi-formal methods" — specs explicit enough to be checked, cheap enough to run continuously.

**Performance as hill-climbing**: with correctness locked in, agent proposes optimization → full DST → if tests pass, measure throughput → keep or revert.

**Human role**: "define the system idea and invariants, review and strengthen the DST harness, set measurable targets, and approve architectural changes. Everything else was the agent running against the harness."

**Reviews become bloom filters** — a fast gate, not the source of correctness.

Source: [Datadog harness-first agents](https://www.datadoghq.com/blog/ai/harness-first-agents/)

## Cursor — Self-Driving Codebases

Built a web browser with ~1,000 commits/hour across 10M tool calls. Almost zero human intervention.

**Architecture evolution**: single agent → shared state (locking hell) → rigid planner/executor (bottleneck) → final: root planner + recursive subplanners + isolated workers with handoffs.

**Key findings**:
- 100% commit correctness caused serialization — accepting some error rate was key
- No cross-talk between workers — convergence through ownership chain
- Duplicate work cheaper than coordination overhead
- Scratchpads should be rewritten, not appended to
- Disk I/O was the bottleneck at scale, not CPU/RAM

Source: [Cursor self-driving codebases](https://cursor.com/blog/self-driving-codebases)

## Uber — Minion + Internal AI Stack

92% of devs use agents monthly. 65-72% of code AI-generated. AI costs up 6x since 2024.

**Minion** (background agent platform):
- Monorepos pre-checked-out and ready. Internal infra access via MCP + AIFX CLI
- Optimized defaults (compiling done, tools installed, helpful AGENTS.md)
- Prompt improvement — analyzes and rewrites prompts for higher success rate
- Web, Slack, GitHub PR, and code review interfaces
- 70% of workloads are toil (migrations, upgrades) — higher accuracy = virtuous cycle

**uReview** (AI code review):
- Multiple specialized bots per PR. Comments graded, low-confidence filtered, merged, categorized
- Quality > quantity — worst thing is noisy low-quality comments
- Devs rate usefulness → feedback loop. Bot comments trending down (focus on quality)

**Code Inbox** (smart PR routing):
- Routes based on ownership, compliance, history, timezone, calendar availability
- Risk profiles highlight high-impact changes for extra scrutiny
- SLOs for response times, auto-reassign, escalation

**Key finding**: Claude Code usage nearly doubled in 3 months (32% → 63%), while IDE tools plateaued. Engineers naturally gravitate to multi-agent workflows. Sharing wins between engineers is the most effective adoption tactic — top-down mandates had limited impact.

Source: [Uber via Pragmatic Engineer](https://newsletter.pragmaticengineer.com/p/how-uber-uses-ai-for-development)

## Convergent Architecture

The stable convergence is not one multi-agent topology. Planner, evaluator,
context-reset, and worker structures change as models and tasks improve. The
durable properties are repository legibility, isolated execution, deterministic
control-plane transitions, scoped authority, outcome-based feedback, bounded
recovery, and a path that turns recurring failures into stronger infrastructure.
