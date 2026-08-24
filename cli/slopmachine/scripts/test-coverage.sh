#!/usr/bin/env bash
set -euo pipefail

coverage_root=$(mktemp -d "${TMPDIR:-/tmp}/slopmachine-coverage.XXXXXX")
trap 'rm -rf "$coverage_root"' EXIT

check_profile() {
  local label=$1
  local package=$2
  local minimum=$3
  local import_path
  local percent

  import_path=$(go list -f '{{.ImportPath}}' "$package")
  percent=$(
    awk -v package="$import_path" '
      NR == 1 { next }
      {
        statements[$1] = $2
        if ($3 > 0) { covered[$1] = 1 }
      }
      END {
        for (location in statements) {
          file = location
          sub(/:[^:]+$/, "", file)
          directory = file
          sub("/[^/]+$", "", directory)
          if (directory != package) { continue }
          total += statements[location]
          if (covered[location]) { hit += statements[location] }
        }
        if (total == 0) { exit 2 }
        printf "%.1f\n", hit * 100 / total
      }
    ' "$coverage_profile"
  )
  awk -v value="$percent" -v minimum="$minimum" 'BEGIN { exit !(value + 0 >= minimum + 0) }' || {
    echo "coverage: $label is ${percent}%, below ${minimum}%" >&2
    return 1
  }
  echo "coverage: $label ${percent}% (minimum ${minimum}%)"
}

coverage_profile="$coverage_root/all.out"
raw_dir="$coverage_root/cli-integration"
mkdir "$raw_dir"

# One instrumented pass supplies the ordinary suite, race proof, package
# coverage, and child-process CLI coverage. SLOPMACHINE_COVERAGE_DIR reaches
# only the child binary; the parent test process writes the aggregate profile.
SLOPMACHINE_COVERAGE_DIR="$raw_dir" \
  go test -count=1 -race -covermode=atomic \
    -coverprofile="$coverage_profile" -p 4 -parallel 4 ./...

check_profile cli ./cmd/slopmachine 80
check_profile buildinfo ./internal/buildinfo 80
check_profile forge ./internal/forge 80
check_profile machine ./internal/machine 90
check_profile repo ./internal/repo 80
check_profile serve ./internal/serve 80
check_profile status ./internal/status 90
check_profile store ./internal/store 80
check_profile watch ./internal/watch 90

integration_percent=$(
  go tool covdata percent -i="$raw_dir" |
    awk '$1 == "github.com/uinaf/ffss/cli/slopmachine/cmd/slopmachine" { gsub(/%/, "", $3); print $3 }'
)
awk -v value="$integration_percent" 'BEGIN { exit !(value + 0 >= 55) }' || {
  echo "coverage: CLI integration is ${integration_percent}%, below 55%" >&2
  exit 1
}
echo "coverage: CLI integration ${integration_percent}% (minimum 55%)"
