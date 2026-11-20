#!/bin/sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
SKILL_DIR=$(CDPATH= cd -- "${SKILL_DIR:-$SCRIPT_DIR/..}" && pwd)
MANIFEST_FILE="${CHERRI_MANIFEST_FILE:-$SKILL_DIR/compatibility-manifest.json}"

# Parse the original bytes before shell command substitution can remove NULs.
# The parser accepts an object root, exact top-level keys, and strict UTF-8.
json_parse_object() {
  LC_ALL=C awk -v wanted="${1:-}" -v mode="${2:-}" -f "$SKILL_DIR/scripts/json-object.awk"
}

# Some awk implementations treat NUL as an input terminator. Reject those bytes
# independently before parsing, and cap the original file as well as the parser.
json_file_is_valid_object() {
  _json_file=$1
  _json_size=$(wc -c < "$_json_file") || return 1
  [ "$_json_size" -le 8388608 ] || return 1
  _json_without_nul=$(LC_ALL=C tr -d '\000' < "$_json_file" | wc -c) || return 1
  [ "$_json_size" -eq "$_json_without_nul" ] || return 1
  json_parse_object < "$_json_file"
}

json_extract_string() {
  printf '%s' "$1" | json_parse_object "$2"
}

json_is_valid_object() {
  printf '%s' "$1" | json_parse_object
}

json_has_field() {
  printf '%s' "$1" | json_parse_object "$2" exists
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
  if ! json_file_is_valid_object "$MANIFEST_FILE"; then
    echo "Compatibility manifest is not valid JSON: $MANIFEST_FILE" >&2
    return 1
  fi
  MANIFEST_RAW=$(cat "$MANIFEST_FILE")

  MANIFEST_LANG_VERSION=$(json_extract_string "$MANIFEST_RAW" "languageVersion")
  MANIFEST_CATALOG_VERSION=$(json_extract_string "$MANIFEST_RAW" "catalogSchemaVersion")
  MANIFEST_PROTOCOL_VERSION=$(json_extract_string "$MANIFEST_RAW" "protocolVersion")
  MANIFEST_SCHEMA_FP=$(json_extract_string "$MANIFEST_RAW" "schemaFingerprint")
  MANIFEST_CODEC_FP=$(json_extract_string "$MANIFEST_RAW" "codecFingerprint")
  MANIFEST_MACRO_FP=$(json_extract_string "$MANIFEST_RAW" "macroFingerprint")
  MANIFEST_COMPILER_REPO=$(json_extract_string "$MANIFEST_RAW" "compilerRepo")
  MANIFEST_COMPILER_COMMIT=$(json_extract_string "$MANIFEST_RAW" "compilerCommit")
  MANIFEST_DOCS_REPO=$(json_extract_string "$MANIFEST_RAW" "docsRepo")
  MANIFEST_DOCS_COMMIT=$(json_extract_string "$MANIFEST_RAW" "docsCommit")

  if [ -z "$MANIFEST_LANG_VERSION" ]; then
    MANIFEST_LANG_VERSION="2.0"
  fi
  # Manifests from protocol 1.0 predate its explicit field. Their compiler must
  # still expose protocol 1.0; a declared empty/non-string value is never ignored.
  if ! json_has_field "$MANIFEST_RAW" protocolVersion; then
    MANIFEST_PROTOCOL_VERSION="1.0"
  elif [ -z "$MANIFEST_PROTOCOL_VERSION" ]; then
    echo "Compatibility manifest has missing or invalid protocolVersion" >&2
    return 1
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
  # Old releases had no separate compiler macro contract. Once declared, it is
  # mandatory and cannot silently degrade to the native action fingerprint.
  if json_has_field "$MANIFEST_RAW" macroFingerprint; then
    if ! is_hex_len "$MANIFEST_MACRO_FP" 64; then
      echo "[CHERRI_MACRO_MANIFEST_INVALID] Compatibility manifest has missing or invalid macroFingerprint" >&2
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

  export MANIFEST_LANG_VERSION MANIFEST_CATALOG_VERSION MANIFEST_PROTOCOL_VERSION MANIFEST_SCHEMA_FP MANIFEST_CODEC_FP MANIFEST_MACRO_FP
  export MANIFEST_COMPILER_REPO MANIFEST_COMPILER_COMMIT MANIFEST_DOCS_REPO MANIFEST_DOCS_COMMIT
  return 0
}

load_compatibility_manifest

