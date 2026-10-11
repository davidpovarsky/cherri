#!/bin/sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
SKILL_DIR=$(CDPATH= cd -- "${SKILL_DIR:-$SCRIPT_DIR/..}" && pwd)
MANIFEST_FILE="${CHERRI_MANIFEST_FILE:-$SKILL_DIR/compatibility-manifest.json}"

# Extract a top-level string field from a flat JSON object after verifying JSON framing.
json_extract_string() {
  _json_text=$1
  _field_key=$2
  printf '%s\n' "$_json_text" | tr -d '\r' | sed -n 's/.*"'"$_field_key"'"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n 1
}

# Verify that a string is a syntactically well-formed JSON object with balanced outer braces and no unclosed quotes.
json_is_valid_object() {
  _raw=$(printf '%s' "$1" | tr -d '\r\n\t ')
  case "$_raw" in
    \{*\}) ;;
    *) return 1 ;;
  esac
  # Reject malformed key/value syntax such as trailing garbage or unquoted keys
  printf '%s\n' "$1" | awk '
    BEGIN { in_str=0; esc=0; depth=0 }
    {
      for (i=1; i<=length($0); i++) {
        c = substr($0, i, 1)
        if (esc) { esc=0; continue }
        if (c == "\\") { if (in_str) esc=1; continue }
        if (c == "\"") { in_str = !in_str; continue }
        if (!in_str) {
          if (c == "{") depth++
          else if (c == "}") depth--
        }
      }
    }
    END {
      if (in_str != 0 || depth != 0) exit 1
    }
  '
}

is_hex_len() {
  _val=$1
  _len=$2
  [ "${#_val}" -eq "$_len" ] || return 1
  case "$_val" in
    *[!0-9a-fA-F]*|"") return 1 ;;
    *) return 0 ;;
  esac
}

load_compatibility_manifest() {
  if [ ! -f "$MANIFEST_FILE" ]; then
    echo "Compatibility manifest missing: $MANIFEST_FILE" >&2
    return 1
  fi
  MANIFEST_RAW=$(cat "$MANIFEST_FILE")
  if ! json_is_valid_object "$MANIFEST_RAW"; then
    echo "Compatibility manifest is not valid JSON: $MANIFEST_FILE" >&2
    return 1
  fi

  MANIFEST_LANG_VERSION=$(json_extract_string "$MANIFEST_RAW" "languageVersion")
  MANIFEST_CATALOG_VERSION=$(json_extract_string "$MANIFEST_RAW" "catalogSchemaVersion")
  MANIFEST_SCHEMA_FP=$(json_extract_string "$MANIFEST_RAW" "schemaFingerprint")
  MANIFEST_CODEC_FP=$(json_extract_string "$MANIFEST_RAW" "codecFingerprint")
  MANIFEST_COMPILER_REPO=$(json_extract_string "$MANIFEST_RAW" "compilerRepo")
  MANIFEST_COMPILER_COMMIT=$(json_extract_string "$MANIFEST_RAW" "compilerCommit")
  MANIFEST_DOCS_REPO=$(json_extract_string "$MANIFEST_RAW" "docsRepo")
  MANIFEST_DOCS_COMMIT=$(json_extract_string "$MANIFEST_RAW" "docsCommit")

  if [ -z "$MANIFEST_LANG_VERSION" ]; then
    MANIFEST_LANG_VERSION="2.0"
  fi
  if [ -z "$MANIFEST_SCHEMA_FP" ] || ! is_hex_len "$MANIFEST_SCHEMA_FP" 64; then
    echo "Compatibility manifest has missing or invalid schemaFingerprint: ${MANIFEST_SCHEMA_FP:-<empty>}" >&2
    return 1
  fi
  if [ -n "$MANIFEST_CODEC_FP" ]; then
    if ! is_hex_len "$MANIFEST_CODEC_FP" 64; then
      echo "Compatibility manifest has invalid codecFingerprint: $MANIFEST_CODEC_FP" >&2
      return 1
    fi
    if [ "$MANIFEST_CODEC_FP" = "$MANIFEST_SCHEMA_FP" ]; then
      echo "Compatibility manifest codecFingerprint must not duplicate schemaFingerprint" >&2
      return 1
    fi
  fi
  if [ -z "$MANIFEST_COMPILER_COMMIT" ] || ! is_hex_len "$MANIFEST_COMPILER_COMMIT" 40; then
    echo "Compatibility manifest has missing or non-immutable compilerCommit: ${MANIFEST_COMPILER_COMMIT:-<empty>}" >&2
    return 1
  fi
  if [ -z "$MANIFEST_DOCS_COMMIT" ] || ! is_hex_len "$MANIFEST_DOCS_COMMIT" 40; then
    echo "Compatibility manifest has missing or non-immutable docsCommit: ${MANIFEST_DOCS_COMMIT:-<empty>}" >&2
    return 1
  fi

  export MANIFEST_LANG_VERSION MANIFEST_CATALOG_VERSION MANIFEST_SCHEMA_FP MANIFEST_CODEC_FP
  export MANIFEST_COMPILER_REPO MANIFEST_COMPILER_COMMIT MANIFEST_DOCS_REPO MANIFEST_DOCS_COMMIT
  return 0
}

