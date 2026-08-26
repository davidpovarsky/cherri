#!/bin/sh
set -eu
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
. "$SCRIPT_DIR/common.sh"

SIGN=0
SKIP_SETUP=0
for arg in "$@"; do
  case "$arg" in
    --sign) SIGN=1 ;;
    --skip-setup) SKIP_SETUP=1 ;;
    *) echo "Unknown option: $arg" >&2; exit 2 ;;
  esac
done

if [ "$SKIP_SETUP" -eq 0 ]; then
  ensure_cherri
else
  CHERRI_BIN_RESOLVED=$(resolve_cherri) || { echo 'Cherri binary missing' >&2; exit 1; }
fi

printf 'Testing compiler...\n'
"$CHERRI_BIN_RESOLVED" --version >/dev/null

printf 'Testing action discovery wrapper...\n'
sh "$SKILL_DIR/scripts/action.sh" show >/dev/null

printf 'Testing machine-readable catalog wrapper...\n'
"$CHERRI_BIN_RESOLVED" --actions-json | grep -q '"ok":true'
sh "$SKILL_DIR/scripts/catalog.sh" 'is.workflow.actions.alert' >/dev/null

TMP=${TMPDIR:-/tmp}/cherri-skill-test-$$
mkdir -p "$TMP/decompiled" "$TMP/prepared"
trap 'rm -rf "$TMP"' EXIT INT TERM

printf 'Testing unsigned compile wrapper...\n'
cat > "$TMP/test.cherri" <<'SRC'
show("Cherri Open Minis test")
SRC
sh "$SKILL_DIR/scripts/build.sh" "$TMP/test.cherri" "$TMP/test.shortcut" --unsigned >/dev/null
[ -s "$TMP/test.shortcut" ]

printf 'Testing decompile wrapper...\n'
sh "$SKILL_DIR/scripts/decompile.sh" "$TMP/test.shortcut" "$TMP/decompiled" >/dev/null
find "$TMP/decompiled" -maxdepth 1 -type f -name '*.cherri' | grep -q .

printf 'Testing decompiled-source recompilation (self-contained invariant)...\n'
cat > "$TMP/includes.cherri" <<'SRC'
#include 'actions/calendar'
#include 'actions/crypto'

@input = "self test"
@encoded = base64Encode(@input, "None")
@events = getUpcomingEvents(3)
show("Input: {@input}")
show("Base64: {@encoded}")
show("Events: {@events}")
SRC
sh "$SKILL_DIR/scripts/build.sh" "$TMP/includes.cherri" "$TMP/includes.shortcut" --unsigned >/dev/null
[ -s "$TMP/includes.shortcut" ]

rm -rf "$TMP/decompiled-includes"
mkdir -p "$TMP/decompiled-includes"
sh "$SKILL_DIR/scripts/decompile.sh" "$TMP/includes.shortcut" "$TMP/decompiled-includes" >/dev/null
INCLUDES_SRC=$(find "$TMP/decompiled-includes" -maxdepth 1 -type f -name '*.cherri' | head -n 1)
[ -n "$INCLUDES_SRC" ] || { echo 'decompile produced no .cherri source' >&2; exit 1; }

# Required standard action includes must be reconstructed automatically.
grep -q "#include 'actions/calendar'" "$INCLUDES_SRC" || { echo 'missing #include actions/calendar in decompiled source' >&2; exit 1; }
grep -q "#include 'actions/crypto'" "$INCLUDES_SRC" || { echo 'missing #include actions/crypto in decompiled source' >&2; exit 1; }

# The decompiled source must compile without manual edits.
sh "$SKILL_DIR/scripts/build.sh" "$INCLUDES_SRC" "$TMP/recompiled.shortcut" --unsigned >/dev/null
[ -s "$TMP/recompiled.shortcut" ]

printf 'Testing existing-Shortcut prepare/edit workflow...\n'
PREPARE_OUT=$(sh "$SKILL_DIR/scripts/prepare-edit.sh" "$TMP/includes.shortcut" "$TMP/prepared" 2>&1)
printf '%s\n' "$PREPARE_OUT" | grep -q "$TMP/prepared/original/shortcut.plist"
printf '%s\n' "$PREPARE_OUT" | grep -q "$TMP/prepared/source/.*\.cherri"
printf '%s\n' "$PREPARE_OUT" | grep -q "$TMP/prepared/builds/validation.shortcut"
[ -s "$TMP/prepared/builds/validation.shortcut" ]

# AEA1 signed input must be rejected with a clear early diagnostic.
printf 'Testing AEA1 signed-input rejection...\n'
if [ "$SIGN" -eq 1 ]; then
  sh "$SKILL_DIR/scripts/build.sh" "$TMP/test.cherri" "$TMP/test-signed.shortcut" --signed >/dev/null
  [ "$(dd if="$TMP/test-signed.shortcut" bs=1 count=4 2>/dev/null)" = "AEA1" ]
  if sh "$SKILL_DIR/scripts/prepare-edit.sh" "$TMP/test-signed.shortcut" "$TMP/prepared-aea1" >/dev/null 2>&1; then
    echo 'prepare-edit accepted AEA1 input unexpectedly' >&2
    exit 1
  fi
fi

echo 'Cherri Shortcuts skill self-test passed.'
