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

mkdir -p "$CHERRI_HOME/bin"

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

    TMP_BIN="$CACHED_CHERRI.tmp.$$"
    TMP_COMMIT="$TMP_BIN.commit"
    cleanup_tmp() {
      rm -f "$TMP_BIN" "$TMP_COMMIT"
    }
    trap cleanup_tmp EXIT INT TERM

    if [ -n "${CHERRI_PREBUILT_URL:-}" ]; then
      echo "Downloading prebuilt Cherri..."
      if command -v wget >/dev/null 2>&1; then
        wget -q -O "$TMP_BIN" "$CHERRI_PREBUILT_URL"
      elif command -v curl >/dev/null 2>&1; then
        curl -fsSL "$CHERRI_PREBUILT_URL" -o "$TMP_BIN"
      else
        echo "Need wget or curl to download CHERRI_PREBUILT_URL." >&2
        exit 1
      fi
      chmod +x "$TMP_BIN"
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
          echo "Missing:$missing. Install them or provide CHERRI_PREBUILT_URL." >&2
          exit 1
        fi
      fi

      SRC="$CHERRI_HOME/src"
      if [ ! -d "$SRC/.git" ]; then
        rm -rf "$SRC"
        git clone "$CHERRI_REPO" "$SRC"
      else
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
      chmod +x "$TMP_BIN"
      printf '%s\n' "$ACTUAL_SRC_SHA" > "$TMP_COMMIT"
    fi

    if ! validate_cherri_bin "$TMP_BIN"; then
      echo "Newly built/downloaded Cherri binary failed compatibility validation; aborting atomic replacement." >&2
      exit 1
    fi

    mv -f "$TMP_BIN" "$CACHED_CHERRI"
    mv -f "$TMP_COMMIT" "$CACHED_CHERRI.commit"
    trap - EXIT INT TERM
  fi
fi

# Verify or install documentation at pinned commit
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
    if [ -d "$CHERRI_DOCS_DIR" ]; then
      validate_docs_dir "$CHERRI_DOCS_DIR"
    fi
  else
    if [ ! -d "$CHERRI_DOCS_DIR/.git" ] && ! command -v git >/dev/null 2>&1; then
      if command -v apk >/dev/null 2>&1; then
        echo "Installing documentation dependencies: git ca-certificates"
        apk add --no-cache git ca-certificates
      fi
    fi

    if command -v git >/dev/null 2>&1; then
      if [ ! -d "$CHERRI_DOCS_DIR/.git" ]; then
        rm -rf "$CHERRI_DOCS_DIR"
        git clone "$CHERRI_DOCS_REPO" "$CHERRI_DOCS_DIR"
      else
        git -C "$CHERRI_DOCS_DIR" fetch origin "$CHERRI_DOCS_REF" 2>/dev/null || git -C "$CHERRI_DOCS_DIR" fetch origin
      fi
      git -C "$CHERRI_DOCS_DIR" checkout --detach "$CHERRI_DOCS_REF" 2>/dev/null || git -C "$CHERRI_DOCS_DIR" checkout "$CHERRI_DOCS_REF"
      validate_docs_dir "$CHERRI_DOCS_DIR"
    fi
  fi
fi

BIN=$(resolve_cherri) || { echo "No compatible Cherri binary found after setup." >&2; exit 1; }
"$BIN" --version
printf 'Cherri ready: %s\n' "$BIN"
if [ -d "$CHERRI_DOCS_DIR" ]; then
  validate_docs_dir "$CHERRI_DOCS_DIR"
  printf 'Docs ready: %s\n' "$CHERRI_DOCS_DIR"
fi
