#!/bin/sh
# Keeps the plugin copy of the skill identical to skills/stacklit/SKILL.md.
# Use --check in CI or before a release; no argument rewrites the copy.
set -eu
root=$(cd "$(dirname "$0")/.." && pwd)
src="$root/skills/stacklit/SKILL.md"
dst="$root/plugins/stacklit/skills/stacklit/SKILL.md"
if [ "${1:-}" = "--check" ]; then
  cmp -s "$src" "$dst" || { echo "plugin skill copy is stale: run scripts/sync-plugin-skill.sh" >&2; exit 1; }
  exit 0
fi
cp "$src" "$dst"
