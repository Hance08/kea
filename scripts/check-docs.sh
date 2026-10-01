#!/usr/bin/env bash
# check-docs.sh verifies that every repo path and relative link referenced in
# the contributor docs exists. Run from anywhere: scripts/check-docs.sh
set -euo pipefail
cd "$(dirname "$0")/.."

shopt -s nullglob
docs=(AGENTS.md README.md spa/README.md docs/*.md docs/recipes/*.md)
fail=0

for doc in "${docs[@]}"; do
  [ -f "$doc" ] || continue
  dir=$(dirname "$doc")

  # Backticked repo paths such as `internal/service/errors.go` or `migrations/0011_*`.
  while IFS= read -r p; do
    [ -z "$p" ] && continue
    p=${p%%:*}                      # drop a ":line" or ":Symbol" suffix
    if ! compgen -G "$p" >/dev/null; then
      echo "$doc: missing path \`$p\`"
      fail=1
    fi
  done < <(grep -oE '`(cmd|internal|ui|migrations|spa|scripts|docker|docs)/[^` ]*`' "$doc" | tr -d '`' | sort -u || true)

  # Relative markdown links such as [domain](domain.md) or [x](../cmd/root.go).
  while IFS= read -r l; do
    l=${l%%#*}
    [ -z "$l" ] && continue
    case "$l" in http://*|https://*|mailto:*) continue ;; esac
    if [ ! -e "$dir/$l" ]; then
      echo "$doc: broken link ($l)"
      fail=1
    fi
  done < <(grep -oE '\]\([^) ]+\)' "$doc" | sed -E 's/^\]\(//; s/\)$//' | sort -u || true)
done

if [ "$fail" -eq 0 ]; then
  echo "check-docs: all referenced paths exist"
fi
exit "$fail"
