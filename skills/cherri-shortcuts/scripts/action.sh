#!/bin/sh
set -eu
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
. "$SCRIPT_DIR/common.sh"
ensure_cherri
QUERY=${*:-}

# Cherri intentionally exits 1 after non-empty action searches, even when it
# successfully prints an exact or fuzzy match. Treat 0 and 1 as successful
# search outcomes; reserve larger exit codes for an actual execution failure.
set +e
OUTPUT=$("$CHERRI_BIN_RESOLVED" --action="$QUERY" --no-ansi 2>&1)
STATUS=$?
set -e
printf '%s\n' "$OUTPUT"
[ "$STATUS" -le 1 ] || exit "$STATUS"
