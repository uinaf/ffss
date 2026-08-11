#!/usr/bin/env bash

set -euo pipefail

if [[ $# -ne 1 || -z $1 ]]; then
  printf 'usage: %s EXPECTED_BINARY\n' "${0##*/}" >&2
  exit 2
fi

expected_binary=$1
resolved_binary=$(command -v slopshipper || true)

if [[ -z $resolved_binary ]]; then
  printf 'slopshipper install check: slopshipper is not on PATH\n' >&2
  exit 1
fi

if [[ $resolved_binary != "$expected_binary" ]]; then
  printf 'slopshipper install check: PATH resolves %s, expected %s\n' \
    "$resolved_binary" "$expected_binary" >&2
  printf 'slopshipper install check: discovered copies:\n' >&2
  type -a slopshipper >&2 || true
  exit 1
fi

printf 'slopshipper install check: PATH resolves %s\n' "$resolved_binary"
