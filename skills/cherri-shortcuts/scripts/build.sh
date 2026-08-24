#!/bin/sh
set -eu
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
. "$SCRIPT_DIR/common.sh"
ensure_cherri

if [ "$#" -lt 1 ]; then
  echo "Usage: build.sh SOURCE.cherri [OUTPUT.shortcut] [--signed|--unsigned] [--contacts|--anyone]" >&2
  exit 2
fi
SOURCE=$1
shift
OUTPUT=""
MODE=signed
SHARE=anyone

if [ "$#" -gt 0 ]; then
  case "$1" in
    --*) ;;
    *) OUTPUT=$1; shift ;;
  esac
fi
for arg in "$@"; do
  case "$arg" in
    --signed) MODE=signed ;;
    --unsigned) MODE=unsigned ;;
    --contacts) SHARE=contacts ;;
    --anyone) SHARE=anyone ;;
    *) echo "Unknown option: $arg" >&2; exit 2 ;;
  esac
done

[ -f "$SOURCE" ] || { echo "Source not found: $SOURCE" >&2; exit 1; }
SOURCE_DIR=$(CDPATH= cd -- "$(dirname -- "$SOURCE")" && pwd)
SOURCE_ABS="$SOURCE_DIR/$(basename -- "$SOURCE")"
if [ -z "$OUTPUT" ]; then
  OUTPUT=${SOURCE_ABS%.*}.shortcut
fi
case "$OUTPUT" in
  /*) OUTPUT_ABS=$OUTPUT ;;
  *) OUTPUT_ABS="$(pwd)/$OUTPUT" ;;
esac
mkdir -p "$(dirname -- "$OUTPUT_ABS")"
rm -f "$OUTPUT_ABS"

if [ "$MODE" = signed ]; then
  # Signed output honors --output because the signing service writes to
  # Cherri's outputPath. Cherri cleans up the intermediary unsigned file.
  "$CHERRI_BIN_RESOLVED" "$SOURCE_ABS" --output="$OUTPUT_ABS" --hubsign --share="$SHARE" --no-ansi
  [ -s "$OUTPUT_ABS" ] || { echo "Compiler did not create: $OUTPUT_ABS" >&2; exit 1; }
  HEADER=$(dd if="$OUTPUT_ABS" bs=1 count=4 2>/dev/null || true)
  [ "$HEADER" = "AEA1" ] || { echo "Signing result is not an AEA1 Shortcut: $OUTPUT_ABS" >&2; exit 1; }
else
  # Cherri deliberately writes skip-sign output beside the source as
  # <workflowName>_unsigned.shortcut; --output only applies to the signed
  # destination. Run the compiler, then pick the freshly-written unsigned file
  # and move it to the caller's requested destination.
  STATE=${TMPDIR:-/tmp}/cherri-unsigned-before-$$
  : > "$STATE"
  for f in "$SOURCE_DIR"/*_unsigned.shortcut; do
    [ -f "$f" ] || continue
    cksum "$f" >> "$STATE" 2>/dev/null || true
  done

  "$CHERRI_BIN_RESOLVED" "$SOURCE_ABS" --skip-sign --no-ansi

  GENERATED=""
  for f in "$SOURCE_DIR"/*_unsigned.shortcut; do
    [ -f "$f" ] || continue
    LINE=$(cksum "$f" 2>/dev/null || true)
    if ! grep -Fqx -- "$LINE" "$STATE" 2>/dev/null; then
      GENERATED=$f
      break
    fi
  done
  rm -f "$STATE"

  # If Cherri overwrote an identical pre-existing file, the checksum is
  # unchanged. In that case the most recently touched unsigned file is still
  # the correct compiler output.
  if [ -z "$GENERATED" ]; then
    GENERATED=$(ls -1t "$SOURCE_DIR"/*_unsigned.shortcut 2>/dev/null | head -n 1 || true)
  fi
  [ -n "$GENERATED" ] && [ -s "$GENERATED" ] || {
    echo "Compiler succeeded but no unsigned Shortcut output was found beside $SOURCE_ABS" >&2
    exit 1
  }

  if [ "$GENERATED" != "$OUTPUT_ABS" ]; then
    mv -f "$GENERATED" "$OUTPUT_ABS"
  fi
fi

printf '%s\n' "$OUTPUT_ABS"
