#!/bin/sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
. "$SCRIPT_DIR/common.sh"

UPDATE=0
FORCE=0
OFFLINE="${CHERRI_OFFLINE:-0}"
for arg in "$@"; do
  case "$arg" in
    --update) UPDATE=1 ;;
    --force) FORCE=1 ;;
    --offline) OFFLINE=1 ;;
    *) echo "Unknown option: $arg" >&2; exit 2 ;;
  esac
done

if ! is_hex_len "$CHERRI_REF" 40 || ! is_hex_len "$CHERRI_DOCS_REF" 40; then
  echo "[CHERRI_INSTALL_IMMUTABLE_REFS_REQUIRED] Install compiler and docs using exact 40-digit commit IDs." >&2
  exit 1
fi

mkdir -p "$CHERRI_HOME/bin"

# Keep the active compiler/docs pair untouched until both candidates validate.
# Backup files remain available until the installed pair also passes validation.
# This handles command failures/signals; it is not a power-loss journal.
BIN_STAGE=""
DOCS_STAGE=""
INSTALL_LOCKED=0
INSTALL_COMMITTED=0
OLD_BIN_PRESENT=0
OLD_COMMIT_PRESENT=0
OLD_CHECKSUM_PRESENT=0
OLD_DOCS_PRESENT=0
BIN_PROMOTING=0
DOCS_PROMOTING=0

begin_install() {
  if [ "$INSTALL_LOCKED" -eq 0 ]; then
    if ! mkdir "$CHERRI_HOME/.install-lock" 2>/dev/null; then
      echo "[CHERRI_INSTALL_BUSY] Another install owns $CHERRI_HOME/.install-lock; inspect an interrupted install before removing its lock." >&2
      exit 1
    fi
    INSTALL_LOCKED=1
  fi
}

# The rollback decision uses the rename result on disk. A signal may be
# delivered after mv completes but before the shell executes its next line.
restore_install_file() {
  _restore_dest=$1
  _restore_backup=$2
  _restore_candidate=$3
  _restore_existed=$4
  if [ -e "$_restore_backup" ] || [ -L "$_restore_backup" ]; then
    if rm -f "$_restore_dest"; then
      mv "$_restore_backup" "$_restore_dest" || ROLLBACK_OK=0
    else
      ROLLBACK_OK=0
    fi
  elif [ "$_restore_existed" -eq 0 ] && [ ! -e "$_restore_candidate" ] && [ ! -L "$_restore_candidate" ]; then
    rm -f "$_restore_dest" || ROLLBACK_OK=0
  fi
}

finish_install() {
  INSTALL_STATUS=$?
  trap - EXIT
  # Finish the bounded rollback before honoring further handled signals.
  trap '' HUP INT TERM
  set +e
  ROLLBACK_OK=1
  if [ "$INSTALL_COMMITTED" -eq 0 ]; then
    if [ "$DOCS_PROMOTING" -eq 1 ]; then
      if [ -e "$DOCS_STAGE/previous" ] || [ -L "$DOCS_STAGE/previous" ]; then
        if rm -rf "$CHERRI_DOCS_DIR"; then
          mv "$DOCS_STAGE/previous" "$CHERRI_DOCS_DIR" || ROLLBACK_OK=0
        else
          ROLLBACK_OK=0
        fi
      elif [ "$OLD_DOCS_PRESENT" -eq 0 ] && [ ! -e "$DOCS_STAGE/candidate" ]; then
        rm -rf "$CHERRI_DOCS_DIR" || ROLLBACK_OK=0
      fi
    fi
    if [ "$BIN_PROMOTING" -eq 1 ]; then
      restore_install_file "$CACHED_CHERRI.sha256" "$BIN_STAGE/previous.sha256" "$BIN_STAGE/candidate.sha256" "$OLD_CHECKSUM_PRESENT"
      restore_install_file "$CACHED_CHERRI.commit" "$BIN_STAGE/previous.commit" "$BIN_STAGE/candidate.commit" "$OLD_COMMIT_PRESENT"
      restore_install_file "$CACHED_CHERRI" "$BIN_STAGE/previous" "$BIN_STAGE/candidate" "$OLD_BIN_PRESENT"
    fi
  fi
  if [ "$ROLLBACK_OK" -eq 1 ]; then
    if [ -n "$BIN_STAGE" ]; then rm -rf "$BIN_STAGE"; fi
    if [ -n "$DOCS_STAGE" ]; then rm -rf "$DOCS_STAGE"; fi
    if [ "$INSTALL_LOCKED" -eq 1 ]; then rmdir "$CHERRI_HOME/.install-lock"; fi
  else
    echo "[CHERRI_INSTALL_ROLLBACK_FAILED] Preserved recovery files in $BIN_STAGE and $DOCS_STAGE; install lock retained." >&2
    INSTALL_STATUS=1
  fi
  exit "$INSTALL_STATUS"
}
trap finish_install EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

