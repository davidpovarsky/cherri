#!/bin/sh
# prepare-edit.sh — prepare a deterministic editable workspace from an existing
# Apple Shortcut plist (the first-class "edit an existing Shortcut" workflow).
#
# Usage:
#   prepare-edit.sh INPUT_PLIST_OR_UNSIGNED_SHORTCUT [WORKSPACE_DIR]
#
# INPUT may be:
#   * an XML plist
#   * a binary plist
#   * an unsigned .shortcut whose contents are a workflow plist
#   * an explicit filesystem path (attached/shared file) — pass it exactly
#
# The helper NEVER performs the user's semantic edit itself and NEVER signs.
# It creates a safe, deterministic workspace, decompiles with the real Cherri
# decompiler, and immediately proves the generated source recompiles unsigned.
#
# On success it reports (one per line, paths are absolute):
#   original copy path
#   editable .cherri path
#   validation unsigned .shortcut path
#   workspace path
#
# Exit codes:
#   0  success (workspace ready for editing)
#   1  input missing / no decompiled source produced
#   2  usage error
#   3  input is a signed AEA1 Shortcut (direct extraction not supported yet)
#   4  decompiled source does not recompile (workspace preserved for diagnosis)
set -eu
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
. "$SCRIPT_DIR/common.sh"
ensure_cherri

if [ "$#" -lt 1 ]; then
  echo "Usage: prepare-edit.sh INPUT_PLIST_OR_UNSIGNED_SHORTCUT [WORKSPACE_DIR]" >&2
  exit 2
fi

INPUT=$1
[ -f "$INPUT" ] || { echo "Input not found: $INPUT" >&2; exit 1; }
INPUT_DIR=$(CDPATH= cd -- "$(dirname -- "$INPUT")" && pwd)
INPUT_ABS="$INPUT_DIR/$(basename -- "$INPUT")"

# Early format diagnostic: signed AEA1 input cannot be unpacked yet.
# Never pass AEA1 bytes into the plist import and produce confusing errors.
HEADER=$(dd if="$INPUT_ABS" bs=1 count=4 2>/dev/null || true)
if [ "$HEADER" = "AEA1" ]; then
  echo "Input is a signed AEA1 Shortcut." >&2
  echo "Direct signed-shortcut extraction is not yet supported." >&2
  echo "Use your iOS helper Shortcut / iCloud-link workflow to provide an" >&2
  echo "unsigned plist representation of the Shortcut instead." >&2
  exit 3
fi

# Deterministic workspace layout:
#   <workspace>/
#     original/shortcut.plist     preserved input (audit/recovery)
#     source/shortcut.cherri      editable Cherri source
#     builds/validation.shortcut  unsigned validation build
#     builds/final.shortcut       signed final output (after agent edits)
BASE_NAME=$(basename -- "$INPUT_ABS")
SAFE_NAME=$(printf '%s' "${BASE_NAME%.*}" | tr -c 'A-Za-z0-9._-' '_')
STAMP=$(date +%Y%m%d-%H%M%S)
if [ "$#" -ge 2 ] && [ -n "$2" ]; then
  WORKSPACE=$2
else
  WORKSPACE="${CHERRI_EDITS_DIR:-/var/minis/shared/cherri-edits}/$SAFE_NAME-$STAMP"
fi
mkdir -p "$WORKSPACE/original" "$WORKSPACE/source" "$WORKSPACE/builds"

# 1. Preserve the original input. Never modify the caller's file.
ORIGINAL="$WORKSPACE/original/shortcut.plist"
cp -f "$INPUT_ABS" "$ORIGINAL"

# 2. Decompile with the real Cherri decompiler into the workspace source dir.
sh "$SCRIPT_DIR/decompile.sh" "$ORIGINAL" "$WORKSPACE/source" >/dev/null

# 3. Locate the generated .cherri source deterministically.
CHERRI_SRC=""
for f in "$WORKSPACE/source"/*.cherri; do
  [ -f "$f" ] || continue
  CHERRI_SRC=$f
  break
done
if [ -z "$CHERRI_SRC" ]; then
  echo "Decompile produced no .cherri source in $WORKSPACE/source" >&2
  exit 1
fi

# 4. Immediately compile the generated source unsigned. If recompilation
#    fails, fail clearly, preserve the workspace, and report the error.
#    Never silently edit the source to hide a decompiler defect.
VALIDATION="$WORKSPACE/builds/validation.shortcut"
if ! VALIDATION_OUTPUT=$(sh "$SCRIPT_DIR/build.sh" "$CHERRI_SRC" "$VALIDATION" --unsigned 2>&1); then
  echo "Prepared source does not recompile (decompiler defect or unsupported action)." >&2
  echo "Workspace preserved for diagnosis: $WORKSPACE" >&2
  echo "Decompiled source: $CHERRI_SRC" >&2
  printf '%s\n' "$VALIDATION_OUTPUT" >&2
  exit 4
fi

printf '%s\n' "$ORIGINAL"
printf '%s\n' "$CHERRI_SRC"
printf '%s\n' "$VALIDATION"
printf '%s\n' "$WORKSPACE"
