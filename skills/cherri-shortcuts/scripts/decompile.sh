#!/bin/sh
set -eu
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
. "$SCRIPT_DIR/common.sh"
ensure_cherri

if [ "$#" -lt 1 ]; then
  echo "Usage: decompile.sh INPUT_SHORTCUT_OR_PLIST_OR_ICLOUD_URL [OUTPUT_DIR]" >&2
  exit 2
fi
INPUT=$1
OUTDIR=${2:-$(pwd)}
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
# explicit output directory so callers get deterministic workspace placement.
"$CHERRI_BIN_RESOLVED" --import="$IMPORT_VALUE" --output="$OUTDIR" --no-toolkit --no-ansi
printf 'Decompiled output directory: %s\n' "$OUTDIR"
