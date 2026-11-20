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
  if ! "$_migration" --inspect-build-json "$_migration" > "$_identity_file" 2>/dev/null || ! json_file_is_valid_object "$_identity_file"; then
    echo "[CHERRI_TOOL_IDENTITY_INVALID] Cannot inspect migration tool role and embedded build identity" >&2
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
  _identity_sha=$(json_parse_object sha256 < "$_identity_file")
  _receipt_sha=$(tr -d '\r\n ' < "$_migration.sha256" | tr 'A-F' 'a-f')
  if [ "$_identity_sha" != "$_receipt_sha" ]; then
    echo "[CHERRI_TOOL_IDENTITY_INVALID] Inspected migration bytes differ from their SHA-256 receipt" >&2
    return 1
  fi
  if [ "$#" -gt 1 ]; then cat "$_identity_file" > "$2"; fi
)

validate_cherri_installation() (
  _compiler=$1
  _migration=$2
  if [ ! -f "$_compiler" ]; then
    echo "[CHERRI_COMPILER_NOT_FOUND] Cherri binary not found: $_compiler" >&2
    return 1
  fi
  # Authenticate the mandatory byte receipt before any installed code runs.
  require_install_tool_receipts "$_compiler" compiler || return 1
  validate_cherri_bin "$_compiler" || return 1
  _migration_identity_file=""
  _identity_file=$(umask 077; mktemp "${TMPDIR:-/tmp}/cherri-compiler-identity.XXXXXX") || return 1
  trap 'rm -f "$_identity_file"; if [ -n "$_migration_identity_file" ]; then rm -f "$_migration_identity_file"; fi' 0
  trap 'exit 129' HUP
  trap 'exit 130' INT
  trap 'exit 143' TERM
  _migration_identity_file=$(umask 077; mktemp "${TMPDIR:-/tmp}/cherri-pair-migration-identity.XXXXXX") || return 1
  validate_cherri_migration_bin "$_migration" "$_migration_identity_file" || return 1
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
  for _platform_key in goos goarch; do
    _compiler_platform=$(json_parse_object "$_platform_key" < "$_identity_file")
    _migration_platform=$(json_parse_object "$_platform_key" < "$_migration_identity_file")
    if [ -z "$_compiler_platform" ] || [ "$_compiler_platform" != "$_migration_platform" ]; then
      echo "[CHERRI_TOOL_PLATFORM_MISMATCH] Compiler and migration tool $_platform_key identities differ" >&2
      return 1
    fi
  done
)

validate_cherri_git_index() (
  _index_directory=$1
  _index_scope=$2
  if ! _index_entries=$(git --no-replace-objects -C "$_index_directory" ls-files -v); then
    echo "[CHERRI_${_index_scope}_GIT_INVALID] Cannot inspect tracked-file index flags" >&2
    return 1
  fi
  # With -v, lowercase flags mean assume-unchanged and S means skip-worktree.
  # Both can hide edited tracked bytes from status and Go's VCS stamp. Reject
  # this unverified state without changing the caller's index or local files.
  if ! printf '%s\n' "$_index_entries" | LC_ALL=C awk 'substr($0, 1, 1) ~ /^[a-zS]$/ { bad=1 } END { exit bad }'; then
    echo "[CHERRI_${_index_scope}_INDEX_UNVERIFIED] Git index flags can hide tracked-file changes; preserve and review the checkout before setup." >&2
    return 1
  fi
)

validate_cherri_installed_docs() (
  _installed_docs=$1
  validate_docs_dir "$_installed_docs" || return 1
  if [ ! -e "$_installed_docs/.git" ]; then
    echo "[CHERRI_DOCS_GIT_MISSING] Installed documentation lacks immutable Git provenance: $_installed_docs" >&2
    return 1
  fi
  validate_cherri_git_index "$_installed_docs" DOCS || return 1
  if ! _installed_docs_state=$(git --no-replace-objects -C "$_installed_docs" status --porcelain); then
    echo "[CHERRI_DOCS_GIT_INVALID] Cannot verify installed documentation bytes" >&2
    return 1
  fi
  if [ -n "$_installed_docs_state" ]; then
    echo "[CHERRI_DOCS_DIRTY] Installed documentation differs from its pinned commit" >&2
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
