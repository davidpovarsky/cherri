#!/bin/sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
SKILL_DIR=$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)

: "${CHERRI_HOME:=/var/minis/cherri}"
: "${CHERRI_REPO:=https://github.com/davidpovarsky/cherri.git}"
: "${CHERRI_REF:=main}"
: "${CHERRI_DOCS_REPO:=https://github.com/electrikmilk/cherrilang.org.git}"
: "${CHERRI_DOCS_REF:=main}"
: "${CHERRI_DOCS_DIR:=$CHERRI_HOME/docs}"

BUNDLED_CHERRI="$SKILL_DIR/bin/cherri"
CACHED_CHERRI="$CHERRI_HOME/bin/cherri"

resolve_cherri() {
  if [ -n "${CHERRI_BIN:-}" ] && [ -f "$CHERRI_BIN" ]; then
    printf '%s\n' "$CHERRI_BIN"
    return 0
  fi
  if [ -f "$BUNDLED_CHERRI" ]; then
    chmod +x "$BUNDLED_CHERRI" 2>/dev/null || true
    printf '%s\n' "$BUNDLED_CHERRI"
    return 0
  fi
  if [ -x "$CACHED_CHERRI" ]; then
    printf '%s\n' "$CACHED_CHERRI"
    return 0
  fi
  if command -v cherri >/dev/null 2>&1; then
    command -v cherri
    return 0
  fi
  return 1
}

ensure_cherri() {
  if CHERRI_BIN_RESOLVED=$(resolve_cherri); then
    export CHERRI_BIN_RESOLVED
    return 0
  fi
  sh "$SKILL_DIR/scripts/setup.sh"
  CHERRI_BIN_RESOLVED=$(resolve_cherri) || {
    echo "Cherri setup completed but no compiler was found." >&2
    exit 1
  }
  export CHERRI_BIN_RESOLVED
}
