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

TMP=${TMPDIR:-/tmp}/cherri-skill-test-$$
mkdir -p "$TMP/decompiled"
trap 'rm -rf "$TMP"' EXIT INT TERM
cat > "$TMP/test.cherri" <<'SRC'
show("Cherri Open Minis test")
SRC

printf 'Testing unsigned compile wrapper...\n'
sh "$SKILL_DIR/scripts/build.sh" "$TMP/test.cherri" "$TMP/test.shortcut" --unsigned >/dev/null
[ -s "$TMP/test.shortcut" ]

printf 'Testing decompile wrapper...\n'
sh "$SKILL_DIR/scripts/decompile.sh" "$TMP/test.shortcut" "$TMP/decompiled" >/dev/null
find "$TMP/decompiled" -maxdepth 1 -type f -name '*.cherri' | grep -q .

if [ "$SIGN" -eq 1 ]; then
  printf 'Testing HubSign...\n'
  sh "$SKILL_DIR/scripts/build.sh" "$TMP/test.cherri" "$TMP/test-signed.shortcut" --signed >/dev/null
  [ "$(dd if="$TMP/test-signed.shortcut" bs=1 count=4 2>/dev/null)" = "AEA1" ]
fi

echo 'Cherri Shortcuts skill self-test passed.'
