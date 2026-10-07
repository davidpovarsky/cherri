#!/bin/sh
# Print Cherri's machine-readable action catalog (cherri --actions-json).
# Optional argument: grep pattern to filter entries, e.g.
#   sh scripts/catalog.sh '"shortcutIdentifier": "is.workflow.actions.alert"'
set -eu
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
. "$SCRIPT_DIR/common.sh"

ensure_cherri

PATTERN=${1:-}
if [ -z "$PATTERN" ]; then
  exec "$CHERRI_BIN_RESOLVED" --actions-json
fi

"$CHERRI_BIN_RESOLVED" --actions-json | tr ',' '\n' | grep -F -- "$PATTERN" || true
