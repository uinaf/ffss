#!/usr/bin/env bash
# Refresh the vendored @uinaf/design CSS pair from the npm registry.
#
# The control plane is loopback-only and ships as one binary, so the stylesheet
# is embedded rather than loaded from design.uinaf.dev. Pass a version to move
# the pin; with no argument this re-fetches the pinned one. The pin and the
# digests live in internal/serve/design_css_test.go, which this script is the
# only writer of, so moving a version and recording it cannot come apart.
set -euo pipefail

cd "$(dirname "$0")/.."

test_file=internal/serve/design_css_test.go
pinned=$(sed -n 's/^const designCSSVersion = "\(.*\)"$/\1/p' "$test_file")
version="${1:-$pinned}"

if [[ -z "$version" ]]; then
  echo "sync-design-css: no version pinned in $test_file and none given" >&2
  exit 1
fi
if ! command -v gofmt >/dev/null; then
  echo "sync-design-css: gofmt is required to rewrite $test_file" >&2
  exit 1
fi

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

curl -fsSL "https://registry.npmjs.org/@uinaf/design/-/design-${version}.tgz" |
  tar -xzf - -C "$work" package/dist/css/tokens.css package/dist/css/components.css

cp "$work/package/dist/css/tokens.css" internal/serve/static/tokens.css
cp "$work/package/dist/css/components.css" internal/serve/static/components.css

digest() { shasum -a 256 "$1" | cut -d' ' -f1; }
tab=$'\t'

sed -i.bak \
  -e "s|^const designCSSVersion = .*|const designCSSVersion = \"${version}\"|" \
  -e "s|^[[:space:]]*\"static/tokens\.css\":.*|${tab}\"static/tokens.css\": \"$(digest internal/serve/static/tokens.css)\",|" \
  -e "s|^[[:space:]]*\"static/components\.css\":.*|${tab}\"static/components.css\": \"$(digest internal/serve/static/components.css)\",|" \
  "$test_file"
rm -f "$test_file.bak"
gofmt -w "$test_file"

echo "pinned @uinaf/design@${version} in $test_file"
git --no-pager diff --stat -- internal/serve/static "$test_file"
