# Add a Signed Homebrew Fallback for a Non-Go CLI

## Problem/Feature Description

Acme Tools publishes `envctl`, a non-Go CLI, and needs its release workflow to
update `acme-tools/homebrew-tap`. Its existing formula generator ends with an
ordinary local commit and push. The tap now requires verified commits, so a bot
name and noreply email are insufficient.

The generator has no native GitHub App-signed commit mode. Preserve its useful
formula-generation behavior, but prevent it from committing. After a release is
published, a dependent Linux job reads source state with the source
repository's read-only workflow token, then mints a separate short-lived App
token scoped only to the tap. It deterministically prepares only
`Formula/envctl.rb` and commits that path with the full-SHA-pinned
`pgaskin/push-signed-commits` action. Stage no other path, use the observed tap
checkout parent as the atomic expected head, and fail rather than overwrite if
the tap advances. Read back the resulting tap commit and fail if GitHub does
not report `verification.verified: true`.

The handoff must use durable state. It should run whenever the exact trusted
release tag is published and immutable but tap parity is missing, including a
later recovery run. Do not gate repair solely on semantic-release's
`new_release_published` output.

## Output Specification

Update `.github/workflows/release.yml` and write a short `SETUP.md`. Document
the release Environment's `RELEASE_APP_CLIENT_ID` variable and
`RELEASE_APP_PRIVATE_KEY` secret, the tap-only token scope, and
`contents: write`. Keep the source workflow token at `contents: read`. Do not
add a PAT, custom bot identity, ordinary `git push`, or a manual tap PR.

## Input Files

The current workflow below publishes through semantic-release and exposes
`new_release_published` and `new_release_version` as job outputs. It currently
runs a formula generator directly after semantic-release using the default
repository token. Replace only that Homebrew handoff; preserve the existing
release system, but replace the transient output gate with exact release-state
discovery and an idempotent parity check.

=============== FILE: .github/workflows/release.yml ===============
name: release

on:
  push:
    branches: [main]

permissions:
  contents: write
  issues: write
  pull-requests: write
  id-token: write

jobs:
  release:
    runs-on: ubuntu-latest
    outputs:
      new_release_published: ${{ steps.semrel.outputs.new_release_published }}
      new_release_version: ${{ steps.semrel.outputs.new_release_version }}
    steps:
      - uses: actions/checkout@08eba0b27e820071cde6df949e0beb9ba4906955 # v4.3.0
        with:
          fetch-depth: 0
      - uses: actions/setup-node@49933ea5288caeca8642d1e84afbd3f7d6820020 # v4.4.0
        with:
          node-version: 24
      - run: npm ci
      - id: semrel
        uses: cycjimmy/semantic-release-action@b12c8f6015dc215fe37bc154d4ad456dd3833c90 # v6.0.0
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}

      - name: Update Homebrew formula
        if: steps.semrel.outputs.new_release_published == 'true'
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          VERSION: ${{ steps.semrel.outputs.new_release_version }}
        run: |
          git clone https://x-access-token:${GITHUB_TOKEN}@github.com/acme-tools/homebrew-tap.git tap
          node scripts/gen-formula.mjs --version "$VERSION" --out tap/Formula/envctl.rb
          cd tap
          git config user.name "acme-release-bot"
          git config user.email "acme-release-bot@users.noreply.github.com"
          git add -A
          git commit -m "envctl $VERSION"
          git push origin main
=============== END FILE ===============

=============== FILE: scripts/gen-formula.mjs ===============
// Renders Formula/envctl.rb from the published release's asset URLs and
// SHA-256 digests. Writes the file only; committing is the caller's job.
import { writeFileSync } from "node:fs";
import { parseArgs } from "node:util";

const { values } = parseArgs({ options: { version: { type: "string" }, out: { type: "string" } } });
const base = `https://github.com/acme-tools/envctl/releases/download/v${values.version}`;
const res = await fetch(`${base}/checksums.txt`);
if (!res.ok) throw new Error(`checksums.txt: ${res.status}`);
const sums = Object.fromEntries(
  (await res.text()).trim().split("\n").map((l) => l.split(/\s+/).reverse()),
);
const asset = (name) => `    url "${base}/${name}"\n    sha256 "${sums[name]}"`;
writeFileSync(
  values.out,
  `class Envctl < Formula
  desc "Manage environment configuration"
  homepage "https://github.com/acme-tools/envctl"
  version "${values.version}"
  on_macos do
${asset("envctl-darwin-arm64.tar.gz")}
  end
  on_linux do
${asset("envctl-linux-amd64.tar.gz")}
  end
  def install
    bin.install "envctl"
  end
end
`,
);
=============== END FILE ===============
