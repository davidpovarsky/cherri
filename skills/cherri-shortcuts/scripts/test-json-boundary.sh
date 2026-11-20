#!/bin/sh
set -eu
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
. "$SCRIPT_DIR/common.sh"

# Execute the actual portable parser with original byte files. These are
# explicitly synthetic parser controls, not compiler or installed-package proof.
CONTROL_DIR=$(umask 077; mktemp -d "${TMPDIR:-/tmp}/cherri-json-controls.XXXXXX")
trap 'rm -rf "$CONTROL_DIR"' 0
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM
CONTROL_FILE="$CONTROL_DIR/value.json"
CONTROL_COUNT=0

reject_control() {
  if json_file_is_valid_object "$CONTROL_FILE"; then
    echo "[CHERRI_JSON_CONTROL_FAILED] Accepted $1" >&2
    exit 1
  fi
  CONTROL_COUNT=$((CONTROL_COUNT + 1))
}

printf '%s' '{"field":"עברית🙂","empty":"","nested":[true,false,null,0,-2.5e+3,{"text":"line\nbreak","astral":"\ud83d\ude42"}]}' > "$CONTROL_FILE"
json_file_is_valid_object "$CONTROL_FILE"
[ "$(json_parse_object field < "$CONTROL_FILE")" = 'עברית🙂' ]
json_parse_object empty exists < "$CONTROL_FILE"
[ -z "$(json_parse_object empty < "$CONTROL_FILE")" ]
CONTROL_COUNT=$((CONTROL_COUNT + 1))

printf '{"field":"valid"}\000' > "$CONTROL_FILE"
reject_control raw_nul_after_object
printf '{"field":"a\000b"}' > "$CONTROL_FILE"
reject_control raw_nul_in_string
printf '{"field":"\377"}' > "$CONTROL_FILE"
reject_control invalid_utf8
printf '%s' '{"field":1,"\u0066ield":2}' > "$CONTROL_FILE"
reject_control escaped_duplicate_key
printf '%s' '{"field":"value" "other":0}' > "$CONTROL_FILE"
reject_control missing_comma
printf '%s' '{"field":01}' > "$CONTROL_FILE"
reject_control invalid_number
printf '%s' '{"field":"\ud800"}' > "$CONTROL_FILE"
reject_control unpaired_surrogate
printf '%s' '{"field":"one"}{"field":"two"}' > "$CONTROL_FILE"
reject_control concatenated_objects
printf '%s' '{"field":"\x"}' > "$CONTROL_FILE"
reject_control invalid_escape

printf 'CHERRI_JSON_BOUNDARY_CONTROLS pass=%s scope=synthetic_parser_only\n' "$CONTROL_COUNT"