load_compatibility_manifest

: "${CHERRI_HOME:=/var/minis/cherri}"
: "${CHERRI_REPO:=${MANIFEST_COMPILER_REPO:-https://github.com/davidpovarsky/cherri.git}}"
: "${CHERRI_REF:=$MANIFEST_COMPILER_COMMIT}"
: "${CHERRI_DOCS_REPO:=${MANIFEST_DOCS_REPO:-https://github.com/davidpovarsky/cherrilang.org.git}}"
: "${CHERRI_DOCS_REF:=$MANIFEST_DOCS_COMMIT}"
: "${CHERRI_DOCS_DIR:=$CHERRI_HOME/docs}"

BUNDLED_CHERRI="$SKILL_DIR/bin/cherri"
CACHED_CHERRI="$CHERRI_HOME/bin/cherri"

validate_cherri_bin() {
  _cand=$1
  if [ -z "$_cand" ] || [ ! -f "$_cand" ]; then
    echo "Cherri binary not found: ${_cand:-<empty>}" >&2
    return 1
  fi
  chmod +x "$_cand" 2>/dev/null || true
  if [ ! -x "$_cand" ]; then
    echo "Cherri binary is not executable: $_cand" >&2
    return 1
  fi

  if ! _cap_out=$("$_cand" --capabilities-json 2>/dev/null); then
    echo "Cherri binary failed --capabilities-json: $_cand" >&2
    return 1
  fi
  if [ -z "$_cap_out" ] || ! json_is_valid_object "$_cap_out"; then
    echo "Cherri binary returned malformed JSON from --capabilities-json: $_cand" >&2
    return 1
  fi

  _actual_lang=$(json_extract_string "$_cap_out" "languageVersion")
  _actual_cat=$(json_extract_string "$_cap_out" "catalogSchemaVersion")
  _actual_fp=$(json_extract_string "$_cap_out" "schemaFingerprint")
  _actual_codec_fp=$(json_extract_string "$_cap_out" "codecFingerprint")

  if [ "$_actual_lang" != "$MANIFEST_LANG_VERSION" ]; then
    echo "Cherri languageVersion mismatch in $_cand: expected $MANIFEST_LANG_VERSION, got ${_actual_lang:-<missing>}" >&2
    return 1
  fi
  if [ -n "$MANIFEST_CATALOG_VERSION" ] && [ -n "$_actual_cat" ] && [ "$_actual_cat" != "$MANIFEST_CATALOG_VERSION" ]; then
    echo "Cherri catalogSchemaVersion mismatch in $_cand: expected $MANIFEST_CATALOG_VERSION, got $_actual_cat" >&2
    return 1
  fi
  if [ -z "$_actual_fp" ] || ! is_hex_len "$_actual_fp" 64; then
    echo "Cherri binary returned missing or malformed schemaFingerprint in $_cand: ${_actual_fp:-<missing>}" >&2
    return 1
  fi
  if [ "$_actual_fp" != "$MANIFEST_SCHEMA_FP" ]; then
    echo "Cherri schemaFingerprint mismatch in $_cand: expected $MANIFEST_SCHEMA_FP, got $_actual_fp" >&2
    return 1
  fi
  if [ -n "$_actual_codec_fp" ]; then
    if ! is_hex_len "$_actual_codec_fp" 64 || [ "$_actual_codec_fp" = "$_actual_fp" ]; then
      echo "Cherri binary returned invalid or non-independent codecFingerprint in $_cand: $_actual_codec_fp" >&2
      return 1
    fi
    if [ -n "$MANIFEST_CODEC_FP" ] && [ "$_actual_codec_fp" != "$MANIFEST_CODEC_FP" ]; then
      echo "Cherri codecFingerprint mismatch in $_cand: expected $MANIFEST_CODEC_FP, got $_actual_codec_fp" >&2
      return 1
    fi
  fi

  if [ -f "$_cand.commit" ]; then
    _recorded_sha=$(tr -d '\r\n ' < "$_cand.commit")
    if [ -n "$_recorded_sha" ] && [ "$_recorded_sha" != "$CHERRI_REF" ]; then
      echo "Cherri binary commit mismatch in $_cand: expected $CHERRI_REF, got $_recorded_sha" >&2
      return 1
    fi
  fi

  return 0
}

