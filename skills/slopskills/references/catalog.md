# Recommended Skills

Select by repository evidence and task. Sources are installation coordinates;
check their current skill listing before installing. These are recommendations,
not a blanket bundle.

## React and Web UI

| Skill | Install source | Evidence and use |
| --- | --- | --- |
| `react-ban-use-effect` | [uinaf/agent-skills](https://github.com/uinaf/agent-skills) | React code; effect implementation, refactoring, or effect policy. |
| `react-doctor` | [millionco/react-doctor](https://github.com/millionco/react-doctor) | React code; feature or bug verification and React diagnostics. |
| `shadcn` | [shadcn/ui](https://github.com/shadcn-ui/ui) | A `components.json` configuration, existing shadcn components, or an explicit shadcn setup task; component composition, registries, presets, or styling. React alone does not qualify. |
| `tanstack-query` | [tanstack-skills/tanstack-skills](https://github.com/tanstack-skills/tanstack-skills) | A TanStack Query dependency or imports; fetching, query keys, mutations, caching, or invalidation. |
| `tanstack-form` | [tanstack-skills/tanstack-skills](https://github.com/tanstack-skills/tanstack-skills) | A TanStack Form dependency or imports; form state, field validation, or submission. |
| `tanstack-start` | [tanstack-skills/tanstack-skills](https://github.com/tanstack-skills/tanstack-skills) | TanStack Start dependency/configuration; routes, server functions, SSR, or deployment. Router alone does not imply Start. |

React alone does not select all TanStack skills. Match the actual TanStack
package, including non-React adapters, and check guidance against the installed
major version before using its APIs.

## Swift

| Skill | Install source | Evidence and use |
| --- | --- | --- |
| `swiftui-expert-skill` | [AvdLee/SwiftUI-Agent-Skill](https://github.com/AvdLee/SwiftUI-Agent-Skill) | SwiftUI imports or an explicit SwiftUI UI task; skip headless Swift and UIKit-only targets. |
| `swift-testing-expert` | [AvdLee/Swift-Testing-Agent-Skill](https://github.com/AvdLee/Swift-Testing-Agent-Skill) | Swift test targets or planned tests; Swift Testing, test architecture, or requested XCTest migration. Installation does not authorize migration. |
| `swift-concurrency` | [AvdLee/Swift-Concurrency-Agent-Skill](https://github.com/AvdLee/Swift-Concurrency-Agent-Skill) | Swift code; async/await, actors, isolation, Sendable, or concurrency migration. |
| Xcode build skills below | [AvdLee/Xcode-Build-Optimization-Agent-Skill](https://github.com/AvdLee/Xcode-Build-Optimization-Agent-Skill) | Xcode projects/build workflows; build timing, diagnostics, or optimization. Use the SPM specialist alone for a matching SwiftPM task. |

The Xcode source's orchestrator requires all six skills:

- `xcode-build-orchestrator`
- `xcode-build-benchmark`
- `xcode-compilation-analyzer`
- `xcode-project-analyzer`
- `spm-build-analysis`
- `xcode-build-fixer`

Select these names explicitly for the full optimization workflow. Individual
specialists can be installed for standalone tasks; do not install the
orchestrator alone. Installing skills does not authorize benchmarks or build
setting changes outside the original task.

## Effect

| Skill | Install source | Evidence and use |
| --- | --- | --- |
| `effect-ts` | [Effect-TS/skills](https://github.com/Effect-TS/skills) | The TypeScript `effect` dependency or imports; repository setup and routing to Effect's bundled guidance. React effects and Swift side effects do not qualify. |

The Effect setup skill includes a dependency installation step. In an existing
repository, preserve its version and use the installed package's guidance;
equipping skills does not authorize an upgrade to a release candidate.
