#!/usr/bin/env bash
set -euo pipefail
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
QA_TEST_DIR="$(mktemp -d)"
trap 'rm -rf "$QA_TEST_DIR"' EXIT
swiftc -parse-as-library \
  "$ROOT_DIR/tools/shortcuts-simulator-qa/XcodeProject/Shared/EvidenceGeometry.swift" \
  "$ROOT_DIR/tools/shortcuts-simulator-qa/XcodeProject/Shared/CallbackPayloadDiagnostics.swift" \
  "$ROOT_DIR/tools/shortcuts-simulator-qa/verify_foundation_contracts.swift" \
  -o "$QA_TEST_DIR/qa-foundation-contracts"
"$QA_TEST_DIR/qa-foundation-contracts"
