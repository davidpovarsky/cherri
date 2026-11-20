#!/bin/sh
# Sourced after common.sh. Public compile wrappers may use a development
# compiler; doctor and setup require the complete immutable installed tool pair.

cherri_migration_for_compiler() {
  if [ -n "${CHERRI_MIGRATE_BIN:-}" ]; then
    printf '%s\n' "$CHERRI_MIGRATE_BIN"
  else
    printf '%s/cherri-migrate\n' "$(dirname -- "$1")"
  fi
}

require_install_tool_receipts() (
  _tool_path=$1
  _tool_name=$2
  if [ ! -f "$_tool_path.commit" ] || [ ! -f "$_tool_path.sha256" ]; then
    echo "[CHERRI_TOOL_RECEIPT_MISSING] Installed $_tool_name needs source and SHA-256 receipts: $_tool_path" >&2
    return 1
  fi
  _tool_source=$(tr -d '\r\n ' < "$_tool_path.commit")
  if [ "$_tool_source" != "$CHERRI_REF" ]; then
    echo "[CHERRI_TOOL_SOURCE_MISMATCH] Installed $_tool_name source revision receipt differs from $CHERRI_REF" >&2
    return 1
  fi
  verify_cherri_checksum "$_tool_path" "$(tr -d '\r\n ' < "$_tool_path.sha256")"
)

validate_cherri_migration_bin() (
  _migration=$1
  if [ ! -f "$_migration" ]; then
    echo "[CHERRI_MIGRATOR_NOT_FOUND] Installed migration tool is missing: $_migration" >&2
    return 1
  fi
  require_install_tool_receipts "$_migration" migration || return 1
  validate_cherri_bin "$_migration" || return 1
  _identity_file=$(umask 077; mktemp "${TMPDIR:-/tmp}/cherri-migration-identity.XXXXXX") || return 1
  trap 'rm -f "$_identity_file"' 0
  trap 'exit 129' HUP
  trap 'exit 130' INT
  trap 'exit 143' TERM
  if ! "$_migration" --capabilities-json > "$_identity_file" 2>/dev/null || ! json_file_is_valid_object "$_identity_file"; then
    echo "[CHERRI_TOOL_IDENTITY_INVALID] Cannot read migration tool role and build identity" >&2
    return 1
  fi
  if [ "$(json_parse_object tool < "$_identity_file")" != "cherri-migrate" ] ||
     [ "$(json_parse_object modulePath < "$_identity_file")" != "github.com/electrikmilk/cherri/cmd/cherri-migrate" ]; then
    echo "[CHERRI_TOOL_ROLE_MISMATCH] Installed migration tool role/main package is not cherri-migrate" >&2
    return 1
  fi
  if [ "$(json_parse_object sourceCommit < "$_identity_file")" != "$CHERRI_REF" ] ||
     [ "$(json_parse_object buildState < "$_identity_file")" != "clean" ]; then
    echo "[CHERRI_TOOL_SOURCE_MISMATCH] Installed migration tool source revision is not the exact clean $CHERRI_REF" >&2
    return 1
  fi
)

validate_cherri_installation() (
  _compiler=$1
  _migration=$2
  # Preserve the precise capability diagnostics before inspecting source/role.
  validate_cherri_bin "$_compiler" || return 1
  require_install_tool_receipts "$_compiler" compiler || return 1
  validate_cherri_migration_bin "$_migration" || return 1
  _identity_file=$(umask 077; mktemp "${TMPDIR:-/tmp}/cherri-compiler-identity.XXXXXX") || return 1
  trap 'rm -f "$_identity_file"' 0
  trap 'exit 129' HUP
  trap 'exit 130' INT
  trap 'exit 143' TERM
  if ! "$_migration" --inspect-build-json "$_compiler" > "$_identity_file" 2>/dev/null || ! json_file_is_valid_object "$_identity_file"; then
    echo "[CHERRI_TOOL_IDENTITY_INVALID] Cannot inspect compiler tool role and embedded clean source revision" >&2
    return 1
  fi
  if [ "$(json_parse_object tool < "$_identity_file")" != "cherri" ] ||
     [ "$(json_parse_object modulePath < "$_identity_file")" != "github.com/electrikmilk/cherri" ]; then
    echo "[CHERRI_TOOL_ROLE_MISMATCH] Installed compiler tool role/main package is not cherri" >&2
    return 1
  fi
  if [ "$(json_parse_object sourceCommit < "$_identity_file")" != "$CHERRI_REF" ] ||
     [ "$(json_parse_object buildState < "$_identity_file")" != "clean" ]; then
    echo "[CHERRI_TOOL_SOURCE_MISMATCH] Installed compiler source revision is not the exact clean $CHERRI_REF" >&2
    return 1
  fi
  _identity_sha=$(json_parse_object sha256 < "$_identity_file")
  _receipt_sha=$(tr -d '\r\n ' < "$_compiler.sha256" | tr 'A-F' 'a-f')
  if [ "$_identity_sha" != "$_receipt_sha" ]; then
    echo "[CHERRI_TOOL_IDENTITY_INVALID] Inspected compiler bytes differ from their SHA-256 receipt" >&2
    return 1
  fi
)

resolve_cherri_installation() {
  if [ -n "${CHERRI_BIN:-}" ]; then
    _explicit_migration=$(cherri_migration_for_compiler "$CHERRI_BIN")
    if validate_cherri_installation "$CHERRI_BIN" "$_explicit_migration"; then
      printf '%s\n' "$CHERRI_BIN"
      return 0
    fi
    echo "Explicit Cherri installation is incomplete or incompatible: $CHERRI_BIN" >&2
    return 1
  fi
  for _installed_candidate in "$CACHED_CHERRI" "$BUNDLED_CHERRI"; do
    if [ -f "$_installed_candidate" ]; then
      _installed_migration=$(cherri_migration_for_compiler "$_installed_candidate")
      if validate_cherri_installation "$_installed_candidate" "$_installed_migration" 2>/dev/null; then
        printf '%s\n' "$_installed_candidate"
        return 0
      fi
    fi
  done
  if command -v cherri >/dev/null 2>&1; then
    _installed_candidate=$(command -v cherri)
    _installed_migration=$(cherri_migration_for_compiler "$_installed_candidate")
    if validate_cherri_installation "$_installed_candidate" "$_installed_migration" 2>/dev/null; then
      printf '%s\n' "$_installed_candidate"
      return 0
    fi
  fi
  return 1
}
