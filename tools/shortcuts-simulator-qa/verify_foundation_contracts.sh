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

swiftc -parse-as-library \
  "$ROOT_DIR/tools/shortcuts-simulator-qa/XcodeProject/Shared/LibraryForegroundGate.swift" \
  "$ROOT_DIR/tools/shortcuts-simulator-qa/verify_import_foreground.swift" \
  -o "$QA_TEST_DIR/qa-import-foreground-contracts"
"$QA_TEST_DIR/qa-import-foreground-contracts"


swiftc -parse-as-library \
  "$ROOT_DIR/tools/shortcuts-simulator-qa/XcodeProject/Shared/EditorForegroundRecovery.swift" \
  "$ROOT_DIR/tools/shortcuts-simulator-qa/verify_editor_foreground.swift" \
  -o "$QA_TEST_DIR/qa-editor-foreground-contracts"
"$QA_TEST_DIR/qa-editor-foreground-contracts"

swiftc -parse-as-library \
  "$ROOT_DIR/tools/shortcuts-simulator-qa/XcodeProject/Shared/DebugLabelPreviewPolicy.swift" \
  "$ROOT_DIR/tools/shortcuts-simulator-qa/verify_debug_label_preview.swift" \
  -o "$QA_TEST_DIR/qa-debug-label-preview-contracts"
"$QA_TEST_DIR/qa-debug-label-preview-contracts"

swiftc -parse-as-library \
  "$ROOT_DIR/tools/shortcuts-simulator-qa/XcodeProject/Shared/DifferentialImportTiming.swift" \
  "$ROOT_DIR/tools/shortcuts-simulator-qa/verify_differential_import_timing.swift" \
  -o "$QA_TEST_DIR/qa-differential-import-timing-contracts"
"$QA_TEST_DIR/qa-differential-import-timing-contracts"

python3 "$ROOT_DIR/tools/shortcuts-simulator-qa/verify_optional_receiver.py"
python3 "$ROOT_DIR/tools/shortcuts-simulator-qa/verify_primary_no_input.py" \
  --artifacts-dir "$QA_TEST_DIR/primary-no-input"

python3 "$ROOT_DIR/tools/shortcuts-simulator-qa/verify_primary_no_input_r2.py" \
  --artifacts-dir "$QA_TEST_DIR/primary-no-input-r2"

swiftc -parse-as-library \
  "$ROOT_DIR/tools/shortcuts-simulator-qa/XcodeProject/Shared/ImportSheetSelection.swift" \
  "$ROOT_DIR/tools/shortcuts-simulator-qa/verify_import_sheet_selection.swift" \
  -o "$QA_TEST_DIR/qa-import-sheet-selection-contracts"
"$QA_TEST_DIR/qa-import-sheet-selection-contracts"

swiftc -parse-as-library \
  "$ROOT_DIR/tools/shortcuts-simulator-qa/XcodeProject/Shared/ImportSheetSelection.swift" \
  "$ROOT_DIR/tools/shortcuts-simulator-qa/verify_imported_editor_navigation.swift" \
  -o "$QA_TEST_DIR/qa-imported-editor-navigation-contracts"
"$QA_TEST_DIR/qa-imported-editor-navigation-contracts"

swiftc -parse-as-library \
  "$ROOT_DIR/tools/shortcuts-simulator-qa/XcodeProject/Shared/ImportSheetSelection.swift" \
  "$ROOT_DIR/tools/shortcuts-simulator-qa/verify_imported_library_identity.swift" \
  -o "$QA_TEST_DIR/qa-imported-library-identity-contracts"
"$QA_TEST_DIR/qa-imported-library-identity-contracts"