if [ -n "${CHERRI_BIN:-}" ]; then
  if ! validate_cherri_bin "$CHERRI_BIN"; then
    echo "Explicit CHERRI_BIN failed compatibility validation: $CHERRI_BIN" >&2
    exit 1
  fi
  echo "Using explicit compatible CHERRI_BIN: $CHERRI_BIN"
else
  HAVE_COMPATIBLE_BIN=0
  if [ -f "$CACHED_CHERRI" ] && [ "$UPDATE" -eq 0 ] && [ "$FORCE" -eq 0 ]; then
    if validate_cherri_bin "$CACHED_CHERRI"; then
      echo "Using compatible cached Cherri: $CACHED_CHERRI"
      HAVE_COMPATIBLE_BIN=1
    else
      echo "Cached Cherri is outdated or incompatible; rebuild required." >&2
    fi
  fi

  if [ "$HAVE_COMPATIBLE_BIN" -eq 0 ] && [ -f "$BUNDLED_CHERRI" ] && [ "$UPDATE" -eq 0 ] && [ "$FORCE" -eq 0 ]; then
    if validate_cherri_bin "$BUNDLED_CHERRI"; then
      echo "Using compatible bundled Cherri: $BUNDLED_CHERRI"
      HAVE_COMPATIBLE_BIN=1
    else
      echo "Bundled Cherri is outdated or incompatible; will use fresh build." >&2
    fi
  fi

  if [ "$HAVE_COMPATIBLE_BIN" -eq 0 ]; then
    if [ "$OFFLINE" = "1" ]; then
      echo "Cannot install or update Cherri in offline mode: no compatible cached or bundled binary available." >&2
      exit 1
    fi
    for INSTALL_FILE in "$CACHED_CHERRI" "$CACHED_CHERRI.commit" "$CACHED_CHERRI.sha256"; do
      if [ -d "$INSTALL_FILE" ]; then
        echo "[CHERRI_INSTALL_PATH_INVALID] Expected an install file, found a directory: $INSTALL_FILE" >&2
        exit 1
      fi
    done
    begin_install
    BIN_STAGE=$(mktemp -d "$CHERRI_HOME/bin/.cherri-install.XXXXXX")
    TMP_BIN="$BIN_STAGE/candidate"
    TMP_COMMIT="$TMP_BIN.commit"

    if [ -n "${CHERRI_PREBUILT_URL:-}" ]; then
      if ! is_hex_len "${CHERRI_PREBUILT_SHA256:-}" 64; then
        echo "[CHERRI_PREBUILT_CHECKSUM_REQUIRED] CHERRI_PREBUILT_SHA256 must contain the expected 64-digit SHA-256 for CHERRI_PREBUILT_URL." >&2
        exit 1
      fi
      echo "Downloading prebuilt Cherri..."
      if command -v wget >/dev/null 2>&1; then
        wget -q -O "$TMP_BIN" "$CHERRI_PREBUILT_URL"
      elif command -v curl >/dev/null 2>&1; then
        curl -fsSL "$CHERRI_PREBUILT_URL" -o "$TMP_BIN"
      else
        echo "Need wget or curl to download CHERRI_PREBUILT_URL." >&2
        exit 1
      fi
      # Verify downloaded bytes before chmod or executing capabilities/version.
      verify_cherri_checksum "$TMP_BIN" "$CHERRI_PREBUILT_SHA256"
      printf '%s\n' "$CHERRI_REF" > "$TMP_COMMIT"
    else
      missing=""
      command -v git >/dev/null 2>&1 || missing="$missing git"
      if [ ! -f "$CHERRI_REPO/.cherri-prebuilt-bin" ]; then
        command -v go >/dev/null 2>&1 || missing="$missing go"
      fi
      if [ -n "$missing" ]; then
        if command -v apk >/dev/null 2>&1; then
          echo "Installing Alpine build dependencies:$missing ca-certificates"
          apk add --no-cache git go ca-certificates
        else
          echo "Missing:$missing. Install them or provide CHERRI_PREBUILT_URL and CHERRI_PREBUILT_SHA256." >&2
          exit 1
        fi
      fi

      SRC="$CHERRI_HOME/src"
      if [ ! -e "$SRC" ]; then
        git clone "$CHERRI_REPO" "$SRC"
      elif [ ! -d "$SRC/.git" ]; then
        echo "[CHERRI_SOURCE_UNMANAGED] Refusing to replace a non-clone source cache: $SRC" >&2
        exit 1
      else
        if ! SOURCE_STATE=$(git -C "$SRC" status --porcelain); then
          echo "[CHERRI_SOURCE_GIT_INVALID] Cannot verify source cache state; active installation unchanged." >&2
          exit 1
        fi
        if [ -n "$SOURCE_STATE" ]; then
          echo "[CHERRI_SOURCE_DIRTY] Source cache has local changes; preserve or commit them before setup." >&2
          exit 1
        fi
        git -C "$SRC" fetch origin "$CHERRI_REF" 2>/dev/null || git -C "$SRC" fetch origin
      fi
      git -C "$SRC" checkout --detach "$CHERRI_REF" 2>/dev/null || git -C "$SRC" checkout "$CHERRI_REF"

      ACTUAL_SRC_SHA=$(git -C "$SRC" rev-parse HEAD)
      if is_hex_len "$CHERRI_REF" 40 && [ "$ACTUAL_SRC_SHA" != "$CHERRI_REF" ]; then
        echo "Source checkout SHA mismatch: expected $CHERRI_REF, got $ACTUAL_SRC_SHA" >&2
        exit 1
      fi

      echo "Building Cherri ($ACTUAL_SRC_SHA) for $(uname -s)/$(uname -m)..."
      if [ -f "$SRC/.cherri-prebuilt-bin" ]; then
        cp "$SRC/.cherri-prebuilt-bin" "$TMP_BIN"
      else
        (
          cd "$SRC"
          CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o "$TMP_BIN" .
        )
      fi
      printf '%s\n' "$ACTUAL_SRC_SHA" > "$TMP_COMMIT"
    fi

    cherri_file_sha256 "$TMP_BIN" > "$TMP_BIN.sha256"
    chmod +x "$TMP_BIN"
    if ! validate_cherri_bin "$TMP_BIN"; then
      echo "Newly built/downloaded Cherri binary failed compatibility validation; active installation unchanged." >&2
      exit 1
    fi
  fi