validate_docs_dir() {
  _docs=$1
  if [ -z "$_docs" ] || [ ! -d "$_docs" ]; then
    echo "Cherri docs directory missing: ${_docs:-<empty>}" >&2
    return 1
  fi

  _actions_md=""
  if [ -f "$_docs/fork-language/actions.md" ]; then
    _actions_md="$_docs/fork-language/actions.md"
  elif [ -f "$_docs/docs/language-v2/actions-reference.md" ]; then
    _actions_md="$_docs/docs/language-v2/actions-reference.md"
  elif [ -f "$_docs/actions.md" ]; then
    _actions_md="$_docs/actions.md"
  else
    echo "Cherri docs missing actions reference in $_docs" >&2
    return 1
  fi

  _docs_fp=$(sed -n 's/.*\*\*Schema Fingerprint:\*\*[[:space:]]*`\([^`]*\)`.*/\1/p' "$_actions_md" | tr -d '\r' | head -n 1)
  if [ -z "$_docs_fp" ] || ! is_hex_len "$_docs_fp" 64; then
    echo "Cherri docs missing or malformed Schema Fingerprint in $_actions_md" >&2
    return 1
  fi
  if [ "$_docs_fp" != "$MANIFEST_SCHEMA_FP" ]; then
    echo "Cherri docs Schema Fingerprint mismatch: expected $MANIFEST_SCHEMA_FP, got $_docs_fp" >&2
    return 1
  fi

  if [ -d "$_docs/.git" ] && command -v git >/dev/null 2>&1; then
    _docs_head=$(git -C "$_docs" rev-parse HEAD 2>/dev/null || true)
    if is_hex_len "$CHERRI_DOCS_REF" 40 && [ -n "$_docs_head" ] && [ "$_docs_head" != "$CHERRI_DOCS_REF" ]; then
      echo "Cherri docs commit mismatch in $_docs: expected $CHERRI_DOCS_REF, got $_docs_head" >&2
      return 1
    fi
  fi

  return 0
}

resolve_cherri() {
  # 1. Explicit CHERRI_BIN override must be validated strictly; never fall back on failure.
  if [ -n "${CHERRI_BIN:-}" ]; then
    if validate_cherri_bin "$CHERRI_BIN"; then
      printf '%s\n' "$CHERRI_BIN"
      return 0
    fi
    echo "Explicit CHERRI_BIN is missing or incompatible: $CHERRI_BIN" >&2
    return 1
  fi

  # 2. Prefer a verified cached binary first so newly built compatible caches win over stale bundled binaries.
  if [ -f "$CACHED_CHERRI" ]; then
    if validate_cherri_bin "$CACHED_CHERRI" 2>/dev/null; then
      printf '%s\n' "$CACHED_CHERRI"
      return 0
    fi
  fi

  # 3. Check bundled binary if present and compatible.
  if [ -f "$BUNDLED_CHERRI" ]; then
    if validate_cherri_bin "$BUNDLED_CHERRI" 2>/dev/null; then
      printf '%s\n' "$BUNDLED_CHERRI"
      return 0
    fi
  fi

  # 4. Check PATH binary only if it passes full compatibility validation.
  if command -v cherri >/dev/null 2>&1; then
    _path_bin=$(command -v cherri)
    if validate_cherri_bin "$_path_bin" 2>/dev/null; then
      printf '%s\n' "$_path_bin"
      return 0
    fi
  fi

  return 1
}

ensure_cherri() {
  if [ -n "${CHERRI_BIN:-}" ]; then
    CHERRI_BIN_RESOLVED=$(resolve_cherri) || exit 1
    export CHERRI_BIN_RESOLVED
    return 0
  fi
  if CHERRI_BIN_RESOLVED=$(resolve_cherri); then
    export CHERRI_BIN_RESOLVED
    return 0
  fi
  sh "$SKILL_DIR/scripts/setup.sh"
  CHERRI_BIN_RESOLVED=$(resolve_cherri) || {
    echo "Cherri setup completed but no compatible compiler was found." >&2
    exit 1
  }
  export CHERRI_BIN_RESOLVED
}
