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

CHERRI_DOCS_COMMIT="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["docsCommit"])' "$ROOT_DIR/skills/cherri-shortcuts/compatibility-manifest.json")"
mkdir -p "$IOS_DIR/.build"
python3 "$IOS_DIR/scripts/build_documentation_index.py" \
  --docs "$IOS_DIR/.build/documentation-source/$CHERRI_DOCS_COMMIT" --fetch \
  > "$IOS_DIR/.build/documentation-receipt.json"

"$IOS_DIR/scripts/build_cherri_core.sh"

(
  cd "$IOS_DIR"
  xcodegen generate
  xcodebuild -project Cherri.xcodeproj -scheme Cherri -resolvePackageDependencies \
    -clonedSourcePackagesDirPath .build/SourcePackages
  python3 scripts/prepare_editor_dependency.py --receipt .build/editor-dependency-receipt.json
)

echo "Generated $IOS_DIR/Cherri.xcodeproj"
