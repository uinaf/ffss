#!/usr/bin/env bash

set -euo pipefail

repository_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
check_script=$repository_root/scripts/check-install-path.sh
scratch=$(mktemp -d "${TMPDIR:-/tmp}/slopmachine-path-test.XXXXXX")
trap 'rm -rf "$scratch"' EXIT

go_bin=$scratch/home/.local/share/mise/installs/go/1.26.2/bin
homebrew_bin=$scratch/opt/homebrew/bin
mkdir -p "$go_bin" "$homebrew_bin"

for binary in "$go_bin/slopmachine" "$homebrew_bin/slopmachine"; do
  printf '#!/bin/sh\nexit 0\n' > "$binary"
  chmod 755 "$binary"
done

expected_binary=$homebrew_bin/slopmachine

PATH="$homebrew_bin:$go_bin:/usr/bin:/bin" \
  "$check_script" "$expected_binary" >/dev/null

if output=$(PATH="$go_bin:$homebrew_bin:/usr/bin:/bin" \
    "$check_script" "$expected_binary" 2>&1); then
  printf 'expected a Go-installed binary ahead of Homebrew to fail\n' >&2
  exit 1
fi

case "$output" in
  *"PATH resolves $go_bin/slopmachine, expected $expected_binary"*) ;;
  *)
    printf 'unexpected shadowing diagnostic:\n%s\n' "$output" >&2
    exit 1
    ;;
esac

case "$output" in
  *"$go_bin/slopmachine"*"$homebrew_bin/slopmachine"*) ;;
  *)
    printf 'diagnostic did not list both discovered copies:\n%s\n' "$output" >&2
    exit 1
    ;;
esac

if output=$(PATH="/usr/bin:/bin" "$check_script" "$expected_binary" 2>&1); then
  printf 'expected a missing binary to fail\n' >&2
  exit 1
fi
case "$output" in
  *'slopmachine is not on PATH'*) ;;
  *)
    printf 'unexpected missing-binary diagnostic:\n%s\n' "$output" >&2
    exit 1
    ;;
esac

printf 'install PATH tests passed\n'
