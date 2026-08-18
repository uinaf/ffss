#!/usr/bin/env bash
set -euo pipefail

coverage_root=$(mktemp -d "${TMPDIR:-/tmp}/slopmachine-coverage.XXXXXX")
trap 'rm -rf "$coverage_root"' EXIT

check_profile() {
  local label=$1
  local package=$2
  local minimum=$3
  local profile="$coverage_root/$label.out"
  local percent

  go test -count=1 -coverprofile="$profile" "$package"
  percent=$(go tool cover -func="$profile" | awk '/^total:/ { gsub(/%/, "", $3); print $3 }')
  awk -v value="$percent" -v minimum="$minimum" 'BEGIN { exit !(value + 0 >= minimum + 0) }' || {
    echo "coverage: $label is ${percent}%, below ${minimum}%" >&2
    return 1
  }
  echo "coverage: $label ${percent}% (minimum ${minimum}%)"
}

check_profile cli ./cmd/slopmachine 80
check_profile buildinfo ./internal/buildinfo 80
check_profile forge ./internal/forge 80
check_profile machine ./internal/machine 90
check_profile repo ./internal/repo 80
check_profile serve ./internal/serve 80
check_profile status ./internal/status 90
check_profile store ./internal/store 80
check_profile watch ./internal/watch 90

# The CLI integration tests execute a built child process. Instrument that binary
# separately so those end-to-end paths cannot disappear behind parent test coverage.
raw_dir="$coverage_root/cli-integration"
mkdir "$raw_dir"
SLOPMACHINE_COVERAGE_DIR="$raw_dir" GOCOVERDIR="$raw_dir" go test -count=1 ./cmd/slopmachine
integration_percent=$(
  go tool covdata percent -i="$raw_dir" |
    awk '$1 == "github.com/uinaf/ffss/cli/slopmachine/cmd/slopmachine" { gsub(/%/, "", $3); print $3 }'
)
awk -v value="$integration_percent" 'BEGIN { exit !(value + 0 >= 55) }' || {
  echo "coverage: CLI integration is ${integration_percent}%, below 55%" >&2
  exit 1
}
echo "coverage: CLI integration ${integration_percent}% (minimum 55%)"
