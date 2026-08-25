#!/bin/bash
set -euo pipefail

IOS_DIR="$(cd "$(dirname "$0")/.." && pwd)"
ROOT_DIR="$(cd "$IOS_DIR/.." && pwd)"

if ! command -v xcodegen >/dev/null 2>&1; then
  echo "xcodegen is required. Install it with: brew install xcodegen" >&2
  exit 1
fi

(
  cd "$IOS_DIR/WebPreview"
  npm install --no-audit --no-fund
  npm run build
)

"$IOS_DIR/scripts/build_cherri_core.sh"

(
  cd "$IOS_DIR"
  xcodegen generate
)

echo "Generated $IOS_DIR/Cherri.xcodeproj"
