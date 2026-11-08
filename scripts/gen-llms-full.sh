#!/bin/sh
# Builds llms-full.txt, one file with the key docs for LLM consumption.
# No argument rewrites the file; --check exits 1 if the checked-in copy is stale.
set -eu
root=$(cd "$(dirname "$0")/.." && pwd)
cd "$root"
out=llms-full.txt
tmp=$(mktemp)
trap 'rm -f "$tmp"' EXIT
{
  cat llms.txt
  for f in README.md docs/USAGE.md docs/INSTALL.md docs/LANGUAGES.md docs/FAQ.md docs/PRIVACY.md docs/ARCHITECTURE.md skills/stacklit/SKILL.md; do
    printf '\n\n---\n\n<!-- source: %s -->\n\n' "$f"
    cat "$f"
  done
} > "$tmp"
if [ "${1:-}" = "--check" ]; then
  cmp -s "$tmp" "$out" || { echo "$out is stale: run scripts/gen-llms-full.sh" >&2; exit 1; }
  exit 0
fi
cp "$tmp" "$out"
