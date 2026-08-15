#!/usr/bin/env bash

# Harness conformance: prove a skill-less shell agent — no harness features,
# no bundled skill, no JSON tooling beyond the shell — can drive one complete
# multi-unit run using only `status --json --fields`, `schema`, stdin
# payloads, and the executable commands `next_action` returns.

set -euo pipefail

repository_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)

# next_action must be an executable slopshipper command. Placeholders in
# angle brackets are documented fields for the caller to fill; anything that
# names a harness capability is a protocol violation.
assert_conformant() {
  local action="$1"
  if [[ -z $action ]]; then
    printf 'conformance: empty next_action outside a terminal state\n' >&2
    return 1
  fi
  if [[ $action != slopshipper\ * ]]; then
    printf 'conformance: next_action is not an executable slopshipper command: %s\n' "$action" >&2
    return 1
  fi
  if grep -qiE 'subagent|spawn|/loop|skill|claude|cursor|codex|copilot' <<<"$action"; then
    printf 'conformance: next_action leaks a harness idiom: %s\n' "$action" >&2
    return 1
  fi
  # The driver executes next_action verbatim, so it must be exactly one
  # command: no separators, pipes, substitutions, or extra lines. Angle
  # brackets stay legal — they are documented placeholders.
  if [[ $action == *$'\n'* ]] || grep -qE '[;&|$`]|<\(|>\(' <<<"$action"; then
    printf 'conformance: next_action is not a single plain command: %s\n' "$action" >&2
    return 1
  fi
  return 0
}

if [[ ${1:-} == --self-test ]]; then
  assert_conformant "slopshipper release --revision 2 --run='demo'"
  assert_conformant "slopshipper verify --cmd '<verification command>' --run='demo'"
  # shellcheck disable=SC2016 # the $() sample is a deliberate literal
  for corrupt in \
    '' \
    'run the /slopshipper skill' \
    'spawn a subagent to continue' \
    'claude -p "continue the run"' \
    'use cursor /loop until done' \
    'slopshipper status; rm -rf .' \
    'slopshipper build && slopshipper verify' \
    'slopshipper verify --cmd "$(cat /etc/hostname)"' \
    $'slopshipper build\nslopshipper verify'; do
    if assert_conformant "$corrupt" 2>/dev/null; then
      printf 'conformance self-test: accepted corrupt next_action: %q\n' "$corrupt" >&2
      exit 1
    fi
  done
  printf 'conformance self-test passed\n'
  exit 0
fi

scratch=$(mktemp -d "${TMPDIR:-/tmp}/slopshipper-conformance.XXXXXX")
trap 'rm -rf "$scratch"' EXIT

# next_action names the installed binary; put the fixture build on PATH the
# way a real installation would be.
mkdir -p "$scratch/bin"
binary=$scratch/bin/slopshipper
(cd "$repository_root" && go build -o "$binary" ./cmd/slopshipper)
export PATH=$scratch/bin:$PATH

workdir=$scratch/repo
mkdir -p "$workdir"
git -C "$workdir" init --quiet
export SLOPSHIPPER_DB=$scratch/conformance.sqlite
cd "$workdir"

# Extract one string field from the pretty-printed status JSON. The fixture
# controls every identifier, so values never contain escaped quotes.
field() {
  local name=$1
  sed -n 's/^  "'"$name"'": "\{0,1\}\([^"]*\)"\{0,1\},\{0,1\}$/\1/p' <<<"$2" |
    head -1 |
    sed 's/\\u003c/</g; s/\\u003e/>/g; s/\\u0026/\&/g'
}

# The protocol advertises its own contract before the loop starts.
"$binary" schema --command intake >/dev/null
"$binary" status --json >/dev/null

"$binary" init --run conform >/dev/null

states_seen=""
commands_seen=""
for _ in $(seq 1 40); do
  status_json=$("$binary" status --json --fields state,next_action,intake_revision --run conform)
  state=$(field state "$status_json")
  action=$(field next_action "$status_json")
  states_seen="$states_seen $state"

  if [[ $state == RUN_DONE ]]; then
    break
  fi
  assert_conformant "$action"

  # Fill documented placeholders; everything else runs verbatim.
  action=${action//\'<verification command>\'/true}
  action=${action//\'<signal>\'/merged}

  command_word=$(sed -n 's/^slopshipper \([a-z]*\).*/\1/p' <<<"$action")
  commands_seen="$commands_seen $command_word"
  case "$command_word" in
    intake)
      eval "$action" >/dev/null <<'JSON'
{
  "delivery_mode": "pr-hold",
  "required_reviewers": ["autoreview"],
  "series_bound": 2,
  "units": [
    {"id": "c1", "title": "first conformance unit", "blockers": []},
    {"id": "c2", "title": "second conformance unit", "blockers": ["c1"]}
  ]
}
JSON
      ;;
    review)
      eval "$action" >/dev/null <<'JSON'
{"reviewer":"autoreview","verdict":"clean","artifact_ref":"autoreview://conformance"}
JSON
      ;;
    deliver)
      eval "$action" >/dev/null <<'JSON'
{"delivery_mode":"pr-hold","pr_url":"https://example.invalid/pr/1"}
JSON
      ;;
    observe)
      # The unit placeholder is a documented fill; a single delivered unit
      # arrives already named inline in next_action.
      if [[ $action == *"'<unit>'"* ]]; then
        delivered_json=$("$binary" status --json --fields delivered_units --run conform)
        delivered_unit=$(sed -n 's/^    "\([^"]*\)",\{0,1\}$/\1/p' <<<"$delivered_json" | head -1)
        if [[ -z $delivered_unit ]]; then
          printf 'conformance: observe demanded with no delivered unit\n' >&2
          exit 1
        fi
        action=${action//\'<unit>\'/$delivered_unit}
      fi
      eval "$action" >/dev/null
      ;;
    *)
      eval "$action" >/dev/null
      ;;
  esac
done

final_json=$("$binary" status --json --fields state,completed_units --run conform)
final_state=$(field state "$final_json")
if [[ $final_state != RUN_DONE ]]; then
  printf 'conformance: run did not finish; states:%s\n' "$states_seen" >&2
  exit 1
fi
completed=$(sed -n 's/^  "completed_units": \([0-9]*\),\{0,1\}$/\1/p' <<<"$final_json")
if [[ $completed != 2 ]]; then
  printf 'conformance: expected 2 completed units, got %s\n' "$completed" >&2
  exit 1
fi
for expected in INTAKE BUILD REVIEW DELIVER AWAITING_SIGNALS RUN_DONE; do
  case "$states_seen" in
    *" $expected"*) ;;
    *)
      printf 'conformance: never observed state %s in:%s\n' "$expected" "$states_seen" >&2
      exit 1
      ;;
  esac
done
# The protocol must have demanded every documented step, not skipped edges.
for expected in intake release build verify review deliver observe; do
  case "$commands_seen" in
    *" $expected"*) ;;
    *)
      printf 'conformance: next_action never demanded %s; commands:%s\n' "$expected" "$commands_seen" >&2
      exit 1
      ;;
  esac
done
if [[ -n $(git -C "$workdir" status --porcelain) ]]; then
  printf 'conformance: the protocol left files in the worktree\n' >&2
  git -C "$workdir" status --porcelain >&2
  exit 1
fi

printf 'conformance passed: skill-less shell agent drove %s transitions to RUN_DONE\n' "$(wc -w <<<"$states_seen" | tr -d ' ')"
