#!/bin/sh
set -eu
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
. "$SCRIPT_DIR/common.sh"

status=0
printf 'Platform: %s %s\n' "$(uname -s 2>/dev/null || echo unknown)" "$(uname -m 2>/dev/null || echo unknown)"
printf 'Skill dir: %s\n' "$SKILL_DIR"

if BIN=$(resolve_cherri); then
  printf 'Cherri: %s\n' "$BIN"
  "$BIN" --version || status=1
  if CAP_OUTPUT=$("$BIN" --capabilities-json 2>&1); then
    if printf '%s\n' "$CAP_OUTPUT" | grep -q '"languageVersion":"2.0"'; then
      printf 'Cherri Language: v2.0 ready\n'
      MANIFEST="$SKILL_DIR/compatibility-manifest.json"
      if [ -f "$MANIFEST" ]; then
        EXPECTED_FP=$(grep '"schemaFingerprint"' "$MANIFEST" | sed 's/.*: *"\([^"]*\)".*/\1/')
        if [ -n "$EXPECTED_FP" ]; then
          ACTUAL_FP=$(printf '%s\n' "$CAP_OUTPUT" | grep '"schemaFingerprint"' | sed 's/.*: *"\([^"]*\)".*/\1/')
          if [ -n "$ACTUAL_FP" ] && [ "$EXPECTED_FP" != "$ACTUAL_FP" ]; then
            printf 'Cherri: Schema fingerprint mismatch (expected %s, got %s)\n' "$EXPECTED_FP" "$ACTUAL_FP" >&2
            status=1
          else
            printf 'Schema fingerprint: %s (verified)\n' "$EXPECTED_FP"
          fi
        fi
      fi
    else
      echo 'Cherri: Incompatible compiler (missing languageVersion 2.0)' >&2
      status=1
    fi
  else
    echo 'Cherri: Incompatible compiler (lacks --capabilities-json)' >&2
    status=1
  fi
else
  echo 'Cherri: MISSING (run scripts/setup.sh)'
  status=1
fi

if [ -d "$CHERRI_DOCS_DIR" ]; then
  printf 'Docs: %s\n' "$CHERRI_DOCS_DIR"
else
  echo 'Docs: MISSING (setup.sh will clone upstream docs when git is available)'
fi

for cmd in sh grep sed awk; do
  if command -v "$cmd" >/dev/null 2>&1; then
    printf '%s: ok\n' "$cmd"
  else
    printf '%s: missing\n' "$cmd"
    status=1
  fi
done

exit "$status"
