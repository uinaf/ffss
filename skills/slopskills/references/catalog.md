# Recommended skills and lint plugins

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

Match the actual TanStack package, not React. A TanStack package that ships
no skill is a gap; report it.

## Swift

| Skill | Install source | Evidence and use |
| --- | --- | --- |
| `swiftui-expert-skill` | [AvdLee/SwiftUI-Agent-Skill](https://github.com/AvdLee/SwiftUI-Agent-Skill) | SwiftUI imports or an explicit SwiftUI UI task; skip headless Swift and UIKit-only targets. |
| `swift-testing-expert` | [AvdLee/Swift-Testing-Agent-Skill](https://github.com/AvdLee/Swift-Testing-Agent-Skill) | Swift test targets or planned tests; Swift Testing, test architecture, or requested XCTest migration. Installation does not authorize migration. |
| `swift-concurrency` | [AvdLee/Swift-Concurrency-Agent-Skill](https://github.com/AvdLee/Swift-Concurrency-Agent-Skill) | Swift code; async/await, actors, isolation, Sendable, or concurrency migration. |
| Xcode build skills | [AvdLee/Xcode-Build-Optimization-Agent-Skill](https://github.com/AvdLee/Xcode-Build-Optimization-Agent-Skill) | Xcode projects/build workflows; build timing, diagnostics, or optimization. Use the SPM specialist alone for a matching SwiftPM task. |

`xcode-build-orchestrator` needs every specialist the source lists;
specialists can be installed alone.

Benchmarks and build-setting changes stay within the original task.

## Android

| Skill | Install source | Evidence and use |
| --- | --- | --- |
| Skills picked by name | [android/skills](https://github.com/android/skills) | Android, Kotlin, or Jetpack Compose apps. Pick each skill from stack evidence: `r8-analyzer` when minify is on, `media3-cast-integration` for Cast, `leanback-to-compose-tv-migration` only for Leanback apps. |

Install each skill by name at project scope, never `--all`:

```sh
npx skills add android/skills --skill <name> --agent claude-code --agent codex --yes
```

Run one-off audits such as `play-policy-insights`, `android-intent-security`,
and `android-permissions-security`; don't vendor them.

## Effect

Effect ships its agent guide in the package; install no repo skill. Evidence:
the TypeScript `effect` dependency or imports. React effects and Swift side
effects do not qualify.

When `node_modules/effect/AGENTS.md` exists, step 5 adds this pointer:

```md
Before writing Effect code, read `node_modules/effect/AGENTS.md` completely and
follow required links. Look up uncovered APIs in `node_modules/effect/src`.
```

Keep the installed `effect` version; no upgrade to a release candidate. Once
the pointer is in place, remove a repo-local `effect-ts` skill.

## Lint plugins

Install these as dev dependencies and register them in the repo's Oxlint or
Vite+ lint config; each README has the setup.

| Plugin | Install source | Evidence and use |
| --- | --- | --- |
| `@uinaf/oxlint-plugin-no-comments` | [uinaf/oxlint-plugin-no-comments](https://github.com/uinaf/oxlint-plugin-no-comments#install) | JS or TS linted by Oxlint or Vite+; enforces the comment rule. With anti-slop vendored, set `safetyMarkers` to its `markers`. |

Run the rule once before enabling it. Existing comments fail the gate: clean
them with slopclean in the same change, or report the count when it is too
large for one change.
