#!/bin/sh
# Runs hooks/merge-guard against a stand-in gh that serves post-jq fixtures.
set -eu

root=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
mkdir -p "$work/bin" "$work/data"
cat > "$work/bin/gh" <<'GH'
#!/bin/sh
printf '%s\n' "$*" >> "$DATA/calls"
case "$1 $2" in
  "pr view") cat "$DATA/view" ;;
  "api graphql") cat "$DATA/threads" ;;
  *)
    case "$*" in
      *reactions*) cat "$DATA/reactions" ;;
      */comments*) cat "$DATA/comments" ;;
      */reviews*) cat "$DATA/reviews" ;;
    esac ;;
esac
GH
chmod +x "$work/bin/gh"
export DATA="$work/data" PATH="$work/bin:$PATH"

head=0123456789abcdef0123456789abcdef01234567
done_row="| 📝 **Code Review** | ✅ **Completed** <relative-time datetime=\"x\">x</relative-time> | \`0123456\` | New commits |"
running_row="| 📝 **Code Review** | 🔄 **Running** since <relative-time datetime=\"x\">x</relative-time> | \`0123456\` | New commits |"
old_row="| 📝 **Code Review** | ✅ **Completed** <relative-time datetime=\"x\">x</relative-time> | \`fedcba9\` | PR opened |"

fixture() {
  printf '7\nhttps://github.com/acme/app/pull/7\n%s\nOPEN\n%s\n%s\n%s\n' "$head" "${DECISION:-}" "${REQUESTED:-}" "${AGE:-600}" > "$DATA/view"
  printf '%s' "${THREADS:-}" > "$DATA/threads"
  printf '%s' "${EYES:-}" > "$DATA/reactions"
  if [ -n "${NOSUMMARY:-}" ]; then
    : > "$DATA/comments"
  else
    printf '<!-- codex-pull-request-review-summary -->\n| Review | Status | Commit | Review trigger |\n| --- | --- | --- | --- |\n%s\n' "${ROW:-$done_row}" > "$DATA/comments"
  fi
  printf '%s' "${REVIEWS:-}" > "$DATA/reviews"
  : > "$DATA/calls"
}

hook() {
  printf '{"cwd":"%s","tool_input":{"command":"%s"}}' "$work" "$1" | "$root/hooks/merge-guard" 2> "$work/stderr"
}

failures=0
expect() {
  want=$1 name=$2 command=$3
  if hook "$command"; then got=0; else got=$?; fi
  if [ "$got" != "$want" ]; then
    printf 'FAIL %s: exit %s, want %s\n' "$name" "$got" "$want"
    sed 's/^/  /' "$work/stderr"
    failures=$((failures + 1))
  else
    printf 'ok %s\n' "$name"
  fi
}

(fixture); expect 0 "settled PR merges" "gh pr merge 7 --squash"
(fixture); expect 0 "unrelated command skips gh" "git merge origin/main"
[ ! -s "$DATA/calls" ] || { echo "FAIL gh called for an unrelated command"; failures=$((failures + 1)); }
(THREADS="unresolved thread by bot at a.go:3" fixture); expect 2 "unresolved thread blocks" "gh pr merge 7 --squash"
(ROW=$running_row fixture); expect 2 "running Codex review blocks" "cd /tmp && gh pr merge --auto --squash"
(ROW=$old_row EYES="chatgpt-codex-connector[bot]" fixture); expect 2 "eyes without a head review blocks" "gh pr merge 7 -R acme/app"
(EYES="chatgpt-codex-connector[bot]" fixture); expect 0 "eyes with a completed head review merges" "gh pr merge 7 -R acme/app"
(ROW=$old_row fixture); expect 2 "summary without the head blocks" "gh pr merge 7"
(ROW=$old_row REVIEWS=42 fixture); expect 0 "bot review on the head merges" "gh pr merge 7"
(NOSUMMARY=1 AGE=30 fixture); expect 2 "fresh PR without auto-review blocks" "gh pr merge 7"
(NOSUMMARY=1 fixture); expect 0 "PR without auto-review merges after two minutes" "gh pr merge 7"
(REQUESTED=octocat fixture); expect 2 "pending reviewer blocks" "gh pr merge 7"
(DECISION=CHANGES_REQUESTED fixture); expect 2 "change request blocks" "gh pr merge 7"
(THREADS="unresolved thread by bot at a.go:3" fixture); expect 2 "API merge blocks" "gh api -X PUT repos/acme/app/pulls/7/merge"
grep -q 'pr view 7 -R acme/app' "$DATA/calls" || { echo "FAIL API merge did not target acme/app#7"; failures=$((failures + 1)); }

[ "$failures" -eq 0 ]
