#!/bin/sh
set -eu
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
. "$SCRIPT_DIR/common.sh"

QUERY=${*:-}
if [ ! -d "$CHERRI_DOCS_DIR" ]; then
  sh "$SKILL_DIR/scripts/setup.sh"
fi

if [ -z "$QUERY" ]; then
  printf '%s\n' "$CHERRI_DOCS_DIR"
  exit 0
fi

grep -Rni -I --include='*.md' --include='*.html' --include='*.txt' -- "$QUERY" "$CHERRI_DOCS_DIR" 2>/dev/null | head -n 80 || true
