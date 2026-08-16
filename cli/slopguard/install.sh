#!/bin/sh

set -eu

# slopguard release assets ship from the ffsstack monorepo starting with
# the first slopguard release (uinaf/slopshipper#50); earlier artifacts are
# autoreview releases in the legacy repository.
repository_url=${SLOPGUARD_INSTALL_REPOSITORY_URL:-https://github.com/uinaf/ffsstack}
while [ "${repository_url%/}" != "$repository_url" ]; do
  repository_url=${repository_url%/}
done
requested_version=latest
destination=
temporary_directory=
destination_temporary=

usage() {
  cat <<'EOF'
Install slopguard on Linux.

Usage: install.sh [--version VERSION] [--dest DIRECTORY]

Options:
  --version VERSION  Install VERSION (for example v0.4.0); defaults to latest.
  --dest DIRECTORY   Install into DIRECTORY; defaults to $HOME/.local/bin.
  -h, --help         Show this help.
EOF
}

fail() {
  printf 'slopguard installer: %s\n' "$*" >&2
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
# tags (slopguard/vX.Y.Z); "latest" means this member's newest release.
api_url=${SLOPGUARD_INSTALL_API_URL:-https://api.github.com/repos/uinaf/ffsstack/releases}
if [ "$requested_version" = latest ]; then
  # Tag listing over git avoids the unauthenticated API quota entirely;
  # any git failure (absent, blocked, offline proto) falls back to the API.
  release_tag=
  if command -v git >/dev/null 2>&1; then
    release_tag=$(git ls-remote --tags "$repository_url.git" "refs/tags/slopguard/v*" 2>/dev/null |
      sed 's|.*refs/tags/slopguard/||; s|\^{}$||' | sort -u -V | tail -1) || release_tag=
    # A crash between tag push and publish leaves a tag whose release is
    # still a draft, so its assets are not public yet. Probe one asset and
    # fall back to the API, which lists published releases only.
    if [ -n "$release_tag" ]; then
      curl --proto '=https' --tlsv1.2 -fsSL -o /dev/null \
        "$repository_url/releases/download/slopguard%2F$release_tag/checksums.txt" ||
        release_tag=
    fi
  fi
  if [ -z "$release_tag" ]; then
    release_tag=
    page=1
    while [ "$page" -le 10 ]; do
      body=$(curl --proto '=https' --tlsv1.2 -fsSL \
        -H "Accept: application/vnd.github+json" "$api_url?per_page=100&page=$page") ||
        fail "failed to resolve the latest release"
      release_tag=$(printf '%s' "$body" |
        grep -o '"tag_name": *"slopguard/v[0-9.]*"' | head -1 | sed 's/.*"slopguard\///; s/"$//')
      [ -n "$release_tag" ] && break
      case "$body" in *'"tag_name"'*) page=$((page + 1)) ;; *) break ;; esac
    done
  fi
  [ -n "$release_tag" ] || fail "no slopguard release found"
else
  case "$requested_version" in
    v*) release_tag=$requested_version ;;
    *) release_tag=v$requested_version ;;
  esac
fi

valid_release_tag "$release_tag" || fail "invalid release version: $release_tag"

temporary_directory=$(mktemp -d "${TMPDIR:-/tmp}/slopguard-install.XXXXXX") ||
  fail "could not create a temporary directory"
archive=slopguard_${release_tag}_linux_${architecture}.tar.gz
archive_path=$temporary_directory/$archive
checksums_path=$temporary_directory/checksums.txt
release_url=$repository_url/releases/download/slopguard%2F$release_tag

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
tar -xzf "$archive_path" -C "$extract_directory" slopguard ||
  fail "could not extract slopguard from $archive"
[ -f "$extract_directory/slopguard" ] || fail "archive does not contain slopguard"

mkdir -p "$destination" || fail "could not create destination: $destination"
[ ! -d "$destination/slopguard" ] ||
  fail "destination path is a directory: $destination/slopguard"
destination_temporary=$(mktemp "$destination/.slopguard.XXXXXX") ||
  fail "could not create an atomic destination file"
cp "$extract_directory/slopguard" "$destination_temporary" ||
  fail "could not copy slopguard into the destination"
chmod 755 "$destination_temporary" || fail "could not make slopguard executable"
mv -f "$destination_temporary" "$destination/slopguard" ||
  fail "could not replace $destination/slopguard"
destination_temporary=

printf 'Installed slopguard %s to %s/slopguard\n' "$release_tag" "$destination"
