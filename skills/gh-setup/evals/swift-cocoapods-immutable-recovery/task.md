# Automate a Signed SwiftPM and CocoaPods Release

## Problem/Feature Description

Northwind Mobile publishes `OrbitKit` through both Swift Package Manager and
CocoaPods. Conventional commits on `main` should determine the next version.
The release must deterministically update the checked-in version in
`Package.swift` and `OrbitKit.podspec`, commit those files through GitHub's
App-signed API path, create the tag on that verified commit, push the podspec to
CocoaPods trunk, and publish a metadata-only immutable GitHub Release.

The `release` Environment holds `RELEASE_APP_CLIENT_ID`,
`RELEASE_APP_PRIVATE_KEY`, and `COCOAPODS_TRUNK_TOKEN`. A preflight head check
and Actions concurrency are not an atomic branch lock. Use plugin v1.0.1 only
if a concrete external branch lease blocks every merge and direct push from
before semantic-release starts release analysis through the plugin's API ref
update. Otherwise use a full-SHA-pinned App-signed API integration with the
analyzed SHA as its expected head. The selected writeback may receive only the
existing regular version files and no custom identity. If plugin v1.0.1 is
selected, an `if: always()` step must immediately restore a credential-free
`origin` afterward.

CocoaPods trunk and GitHub Releases are separate immutable boundaries. Provide
a validated exact-tag recovery path for either `tag + pod, no GitHub Release`
or `tag + GitHub Release, no pod`. It must publish only the missing boundary,
reread full parity, and never create another bump or depend on a normal
semantic-release rerun.

## Output Specification

Produce:

- `.github/workflows/ci.yml` with verification, release, and validated recovery
- `.releaserc.json`
- `scripts/prepare-release.sh`
- `scripts/publish-cocoapods.sh`
- `SETUP.md` documenting the Environment credentials and App scope

Use full-SHA pins with exact version comments for high-trust actions.

## Input Files

=============== FILE: Package.swift ===============
// swift-tools-version: 6.0
import PackageDescription

let orbitKitVersion = "1.4.2"

let package = Package(
  name: "OrbitKit",
  products: [.library(name: "OrbitKit", targets: ["OrbitKit"])],
  targets: [.target(name: "OrbitKit"), .testTarget(name: "OrbitKitTests", dependencies: ["OrbitKit"])]
)
=============== END FILE ===============

=============== FILE: OrbitKit.podspec ===============
Pod::Spec.new do |spec|
  spec.name = "OrbitKit"
  spec.version = "1.4.2"
  spec.summary = "Shared mobile primitives"
  spec.source = { :git => "https://github.com/northwind-mobile/orbit-kit.git", :tag => "v#{spec.version}" }
  spec.source_files = "Sources/OrbitKit/**/*.swift"
  spec.ios.deployment_target = "15.0"
end
=============== END FILE ===============
