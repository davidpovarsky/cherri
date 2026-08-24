#!/bin/sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
. "$SCRIPT_DIR/common.sh"

UPDATE=0
FORCE=0
for arg in "$@"; do
  case "$arg" in
    --update) UPDATE=1 ;;
    --force) FORCE=1 ;;
    *) echo "Unknown option: $arg" >&2; exit 2 ;;
  esac
done

mkdir -p "$CHERRI_HOME/bin"

if [ -f "$BUNDLED_CHERRI" ] && [ "$FORCE" -eq 0 ]; then
  chmod +x "$BUNDLED_CHERRI" 2>/dev/null || true
  echo "Using bundled Cherri: $BUNDLED_CHERRI"
elif [ -x "$CACHED_CHERRI" ] && [ "$UPDATE" -eq 0 ] && [ "$FORCE" -eq 0 ]; then
  echo "Using cached Cherri: $CACHED_CHERRI"
else
  if [ -n "${CHERRI_PREBUILT_URL:-}" ]; then
    echo "Downloading prebuilt Cherri..."
    if command -v wget >/dev/null 2>&1; then
      wget -q -O "$CACHED_CHERRI.tmp" "$CHERRI_PREBUILT_URL"
    elif command -v curl >/dev/null 2>&1; then
      curl -fL "$CHERRI_PREBUILT_URL" -o "$CACHED_CHERRI.tmp"
    else
      echo "Need wget or curl to download CHERRI_PREBUILT_URL." >&2
      exit 1
    fi
    mv "$CACHED_CHERRI.tmp" "$CACHED_CHERRI"
    chmod +x "$CACHED_CHERRI"
  else
    missing=""
    command -v git >/dev/null 2>&1 || missing="$missing git"
    command -v go >/dev/null 2>&1 || missing="$missing go"
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
      git clone --depth 1 --branch "$CHERRI_REF" "$CHERRI_REPO" "$SRC"
    elif [ "$UPDATE" -eq 1 ] || [ "$FORCE" -eq 1 ]; then
      git -C "$SRC" fetch --depth 1 origin "$CHERRI_REF"
      git -C "$SRC" reset --hard FETCH_HEAD
    fi

    echo "Building Cherri for $(uname -s)/$(uname -m)..."
    (
      cd "$SRC"
      CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o "$CACHED_CHERRI" .
    )
    chmod +x "$CACHED_CHERRI"
  fi
fi

# The ready-made ARM64 skill avoids Go compilation, but local docs still need
# git. Install only the lightweight docs dependencies when they are absent.
if [ ! -d "$CHERRI_DOCS_DIR/.git" ] && ! command -v git >/dev/null 2>&1; then
  if command -v apk >/dev/null 2>&1; then
    echo "Installing documentation dependencies: git ca-certificates"
    apk add --no-cache git ca-certificates
  fi
fi

if command -v git >/dev/null 2>&1; then
  if [ ! -d "$CHERRI_DOCS_DIR/.git" ]; then
    rm -rf "$CHERRI_DOCS_DIR"
    git clone --depth 1 --branch "$CHERRI_DOCS_REF" "$CHERRI_DOCS_REPO" "$CHERRI_DOCS_DIR" || true
  elif [ "$UPDATE" -eq 1 ]; then
    git -C "$CHERRI_DOCS_DIR" fetch --depth 1 origin "$CHERRI_DOCS_REF" || true
    git -C "$CHERRI_DOCS_DIR" reset --hard FETCH_HEAD || true
  fi
fi

BIN=$(resolve_cherri) || { echo "No Cherri binary found after setup." >&2; exit 1; }
"$BIN" --version
printf 'Cherri ready: %s\n' "$BIN"
[ -d "$CHERRI_DOCS_DIR" ] && printf 'Docs ready: %s\n' "$CHERRI_DOCS_DIR" || true