fi

# Validate an existing docs checkout or prepare a separate pinned candidate.
DOCS_VALID=0
if [ -d "$CHERRI_DOCS_DIR" ] && [ "$UPDATE" -eq 0 ] && [ "$FORCE" -eq 0 ]; then
  if validate_docs_dir "$CHERRI_DOCS_DIR"; then
    DOCS_VALID=1
  else
    if [ "$OFFLINE" = "1" ]; then
      echo "Existing docs in $CHERRI_DOCS_DIR failed compatibility validation in offline mode." >&2
      exit 1
    fi
  fi
fi

if [ "$DOCS_VALID" -eq 0 ]; then
  if [ "$OFFLINE" = "1" ]; then
    validate_docs_dir "$CHERRI_DOCS_DIR"
  else
    if ! command -v git >/dev/null 2>&1; then
      if command -v apk >/dev/null 2>&1; then
        echo "Installing documentation dependencies: git ca-certificates"
        apk add --no-cache git ca-certificates
      else
        echo "[CHERRI_DOCS_GIT_UNAVAILABLE] Cannot install pinned documentation without git." >&2
        exit 1
      fi
    fi
    if [ -e "$CHERRI_DOCS_DIR" ]; then
      if [ ! -d "$CHERRI_DOCS_DIR/.git" ]; then
        echo "[CHERRI_DOCS_UNMANAGED] Refusing to replace non-clone or linked-worktree documentation: $CHERRI_DOCS_DIR" >&2
        exit 1
      fi
      if ! DOCS_STATE=$(git -C "$CHERRI_DOCS_DIR" status --porcelain); then
        echo "[CHERRI_DOCS_GIT_INVALID] Cannot verify documentation state; active installation unchanged." >&2
        exit 1
      fi
      if [ -n "$DOCS_STATE" ]; then
        echo "[CHERRI_DOCS_DIRTY] Documentation has local changes; active installation unchanged." >&2
        exit 1
      fi
    fi
    begin_install
    DOCS_PARENT=$(dirname -- "$CHERRI_DOCS_DIR")
    mkdir -p "$DOCS_PARENT"
    DOCS_STAGE=$(mktemp -d "$DOCS_PARENT/.cherri-docs.XXXXXX")
    git clone "$CHERRI_DOCS_REPO" "$DOCS_STAGE/candidate"
    git -C "$DOCS_STAGE/candidate" checkout --detach "$CHERRI_DOCS_REF" 2>/dev/null || git -C "$DOCS_STAGE/candidate" checkout "$CHERRI_DOCS_REF"
    validate_docs_dir "$DOCS_STAGE/candidate"
  fi
