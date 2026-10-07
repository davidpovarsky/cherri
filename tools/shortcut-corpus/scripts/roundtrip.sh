#!/bin/sh
# Structural round-trip verification:
#
#   cherri source -> unsigned plist -> decompile -> recompile -> compare
#
# Usage: sh tools/shortcut-corpus/scripts/roundtrip.sh path/to/source.cherri
# Environment:
#   CHERRI_BIN   Cherri binary (default: cherri on PATH)
#   CORPUS_BIN   shortcut-corpus binary (default: built into a temp dir)
set -eu

SRC=${1:?usage: roundtrip.sh source.cherri}
CHERRI_BIN=${CHERRI_BIN:-cherri}
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
TOOL_DIR=$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)

if ! command -v "$CORPUS_BIN" >/dev/null 2>&1 && [ -z "${CORPUS_BIN:-}" ]; then
  CORPUS_BIN="$(mktemp -d)/shortcut-corpus"
  (cd "$TOOL_DIR/../.." && go build -o "$CORPUS_BIN" ./tools/shortcut-corpus)
fi

WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT INT TERM

echo "[1/4] Compiling $SRC"
"$CHERRI_BIN" "$SRC" -o "$WORK/a.shortcut" --skip-sign >/dev/null

echo "[2/4] Decompiling"
"$CHERRI_BIN" --import "$WORK/a.shortcut" -o "$WORK/b.cherri" >/dev/null

echo "[3/4] Recompiling decompiled source"
"$CHERRI_BIN" "$WORK/b.cherri" -o "$WORK/b.shortcut" --skip-sign >/dev/null

echo "[4/4] Comparing structure"
"$CORPUS_BIN" compare "$WORK/a.shortcut" "$WORK/b.shortcut"
