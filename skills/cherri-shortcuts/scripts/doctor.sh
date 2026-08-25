#!/bin/sh
set -eu
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
. "$SCRIPT_DIR/common.sh"

status=0
printf 'Platform: %s %s\n' "$(uname -s 2>/dev/null || echo unknown)" "$(uname -m 2>/dev/null || echo unknown)"
printf 'Skill dir: %s\n' "$SKILL_DIR"

if BIN=$(resolve_cherri); then
  printf 'Cherri: %s\n' "$BIN"
  "$BIN" --version || status=1
else
  echo 'Cherri: MISSING (run scripts/setup.sh)'
  status=1
fi

if [ -d "$CHERRI_DOCS_DIR" ]; then
  printf 'Docs: %s\n' "$CHERRI_DOCS_DIR"
else
  echo 'Docs: MISSING (setup.sh will clone upstream docs when git is available)'
fi

for cmd in sh grep sed awk; do
  if command -v "$cmd" >/dev/null 2>&1; then
    printf '%s: ok\n' "$cmd"
  else
    printf '%s: missing\n' "$cmd"
    status=1
  fi
done

exit "$status"
