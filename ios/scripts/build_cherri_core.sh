#!/bin/bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/../.." && pwd)"
BUILD_DIR="$ROOT_DIR/.build/ios-cherri-core"
VENDOR_DIR="$ROOT_DIR/ios/Vendor"
XCFRAMEWORK="$VENDOR_DIR/CherriCore.xcframework"
DEPLOYMENT_TARGET="${IOS_DEPLOYMENT_TARGET:-17.0}"

rm -rf "$BUILD_DIR" "$XCFRAMEWORK"
mkdir -p "$BUILD_DIR" "$VENDOR_DIR"

build_slice() {
  local name="$1"
  local sdk="$2"
  local platform="$3"
  local slice_dir="$BUILD_DIR/$name"
  local wrapper="$slice_dir/clangwrap.sh"

  mkdir -p "$slice_dir/Headers"

  cat > "$wrapper" <<EOF
#!/bin/bash
set -euo pipefail
SDK_PATH="\$(xcrun --sdk $sdk --show-sdk-path)"
CLANG="\$(xcrun --sdk $sdk --find clang)"
exec "\$CLANG" -arch arm64 -isysroot "\$SDK_PATH" -m${platform}-version-min=$DEPLOYMENT_TARGET "\$@"
EOF
  chmod +x "$wrapper"

  (
    cd "$ROOT_DIR"
    GOOS=ios \
    GOARCH=arm64 \
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

build_slice "iphoneos" "iphoneos" "ios"
build_slice "iphonesimulator" "iphonesimulator" "ios-simulator"

xcodebuild -create-xcframework \
  -library "$BUILD_DIR/iphoneos/libCherriCore.a" \
  -headers "$BUILD_DIR/iphoneos/Headers" \
  -library "$BUILD_DIR/iphonesimulator/libCherriCore.a" \
  -headers "$BUILD_DIR/iphonesimulator/Headers" \
  -output "$XCFRAMEWORK"

echo "Built $XCFRAMEWORK"
