# Recommended skills

Match each row against repository evidence and the task. Sources are install
coordinates.

## React and Web UI

| Skill | Install source | Evidence and use |
| --- | --- | --- |
| `react-ban-use-effect` | [uinaf/agent-skills](https://github.com/uinaf/agent-skills) | React code; effect implementation, refactoring, or effect policy. |
| `react-doctor` | [millionco/react-doctor](https://github.com/millionco/react-doctor) | React code; feature or bug verification and React diagnostics. |
| `shadcn` | [shadcn/ui](https://github.com/shadcn-ui/ui) | A `components.json` configuration, existing shadcn components, or an explicit shadcn setup task; component composition, registries, presets, or styling. React alone does not qualify. |
| `react-router` | `@tanstack/react-router` via [TanStack Intent](https://tanstack.com/intent) | A TanStack Router dependency; routes, params, search params, loaders, or navigation. |
| `react-start` | `@tanstack/react-start` via [TanStack Intent](https://tanstack.com/intent) | A TanStack Start dependency or configuration; server functions, server routes, middleware, SSR, or deployment. Router alone does not imply Start. |

Match the actual TanStack package, not React. Query and Form ship no skills
yet; report them as gaps.

## Swift

| Skill | Install source | Evidence and use |
| --- | --- | --- |
| `swiftui-expert-skill` | [AvdLee/SwiftUI-Agent-Skill](https://github.com/AvdLee/SwiftUI-Agent-Skill) | SwiftUI imports or an explicit SwiftUI UI task; skip headless Swift and UIKit-only targets. |
| `swift-testing-expert` | [AvdLee/Swift-Testing-Agent-Skill](https://github.com/AvdLee/Swift-Testing-Agent-Skill) | Swift test targets or planned tests; Swift Testing, test architecture, or requested XCTest migration. Installation does not authorize migration. |
| `swift-concurrency` | [AvdLee/Swift-Concurrency-Agent-Skill](https://github.com/AvdLee/Swift-Concurrency-Agent-Skill) | Swift code; async/await, actors, isolation, Sendable, or concurrency migration. |
| Xcode build skills below | [AvdLee/Xcode-Build-Optimization-Agent-Skill](https://github.com/AvdLee/Xcode-Build-Optimization-Agent-Skill) | Xcode projects/build workflows; build timing, diagnostics, or optimization. Use the SPM specialist alone for a matching SwiftPM task. |

The Xcode orchestrator needs all six; specialists can be installed alone:

- `xcode-build-orchestrator`
- `xcode-build-benchmark`
- `xcode-compilation-analyzer`
- `xcode-project-analyzer`
- `spm-build-analysis`
- `xcode-build-fixer`

Benchmarks and build-setting changes stay within the original task.

## Effect

| Skill | Install source | Evidence and use |
| --- | --- | --- |
| `effect-ts` | [Effect-TS/skills](https://github.com/Effect-TS/skills) | The TypeScript `effect` dependency or imports; repository setup and routing to Effect's bundled guidance. React effects and Swift side effects do not qualify. |

The Effect setup skill installs a dependency. In an existing repo, keep the
installed `effect` version; no upgrade to a release candidate.
