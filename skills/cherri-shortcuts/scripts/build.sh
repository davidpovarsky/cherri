#!/bin/sh
set -eu
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
. "$SCRIPT_DIR/common.sh"

if [ "$#" -lt 1 ]; then
  echo "Usage: build.sh SOURCE.cherri [OUTPUT.shortcut] [--signed|--unsigned] [--contacts|--anyone] [--original=ORIGINAL.plist --allowed-delta=POLICY.json]" >&2
  exit 2
fi
SOURCE=$1
shift
OUTPUT=""
MODE=signed
SHARE=anyone
ORIGINAL=""
POLICY=""

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
    --original=*) ORIGINAL=${arg#--original=} ;;
    --allowed-delta=*) POLICY=${arg#--allowed-delta=} ;;
    *) echo "Unknown option: $arg" >&2; exit 2 ;;
  esac
done

[ -f "$SOURCE" ] || { echo "Source not found: $SOURCE" >&2; exit 1; }
SOURCE_DIR=$(CDPATH= cd -- "$(dirname -- "$SOURCE")" && pwd)
SOURCE_ABS="$SOURCE_DIR/$(basename -- "$SOURCE")"
if [ -z "$OUTPUT" ]; then
  OUTPUT=${SOURCE_ABS%.*}.shortcut
fi
mkdir -p "$(dirname -- "$OUTPUT")"
OUTPUT_DIR=$(CDPATH= cd -- "$(dirname -- "$OUTPUT")" && pwd)
OUTPUT_ABS="$OUTPUT_DIR/$(basename -- "$OUTPUT")"
if [ "$SOURCE_ABS" = "$OUTPUT_ABS" ] || [ "$SOURCE_ABS" -ef "$OUTPUT_ABS" ]; then
  echo "[CHERRI_BUILD_INPUT_ALIAS] Output must not replace the source." >&2
  exit 2
fi
if [ -e "$OUTPUT_ABS" ] && [ ! -f "$OUTPUT_ABS" ]; then
  echo "[CHERRI_BUILD_OUTPUT_TYPE] Output is not a regular file." >&2
  exit 2
fi

# Prepared workspaces carry their own immutable original. Imported source copied
# elsewhere must name that original explicitly before the signing step.
if [ "$MODE" = signed ]; then
  if [ -z "$ORIGINAL" ] && [ -f "$SOURCE_DIR/../original/shortcut.plist" ]; then
    ORIGINAL="$SOURCE_DIR/../original/shortcut.plist"
  fi
fi
if [ -n "$ORIGINAL" ] || [ -n "$POLICY" ]; then
  if [ -z "$ORIGINAL" ] || [ -z "$POLICY" ] || [ ! -f "$ORIGINAL" ] || [ ! -f "$POLICY" ]; then
    echo "[CHERRI_EDIT_POLICY_REQUIRED] Provide an existing original and explicit allowed-delta policy." >&2
    exit 2
  fi
  if [ "$ORIGINAL" -ef "$OUTPUT_ABS" ] || [ "$POLICY" -ef "$OUTPUT_ABS" ]; then
    echo "[CHERRI_BUILD_INPUT_ALIAS] Output must not replace the original or edit policy." >&2
    exit 2
  fi
fi

ensure_cherri
STAGE=$(mktemp -d "$OUTPUT_DIR/.cherri-build.XXXXXXXX")
trap 'rm -rf "$STAGE"' EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
UNSIGNED="$STAGE/verified-unsigned.shortcut"

# The explicit V2 compile command writes exactly this file. No sibling-file
# discovery, preexisting output deletion, or second compilation is needed.
# With no immutable original, the canonical session must prove this compilation
# did not load native workflow provenance (including through local modules).
# The decision comes from actual lowering, independent of source formatting.
set -- compile "$SOURCE_ABS" --skip-sign "--output=$UNSIGNED" --no-ansi
if [ "$MODE" = signed ] && [ -z "$ORIGINAL" ]; then
  set -- "$@" --require-authored-workflow
fi
"$CHERRI_BIN_RESOLVED" "$@"
[ -s "$UNSIGNED" ] || { echo "Unsigned compilation produced no Shortcut." >&2; exit 1; }
if [ -n "$ORIGINAL" ]; then
  if ! GATE=$("$CHERRI_BIN_RESOLVED" verify-edit "$ORIGINAL" "$UNSIGNED" "--allowed-delta=$POLICY" --json 2>&1); then
    printf '%s\n' "$GATE" >&2
    echo "[CHERRI_EDIT_GATE_FAILED] Finalization stopped; original and previous output are preserved." >&2
    exit 1
  fi
else
  if ! GATE=$("$CHERRI_BIN_RESOLVED" verify-edit "$UNSIGNED" "$UNSIGNED" --json 2>&1); then
    printf '%s\n' "$GATE" >&2
    echo "[CHERRI_EDIT_GATE_FAILED] Compiler comparison entrypoint is required." >&2
    exit 1
  fi
fi
if ! json_is_valid_object "$GATE" || ! printf '%s\n' "$GATE" | grep -Eq '"ok"[[:space:]]*:[[:space:]]*true'; then
  echo "[CHERRI_EDIT_GATE_FAILED] Comparison returned no successful structured receipt." >&2
  exit 1
fi

if [ "$MODE" = signed ]; then
  SIGNED="$STAGE/$(basename -- "$OUTPUT_ABS")"
  # Sign the same bytes that passed the complete document gate. The public sign
  # command retains its configured service selection and AEA1 validation.
  if [ -n "${CHERRI_SIGNING_SERVER:-}" ]; then
    "$CHERRI_BIN_RESOLVED" sign "$UNSIGNED" "--output=$SIGNED" "--source-file=$SOURCE_ABS" "--signing-server=$CHERRI_SIGNING_SERVER" "--share=$SHARE" --no-ansi
  else
    "$CHERRI_BIN_RESOLVED" sign "$UNSIGNED" "--output=$SIGNED" "--source-file=$SOURCE_ABS" --hubsign "--share=$SHARE" --no-ansi
  fi
  HEADER=$(dd if="$SIGNED" bs=1 count=4 2>/dev/null || true)
  if [ ! -s "$SIGNED" ] || [ "$HEADER" != AEA1 ]; then
    echo "[CHERRI_SIGN_INVALID] Signing did not produce an AEA1 Shortcut." >&2
    exit 1
  fi
  mv -f "$SIGNED" "$OUTPUT_ABS"
else
  mv -f "$UNSIGNED" "$OUTPUT_ABS"
fi
printf '%s\n' "$OUTPUT_ABS"
