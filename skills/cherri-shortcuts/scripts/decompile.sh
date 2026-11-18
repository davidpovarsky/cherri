#!/bin/sh
set -eu
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
. "$SCRIPT_DIR/common.sh"
ensure_cherri

if [ "$#" -lt 1 ]; then
  echo "Usage: decompile.sh INPUT_SHORTCUT_OR_PLIST_OR_ICLOUD_URL [OUTPUT_DIR] [--mode=v2|legacy]" >&2
  exit 2
fi
INPUT=$1
shift
OUTDIR=$(pwd)
MODE=v2
if [ "$#" -gt 0 ]; then
  case "$1" in --*) ;; *) OUTDIR=$1; shift ;; esac
fi
for arg in "$@"; do
  case "$arg" in
    --mode=v2) MODE=v2 ;;
    --legacy|--mode=legacy) MODE=legacy ;;
    *) echo "Unknown option: $arg" >&2; exit 2 ;;
  esac
done
mkdir -p "$OUTDIR"
OUTDIR=$(CDPATH= cd -- "$OUTDIR" && pwd)

case "$INPUT" in
  http://*|https://*) IMPORT_VALUE=$INPUT ;;
  *)
    case "$INPUT" in
      /*) IMPORT_VALUE=$INPUT ;;
      *) IMPORT_VALUE=$(CDPATH= cd -- "$(dirname -- "$INPUT")" && pwd)/$(basename -- "$INPUT") ;;
    esac
    ;;
esac

# Cherri otherwise writes decompiled source beside the imported file. Pass an
# explicit output directory and provenance-preserving V2 hybrid importer.
SOURCE_OUTPUT="$OUTDIR/shortcut.cherri"
if [ -e "$SOURCE_OUTPUT" ]; then
  echo "[CHERRI_EDIT_SOURCE_EXISTS] Choose a new output directory; existing source is preserved." >&2
  exit 6
fi
"$CHERRI_BIN_RESOLVED" import "$IMPORT_VALUE" "--mode=$MODE" --fidelity=hybrid --output="$SOURCE_OUTPUT"
printf 'Decompiled output directory: %s\n' "$OUTDIR"