fi

# Every candidate is compatible before the first active file is moved.
if [ -n "$BIN_STAGE" ]; then
  if [ -e "$CACHED_CHERRI" ] || [ -L "$CACHED_CHERRI" ]; then OLD_BIN_PRESENT=1; fi
  if [ -e "$CACHED_CHERRI.commit" ] || [ -L "$CACHED_CHERRI.commit" ]; then OLD_COMMIT_PRESENT=1; fi
  if [ -e "$CACHED_CHERRI.sha256" ] || [ -L "$CACHED_CHERRI.sha256" ]; then OLD_CHECKSUM_PRESENT=1; fi
  BIN_PROMOTING=1
  if [ "$OLD_BIN_PRESENT" -eq 1 ]; then mv "$CACHED_CHERRI" "$BIN_STAGE/previous"; fi
  if [ "$OLD_COMMIT_PRESENT" -eq 1 ]; then mv "$CACHED_CHERRI.commit" "$BIN_STAGE/previous.commit"; fi
  if [ "$OLD_CHECKSUM_PRESENT" -eq 1 ]; then mv "$CACHED_CHERRI.sha256" "$BIN_STAGE/previous.sha256"; fi
  mv "$TMP_BIN" "$CACHED_CHERRI"
  mv "$TMP_COMMIT" "$CACHED_CHERRI.commit"
  mv "$TMP_BIN.sha256" "$CACHED_CHERRI.sha256"
fi
if [ -n "$DOCS_STAGE" ]; then
  if [ -e "$CHERRI_DOCS_DIR" ] || [ -L "$CHERRI_DOCS_DIR" ]; then OLD_DOCS_PRESENT=1; fi
  DOCS_PROMOTING=1
  if [ "$OLD_DOCS_PRESENT" -eq 1 ]; then mv "$CHERRI_DOCS_DIR" "$DOCS_STAGE/previous"; fi
  mv "$DOCS_STAGE/candidate" "$CHERRI_DOCS_DIR"
fi

if [ -n "$BIN_STAGE" ]; then
  # A damaged newly promoted cache must roll back, not appear successful by
  # selecting an unrelated compatible bundled or PATH compiler.
  if ! validate_cherri_bin "$CACHED_CHERRI"; then
    echo "[CHERRI_INSTALL_PROMOTED_INVALID] Promoted compiler failed validation; restoring the preceding installation." >&2
    exit 1
  fi
  BIN=$CACHED_CHERRI
else
  BIN=$(resolve_cherri) || { echo "No compatible Cherri binary found after setup." >&2; exit 1; }
fi
"$BIN" --version
if [ -d "$CHERRI_DOCS_DIR" ]; then
  validate_docs_dir "$CHERRI_DOCS_DIR"
  printf 'Docs ready: %s\n' "$CHERRI_DOCS_DIR"
fi
INSTALL_COMMITTED=1
printf 'Cherri ready: %s\n' "$BIN"