if [ -z "${CHERRI_HOME:-}" ]; then
  if [ -d "/var/minis" ] && [ -w "/var/minis" ]; then
    CHERRI_HOME="/var/minis/cherri"
  elif mkdir -p /var/minis 2>/dev/null; then
    CHERRI_HOME="/var/minis/cherri"
  else
    CHERRI_HOME="${HOME:-/tmp}/.cherri"
  fi
fi
: "${CHERRI_REPO:=${MANIFEST_COMPILER_REPO:-https://github.com/davidpovarsky/cherri.git}}"
: "${CHERRI_REF:=$MANIFEST_COMPILER_COMMIT}"
: "${CHERRI_DOCS_REPO:=${MANIFEST_DOCS_REPO:-https://github.com/davidpovarsky/cherrilang.org.git}}"
: "${CHERRI_DOCS_REF:=$MANIFEST_DOCS_COMMIT}"
: "${CHERRI_DOCS_DIR:=$CHERRI_HOME/docs}"

BUNDLED_CHERRI="$SKILL_DIR/bin/cherri"
CACHED_CHERRI="$CHERRI_HOME/bin/cherri"

cherri_file_sha256() {
  _sum_path=$1
  if command -v sha256sum >/dev/null 2>&1; then
    _sum_value=$(sha256sum "$_sum_path" | awk '{print $1}')
  elif command -v shasum >/dev/null 2>&1; then
    _sum_value=$(shasum -a 256 "$_sum_path" | awk '{print $1}')
  elif command -v openssl >/dev/null 2>&1; then
    _sum_value=$(openssl dgst -sha256 "$_sum_path" | awk '{print $NF}')
  else
    echo "[CHERRI_CHECKSUM_UNAVAILABLE] Need sha256sum, shasum, or openssl to verify compiler bytes." >&2
    return 1
  fi
  if ! is_hex_len "$_sum_value" 64; then
    echo "[CHERRI_CHECKSUM_INVALID] Could not hash compiler bytes: $_sum_path" >&2
    return 1
  fi
  printf '%s\n' "$_sum_value" | tr 'A-F' 'a-f'
}

verify_cherri_checksum() {
  _verify_path=$1
  _verify_expected=$(printf '%s' "$2" | tr 'A-F' 'a-f')
  if ! is_hex_len "$_verify_expected" 64; then
    echo "[CHERRI_CHECKSUM_INVALID] Expected a 64-digit compiler SHA-256." >&2
    return 1
  fi
  _verify_actual=$(cherri_file_sha256 "$_verify_path") || return 1
  if [ "$_verify_actual" != "$_verify_expected" ]; then
    echo "[CHERRI_CHECKSUM_MISMATCH] Compiler bytes differ from the expected SHA-256: $_verify_path" >&2
    return 1
  fi
}

validate_cherri_bin() (
  _cand=$1
  if [ -z "$_cand" ] || [ ! -f "$_cand" ]; then
    echo "[CHERRI_COMPILER_NOT_FOUND] Cherri binary not found: ${_cand:-<empty>}" >&2
    return 1
  fi
  if [ -f "$_cand.sha256" ]; then
    verify_cherri_checksum "$_cand" "$(tr -d '\r\n ' < "$_cand.sha256")" || return 1
  fi
  chmod +x "$_cand" 2>/dev/null || true
  if [ ! -x "$_cand" ]; then
    echo "Cherri binary is not executable: $_cand" >&2
    return 1
  fi

  _cap_file=$(umask 077; mktemp "${TMPDIR:-/tmp}/cherri-capabilities.XXXXXX") || {
    echo "Cannot create a private capabilities validation file" >&2
    return 1
  }
  trap 'rm -f "$_cap_file"' 0
  trap 'exit 129' HUP
  trap 'exit 130' INT
  trap 'exit 143' TERM
  if ! "$_cand" --capabilities-json > "$_cap_file" 2>/dev/null; then
    echo "Cherri binary failed --capabilities-json: $_cand" >&2
    return 1
  fi
  if ! json_file_is_valid_object "$_cap_file"; then
    echo "Cherri binary returned malformed JSON from --capabilities-json: $_cand" >&2
    return 1
  fi

  _actual_lang=$(json_parse_object languageVersion < "$_cap_file")
  _actual_cat=$(json_parse_object catalogSchemaVersion < "$_cap_file")
  _actual_protocol=$(json_parse_object protocolVersion < "$_cap_file")
  _actual_fp=$(json_parse_object schemaFingerprint < "$_cap_file")
  _actual_codec_fp=$(json_parse_object codecFingerprint < "$_cap_file")
  _actual_macro_fp=$(json_parse_object macroFingerprint < "$_cap_file")

  if [ "$_actual_lang" != "$MANIFEST_LANG_VERSION" ]; then
    echo "Cherri languageVersion mismatch in $_cand: expected $MANIFEST_LANG_VERSION, got ${_actual_lang:-<missing>}" >&2
    return 1
  fi
  if [ -n "$MANIFEST_CATALOG_VERSION" ] && [ "$_actual_cat" != "$MANIFEST_CATALOG_VERSION" ]; then
    echo "Cherri catalogSchemaVersion mismatch in $_cand: expected $MANIFEST_CATALOG_VERSION, got ${_actual_cat:-<missing>}" >&2
    return 1
  fi
  if [ "$_actual_protocol" != "$MANIFEST_PROTOCOL_VERSION" ]; then
    echo "Cherri protocolVersion mismatch in $_cand: expected $MANIFEST_PROTOCOL_VERSION, got ${_actual_protocol:-<missing>}" >&2
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
  if [ -n "$MANIFEST_CODEC_FP" ] && [ -z "$_actual_codec_fp" ]; then
    echo "Cherri binary returned missing codecFingerprint in $_cand" >&2
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

  if [ -n "$MANIFEST_MACRO_FP" ]; then
    if [ -z "$_actual_macro_fp" ]; then
      echo "[CHERRI_MACRO_FINGERPRINT_MISSING] Cherri binary does not expose the required macroFingerprint" >&2
      return 1
    fi
    if ! is_hex_len "$_actual_macro_fp" 64; then
      echo "[CHERRI_MACRO_FINGERPRINT_INVALID] Cherri binary returned a malformed macroFingerprint" >&2
      return 1
    fi
    if [ "$_actual_macro_fp" != "$MANIFEST_MACRO_FP" ]; then
      echo "[CHERRI_MACRO_FINGERPRINT_MISMATCH] Cherri macroFingerprint mismatch: expected $MANIFEST_MACRO_FP, got $_actual_macro_fp" >&2
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
)

