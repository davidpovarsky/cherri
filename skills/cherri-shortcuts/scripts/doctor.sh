#!/bin/sh
set -eu
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
. "$SCRIPT_DIR/common.sh"

status=0
printf 'Platform: %s %s\n' "$(uname -s 2>/dev/null || echo unknown)" "$(uname -m 2>/dev/null || echo unknown)"
printf 'Skill dir: %s\n' "$SKILL_DIR"
printf 'Pinned compiler commit: %s\n' "$MANIFEST_COMPILER_COMMIT"
printf 'Pinned docs commit: %s\n' "$MANIFEST_DOCS_COMMIT"

if BIN=$(resolve_cherri); then
  printf 'Cherri: %s\n' "$BIN"
  "$BIN" --version || status=1
  if validate_cherri_bin "$BIN"; then
    printf 'Cherri Language: v%s ready\n' "$MANIFEST_LANG_VERSION"
    printf 'Schema fingerprint: %s (verified)\n' "$MANIFEST_SCHEMA_FP"
    if [ -n "$MANIFEST_CODEC_FP" ]; then
      printf 'Codec fingerprint: %s (verified)\n' "$MANIFEST_CODEC_FP"
    fi
  else
    status=1
  fi
else
  echo 'Cherri: MISSING or INCOMPATIBLE (run scripts/setup.sh)' >&2
  status=1
fi

if [ -d "$CHERRI_DOCS_DIR" ]; then
  if validate_docs_dir "$CHERRI_DOCS_DIR"; then
    printf 'Docs: %s (verified)\n' "$CHERRI_DOCS_DIR"
  else
    status=1
  fi
else
  echo 'Docs: MISSING (run scripts/setup.sh)' >&2
  status=1
fi

for cmd in sh grep sed awk; do
  if command -v "$cmd" >/dev/null 2>&1; then
    printf '%s: ok\n' "$cmd"
  else
    printf '%s: missing\n' "$cmd" >&2
    status=1
  fi
done

exit "$status"
