#!/bin/sh

set -eu

# slopmachine release assets ship from the ffsstack monorepo starting with
# the first slopmachine release (uinaf/slopshipper#50).
repository_url=${SLOPMACHINE_INSTALL_REPOSITORY_URL:-https://github.com/uinaf/ffsstack}
while [ "${repository_url%/}" != "$repository_url" ]; do
  repository_url=${repository_url%/}
done
requested_version=latest
destination=
temporary_directory=
destination_temporary=

usage() {
  cat <<'EOF'
Install slopmachine on Linux.

Usage: install.sh [--version VERSION] [--dest DIRECTORY]

Options:
  --version VERSION  Install VERSION (for example v0.4.0); defaults to latest.
  --dest DIRECTORY   Install into DIRECTORY; defaults to $HOME/.local/bin.
  -h, --help         Show this help.
EOF
}

fail() {
  printf 'slopmachine installer: %s\n' "$*" >&2
  exit 1
}

cleanup() {
  if [ -n "$destination_temporary" ]; then
    rm -f -- "$destination_temporary"
  fi
  if [ -n "$temporary_directory" ]; then
    rm -rf -- "$temporary_directory"
  fi
}

trap cleanup 0
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

require_value() {
  [ "$#" -ge 2 ] || fail "$1 requires a value"
  [ -n "$2" ] || fail "$1 requires a non-empty value"
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --version)
      require_value "$@"
      requested_version=$2
      shift 2
      ;;
    --dest)
      require_value "$@"
      destination=$2
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      fail "unknown argument: $1"
      ;;
  esac
done

[ -n "$destination" ] || {
  [ -n "${HOME:-}" ] || fail "HOME is required when --dest is not provided"
  destination=$HOME/.local/bin
}
[ -n "$destination" ] || fail "destination must not be empty"
case "$repository_url" in
  https://*) ;;
  *) fail "release repository URL must use HTTPS" ;;
esac

command -v uname >/dev/null 2>&1 || fail "required command not found: uname"
operating_system=$(uname -s)
[ "$operating_system" = Linux ] ||
  fail "unsupported operating system: $operating_system"
machine_architecture=$(uname -m)
case "$machine_architecture" in
  x86_64|amd64) architecture=amd64 ;;
  aarch64|arm64) architecture=arm64 ;;
  *) fail "unsupported architecture: $machine_architecture" ;;
esac

for required_command in curl tar mktemp sha256sum mkdir cp chmod mv rm; do
  command -v "$required_command" >/dev/null 2>&1 ||
    fail "required command not found: $required_command"
done

valid_release_tag() {
  case "$1" in
    v*) ;;
    *) return 1 ;;
  esac

  major=${1#v}
  case "$major" in
    *.*.*) ;;
    *) return 1 ;;
  esac
  remainder=${major#*.}
  major=${major%%.*}
  minor=${remainder%%.*}
  patch=${remainder#*.}
  case "$patch" in
    *.*) return 1 ;;
  esac
  for number in "$major" "$minor" "$patch"; do
    case "$number" in
      ''|*[!0-9]*) return 1 ;;
    esac
  done
}

# Releases live in the shared ffsstack repository under member-prefixed
# tags (slopmachine/vX.Y.Z); "latest" means this member's newest release.
api_url=${SLOPMACHINE_INSTALL_API_URL:-https://api.github.com/repos/uinaf/ffsstack/releases}
if [ "$requested_version" = latest ]; then
  release_tag=$(curl --proto '=https' --tlsv1.2 -fsSL \
    -H "Accept: application/vnd.github+json" "$api_url?per_page=100" |
    grep -o '"tag_name": *"slopmachine/v[0-9.]*"' | head -1 | sed 's/.*"slopmachine\///; s/"$//') ||
    fail "failed to resolve the latest release"
  [ -n "$release_tag" ] || fail "no slopmachine release found"
else
  case "$requested_version" in
    v*) release_tag=$requested_version ;;
    *) release_tag=v$requested_version ;;
  esac
