#!/bin/sh
# Generate action documentation from Cherri's shared metadata.
#
# Action signatures and descriptions are generated with the compiler's own
# --docs output so they are never hand-maintained in Markdown. Point the
# result at a checkout of the platform documentation repository:
#
#   origin   https://github.com/davidpovarsky/cherrilang.org.git  (default source)
#   upstream https://github.com/electrikmilk/cherrilang.org.git
#
# Usage: sh scripts/generate-action-docs.sh [output-dir] [category ...]
set -eu

CHERRI_BIN=${CHERRI_BIN:-cherri}
OUT_DIR=${1:-./generated-docs}
shift || true

if ! command -v "$CHERRI_BIN" >/dev/null 2>&1; then
  echo "Cherri binary not found; set CHERRI_BIN or install cherri." >&2
  exit 1
fi

mkdir -p "$OUT_DIR"

CATEGORIES="$@"
if [ -z "$CATEGORIES" ]; then
  CATEGORIES="basic"
fi

for category in $CATEGORIES; do
  echo "Generating $category..."
  "$CHERRI_BIN" --docs="$category" --no-ansi > "$OUT_DIR/$category.md"
done

echo "Generated action documentation in $OUT_DIR"
echo "Copy or move these files into the cherrilang.org repository to publish."
