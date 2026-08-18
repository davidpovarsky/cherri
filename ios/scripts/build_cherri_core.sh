#!/bin/bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/../.." && pwd)"
BUILD_DIR="$ROOT_DIR/.build/ios-cherri-core"
VENDOR_DIR="$ROOT_DIR/ios/Vendor"
XCFRAMEWORK="$VENDOR_DIR/CherriCore.xcframework"
DEPLOYMENT_TARGET="${IOS_DEPLOYMENT_TARGET:-17.0}"
PARSER_FILE="$ROOT_DIR/parser.go"
PARSER_BACKUP="$BUILD_DIR/parser.go.original"

rm -rf "$BUILD_DIR" "$XCFRAMEWORK"
mkdir -p "$BUILD_DIR" "$VENDOR_DIR"

# Cherri is primarily a CLI and parserError() intentionally terminates the CLI
# with os.Exit(1). An in-process iOS compiler cannot allow a source error to
# terminate the host app. Keep the upstream parser source untouched in git and
# apply the smallest mobile-only adaptation while building the XCFramework.
# compileForMobile already recovers embeddedCompilerPanic and turns it into a
# structured CompilationDiagnostic for Swift.
cp "$PARSER_FILE" "$PARSER_BACKUP"
restore_parser() {
  if [ -f "$PARSER_BACKUP" ]; then
    cp "$PARSER_BACKUP" "$PARSER_FILE"
  fi
}
trap restore_parser EXIT

python3 - "$PARSER_FILE" <<'PY'
from pathlib import Path
import sys

path = Path(sys.argv[1])
source = path.read_text()
needle = "func parserError(message string) {\n"
replacement = """func parserError(message string) {
\tif embeddedCompilerMode {
\t\tpanic(embeddedCompilerPanic{message: message})
\t}
"""

if source.count(needle) != 1:
    raise SystemExit("Expected exactly one parserError function while preparing the iOS compiler")

path.write_text(source.replace(needle, replacement, 1))
PY

build_slice() {
  local name="$1"
  local sdk="$2"
  local platform="$3"
  local goarch="$4"
  local clangarch="$5"
  local slice_dir="$BUILD_DIR/$name"
  local wrapper="$slice_dir/clangwrap.sh"

  mkdir -p "$slice_dir/Headers"

  cat > "$wrapper" <<EOF
#!/bin/bash
set -euo pipefail
SDK_PATH="\$(xcrun --sdk $sdk --show-sdk-path)"
CLANG="\$(xcrun --sdk $sdk --find clang)"
exec "\$CLANG" -arch $clangarch -isysroot "\$SDK_PATH" -m${platform}-version-min=$DEPLOYMENT_TARGET "\$@"
EOF
  chmod +x "$wrapper"

  (
    cd "$ROOT_DIR"
    GOOS=ios \
    GOARCH="$goarch" \
    CGO_ENABLED=1 \
    CC="$wrapper" \
      go build -trimpath -buildmode=c-archive -o "$slice_dir/libCherriCore.a" .
  )

  cp "$slice_dir/libCherriCore.h" "$slice_dir/Headers/CherriCore.h"
  cat > "$slice_dir/Headers/module.modulemap" <<'EOF'
module CherriCore {
  header "CherriCore.h"
  export *
}
EOF
}

build_slice "iphoneos-arm64" "iphoneos" "ios" "arm64" "arm64"
build_slice "iphonesimulator-arm64" "iphonesimulator" "ios-simulator" "arm64" "arm64"
build_slice "iphonesimulator-x86_64" "iphonesimulator" "ios-simulator" "amd64" "x86_64"

# Restore the upstream source before the remaining Xcode build/test steps so the
# working tree is clean; the mobile adaptation is already compiled into the
# static archives above.
restore_parser
trap - EXIT

SIMULATOR_DIR="$BUILD_DIR/iphonesimulator-universal"
mkdir -p "$SIMULATOR_DIR/Headers"
cp "$BUILD_DIR/iphonesimulator-arm64/Headers/CherriCore.h" "$SIMULATOR_DIR/Headers/CherriCore.h"
cp "$BUILD_DIR/iphonesimulator-arm64/Headers/module.modulemap" "$SIMULATOR_DIR/Headers/module.modulemap"
lipo -create \
  "$BUILD_DIR/iphonesimulator-arm64/libCherriCore.a" \
  "$BUILD_DIR/iphonesimulator-x86_64/libCherriCore.a" \
  -output "$SIMULATOR_DIR/libCherriCore.a"

xcodebuild -create-xcframework \
  -library "$BUILD_DIR/iphoneos-arm64/libCherriCore.a" \
  -headers "$BUILD_DIR/iphoneos-arm64/Headers" \
  -library "$SIMULATOR_DIR/libCherriCore.a" \
  -headers "$SIMULATOR_DIR/Headers" \
  -output "$XCFRAMEWORK"

echo "Built $XCFRAMEWORK"
