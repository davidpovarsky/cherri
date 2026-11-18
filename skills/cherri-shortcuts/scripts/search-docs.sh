#!/bin/sh
set -eu
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
. "$SCRIPT_DIR/common.sh"

SCOPE=active
case "${1:-}" in
  --legacy-reference) SCOPE=legacy; shift ;;
  --help|-h)
    printf '%s\n' 'Usage: search-docs.sh [--legacy-reference] [--] QUERY'
    printf '%s\n' 'Default: active fork-language V2 documentation. Historical Legacy pages require explicit selection.'
    exit 0 ;;
esac
if [ "${1:-}" = "--" ]; then shift; fi
QUERY=${*:-}
if [ ! -d "$CHERRI_DOCS_DIR" ]; then
  sh "$SKILL_DIR/scripts/setup.sh"
fi

if [ "$SCOPE" = legacy ]; then
  SEARCH_DIR="$CHERRI_DOCS_DIR/language"
  if [ ! -d "$SEARCH_DIR" ] || [ -L "$SEARCH_DIR" ]; then
    printf '%s\n' "Historical Legacy documentation directory is missing or linked: $SEARCH_DIR" >&2
    exit 1
  fi
else
  SEARCH_DIR="$CHERRI_DOCS_DIR/fork-language"
  if [ ! -d "$SEARCH_DIR" ] || [ -L "$SEARCH_DIR" ]; then
    printf '%s\n' "Active V2 documentation directory is missing or linked: $SEARCH_DIR" >&2
    exit 1
  fi
fi
if [ -z "$QUERY" ]; then
  printf '%s\n' "$SEARCH_DIR"
  exit 0
fi

RESULTS=$(mktemp "${TMPDIR:-/tmp}/cherri-doc-search.XXXXXX")
trap 'rm -f -- "$RESULTS"' 0 1 2 15
# -r does not follow links encountered inside the selected tree. A missing
# current guide never causes an implicit search of historical programs.
if grep -rni -I --include='*.md' --include='*.html' --include='*.txt' -- "$QUERY" "$SEARCH_DIR" >"$RESULTS"; then
  head -n 80 "$RESULTS"
else
  code=$?
  if [ "$code" -ne 1 ]; then
    exit "$code"
  fi
fi