fi

valid_release_tag "$release_tag" || fail "invalid release version: $release_tag"

temporary_directory=$(mktemp -d "${TMPDIR:-/tmp}/slopmachine-install.XXXXXX") ||
  fail "could not create a temporary directory"
archive=slopmachine_${release_tag}_linux_${architecture}.tar.gz
archive_path=$temporary_directory/$archive
checksums_path=$temporary_directory/checksums.txt
release_url=$repository_url/releases/download/slopmachine%2F$release_tag

curl --proto '=https' --tlsv1.2 -fsSL -o "$archive_path" \
  "$release_url/$archive" || fail "failed to download $archive"
curl --proto '=https' --tlsv1.2 -fsSL -o "$checksums_path" \
  "$release_url/checksums.txt" || fail "failed to download checksums.txt"

checksum_matches=0
expected_checksum=
while read -r checksum filename remainder; do
  [ "$filename" = "$archive" ] || continue
  [ -z "${remainder:-}" ] || fail "malformed checksum entry for $archive"
  checksum_matches=$((checksum_matches + 1))
  expected_checksum=$checksum
done < "$checksums_path"

[ "$checksum_matches" -eq 1 ] ||
  fail "checksums.txt must contain exactly one entry for $archive"
[ "${#expected_checksum}" -eq 64 ] || fail "malformed checksum for $archive"
case "$expected_checksum" in
  *[!0-9A-Fa-f]*) fail "malformed checksum for $archive" ;;
esac

actual_checksum_output=$(sha256sum "$archive_path") ||
  fail "could not calculate the archive checksum"
actual_checksum=${actual_checksum_output%% *}
[ "$actual_checksum" = "$expected_checksum" ] ||
  fail "checksum mismatch for $archive"

extract_directory=$temporary_directory/extract
mkdir "$extract_directory" || fail "could not create the extraction directory"
tar -xzf "$archive_path" -C "$extract_directory" slopmachine ||
  fail "could not extract slopmachine from $archive"
[ -f "$extract_directory/slopmachine" ] || fail "archive does not contain slopmachine"

mkdir -p "$destination" || fail "could not create destination: $destination"
[ ! -d "$destination/slopmachine" ] ||
  fail "destination path is a directory: $destination/slopmachine"
destination_temporary=$(mktemp "$destination/.slopmachine.XXXXXX") ||
  fail "could not create an atomic destination file"
cp "$extract_directory/slopmachine" "$destination_temporary" ||
  fail "could not copy slopmachine into the destination"
chmod 755 "$destination_temporary" || fail "could not make slopmachine executable"
mv -f "$destination_temporary" "$destination/slopmachine" ||
  fail "could not replace $destination/slopmachine"
destination_temporary=

printf 'Installed slopmachine %s to %s/slopmachine\n' "$release_tag" "$destination"

# Non-fatal PATH smoke check: warn when another copy shadows this install.
resolved_binary=$(command -v slopmachine 2>/dev/null || true)
if [ -z "$resolved_binary" ]; then
  printf 'Note: %s is not on PATH in this shell; run %s/slopmachine directly or extend PATH.\n' \
    "$destination" "$destination" >&2
else
  resolved_directory=$(CDPATH='' cd -- "$(dirname -- "$resolved_binary")" 2>/dev/null && pwd -P) ||
    resolved_directory=
  destination_directory=$(CDPATH='' cd -- "$destination" 2>/dev/null && pwd -P) ||
    destination_directory=
  if [ -z "$resolved_directory" ] || [ "$resolved_directory" != "$destination_directory" ]; then
    printf 'Warning: PATH resolves slopmachine to %s, not %s/slopmachine.\n' \
      "$resolved_binary" "$destination" >&2
    printf 'Another copy wins PATH precedence; list every copy with: type -a slopmachine\n' >&2
  fi
fi