validate_docs_dir() {
  _docs=$1
  if [ -z "$_docs" ] || [ ! -d "$_docs" ]; then
    echo "[CHERRI_DOCS_MISSING] Cherri docs directory missing: ${_docs:-<empty>}" >&2
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
    echo "[CHERRI_STALE_DOCS_SCHEMA] Cherri docs Schema Fingerprint mismatch: expected $MANIFEST_SCHEMA_FP, got $_docs_fp" >&2
    return 1
  fi

  if [ -n "$MANIFEST_MACRO_FP" ]; then
    _docs_macro_fp=$(sed -n 's/.*\*\*Compiler Macro Fingerprint:\*\*[[:space:]]*`\([^`]*\)`.*/\1/p' "$_actions_md" | tr -d '\r' | head -n 1)
    if [ -z "$_docs_macro_fp" ]; then
      echo "[CHERRI_DOCS_MACRO_FINGERPRINT_MISSING] Cherri docs do not expose the required Compiler Macro Fingerprint" >&2
      return 1
    fi
    if ! is_hex_len "$_docs_macro_fp" 64; then
      echo "[CHERRI_DOCS_MACRO_FINGERPRINT_INVALID] Cherri docs have a malformed Compiler Macro Fingerprint" >&2
      return 1
    fi
    if [ "$_docs_macro_fp" != "$MANIFEST_MACRO_FP" ]; then
      echo "[CHERRI_STALE_DOCS_MACROS] Cherri docs Compiler Macro Fingerprint mismatch: expected $MANIFEST_MACRO_FP, got $_docs_macro_fp" >&2
      return 1
    fi
  fi

  # Linked worktrees have a .git file; their pins need the same check as clones.
  if [ -e "$_docs/.git" ]; then
    if ! command -v git >/dev/null 2>&1; then
      echo "[CHERRI_DOCS_GIT_UNAVAILABLE] Cannot validate Cherri docs commit without git" >&2
      return 1
    fi
    if ! _docs_head=$(git -C "$_docs" rev-parse HEAD 2>/dev/null); then
      echo "[CHERRI_DOCS_GIT_INVALID] Cannot resolve Cherri docs commit in $_docs" >&2
      return 1
    fi
    if ! is_hex_len "$_docs_head" 40; then
      echo "[CHERRI_DOCS_GIT_INVALID] Cherri docs returned an invalid commit in $_docs" >&2
      return 1
    fi
    if is_hex_len "$CHERRI_DOCS_REF" 40 && [ "$_docs_head" != "$CHERRI_DOCS_REF" ]; then
      echo "[CHERRI_STALE_DOCS_COMMIT] Cherri docs commit mismatch in $_docs: expected $CHERRI_DOCS_REF, got $_docs_head" >&2
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
