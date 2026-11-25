#!/usr/bin/env bash
# Cherri P10R14 — Signed iPadOS 27 Simulator Shortcuts QA Orchestrator
# Implements the 14-gate state machine, signed/unsigned import verification,
# genuine editor card & anchor coverage, correlated x-callback-url runtime
# oracle, NDJSON trace validation, and fail-closed privacy/CI gates.

set -uo pipefail

PROBE_LEVEL="${1:-full}"
DEVICE_PROFILE="${2:-ipad-default}"
SAMPLE_ID="${3:-all}"
CAPTURE_FULL_EDITOR="${4:-true}"
MAX_EDITOR_PAGES="${5:-20}"
REQUIRE_RUNTIME="${6:-false}"
RUN_ID="${GITHUB_RUN_ID:-local-$(date -u +%Y%m%dT%H%M%SZ)}"

# Validate workflow/CLI inputs upfront (fail closed on invalid input)
case "$PROBE_LEVEL" in
  preflight|import|editor|runtime|full) ;;
  *) echo "ERROR: Invalid PROBE_LEVEL '$PROBE_LEVEL'" >&2; exit 2 ;;
esac
case "$DEVICE_PROFILE" in
  ipad-default|iphone-fallback) ;;
  *) echo "ERROR: Invalid DEVICE_PROFILE '$DEVICE_PROFILE'" >&2; exit 2 ;;
esac
case "$SAMPLE_ID" in
  all|qa-collect-counts-v2|qa-collect-counts-legacy|qa-collect-dictionaries-v2|qa-collect-dictionaries-legacy|qa-functional|qa-functional-legacy|qa-scroll-editor|qa-rtl-hebrew|qa-bool-list|qa-short-circuit|qa-short-circuit-local|qa-f01-diag-counter|qa-f01-diag-predicate|qa-f01-diag-control|qa-f01-diag-logic|qa-require|qa-measure|qa-measure-release|qa-label-file-carrier|qa-label-raw-empty|qa-label-raw-grouping|qa-native-boundary-diagnostic|qa-file-materialization-diagnostic|qa-n03-text-equality-diagnostic|qa-optional-forwarding-diagnostic|qa-nested-list-control|qa-nested-list-candidate|qa-export-unicode-diagnostic|qa-export-unicode|qa-export-empty|qa-export-thousand|qa-export-redacted|qa-export-unsupported-file|qa-export-text-file|qa-json-body-control|qa-json-body-null-candidate|qa-json-body-file-utf8|qa-retry-control|qa-retry-http500|qa-retry-http404|qa-retry-http429|qa-retry-transport|qa-paginate-ordered|qa-paginate-cycle|qa-paginate-page-cap|qa-paginate-item-cap|qa-paginate-empty|qa-require-zero-diag|qa-label-file-controls|qa-export-file-control|qa-export-build-only|qa-export-parse-only|qa-storage-text-same-run|qa-storage-scope|qa-storage-delete|qa-storage-text-cross-run|qa-http-json-control|qa-http-json-fetch|qa-number-comparison-diag|qa-number-coercion-control|qa-nil-clear|qa-collect-empty-split|qa-export-file-bytes|qa-file-name-binding|qa-file-name-extension|qa-f04-yield-list|qa-f04-yield-empty-list|qa-f04-record-empty|qa-f01-ifresult-let|qa-f01-ifresult-var|qa-binding-capture|qa-typed-defaults|qa-debug-label-plain-v2|qa-debug-label-plain-legacy|qa-debug-label-dated-v2|qa-debug-label-dated-legacy|qa-debug-label-typed-v2|qa-debug-label-typed-legacy|qa-debug-label-dated-typed-v2|qa-debug-label-dated-typed-legacy|qa-debug-label-text-v2|qa-debug-label-text-legacy|qa-debug-label-dated-text-v2|qa-debug-label-dated-text-legacy) ;;
  *) echo "ERROR: Invalid SAMPLE_ID '$SAMPLE_ID'" >&2; exit 2 ;;
esac
case "$CAPTURE_FULL_EDITOR" in
  true|false) ;;
  *) echo "ERROR: Invalid CAPTURE_FULL_EDITOR '$CAPTURE_FULL_EDITOR'" >&2; exit 2 ;;
esac
case "$REQUIRE_RUNTIME" in
  true|false) ;;
  *) echo "ERROR: Invalid REQUIRE_RUNTIME '$REQUIRE_RUNTIME'" >&2; exit 2 ;;
esac
if ! [[ "$MAX_EDITOR_PAGES" =~ ^([1-9]|1[0-9]|20)$ ]]; then
  echo "ERROR: MAX_EDITOR_PAGES must be an integer in 1..20 (got '$MAX_EDITOR_PAGES')" >&2
  exit 2
fi

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

ARTIFACT_DIR="$ROOT_DIR/artifacts/cherri-ios27-simulator-qa/$RUN_ID"
QA_SCRATCH_DIR="${RUNNER_TEMP:-/tmp}/cherri-qa-scratch-${RUN_ID}"
mkdir -p "$ARTIFACT_DIR/editor" "$ARTIFACT_DIR/runtime" "$ARTIFACT_DIR/tests" "$ARTIFACT_DIR/docs" "$ARTIFACT_DIR/builds" "$QA_SCRATCH_DIR"

SOURCE_SHA="$(git rev-parse HEAD 2>/dev/null || echo "unknown")"
HTTP_PROBE_PID=""
HTTP_PROBE_STARTED="false"
HTTP_PROBE_DIAGNOSTIC_RC=0
POST_TEST_INSPECTION_RC=0
http_probe() {
  # Only fixed, reviewed fixture IDs may select an endpoint implementation.
  case "$TARGET_FIXTURE_ID" in
    qa-json-body-control-v2|qa-json-body-null-candidate-v2|qa-json-body-file-utf8-v2)
      python3 "$ROOT_DIR/tools/shortcuts-simulator-qa/json_body_probe.py" "$@"
      ;;
    qa-retry-control-v2|qa-retry-http500-v2|qa-retry-http404-v2|qa-retry-http429-v2|qa-retry-transport-v2)
      python3 "$ROOT_DIR/tools/shortcuts-simulator-qa/retry_http_probe.py" "$@"
      ;;
    *) echo "ERROR: unreviewed HTTP probe fixture" >&2; return 2 ;;
  esac
}
stop_http_probe() {
  if [ -n "$HTTP_PROBE_PID" ]; then
    touch "$ARTIFACT_DIR/runtime/http-probe.stop"
    wait "$HTTP_PROBE_PID" || true
    HTTP_PROBE_PID=""
  fi
}
trap 'stop_http_probe' EXIT

echo "=== Cherri iPadOS 27 Simulator QA ($RUN_ID) ==="
echo "Probe Level:        $PROBE_LEVEL"
echo "Device Profile:     $DEVICE_PROFILE"
echo "Sample ID:          $SAMPLE_ID"
echo "Capture Full Editor:$CAPTURE_FULL_EDITOR"
echo "Max Editor Pages:   $MAX_EDITOR_PAGES"
echo "Require Runtime:    $REQUIRE_RUNTIME"
echo "Source SHA:         $SOURCE_SHA"

# 14 Progressive Gate Defaults
GATE_RUNNER_AVAILABLE="NOT_RUN"
GATE_SIMULATOR_BOOTED="NOT_RUN"
GATE_SHORTCUTS_APP_INSTALLED="NOT_RUN"
GATE_SHORTCUTS_LAUNCHED="NOT_RUN"
GATE_EDITOR_OPENED="NOT_RUN"
GATE_FILE_IMPORT_ATTEMPTED="NOT_RUN"
GATE_COMPILER_ARTIFACT_IMPORTED="NOT_RUN"
GATE_SHORTCUT_EXISTS_IN_LIBRARY="NOT_RUN"
GATE_ACTION_EDITOR_CAPTURED="NOT_RUN"
GATE_SHORTCUT_RUN_ATTEMPTED="NOT_RUN"
GATE_SHORTCUT_RUN_COMPLETED="NOT_RUN"
GATE_CALLBACK_CAPTURED="NOT_RUN"
GATE_TRACE_CAPTURED="NOT_RUN"
GATE_PHYSICAL_DEVICE_TESTED="BLOCKED_DEVICE"

# Select primary target fixture from SAMPLE_ID
# Nested source paths are fixed here; an inherited environment cannot select a file.
TARGET_SOURCE_RELATIVE=""
HTTP_PROBE_ENABLED="false"
STORAGE_CROSS_RUN_ENABLED="false"
case "$SAMPLE_ID" in
  qa-debug-label-plain-v2)
    TARGET_FIXTURE_ID="qa-debug-label-plain-v2"
    LEGACY_COUNTERPART_ID="qa-debug-label-plain-legacy"
    EXPECTED_SHORTCUT_NAME="qa-debug-label-plain-v2"
    EXPECTED_SEMANTIC_OUTPUT="CHERRI_QA_DEBUG_LABEL:MODE=plain:BUFFER_COUNT=7:TEXT_TYPE64=VGV4dA==:TEXT64=16LXqNeaIPCfjZI=:NUMBER_TYPE64=TnVtYmVy:NUMBER=7.25:MAP_TYPE64=RGljdGlvbmFyeQ==:MAP_KEYS=2:MAP_KIND64=TEFCRUxfTUFQ:MAP_VALUE=7.25:LIST_COUNT=2:LIST_FIRST64=bGVmdA==:LIST_LAST64=cmlnaHQ=:DATE_TYPE64=RGF0ZQ==:DATE_COUNT=1:DATE_DELTA_SECONDS=0:DATE_SOURCE_DELTA_SECONDS=0:FILE_TYPE64=RmlsZQ==:FILE_COUNT=1:FILE_BYTES64=AQIDBA==:FILE_NAME64=bGFiZWwtc291cmNlLmJpbg==:SOURCE_FILE_TYPE64=RmlsZQ==:SOURCE_FILE_BYTES64=AQIDBA==:SOURCE_FILE_NAME64=bGFiZWwtc291cmNlLmJpbg==:METADATA_COUNT=6:M1_KEYS=3:M1_FIRST=1:M1_LAST=1:M1_LABEL64=VEVYVF/Xqda414HXnNeV1rnXnV/wn42S:M2_KEYS=3:M2_FIRST=2:M2_LAST=2:M2_LABEL64=:M3_KEYS=3:M3_FIRST=3:M3_LAST=3:M3_LABEL64=TUFQX0xBQkVM:M4_KEYS=3:M4_FIRST=4:M4_LAST=5:M4_LABEL64=TElTVF9MQUJFTA==:M5_KEYS=3:M5_FIRST=6:M5_LAST=6:M5_LABEL64=REFURV9MQUJFTA==:M6_KEYS=3:M6_FIRST=7:M6_LAST=7:M6_LABEL64=RklMRV9MQUJFTA==:RAW_CLEAR=0:METADATA_CLEAR=0"
    ;;
  qa-debug-label-plain-legacy)
    TARGET_FIXTURE_ID="qa-debug-label-plain-legacy"
    LEGACY_COUNTERPART_ID="qa-debug-label-plain-legacy"
    EXPECTED_SHORTCUT_NAME="qa-debug-label-plain-legacy"
    EXPECTED_SEMANTIC_OUTPUT="CHERRI_QA_DEBUG_LABEL:MODE=plain:BUFFER_COUNT=7:TEXT_TYPE64=VGV4dA==:TEXT64=16LXqNeaIPCfjZI=:NUMBER_TYPE64=TnVtYmVy:NUMBER=7.25:MAP_TYPE64=RGljdGlvbmFyeQ==:MAP_KEYS=2:MAP_KIND64=TEFCRUxfTUFQ:MAP_VALUE=7.25:LIST_COUNT=2:LIST_FIRST64=bGVmdA==:LIST_LAST64=cmlnaHQ=:DATE_TYPE64=RGF0ZQ==:DATE_COUNT=1:DATE_DELTA_SECONDS=0:DATE_SOURCE_DELTA_SECONDS=0:FILE_TYPE64=RmlsZQ==:FILE_COUNT=1:FILE_BYTES64=AQIDBA==:FILE_NAME64=bGFiZWwtc291cmNlLmJpbg==:SOURCE_FILE_TYPE64=RmlsZQ==:SOURCE_FILE_BYTES64=AQIDBA==:SOURCE_FILE_NAME64=bGFiZWwtc291cmNlLmJpbg==:METADATA_COUNT=6:M1_KEYS=3:M1_FIRST=1:M1_LAST=1:M1_LABEL64=VEVYVF/Xqda414HXnNeV1rnXnV/wn42S:M2_KEYS=3:M2_FIRST=2:M2_LAST=2:M2_LABEL64=:M3_KEYS=3:M3_FIRST=3:M3_LAST=3:M3_LABEL64=TUFQX0xBQkVM:M4_KEYS=3:M4_FIRST=4:M4_LAST=5:M4_LABEL64=TElTVF9MQUJFTA==:M5_KEYS=3:M5_FIRST=6:M5_LAST=6:M5_LABEL64=REFURV9MQUJFTA==:M6_KEYS=3:M6_FIRST=7:M6_LAST=7:M6_LABEL64=RklMRV9MQUJFTA==:RAW_CLEAR=0:METADATA_CLEAR=0"
    ;;
  qa-debug-label-dated-v2)
    TARGET_FIXTURE_ID="qa-debug-label-dated-v2"
    LEGACY_COUNTERPART_ID="qa-debug-label-dated-legacy"
    EXPECTED_SHORTCUT_NAME="qa-debug-label-dated-v2"
    EXPECTED_SEMANTIC_OUTPUT="CHERRI_QA_DEBUG_LABEL:MODE=dated:BUFFER_COUNT=7:TEXT_TYPE64=VGV4dA==:TEXT64=16LXqNeaIPCfjZI=:NUMBER_TYPE64=TnVtYmVy:NUMBER=7.25:MAP_TYPE64=RGljdGlvbmFyeQ==:MAP_KEYS=2:MAP_KIND64=TEFCRUxfTUFQ:MAP_VALUE=7.25:LIST_COUNT=2:LIST_FIRST64=bGVmdA==:LIST_LAST64=cmlnaHQ=:DATE_TYPE64=RGF0ZQ==:DATE_COUNT=1:DATE_DELTA_SECONDS=0:DATE_SOURCE_DELTA_SECONDS=0:FILE_TYPE64=RmlsZQ==:FILE_COUNT=1:FILE_BYTES64=AQIDBA==:FILE_NAME64=bGFiZWwtc291cmNlLmJpbg==:SOURCE_FILE_TYPE64=RmlsZQ==:SOURCE_FILE_BYTES64=AQIDBA==:SOURCE_FILE_NAME64=bGFiZWwtc291cmNlLmJpbg==:METADATA_COUNT=6:M1_KEYS=3:M1_FIRST=1:M1_LAST=1:M1_LABEL64=VEVYVF/Xqda414HXnNeV1rnXnV/wn42S:M2_KEYS=3:M2_FIRST=2:M2_LAST=2:M2_LABEL64=:M3_KEYS=3:M3_FIRST=3:M3_LAST=3:M3_LABEL64=TUFQX0xBQkVM:M4_KEYS=3:M4_FIRST=4:M4_LAST=5:M4_LABEL64=TElTVF9MQUJFTA==:M5_KEYS=3:M5_FIRST=6:M5_LAST=6:M5_LABEL64=REFURV9MQUJFTA==:M6_KEYS=3:M6_FIRST=7:M6_LAST=7:M6_LABEL64=RklMRV9MQUJFTA==:RAW_CLEAR=0:METADATA_CLEAR=0"
    ;;
  qa-debug-label-dated-legacy)
    TARGET_FIXTURE_ID="qa-debug-label-dated-legacy"
    LEGACY_COUNTERPART_ID="qa-debug-label-dated-legacy"
    EXPECTED_SHORTCUT_NAME="qa-debug-label-dated-legacy"
    EXPECTED_SEMANTIC_OUTPUT="CHERRI_QA_DEBUG_LABEL:MODE=dated:BUFFER_COUNT=7:TEXT_TYPE64=VGV4dA==:TEXT64=16LXqNeaIPCfjZI=:NUMBER_TYPE64=TnVtYmVy:NUMBER=7.25:MAP_TYPE64=RGljdGlvbmFyeQ==:MAP_KEYS=2:MAP_KIND64=TEFCRUxfTUFQ:MAP_VALUE=7.25:LIST_COUNT=2:LIST_FIRST64=bGVmdA==:LIST_LAST64=cmlnaHQ=:DATE_TYPE64=RGF0ZQ==:DATE_COUNT=1:DATE_DELTA_SECONDS=0:DATE_SOURCE_DELTA_SECONDS=0:FILE_TYPE64=RmlsZQ==:FILE_COUNT=1:FILE_BYTES64=AQIDBA==:FILE_NAME64=bGFiZWwtc291cmNlLmJpbg==:SOURCE_FILE_TYPE64=RmlsZQ==:SOURCE_FILE_BYTES64=AQIDBA==:SOURCE_FILE_NAME64=bGFiZWwtc291cmNlLmJpbg==:METADATA_COUNT=6:M1_KEYS=3:M1_FIRST=1:M1_LAST=1:M1_LABEL64=VEVYVF/Xqda414HXnNeV1rnXnV/wn42S:M2_KEYS=3:M2_FIRST=2:M2_LAST=2:M2_LABEL64=:M3_KEYS=3:M3_FIRST=3:M3_LAST=3:M3_LABEL64=TUFQX0xBQkVM:M4_KEYS=3:M4_FIRST=4:M4_LAST=5:M4_LABEL64=TElTVF9MQUJFTA==:M5_KEYS=3:M5_FIRST=6:M5_LAST=6:M5_LABEL64=REFURV9MQUJFTA==:M6_KEYS=3:M6_FIRST=7:M6_LAST=7:M6_LABEL64=RklMRV9MQUJFTA==:RAW_CLEAR=0:METADATA_CLEAR=0"
    ;;
  qa-debug-label-typed-v2)
    TARGET_FIXTURE_ID="qa-debug-label-typed-v2"
    LEGACY_COUNTERPART_ID="qa-debug-label-typed-legacy"
    EXPECTED_SHORTCUT_NAME="qa-debug-label-typed-v2"
    EXPECTED_SEMANTIC_OUTPUT="CHERRI_QA_DEBUG_LABEL:MODE=typed:BUFFER_COUNT=7:TEXT_TYPE64=VGV4dA==:TEXT64=16LXqNeaIPCfjZI=:NUMBER_TYPE64=TnVtYmVy:NUMBER=7.25:MAP_TYPE64=RGljdGlvbmFyeQ==:MAP_KEYS=2:MAP_KIND64=TEFCRUxfTUFQ:MAP_VALUE=7.25:LIST_COUNT=2:LIST_FIRST64=bGVmdA==:LIST_LAST64=cmlnaHQ=:DATE_TYPE64=RGF0ZQ==:DATE_COUNT=1:DATE_DELTA_SECONDS=0:DATE_SOURCE_DELTA_SECONDS=0:FILE_TYPE64=RmlsZQ==:FILE_COUNT=1:FILE_BYTES64=AQIDBA==:FILE_NAME64=bGFiZWwtc291cmNlLmJpbg==:SOURCE_FILE_TYPE64=RmlsZQ==:SOURCE_FILE_BYTES64=AQIDBA==:SOURCE_FILE_NAME64=bGFiZWwtc291cmNlLmJpbg==:METADATA_COUNT=6:M1_KEYS=3:M1_FIRST=1:M1_LAST=1:M1_LABEL64=VEVYVF/Xqda414HXnNeV1rnXnV/wn42S:M2_KEYS=3:M2_FIRST=2:M2_LAST=2:M2_LABEL64=:M3_KEYS=3:M3_FIRST=3:M3_LAST=3:M3_LABEL64=TUFQX0xBQkVM:M4_KEYS=3:M4_FIRST=4:M4_LAST=5:M4_LABEL64=TElTVF9MQUJFTA==:M5_KEYS=3:M5_FIRST=6:M5_LAST=6:M5_LABEL64=REFURV9MQUJFTA==:M6_KEYS=3:M6_FIRST=7:M6_LAST=7:M6_LABEL64=RklMRV9MQUJFTA==:RAW_CLEAR=0:METADATA_CLEAR=0"
    ;;
  qa-debug-label-typed-legacy)
    TARGET_FIXTURE_ID="qa-debug-label-typed-legacy"
    LEGACY_COUNTERPART_ID="qa-debug-label-typed-legacy"
    EXPECTED_SHORTCUT_NAME="qa-debug-label-typed-legacy"
    EXPECTED_SEMANTIC_OUTPUT="CHERRI_QA_DEBUG_LABEL:MODE=typed:BUFFER_COUNT=7:TEXT_TYPE64=VGV4dA==:TEXT64=16LXqNeaIPCfjZI=:NUMBER_TYPE64=TnVtYmVy:NUMBER=7.25:MAP_TYPE64=RGljdGlvbmFyeQ==:MAP_KEYS=2:MAP_KIND64=TEFCRUxfTUFQ:MAP_VALUE=7.25:LIST_COUNT=2:LIST_FIRST64=bGVmdA==:LIST_LAST64=cmlnaHQ=:DATE_TYPE64=RGF0ZQ==:DATE_COUNT=1:DATE_DELTA_SECONDS=0:DATE_SOURCE_DELTA_SECONDS=0:FILE_TYPE64=RmlsZQ==:FILE_COUNT=1:FILE_BYTES64=AQIDBA==:FILE_NAME64=bGFiZWwtc291cmNlLmJpbg==:SOURCE_FILE_TYPE64=RmlsZQ==:SOURCE_FILE_BYTES64=AQIDBA==:SOURCE_FILE_NAME64=bGFiZWwtc291cmNlLmJpbg==:METADATA_COUNT=6:M1_KEYS=3:M1_FIRST=1:M1_LAST=1:M1_LABEL64=VEVYVF/Xqda414HXnNeV1rnXnV/wn42S:M2_KEYS=3:M2_FIRST=2:M2_LAST=2:M2_LABEL64=:M3_KEYS=3:M3_FIRST=3:M3_LAST=3:M3_LABEL64=TUFQX0xBQkVM:M4_KEYS=3:M4_FIRST=4:M4_LAST=5:M4_LABEL64=TElTVF9MQUJFTA==:M5_KEYS=3:M5_FIRST=6:M5_LAST=6:M5_LABEL64=REFURV9MQUJFTA==:M6_KEYS=3:M6_FIRST=7:M6_LAST=7:M6_LABEL64=RklMRV9MQUJFTA==:RAW_CLEAR=0:METADATA_CLEAR=0"
    ;;
  qa-debug-label-dated-typed-v2)
    TARGET_FIXTURE_ID="qa-debug-label-dated-typed-v2"
    LEGACY_COUNTERPART_ID="qa-debug-label-dated-typed-legacy"
    EXPECTED_SHORTCUT_NAME="qa-debug-label-dated-typed-v2"
    EXPECTED_SEMANTIC_OUTPUT="CHERRI_QA_DEBUG_LABEL:MODE=dated-typed:BUFFER_COUNT=7:TEXT_TYPE64=VGV4dA==:TEXT64=16LXqNeaIPCfjZI=:NUMBER_TYPE64=TnVtYmVy:NUMBER=7.25:MAP_TYPE64=RGljdGlvbmFyeQ==:MAP_KEYS=2:MAP_KIND64=TEFCRUxfTUFQ:MAP_VALUE=7.25:LIST_COUNT=2:LIST_FIRST64=bGVmdA==:LIST_LAST64=cmlnaHQ=:DATE_TYPE64=RGF0ZQ==:DATE_COUNT=1:DATE_DELTA_SECONDS=0:DATE_SOURCE_DELTA_SECONDS=0:FILE_TYPE64=RmlsZQ==:FILE_COUNT=1:FILE_BYTES64=AQIDBA==:FILE_NAME64=bGFiZWwtc291cmNlLmJpbg==:SOURCE_FILE_TYPE64=RmlsZQ==:SOURCE_FILE_BYTES64=AQIDBA==:SOURCE_FILE_NAME64=bGFiZWwtc291cmNlLmJpbg==:METADATA_COUNT=6:M1_KEYS=3:M1_FIRST=1:M1_LAST=1:M1_LABEL64=VEVYVF/Xqda414HXnNeV1rnXnV/wn42S:M2_KEYS=3:M2_FIRST=2:M2_LAST=2:M2_LABEL64=:M3_KEYS=3:M3_FIRST=3:M3_LAST=3:M3_LABEL64=TUFQX0xBQkVM:M4_KEYS=3:M4_FIRST=4:M4_LAST=5:M4_LABEL64=TElTVF9MQUJFTA==:M5_KEYS=3:M5_FIRST=6:M5_LAST=6:M5_LABEL64=REFURV9MQUJFTA==:M6_KEYS=3:M6_FIRST=7:M6_LAST=7:M6_LABEL64=RklMRV9MQUJFTA==:RAW_CLEAR=0:METADATA_CLEAR=0"
    ;;
  qa-debug-label-dated-typed-legacy)
    TARGET_FIXTURE_ID="qa-debug-label-dated-typed-legacy"
    LEGACY_COUNTERPART_ID="qa-debug-label-dated-typed-legacy"
    EXPECTED_SHORTCUT_NAME="qa-debug-label-dated-typed-legacy"
    EXPECTED_SEMANTIC_OUTPUT="CHERRI_QA_DEBUG_LABEL:MODE=dated-typed:BUFFER_COUNT=7:TEXT_TYPE64=VGV4dA==:TEXT64=16LXqNeaIPCfjZI=:NUMBER_TYPE64=TnVtYmVy:NUMBER=7.25:MAP_TYPE64=RGljdGlvbmFyeQ==:MAP_KEYS=2:MAP_KIND64=TEFCRUxfTUFQ:MAP_VALUE=7.25:LIST_COUNT=2:LIST_FIRST64=bGVmdA==:LIST_LAST64=cmlnaHQ=:DATE_TYPE64=RGF0ZQ==:DATE_COUNT=1:DATE_DELTA_SECONDS=0:DATE_SOURCE_DELTA_SECONDS=0:FILE_TYPE64=RmlsZQ==:FILE_COUNT=1:FILE_BYTES64=AQIDBA==:FILE_NAME64=bGFiZWwtc291cmNlLmJpbg==:SOURCE_FILE_TYPE64=RmlsZQ==:SOURCE_FILE_BYTES64=AQIDBA==:SOURCE_FILE_NAME64=bGFiZWwtc291cmNlLmJpbg==:METADATA_COUNT=6:M1_KEYS=3:M1_FIRST=1:M1_LAST=1:M1_LABEL64=VEVYVF/Xqda414HXnNeV1rnXnV/wn42S:M2_KEYS=3:M2_FIRST=2:M2_LAST=2:M2_LABEL64=:M3_KEYS=3:M3_FIRST=3:M3_LAST=3:M3_LABEL64=TUFQX0xBQkVM:M4_KEYS=3:M4_FIRST=4:M4_LAST=5:M4_LABEL64=TElTVF9MQUJFTA==:M5_KEYS=3:M5_FIRST=6:M5_LAST=6:M5_LABEL64=REFURV9MQUJFTA==:M6_KEYS=3:M6_FIRST=7:M6_LAST=7:M6_LABEL64=RklMRV9MQUJFTA==:RAW_CLEAR=0:METADATA_CLEAR=0"
    ;;
  qa-debug-label-text-v2)
    TARGET_FIXTURE_ID="qa-debug-label-text-v2"
    LEGACY_COUNTERPART_ID="qa-debug-label-text-legacy"
    EXPECTED_SHORTCUT_NAME="qa-debug-label-text-v2"
    EXPECTED_SEMANTIC_OUTPUT="CHERRI_QA_DEBUG_LABEL:MODE=text:BUFFER_COUNT=7:TEXT_TYPE64=VGV4dA==:TEXT64=16LXqNeaIPCfjZI=:NUMBER_TYPE64=TnVtYmVy:NUMBER=7.25:MAP_TYPE64=RGljdGlvbmFyeQ==:MAP_KEYS=2:MAP_KIND64=TEFCRUxfTUFQ:MAP_VALUE=7.25:LIST_COUNT=2:LIST_FIRST64=bGVmdA==:LIST_LAST64=cmlnaHQ=:DATE_TYPE64=RGF0ZQ==:DATE_COUNT=1:DATE_DELTA_SECONDS=0:DATE_SOURCE_DELTA_SECONDS=0:FILE_TYPE64=RmlsZQ==:FILE_COUNT=1:FILE_BYTES64=AQIDBA==:FILE_NAME64=bGFiZWwtc291cmNlLmJpbg==:SOURCE_FILE_TYPE64=RmlsZQ==:SOURCE_FILE_BYTES64=AQIDBA==:SOURCE_FILE_NAME64=bGFiZWwtc291cmNlLmJpbg==:METADATA_COUNT=6:M1_KEYS=3:M1_FIRST=1:M1_LAST=1:M1_LABEL64=VEVYVF/Xqda414HXnNeV1rnXnV/wn42S:M2_KEYS=3:M2_FIRST=2:M2_LAST=2:M2_LABEL64=:M3_KEYS=3:M3_FIRST=3:M3_LAST=3:M3_LABEL64=TUFQX0xBQkVM:M4_KEYS=3:M4_FIRST=4:M4_LAST=5:M4_LABEL64=TElTVF9MQUJFTA==:M5_KEYS=3:M5_FIRST=6:M5_LAST=6:M5_LABEL64=REFURV9MQUJFTA==:M6_KEYS=3:M6_FIRST=7:M6_LAST=7:M6_LABEL64=RklMRV9MQUJFTA==:RAW_CLEAR=0:METADATA_CLEAR=0"
    ;;
  qa-debug-label-text-legacy)
    TARGET_FIXTURE_ID="qa-debug-label-text-legacy"
    LEGACY_COUNTERPART_ID="qa-debug-label-text-legacy"
    EXPECTED_SHORTCUT_NAME="qa-debug-label-text-legacy"
    EXPECTED_SEMANTIC_OUTPUT="CHERRI_QA_DEBUG_LABEL:MODE=text:BUFFER_COUNT=7:TEXT_TYPE64=VGV4dA==:TEXT64=16LXqNeaIPCfjZI=:NUMBER_TYPE64=TnVtYmVy:NUMBER=7.25:MAP_TYPE64=RGljdGlvbmFyeQ==:MAP_KEYS=2:MAP_KIND64=TEFCRUxfTUFQ:MAP_VALUE=7.25:LIST_COUNT=2:LIST_FIRST64=bGVmdA==:LIST_LAST64=cmlnaHQ=:DATE_TYPE64=RGF0ZQ==:DATE_COUNT=1:DATE_DELTA_SECONDS=0:DATE_SOURCE_DELTA_SECONDS=0:FILE_TYPE64=RmlsZQ==:FILE_COUNT=1:FILE_BYTES64=AQIDBA==:FILE_NAME64=bGFiZWwtc291cmNlLmJpbg==:SOURCE_FILE_TYPE64=RmlsZQ==:SOURCE_FILE_BYTES64=AQIDBA==:SOURCE_FILE_NAME64=bGFiZWwtc291cmNlLmJpbg==:METADATA_COUNT=6:M1_KEYS=3:M1_FIRST=1:M1_LAST=1:M1_LABEL64=VEVYVF/Xqda414HXnNeV1rnXnV/wn42S:M2_KEYS=3:M2_FIRST=2:M2_LAST=2:M2_LABEL64=:M3_KEYS=3:M3_FIRST=3:M3_LAST=3:M3_LABEL64=TUFQX0xBQkVM:M4_KEYS=3:M4_FIRST=4:M4_LAST=5:M4_LABEL64=TElTVF9MQUJFTA==:M5_KEYS=3:M5_FIRST=6:M5_LAST=6:M5_LABEL64=REFURV9MQUJFTA==:M6_KEYS=3:M6_FIRST=7:M6_LAST=7:M6_LABEL64=RklMRV9MQUJFTA==:RAW_CLEAR=0:METADATA_CLEAR=0"
    ;;
  qa-debug-label-dated-text-v2)
    TARGET_FIXTURE_ID="qa-debug-label-dated-text-v2"
    LEGACY_COUNTERPART_ID="qa-debug-label-dated-text-legacy"
    EXPECTED_SHORTCUT_NAME="qa-debug-label-dated-text-v2"
    EXPECTED_SEMANTIC_OUTPUT="CHERRI_QA_DEBUG_LABEL:MODE=dated-text:BUFFER_COUNT=7:TEXT_TYPE64=VGV4dA==:TEXT64=16LXqNeaIPCfjZI=:NUMBER_TYPE64=TnVtYmVy:NUMBER=7.25:MAP_TYPE64=RGljdGlvbmFyeQ==:MAP_KEYS=2:MAP_KIND64=TEFCRUxfTUFQ:MAP_VALUE=7.25:LIST_COUNT=2:LIST_FIRST64=bGVmdA==:LIST_LAST64=cmlnaHQ=:DATE_TYPE64=RGF0ZQ==:DATE_COUNT=1:DATE_DELTA_SECONDS=0:DATE_SOURCE_DELTA_SECONDS=0:FILE_TYPE64=RmlsZQ==:FILE_COUNT=1:FILE_BYTES64=AQIDBA==:FILE_NAME64=bGFiZWwtc291cmNlLmJpbg==:SOURCE_FILE_TYPE64=RmlsZQ==:SOURCE_FILE_BYTES64=AQIDBA==:SOURCE_FILE_NAME64=bGFiZWwtc291cmNlLmJpbg==:METADATA_COUNT=6:M1_KEYS=3:M1_FIRST=1:M1_LAST=1:M1_LABEL64=VEVYVF/Xqda414HXnNeV1rnXnV/wn42S:M2_KEYS=3:M2_FIRST=2:M2_LAST=2:M2_LABEL64=:M3_KEYS=3:M3_FIRST=3:M3_LAST=3:M3_LABEL64=TUFQX0xBQkVM:M4_KEYS=3:M4_FIRST=4:M4_LAST=5:M4_LABEL64=TElTVF9MQUJFTA==:M5_KEYS=3:M5_FIRST=6:M5_LAST=6:M5_LABEL64=REFURV9MQUJFTA==:M6_KEYS=3:M6_FIRST=7:M6_LAST=7:M6_LABEL64=RklMRV9MQUJFTA==:RAW_CLEAR=0:METADATA_CLEAR=0"
    ;;
  qa-debug-label-dated-text-legacy)
    TARGET_FIXTURE_ID="qa-debug-label-dated-text-legacy"
    LEGACY_COUNTERPART_ID="qa-debug-label-dated-text-legacy"
    EXPECTED_SHORTCUT_NAME="qa-debug-label-dated-text-legacy"
    EXPECTED_SEMANTIC_OUTPUT="CHERRI_QA_DEBUG_LABEL:MODE=dated-text:BUFFER_COUNT=7:TEXT_TYPE64=VGV4dA==:TEXT64=16LXqNeaIPCfjZI=:NUMBER_TYPE64=TnVtYmVy:NUMBER=7.25:MAP_TYPE64=RGljdGlvbmFyeQ==:MAP_KEYS=2:MAP_KIND64=TEFCRUxfTUFQ:MAP_VALUE=7.25:LIST_COUNT=2:LIST_FIRST64=bGVmdA==:LIST_LAST64=cmlnaHQ=:DATE_TYPE64=RGF0ZQ==:DATE_COUNT=1:DATE_DELTA_SECONDS=0:DATE_SOURCE_DELTA_SECONDS=0:FILE_TYPE64=RmlsZQ==:FILE_COUNT=1:FILE_BYTES64=AQIDBA==:FILE_NAME64=bGFiZWwtc291cmNlLmJpbg==:SOURCE_FILE_TYPE64=RmlsZQ==:SOURCE_FILE_BYTES64=AQIDBA==:SOURCE_FILE_NAME64=bGFiZWwtc291cmNlLmJpbg==:METADATA_COUNT=6:M1_KEYS=3:M1_FIRST=1:M1_LAST=1:M1_LABEL64=VEVYVF/Xqda414HXnNeV1rnXnV/wn42S:M2_KEYS=3:M2_FIRST=2:M2_LAST=2:M2_LABEL64=:M3_KEYS=3:M3_FIRST=3:M3_LAST=3:M3_LABEL64=TUFQX0xBQkVM:M4_KEYS=3:M4_FIRST=4:M4_LAST=5:M4_LABEL64=TElTVF9MQUJFTA==:M5_KEYS=3:M5_FIRST=6:M5_LAST=6:M5_LABEL64=REFURV9MQUJFTA==:M6_KEYS=3:M6_FIRST=7:M6_LAST=7:M6_LABEL64=RklMRV9MQUJFTA==:RAW_CLEAR=0:METADATA_CLEAR=0"
    ;;
  qa-collect-counts-v2)
    TARGET_FIXTURE_ID="qa-collect-counts-v2"
    LEGACY_COUNTERPART_ID="qa-collect-counts-legacy"
    EXPECTED_SHORTCUT_NAME="qa-collect-counts-v2"
    EXPECTED_SEMANTIC_OUTPUT="CHERRI_QA_COLLECT_COUNTS:empty=0:single=1:one:many=100:0:99"
    ;;
  qa-collect-counts-legacy)
    TARGET_FIXTURE_ID="qa-collect-counts-legacy"
    LEGACY_COUNTERPART_ID="qa-collect-counts-legacy"
    EXPECTED_SHORTCUT_NAME="qa-collect-counts-legacy"
    EXPECTED_SEMANTIC_OUTPUT="CHERRI_QA_COLLECT_COUNTS:empty=0:single=1:one:many=100:0:99"
    ;;
  qa-collect-dictionaries-v2)
    TARGET_FIXTURE_ID="qa-collect-dictionaries-v2"
    LEGACY_COUNTERPART_ID="qa-collect-dictionaries-legacy"
    EXPECTED_SHORTCUT_NAME="qa-collect-dictionaries-v2"
    EXPECTED_SEMANTIC_OUTPUT="CHERRI_QA_COLLECT_DICTIONARIES:count=2:first=first:last=second:type=Dictionary:nested=2:a:b"
    ;;
  qa-collect-dictionaries-legacy)
    TARGET_FIXTURE_ID="qa-collect-dictionaries-legacy"
    LEGACY_COUNTERPART_ID="qa-collect-dictionaries-legacy"
    EXPECTED_SHORTCUT_NAME="qa-collect-dictionaries-legacy"
    EXPECTED_SEMANTIC_OUTPUT="CHERRI_QA_COLLECT_DICTIONARIES:count=2:first=first:last=second:type=Dictionary:nested=2:a:b"
    ;;
  qa-storage-text-same-run)
    TARGET_FIXTURE_ID="qa-storage-text-same-run-v2"
    LEGACY_COUNTERPART_ID="qa-functional-legacy"
    EXPECTED_SHORTCUT_NAME="qa-storage-text-same-run-v2"
    EXPECTED_SEMANTIC_OUTPUT="CHERRI_QA_STORAGE_TEXT_RESULT:TYPE_OK:VALUE_OK"
    ;;
  qa-storage-scope)
    TARGET_FIXTURE_ID="qa-storage-scope-v2"
    LEGACY_COUNTERPART_ID="qa-functional-legacy"
    EXPECTED_SHORTCUT_NAME="qa-storage-scope-v2"
    EXPECTED_SEMANTIC_OUTPUT="CHERRI_QA_STORAGE_SCOPE_RESULT:GLOBAL_TYPE_OK:GLOBAL_VALUE_OK:LOCAL_TYPE_OK:LOCAL_VALUE_OK"
    ;;
  qa-storage-text-cross-run)
    TARGET_FIXTURE_ID="qa-storage-text-cross-run-v2"
    LEGACY_COUNTERPART_ID="qa-functional-legacy"
    EXPECTED_SHORTCUT_NAME="qa-storage-text-cross-run-v2"
    EXPECTED_SEMANTIC_OUTPUT="CHERRI_QA_STORAGE_CROSS_RUN_RESULT:READ:TYPE_OK:VALUE_OK"
    STORAGE_CROSS_RUN_ENABLED="true"
    ;;
  qa-http-json-control)
    TARGET_FIXTURE_ID="qa-http-json-control-v2"
    LEGACY_COUNTERPART_ID="qa-functional-legacy"
    EXPECTED_SHORTCUT_NAME="qa-http-json-control-v2"
    EXPECTED_SEMANTIC_OUTPUT="CHERRI_QA_HTTP_JSON_CONTROL_RESULT:DICTIONARY_OK:NEXT_OK:ITEMS_OK"
    ;;
  qa-number-coercion-control)
    TARGET_FIXTURE_ID="qa-number-coercion-control-v2"
    LEGACY_COUNTERPART_ID="qa-functional-legacy"
    EXPECTED_SHORTCUT_NAME="qa-number-coercion-control-v2"
    EXPECTED_SEMANTIC_OUTPUT="CHERRI_QA_NUMBER_COERCION_RESULT:DEQ=Y:DNE=N:REQ=Y:RNE=N:BEQ=Y:BNE=N:ONE_NE_ZERO=Y:ZERO_NE_ONE=Y:ONE_EQ_ONE=Y"
    ;;
  qa-number-comparison-diag)
    TARGET_FIXTURE_ID="qa-number-comparison-diag-v2"
    LEGACY_COUNTERPART_ID="qa-functional-legacy"
    EXPECTED_SHORTCUT_NAME="qa-number-comparison-diag-v2"
    EXPECTED_SEMANTIC_OUTPUT="CHERRI_QA_NUMBER_COMPARISON_RESULT:DEQ=Y:DNE=N:REQ=Y:RNE=N:BEQ=Y:BNE=N:ONE_NE_ZERO=Y:ZERO_NE_ONE=Y:ONE_EQ_ONE=Y"
    ;;
  qa-http-json-fetch)
    TARGET_FIXTURE_ID="qa-http-json-fetch-v2"
    LEGACY_COUNTERPART_ID="qa-functional-legacy"
    EXPECTED_SHORTCUT_NAME="qa-http-json-fetch-v2"
    EXPECTED_SEMANTIC_OUTPUT="CHERRI_QA_HTTP_JSON_FETCH_RESULT:DICTIONARY_OK:NEXT_OK:ITEMS_OK"
    ;;
  qa-storage-delete)
    TARGET_FIXTURE_ID="qa-storage-delete-v2"
    LEGACY_COUNTERPART_ID="qa-functional-legacy"
    EXPECTED_SHORTCUT_NAME="qa-storage-delete-v2"
    EXPECTED_SEMANTIC_OUTPUT="CHERRI_QA_STORAGE_DELETE_RESULT:BEFORE_OK:NO_VALUE_OK"
    ;;
  qa-paginate-ordered)
    TARGET_FIXTURE_ID="qa-paginate-ordered-v2"
    TARGET_SOURCE_RELATIVE="p15-paginate-probes/qa-paginate-ordered-v2.cherri"
    LEGACY_COUNTERPART_ID="qa-functional-legacy"
    EXPECTED_SHORTCUT_NAME="qa-paginate-ordered-v2"
    EXPECTED_SEMANTIC_OUTPUT="CHERRI_QA_PAGINATE_PROBE_RESULT:P15_PAGINATE_ORDERED_OK"
    ;;
  qa-paginate-cycle)
    TARGET_FIXTURE_ID="qa-paginate-cycle-v2"
    TARGET_SOURCE_RELATIVE="p15-paginate-probes/qa-paginate-cycle-v2.cherri"
    LEGACY_COUNTERPART_ID="qa-functional-legacy"
    EXPECTED_SHORTCUT_NAME="qa-paginate-cycle-v2"
    EXPECTED_SEMANTIC_OUTPUT="CHERRI_QA_PAGINATE_PROBE_RESULT:P15_PAGINATE_CYCLE_OK"
    ;;
  qa-paginate-page-cap)
    TARGET_FIXTURE_ID="qa-paginate-page-cap-v2"
    TARGET_SOURCE_RELATIVE="p15-paginate-probes/qa-paginate-page-cap-v2.cherri"
    LEGACY_COUNTERPART_ID="qa-functional-legacy"
    EXPECTED_SHORTCUT_NAME="qa-paginate-page-cap-v2"
    EXPECTED_SEMANTIC_OUTPUT="CHERRI_QA_PAGINATE_PROBE_RESULT:P15_PAGINATE_PAGE_CAP_OK"
    ;;
  qa-paginate-item-cap)
    TARGET_FIXTURE_ID="qa-paginate-item-cap-v2"
    TARGET_SOURCE_RELATIVE="p15-paginate-probes/qa-paginate-item-cap-v2.cherri"
    LEGACY_COUNTERPART_ID="qa-functional-legacy"
    EXPECTED_SHORTCUT_NAME="qa-paginate-item-cap-v2"
    EXPECTED_SEMANTIC_OUTPUT="CHERRI_QA_PAGINATE_PROBE_RESULT:P15_PAGINATE_ITEM_CAP_OK"
    ;;
  qa-paginate-empty)
    TARGET_FIXTURE_ID="qa-paginate-empty-v2"
    TARGET_SOURCE_RELATIVE="p15-paginate-probes/qa-paginate-empty-v2.cherri"
    LEGACY_COUNTERPART_ID="qa-functional-legacy"
    EXPECTED_SHORTCUT_NAME="qa-paginate-empty-v2"
    EXPECTED_SEMANTIC_OUTPUT="CHERRI_QA_PAGINATE_PROBE_RESULT:P15_PAGINATE_EMPTY_OK"
    ;;
  qa-label-file-carrier)
    TARGET_FIXTURE_ID="qa-label-file-carrier-v2"
    TARGET_SOURCE_RELATIVE="p15-label-probes/qa-label-file-carrier-v2.cherri"
    LEGACY_COUNTERPART_ID="qa-functional-legacy"
    EXPECTED_SHORTCUT_NAME="qa-label-file-carrier-v2"
    EXPECTED_SEMANTIC_OUTPUT="CHERRI_QA_LABEL_PROBE_RESULT:P15_LABEL_FILE_CARRIER_OK"
    ;;
  qa-label-raw-empty)
    TARGET_FIXTURE_ID="qa-label-raw-empty-v2"
    TARGET_SOURCE_RELATIVE="p15-label-probes/qa-label-raw-empty-v2.cherri"
    LEGACY_COUNTERPART_ID="qa-functional-legacy"
    EXPECTED_SHORTCUT_NAME="qa-label-raw-empty-v2"
    EXPECTED_SEMANTIC_OUTPUT="CHERRI_QA_LABEL_PROBE_RESULT:P15_LABEL_RAW_EMPTY_OK"
    ;;
  qa-label-raw-grouping)
    TARGET_FIXTURE_ID="qa-label-raw-grouping-v2"
    TARGET_SOURCE_RELATIVE="p15-label-probes/qa-label-raw-grouping-v2.cherri"
    LEGACY_COUNTERPART_ID="qa-functional-legacy"
    EXPECTED_SHORTCUT_NAME="qa-label-raw-grouping-v2"
    EXPECTED_SEMANTIC_OUTPUT="CHERRI_QA_LABEL_PROBE_RESULT:P15_LABEL_RAW_GROUPING_OK"
    ;;
  qa-binding-capture)
    TARGET_FIXTURE_ID="CherriBindingCapturePOC"
    TARGET_SOURCE_RELATIVE="p15-binding-defaults/CherriBindingCapturePOC.cherri"
    LEGACY_COUNTERPART_ID="qa-functional-legacy"
    EXPECTED_SHORTCUT_NAME="CherriBindingCapturePOC"
    EXPECTED_SEMANTIC_OUTPUT="CHERRI_QA_BINDING_CAPTURE_RESULT:Q0hFUlJJX0JJTkRJTkdfQ0FQVFVSRV9PSwpOVU1fQ0FQVFVSRUQ9MQpOVU1fQUdBSU49MQpDTElQX0NBUFRVUkVEPWNsaXBfaW5pdGlhbApDTElQX0FHQUlOPWNsaXBfaW5pdGlhbA=="
    ;;
  qa-typed-defaults)
    TARGET_FIXTURE_ID="CherriTypedDefaultsPOC"
    TARGET_SOURCE_RELATIVE="p15-binding-defaults/CherriTypedDefaultsPOC.cherri"
    LEGACY_COUNTERPART_ID="qa-functional-legacy"
    EXPECTED_SHORTCUT_NAME="CherriTypedDefaultsPOC"
    EXPECTED_SEMANTIC_OUTPUT="CHERRI_QA_TYPED_DEFAULTS_RESULT:Q0hFUlJJX1RZUEVEX0RFRkFVTFRTX09LCkRFRkFVTFRTX0VYUExJQ0lUPTA6MDo6MDowCkRFRkFVTFRTX09NSVRURUQ9MTo5OmRlZjoxOjEKREVGQVVMVFNfTUlYRUQ9MDo5OmRlZjowOjE="
    ;;
  qa-n03-text-equality-diagnostic)
    # Exact-equality target only; contradictory actual fields remain failures.
    TARGET_FIXTURE_ID="qa-n03-text-equality-diagnostic-v2"
    LEGACY_COUNTERPART_ID="qa-functional-legacy"
    EXPECTED_SHORTCUT_NAME="qa-n03-text-equality-diagnostic-v2"
    EXPECTED_SEMANTIC_OUTPUT="CHERRI_QA_N03_TEXT_EQ_DIAG:LEFT64=YWJj:SAME64=YWJj:CASE64=QUJD:FOLD_SAME_EQ=Y:FOLD_SAME_NE=N:FOLD_CASE_EQ=N:FOLD_CASE_NE=Y:DYN_SAME_EQ=Y:DYN_SAME_NE=N:DYN_CASE_EQ=N:DYN_CASE_NE=Y:IF_SAME_EQ=Y:IF_SAME_NE=N:IF_CASE_EQ=N:IF_CASE_NE=Y:MATCH_SAME_CS=1:MATCH_CASE_CS=0:MATCH_CASE_CI=1"
    ;;
  qa-nested-list-control|qa-nested-list-candidate)
    # Fixed observation contract; no callback-derived expected value.
    TARGET_FIXTURE_ID="${SAMPLE_ID}-v2"
    LEGACY_COUNTERPART_ID="qa-functional-legacy"
    EXPECTED_SHORTCUT_NAME="$TARGET_FIXTURE_ID"
    EXPECTED_SEMANTIC_OUTPUT=""
    ;;
  qa-optional-forwarding-diagnostic)
    # Tagged Optional diagnostic only. Preserve every contradictory actual field.
    TARGET_FIXTURE_ID="qa-optional-forwarding-diagnostic-v2"
    LEGACY_COUNTERPART_ID="qa-functional-legacy"
    EXPECTED_SHORTCUT_NAME="qa-optional-forwarding-diagnostic-v2"
    EXPECTED_SEMANTIC_OUTPUT="CHERRI_OPTIONAL_FORWARDING_DIAGNOSTIC_V1:NONE=NONE:EMPTY_TEXT=TEXT[]:FALSE=BOOL[false]:ZERO=NUMBER[0]:EMPTY_LIST=LIST[0]:EMPTY_MAP=MAP[0]:LIST=LIST[2|A|B]:MAP=MAP[1|VALUE|Dictionary]:SNAPSHOT=NONE:WRITTEN=TEXT[]:DEFAULT=TEXT[DEFAULT]"
    ;;
  qa-file-materialization-diagnostic)
    # Additive characterization: desired File contract remains independently strict.
    TARGET_FIXTURE_ID="qa-file-materialization-diagnostic-v2"
    LEGACY_COUNTERPART_ID="qa-functional-legacy"
    EXPECTED_SHORTCUT_NAME="qa-file-materialization-diagnostic-v2"
    EXPECTED_SEMANTIC_OUTPUT="CHERRI_QA_FILE_MATERIALIZATION_DIAG:JSON_TYPE64=VGV4dA==:DECODE_TYPE64=RmlsZQ==:NAMED_TYPE64=RmlsZQ==:EMPTY_TYPE64=RmlsZQ==:ONE_TYPE64=RmlsZQ==:BINARY_TYPE64=RmlsZQ==:JSON_COUNT=1:DECODE_COUNT=1:NAMED_COUNT=1:EMPTY_ENTRIES=0:ONE_ENTRIES=1:JSON_BYTES64=eyJib3VuZGFyeSI6IlBVQkxJQ19KU09OX1dJVE5FU1MiLCJlbnRyaWVzIjpbXX0=:DECODE_BYTES64=eyJib3VuZGFyeSI6IlBVQkxJQ19KU09OX1dJVE5FU1MiLCJlbnRyaWVzIjpbXX0=:NAMED_BYTES64=eyJib3VuZGFyeSI6IlBVQkxJQ19KU09OX1dJVE5FU1MiLCJlbnRyaWVzIjpbXX0=:PAYLOAD_TEXT64=Y1hWdmRHVWdJaUJpWVdOcmMyeGhjMmdnWENCdVpYZHNhVzVsQ2tobFluSmxkeURYcWRhNDE0SFhuTmVWMXJuWG5TQmphR1Z5Y25rZzhKK05raUIwYjJ0bGJpQlRXVTVVU0VWVVNVTmZVMFZEVWtWVQ==:RESTORED64=cXVvdGUgIiBiYWNrc2xhc2ggXCBuZXdsaW5lCkhlYnJldyDXqda414HXnNeV1rnXnSBjaGVycnkg8J+NkiB0b2tlbiBTWU5USEVUSUNfU0VDUkVU:NAME64=Ym91bmRhcnkuanNvbg==:STATUS64=VEVYVF9QQVlMT0FEX0VOQ09ERUQ="
    ;;
  qa-native-boundary-diagnostic)
    # Causal characterization only; original acceptance probes remain unchanged.
    TARGET_FIXTURE_ID="qa-native-boundary-diagnostic-v2"
    LEGACY_COUNTERPART_ID="qa-functional-legacy"
    EXPECTED_SHORTCUT_NAME="$TARGET_FIXTURE_ID"
    EXPECTED_SEMANTIC_OUTPUT="CHERRI_QA_NATIVE_BOUNDARY_DIAG:ENTRIES=1:FIRST64=Y1hWdmRHVWdJaUJpWVdOcmMyeGhjMmdnWENCdVpYZHNhVzVsQ2tobFluSmxkeURYcWRhNDE0SFhuTmVWMXJuWG5TQmphR1Z5Y25rZzhKK05raUIwYjJ0bGJpQlRXVTVVU0VWVVNVTmZVMFZEVWtWVQ==:LAST64=Y1hWdmRHVWdJaUJpWVdOcmMyeGhjMmdnWENCdVpYZHNhVzVsQ2tobFluSmxkeURYcWRhNDE0SFhuTmVWMXJuWG5TQmphR1Z5Y25rZzhKK05raUIwYjJ0bGJpQlRXVTVVU0VWVVNVTmZVMFZEVWtWVQ==:TYPE_FILE=Y:TYPE_TEXT=N:TYPE_DICT=N:FILE_TEXT=N:FILE_DICT=N:COERCED_FILE=Y:COERCED_BYTES=Y:COERCED_NAME=Y:NUM_ONE=Y:CLIP_INITIAL=Y:ALIAS_INITIAL=Y:DIRECT_LEN=140:DIRECT_EQ=Y:GETVAR_LEN=140:GETVAR_EQ=Y:TOKEN_LEN=140:TOKEN_EQ=Y:ACTION_LEN=140:ACTION_EQ=Y"
    ;;
  qa-export-unicode-diagnostic)
    # Causal diagnostic only; the original Unicode acceptance stays separate.
    TARGET_FIXTURE_ID="qa-export-unicode-diagnostic-v2"
    LEGACY_COUNTERPART_ID="qa-functional-legacy"
    EXPECTED_SHORTCUT_NAME="qa-export-unicode-diagnostic-v2"
    EXPECTED_SEMANTIC_OUTPUT="CHERRI_QA_EXPORT_UNICODE_DIAG:TYPE=Y:NAME=Y:COUNT=Y:STATUS=Y:PAYLOAD=Y:RESTORED_EQ=Y:RESTORED_NE=N:LITERAL_EQ=Y:LITERAL_NE=N:UNCHANGED=Y:ORIGINAL64=cXVvdGUgIiBiYWNrc2xhc2ggXCBuZXdsaW5lCkhlYnJldyDXqda414HXnNeV1rnXnSBjaGVycnkg8J+NkiB0b2tlbiBTWU5USEVUSUNfU0VDUkVU:PAYLOAD_TEXT64=Y1hWdmRHVWdJaUJpWVdOcmMyeGhjMmdnWENCdVpYZHNhVzVsQ2tobFluSmxkeURYcWRhNDE0SFhuTmVWMXJuWG5TQmphR1Z5Y25rZzhKK05raUIwYjJ0bGJpQlRXVTVVU0VWVVNVTmZVMFZEVWtWVQ==:DECODED_FILE64=cXVvdGUgIiBiYWNrc2xhc2ggXCBuZXdsaW5lCkhlYnJldyDXqda414HXnNeV1rnXnSBjaGVycnkg8J+NkiB0b2tlbiBTWU5USEVUSUNfU0VDUkVU:RESTORED64=cXVvdGUgIiBiYWNrc2xhc2ggXCBuZXdsaW5lCkhlYnJldyDXqda414HXnNeV1rnXnSBjaGVycnkg8J+NkiB0b2tlbiBTWU5USEVUSUNfU0VDUkVU:LITERAL64=cXVvdGUgIiBiYWNrc2xhc2ggXCBuZXdsaW5lCkhlYnJldyDXqda414HXnNeV1rnXnSBjaGVycnkg8J+NkiB0b2tlbiBTWU5USEVUSUNfU0VDUkVU:AFTER64=cXVvdGUgIiBiYWNrc2xhc2ggXCBuZXdsaW5lCkhlYnJldyDXqda414HXnNeV1rnXnSBjaGVycnkg8J+NkiB0b2tlbiBTWU5USEVUSUNfU0VDUkVU:NAME64=Y2hlcnJpLWRlYnVnLmpzb24=:STATUS64=VEVYVF9QQVlMT0FEX0VOQ09ERUQ="
    ;;
  qa-export-unicode)
    TARGET_FIXTURE_ID="qa-export-unicode-v2"
    TARGET_SOURCE_RELATIVE="p15-export-probes/qa-export-unicode-v2.cherri"
    LEGACY_COUNTERPART_ID="qa-functional-legacy"
    EXPECTED_SHORTCUT_NAME="qa-export-unicode-v2"
    EXPECTED_SEMANTIC_OUTPUT="CHERRI_QA_EXPORT_PROBE_RESULT:P15_EXPORT_UNICODE_OK"
    ;;
  qa-export-empty)
    TARGET_FIXTURE_ID="qa-export-empty-v2"
    TARGET_SOURCE_RELATIVE="p15-export-probes/qa-export-empty-v2.cherri"
    LEGACY_COUNTERPART_ID="qa-functional-legacy"
    EXPECTED_SHORTCUT_NAME="qa-export-empty-v2"
    EXPECTED_SEMANTIC_OUTPUT="CHERRI_QA_EXPORT_PROBE_RESULT:P15_EXPORT_EMPTY_OK"
    ;;
  qa-export-thousand)
    TARGET_FIXTURE_ID="qa-export-thousand-v2"
    TARGET_SOURCE_RELATIVE="p15-export-probes/qa-export-thousand-v2.cherri"
    LEGACY_COUNTERPART_ID="qa-functional-legacy"
    EXPECTED_SHORTCUT_NAME="qa-export-thousand-v2"
    EXPECTED_SEMANTIC_OUTPUT="CHERRI_QA_EXPORT_PROBE_RESULT:P15_EXPORT_THOUSAND_OK"
    ;;
  qa-export-redacted)
    TARGET_FIXTURE_ID="qa-export-redacted-v2"
    TARGET_SOURCE_RELATIVE="p15-export-probes/qa-export-redacted-v2.cherri"
    LEGACY_COUNTERPART_ID="qa-functional-legacy"
    EXPECTED_SHORTCUT_NAME="qa-export-redacted-v2"
    EXPECTED_SEMANTIC_OUTPUT="CHERRI_QA_EXPORT_PROBE_RESULT:P15_EXPORT_REDACTED_OK"
    ;;
  qa-export-unsupported-file)
    TARGET_FIXTURE_ID="qa-export-unsupported-file-v2"
    TARGET_SOURCE_RELATIVE="p15-export-probes/qa-export-unsupported-file-v2.cherri"
    LEGACY_COUNTERPART_ID="qa-functional-legacy"
    EXPECTED_SHORTCUT_NAME="qa-export-unsupported-file-v2"
    EXPECTED_SEMANTIC_OUTPUT="CHERRI_QA_EXPORT_PROBE_RESULT:P15_EXPORT_UNSUPPORTED_FILE_OK"
    ;;
  qa-export-text-file)
    TARGET_FIXTURE_ID="qa-export-text-file-v2"
    TARGET_SOURCE_RELATIVE="p15-export-probes/qa-export-text-file-v2.cherri"
    LEGACY_COUNTERPART_ID="qa-functional-legacy"
    EXPECTED_SHORTCUT_NAME="qa-export-text-file-v2"
    EXPECTED_SEMANTIC_OUTPUT="CHERRI_QA_EXPORT_PROBE_RESULT:P15_EXPORT_TEXT_FILE_OK"
    ;;
  qa-require-zero-diag)
    TARGET_FIXTURE_ID="qa-require-zero-diag-v2"
    LEGACY_COUNTERPART_ID="qa-functional-legacy"
    EXPECTED_SHORTCUT_NAME="qa-require-zero-diag-v2"
    EXPECTED_SEMANTIC_OUTPUT="CHERRI_QA_REQUIRE_ZERO_DIAG:L=ok:D=ok:R=ok:B=ok"
    ;;
  qa-label-file-controls)
    TARGET_FIXTURE_ID="qa-label-file-controls-v2"
    LEGACY_COUNTERPART_ID="qa-functional-legacy"
    EXPECTED_SHORTCUT_NAME="qa-label-file-controls-v2"
    EXPECTED_SEMANTIC_OUTPUT="CHERRI_QA_LABEL_FILE_CONTROLS:B=ok:V=ok:VT=ok:L=ok:T=ok:R=ok:S=ok"
    ;;
  qa-export-file-control)
    TARGET_FIXTURE_ID="qa-export-file-control-v2"
    LEGACY_COUNTERPART_ID="qa-functional-legacy"
    EXPECTED_SHORTCUT_NAME="qa-export-file-control-v2"
    EXPECTED_SEMANTIC_OUTPUT="CHERRI_QA_EXPORT_FILE_CONTROL:NAME=ok:TEXT=ok"
    ;;
  qa-export-build-only)
    TARGET_FIXTURE_ID="qa-export-build-only-v2"
    LEGACY_COUNTERPART_ID="qa-functional-legacy"
    EXPECTED_SHORTCUT_NAME="qa-export-build-only-v2"
    EXPECTED_SEMANTIC_OUTPUT="CHERRI_QA_EXPORT_BUILD_ONLY:NAME=ok"
    ;;
  qa-export-parse-only)
    TARGET_FIXTURE_ID="qa-export-parse-only-v2"
    LEGACY_COUNTERPART_ID="qa-functional-legacy"
    EXPECTED_SHORTCUT_NAME="qa-export-parse-only-v2"
    EXPECTED_SEMANTIC_OUTPUT="CHERRI_QA_EXPORT_PARSE_ONLY:SCHEMA=ok:COUNT=ok"
    ;;
  qa-nil-clear)
    TARGET_FIXTURE_ID="qa-nil-clear-v2"
    LEGACY_COUNTERPART_ID="qa-functional-legacy"
    EXPECTED_SHORTCUT_NAME="$TARGET_FIXTURE_ID"
    EXPECTED_SEMANTIC_OUTPUT="CHERRI_QA_NIL_CLEAR:binding=0:assignment=0:control=0:present=1"
    ;;
  qa-collect-empty-split)
    TARGET_FIXTURE_ID="qa-collect-empty-split-v2"
    LEGACY_COUNTERPART_ID="qa-functional-legacy"
    EXPECTED_SHORTCUT_NAME="$TARGET_FIXTURE_ID"
    EXPECTED_SEMANTIC_OUTPUT="CHERRI_QA_COLLECT_EMPTY_SPLIT:direct=0:repeat=0:cleared=0:emptyText=1"
    ;;
  qa-export-file-bytes)
    TARGET_FIXTURE_ID="qa-export-file-bytes-v2"
    LEGACY_COUNTERPART_ID="qa-functional-legacy"
    EXPECTED_SHORTCUT_NAME="qa-export-file-bytes-v2"
    EXPECTED_SEMANTIC_OUTPUT="CHERRI_QA_EXPORT_FILE_BYTES:NAME=ok:TEXT=ok"
    ;;
  qa-file-name-binding)
    TARGET_FIXTURE_ID="qa-file-name-binding-v2"
    LEGACY_COUNTERPART_ID="qa-functional-legacy"
    EXPECTED_SHORTCUT_NAME="$TARGET_FIXTURE_ID"
    EXPECTED_SEMANTIC_OUTPUT="CHERRI_QA_FILE_NAME_BINDING:L=ok:V=ok:I=ok:LT=ok:VT=ok:IT=ok"
    ;;
  qa-file-name-extension)
    TARGET_FIXTURE_ID="qa-file-name-extension-v2"
    LEGACY_COUNTERPART_ID="qa-functional-legacy"
    EXPECTED_SHORTCUT_NAME="$TARGET_FIXTURE_ID"
    EXPECTED_SEMANTIC_OUTPUT="CHERRI_QA_FILE_NAME_EXTENSION:NAME=ok:EXTENSION=ok:TYPE=ok:BYTES=ok:TEXT=ok"
    ;;
  qa-f04-yield-list)
    TARGET_FIXTURE_ID="qa-f04-yield-list-v2"
    LEGACY_COUNTERPART_ID="qa-functional-legacy"
    EXPECTED_SHORTCUT_NAME="$TARGET_FIXTURE_ID"
    EXPECTED_SEMANTIC_OUTPUT="CHERRI_QA_F04_LIST_RESULT:pairs=2:scalars=2:shape=LEFT,TAIL|RIGHT,TAIL"
    ;;
  qa-f04-record-empty)
    TARGET_FIXTURE_ID="qa-f04-record-empty-v2"
    LEGACY_COUNTERPART_ID="qa-functional-legacy"
    EXPECTED_SHORTCUT_NAME="$TARGET_FIXTURE_ID"
    EXPECTED_SEMANTIC_OUTPUT="CHERRI_QA_F04_RECORD_EMPTY_RESULT:outer=2:scalars=2:tags=LEFT,RIGHT:keys=2,2:items=0,0:empty=0:dictionary=Y"
    ;;
  qa-f04-yield-empty-list)
    TARGET_FIXTURE_ID="qa-f04-yield-empty-list-v2"
    LEGACY_COUNTERPART_ID="qa-functional-legacy"
    EXPECTED_SHORTCUT_NAME="$TARGET_FIXTURE_ID"
    EXPECTED_SEMANTIC_OUTPUT="CHERRI_QA_F04_EMPTY_LIST_RESULT:outer=2:scalars=2:shape=0,0"
    ;;
  qa-f01-ifresult-let)
    TARGET_FIXTURE_ID="qa-f01-ifresult-let-v2"
    LEGACY_COUNTERPART_ID="qa-functional-legacy"
    EXPECTED_SHORTCUT_NAME="qa-f01-ifresult-let-v2"
    EXPECTED_SEMANTIC_OUTPUT="CHERRI_QA_F01_IFRESULT:P15_F01_IFRESULT_OK"
    ;;
  qa-f01-ifresult-var)
    TARGET_FIXTURE_ID="qa-f01-ifresult-var-v2"
    LEGACY_COUNTERPART_ID="qa-functional-legacy"
    EXPECTED_SHORTCUT_NAME="qa-f01-ifresult-var-v2"
    EXPECTED_SEMANTIC_OUTPUT="CHERRI_QA_F01_IFRESULT:P15_F01_IFRESULT_OK"
    ;;


  qa-require)
    TARGET_FIXTURE_ID="qa-require-v2"
    LEGACY_COUNTERPART_ID="qa-functional-legacy"
    EXPECTED_SHORTCUT_NAME="qa-require-v2"
    EXPECTED_SEMANTIC_OUTPUT="CHERRI_QA_REQUIRE_RESULT:P15_REQUIRE_OK"
    ;;
  qa-measure)
    TARGET_FIXTURE_ID="qa-measure-v2"
    LEGACY_COUNTERPART_ID="qa-functional-legacy"
    EXPECTED_SHORTCUT_NAME="qa-measure-v2"
    EXPECTED_SEMANTIC_OUTPUT="CHERRI_QA_MEASURE_RESULT:P15_MEASURE_OK"
    ;;
  qa-measure-release)
    TARGET_FIXTURE_ID="qa-measure-release-v2"
    LEGACY_COUNTERPART_ID="qa-functional-legacy"
    EXPECTED_SHORTCUT_NAME="qa-measure-release-v2"
    EXPECTED_SEMANTIC_OUTPUT="CHERRI_QA_MEASURE_RELEASE_RESULT:P15_MEASURE_RELEASE_OK"
    ;;
  qa-json-body-file-utf8)
    TARGET_FIXTURE_ID="qa-json-body-file-utf8-v2"
    LEGACY_COUNTERPART_ID="qa-functional-legacy"
    EXPECTED_SHORTCUT_NAME="qa-json-body-file-utf8-v2"
    EXPECTED_SEMANTIC_OUTPUT="CHERRI_QA_JSON_BODY_FILE_UTF8_RESULT:P15_JSON_BODY_OBSERVED"
    HTTP_PROBE_ENABLED="true"
    ;;
  qa-json-body-control)
    TARGET_FIXTURE_ID="qa-json-body-control-v2"
    LEGACY_COUNTERPART_ID="qa-functional-legacy"
    EXPECTED_SHORTCUT_NAME="qa-json-body-control-v2"
    EXPECTED_SEMANTIC_OUTPUT="CHERRI_QA_JSON_BODY_CONTROL_RESULT:P15_JSON_BODY_OBSERVED"
    HTTP_PROBE_ENABLED="true"
    ;;
  qa-json-body-null-candidate)
    TARGET_FIXTURE_ID="qa-json-body-null-candidate-v2"
    LEGACY_COUNTERPART_ID="qa-functional-legacy"
    EXPECTED_SHORTCUT_NAME="qa-json-body-null-candidate-v2"
    EXPECTED_SEMANTIC_OUTPUT="CHERRI_QA_JSON_BODY_NULL_CANDIDATE_RESULT:P15_JSON_BODY_OBSERVED"
    HTTP_PROBE_ENABLED="true"
    ;;
  qa-retry-control)
    TARGET_FIXTURE_ID="qa-retry-control-v2"
    LEGACY_COUNTERPART_ID="qa-functional-legacy"
    EXPECTED_SHORTCUT_NAME="qa-retry-control-v2"
    EXPECTED_SEMANTIC_OUTPUT="CHERRI_QA_RETRY_CONTROL_RESULT:P15_HTTP_BODY_OK"
    HTTP_PROBE_ENABLED="true"
    ;;
  qa-retry-http500)
    TARGET_FIXTURE_ID="qa-retry-http500-v2"
    LEGACY_COUNTERPART_ID="qa-functional-legacy"
    EXPECTED_SHORTCUT_NAME="qa-retry-http500-v2"
    EXPECTED_SEMANTIC_OUTPUT="CHERRI_QA_RETRY_HTTP500_RESULT:P15_HTTP_BODY_OK"
    HTTP_PROBE_ENABLED="true"
    ;;
  qa-retry-http404)
    TARGET_FIXTURE_ID="qa-retry-http404-v2"
    LEGACY_COUNTERPART_ID="qa-functional-legacy"
    EXPECTED_SHORTCUT_NAME="qa-retry-http404-v2"
    EXPECTED_SEMANTIC_OUTPUT="CHERRI_QA_RETRY_HTTP404_RESULT:P15_HTTP_BODY_OK"
    HTTP_PROBE_ENABLED="true"
    ;;
  qa-retry-http429)
    TARGET_FIXTURE_ID="qa-retry-http429-v2"
    LEGACY_COUNTERPART_ID="qa-functional-legacy"
    EXPECTED_SHORTCUT_NAME="qa-retry-http429-v2"
    EXPECTED_SEMANTIC_OUTPUT="CHERRI_QA_RETRY_HTTP429_RESULT:P15_HTTP_BODY_OK"
    HTTP_PROBE_ENABLED="true"
    ;;
  qa-retry-transport)
    TARGET_FIXTURE_ID="qa-retry-transport-v2"
    LEGACY_COUNTERPART_ID="qa-functional-legacy"
    EXPECTED_SHORTCUT_NAME="qa-retry-transport-v2"
    EXPECTED_SEMANTIC_OUTPUT="CHERRI_QA_RETRY_TRANSPORT_RESULT:P15_HTTP_BODY_OK"
    HTTP_PROBE_ENABLED="true"
    ;;
  qa-f01-diag-counter)
    TARGET_FIXTURE_ID="qa-f01-diag-counter-v2"
    LEGACY_COUNTERPART_ID="qa-functional-legacy"
    EXPECTED_SHORTCUT_NAME="qa-f01-diag-counter-v2"
    EXPECTED_SEMANTIC_OUTPUT="CHERRI_QA_F01_DIAG_COUNTER:calls=1"
    ;;
  qa-f01-diag-predicate)
    TARGET_FIXTURE_ID="qa-f01-diag-predicate-v2"
    LEGACY_COUNTERPART_ID="qa-functional-legacy"
    EXPECTED_SHORTCUT_NAME="qa-f01-diag-predicate-v2"
    EXPECTED_SEMANTIC_OUTPUT="CHERRI_QA_F01_DIAG_PREDICATE:P15_F01_PREDICATE_OK"
    ;;
  qa-f01-diag-control)
    TARGET_FIXTURE_ID="qa-f01-diag-control-v2"
    LEGACY_COUNTERPART_ID="qa-functional-legacy"
    EXPECTED_SHORTCUT_NAME="qa-f01-diag-control-v2"
    EXPECTED_SEMANTIC_OUTPUT="CHERRI_QA_F01_DIAG_CONTROL:P15_F01_CONTROL_OK:calls=1"
    ;;
  qa-f01-diag-logic)
    TARGET_FIXTURE_ID="qa-f01-diag-logic-v2"
    LEGACY_COUNTERPART_ID="qa-functional-legacy"
    EXPECTED_SHORTCUT_NAME="qa-f01-diag-logic-v2"
    EXPECTED_SEMANTIC_OUTPUT="CHERRI_QA_F01_DIAG_LOGIC:P15_F01_LOGIC_OK"
    ;;
  qa-short-circuit-local)
    TARGET_FIXTURE_ID="qa-short-circuit-local-v2"
    LEGACY_COUNTERPART_ID="qa-functional-legacy"
    EXPECTED_SHORTCUT_NAME="qa-short-circuit-local-v2"
    EXPECTED_SEMANTIC_OUTPUT="CHERRI_QA_F01_LOCAL_RESULT:P15_F01_LOCAL_OK:calls=1"
    ;;
  qa-short-circuit)
    TARGET_FIXTURE_ID="qa-short-circuit-v2"
    LEGACY_COUNTERPART_ID="qa-functional-legacy"
    EXPECTED_SHORTCUT_NAME="qa-short-circuit-v2"
    EXPECTED_SEMANTIC_OUTPUT="CHERRI_QA_F01_RESULT:P15_F01_OK"
    ;;
  qa-functional-legacy)
    TARGET_FIXTURE_ID="qa-functional-legacy"
    LEGACY_COUNTERPART_ID="qa-functional-legacy"
    EXPECTED_SHORTCUT_NAME="qa-functional-legacy"
    EXPECTED_SEMANTIC_OUTPUT="CHERRI_QA_RESULT:ok:count=2:mode=legacy"
    ;;
  qa-bool-list)
    TARGET_FIXTURE_ID="qa-bool-list-v2"
    LEGACY_COUNTERPART_ID="qa-functional-legacy"
    EXPECTED_SHORTCUT_NAME="qa-bool-list-v2"
    EXPECTED_SEMANTIC_OUTPUT="CHERRI_QA_BOOL_LIST_RESULT:ok:first=first:last=second"
    ;;
  qa-scroll-editor)
    TARGET_FIXTURE_ID="qa-scroll-editor-v2"
    LEGACY_COUNTERPART_ID="qa-scroll-editor-legacy"
    EXPECTED_SHORTCUT_NAME="qa-scroll-editor-v2"
    EXPECTED_SEMANTIC_OUTPUT="ANCHOR_CARD_26_TEXT_ZETA"
    ;;
  qa-rtl-hebrew)
    TARGET_FIXTURE_ID="qa-rtl-hebrew-v2"
    LEGACY_COUNTERPART_ID="qa-functional-legacy"
    EXPECTED_SHORTCUT_NAME="qa-rtl-hebrew-v2"
    EXPECTED_SEMANTIC_OUTPUT="CHERRI_QA_RTL_RESULT:ok:hebrew"
    ;;
  *)
    TARGET_FIXTURE_ID="qa-functional-v2"
    LEGACY_COUNTERPART_ID="qa-functional-legacy"
    EXPECTED_SHORTCUT_NAME="qa-functional-v2"
    EXPECTED_SEMANTIC_OUTPUT="CHERRI_QA_RESULT:ok:count=2:mode=v2"
    ;;
esac
TARGET_SOURCE_FILE="$ROOT_DIR/tools/shortcuts-simulator-qa/fixtures/${TARGET_SOURCE_RELATIVE:-${TARGET_FIXTURE_ID}.cherri}"
LEGACY_COUNTERPART_SOURCE="$ROOT_DIR/tools/shortcuts-simulator-qa/fixtures/${LEGACY_COUNTERPART_ID}.cherri"

# ---------------------------------------------------------------------------
qa_acquisition_checkpoint() {
  local stage="$1"
  local phase_exit="${2:-}"
  local args=(
    checkpoint --repo "$ROOT_DIR" --root "$ARTIFACT_DIR"
    --source-sha "$SOURCE_SHA" --run-id "$RUN_ID" --fixture-id "$TARGET_FIXTURE_ID"
    --stage "$stage" --require-runtime "$REQUIRE_RUNTIME"
    --gates "$GATE_RUNNER_AVAILABLE" "$GATE_SIMULATOR_BOOTED"
    "$GATE_SHORTCUTS_APP_INSTALLED" "$GATE_SHORTCUTS_LAUNCHED"
    "$GATE_EDITOR_OPENED" "$GATE_FILE_IMPORT_ATTEMPTED"
    "$GATE_COMPILER_ARTIFACT_IMPORTED" "$GATE_SHORTCUT_EXISTS_IN_LIBRARY"
    "$GATE_ACTION_EDITOR_CAPTURED" "$GATE_SHORTCUT_RUN_ATTEMPTED"
    "$GATE_SHORTCUT_RUN_COMPLETED" "$GATE_CALLBACK_CAPTURED"
    "$GATE_TRACE_CAPTURED" "$GATE_PHYSICAL_DEVICE_TESTED"
  )
  if [ "$stage" != "COMPILED" ]; then
    args+=(--udid "$UDID" --nonce "${EXEC_RUN_NONCE:-}")
  fi
  if [ "$stage" = "TEST02_COMPLETED" ]; then
    args+=(--test-exit-code "$phase_exit")
  fi
  python3 "$ROOT_DIR/tools/shortcuts-simulator-qa/qa_terminal_receipts.py" "${args[@]}"
}

# Phase 0: Build Cherri CLI & Compile Synthetic Sanitized Fixtures
# ---------------------------------------------------------------------------
echo "=== Phase 0: Building Cherri & Compiling Synthetic Fixtures ==="
go build -o "$ARTIFACT_DIR/builds/cherri" .
BUILD_RC=$?
if [ "$BUILD_RC" -ne 0 ]; then exit "$BUILD_RC"; fi

python3 - "$ROOT_DIR" "$ARTIFACT_DIR" "$SOURCE_SHA" <<'PY'
import hashlib
import json
import os
import plistlib
import shutil
import subprocess
import sys

root_dir, artifact_dir, source_sha = sys.argv[1], sys.argv[2], sys.argv[3]
sys.path.insert(0, os.path.join(root_dir, "tools", "shortcuts-simulator-qa"))
from retry_http_probe import validate_fixture_graph
from json_body_probe import verify_source as verify_json_body_source, verify_graph as verify_json_body_graph, verify_mutations as verify_json_body_mutations
from export_unicode_diagnostic import validate_export_unicode_graph
from file_materialization_diagnostic import verify_source as verify_file_materialization_source, verify_graph as verify_file_materialization_graph
from n03_text_equality_diagnostic import verify_source as verify_n03_text_equality_source
from optional_forwarding_diagnostic import verify_source as verify_optional_forwarding_source, verify_prepared_unsigned as verify_optional_forwarding_unsigned
from binding_defaults_probe import verify_adapter_source, verify_adapter_graph
from nested_list_runtime import prepared as verify_nested_list_prepared
from f04_record_empty_probe import verify_source as verify_record_empty_source, verify_graph as verify_record_empty_graph
from native_boundary_diagnostic import verify_source as verify_boundary_source, verify_graph as verify_boundary_graph
from fixture_compile_contract import EXPECTED_REJECTIONS, verify_expected_rejection
from debug_label_acceptance import verify_source as verify_label_source, verify_graph as verify_label_graph, verify_causal_mutations as verify_label_mutations
cherri_bin = os.path.join(artifact_dir, "builds", "cherri")
fixtures_dir = os.path.join(root_dir, "tools", "shortcuts-simulator-qa", "fixtures")
builds_dir = os.path.join(artifact_dir, "builds")

fixture_specs = [
    ("qa-nested-list-control-v2", "qa-nested-list-control-v2.cherri", "v2"),
    ("qa-nested-list-candidate-v2", "qa-nested-list-candidate-v2.cherri", "v2"),
    ("qa-json-body-file-utf8-v2", "qa-json-body-file-utf8-v2.cherri", "v2"),
    ("qa-json-body-control-v2", "qa-json-body-control-v2.cherri", "v2"),
    ("qa-json-body-null-candidate-v2", "qa-json-body-null-candidate-v2.cherri", "v2"),
    ("qa-debug-label-plain-v2", "qa-debug-label-plain-v2.cherri", "v2"),
    ("qa-debug-label-plain-legacy", "qa-debug-label-plain-legacy.cherri", "legacy"),
    ("qa-debug-label-dated-v2", "qa-debug-label-dated-v2.cherri", "v2"),
    ("qa-debug-label-dated-legacy", "qa-debug-label-dated-legacy.cherri", "legacy"),
    ("qa-debug-label-typed-v2", "qa-debug-label-typed-v2.cherri", "v2"),
    ("qa-debug-label-typed-legacy", "qa-debug-label-typed-legacy.cherri", "legacy"),
    ("qa-debug-label-dated-typed-v2", "qa-debug-label-dated-typed-v2.cherri", "v2"),
    ("qa-debug-label-dated-typed-legacy", "qa-debug-label-dated-typed-legacy.cherri", "legacy"),
    ("qa-debug-label-text-v2", "qa-debug-label-text-v2.cherri", "v2"),
    ("qa-debug-label-text-legacy", "qa-debug-label-text-legacy.cherri", "legacy"),
    ("qa-debug-label-dated-text-v2", "qa-debug-label-dated-text-v2.cherri", "v2"),
    ("qa-debug-label-dated-text-legacy", "qa-debug-label-dated-text-legacy.cherri", "legacy"),
    ("CherriBindingCapturePOC", "p15-binding-defaults/CherriBindingCapturePOC.cherri", "v2"),
    ("CherriTypedDefaultsPOC", "p15-binding-defaults/CherriTypedDefaultsPOC.cherri", "v2"),
    ("qa-storage-text-same-run-v2", "qa-storage-text-same-run-v2.cherri", "v2"),
    ("qa-storage-scope-v2", "qa-storage-scope-v2.cherri", "v2"),
    ("qa-storage-delete-v2", "qa-storage-delete-v2.cherri", "v2"),
    ("qa-storage-text-cross-run-v2", "qa-storage-text-cross-run-v2.cherri", "v2"),
    ("qa-http-json-control-v2", "qa-http-json-control-v2.cherri", "v2"),
    ("qa-http-json-fetch-v2", "qa-http-json-fetch-v2.cherri", "v2"),
    ("qa-number-comparison-diag-v2", "qa-number-comparison-diag-v2.cherri", "v2"),
    ("qa-number-coercion-control-v2", "qa-number-coercion-control-v2.cherri", "v2"),
    ("qa-paginate-ordered-v2", "p15-paginate-probes/qa-paginate-ordered-v2.cherri", "v2"),
    ("qa-paginate-cycle-v2", "p15-paginate-probes/qa-paginate-cycle-v2.cherri", "v2"),
    ("qa-paginate-page-cap-v2", "p15-paginate-probes/qa-paginate-page-cap-v2.cherri", "v2"),
    ("qa-paginate-item-cap-v2", "p15-paginate-probes/qa-paginate-item-cap-v2.cherri", "v2"),
    ("qa-paginate-empty-v2", "p15-paginate-probes/qa-paginate-empty-v2.cherri", "v2"),
    ("qa-collect-counts-v2", "qa-collect-counts-v2.cherri", "v2"),
    ("qa-collect-counts-legacy", "qa-collect-counts-legacy.cherri", "legacy"),
    ("qa-collect-dictionaries-v2", "qa-collect-dictionaries-v2.cherri", "v2"),
    ("qa-collect-dictionaries-legacy", "qa-collect-dictionaries-legacy.cherri", "legacy"),
    ("qa-retry-control-v2", "qa-retry-control-v2.cherri", "v2"),
    ("qa-retry-http500-v2", "qa-retry-http500-v2.cherri", "v2"),
    ("qa-retry-http404-v2", "qa-retry-http404-v2.cherri", "v2"),
    ("qa-retry-http429-v2", "qa-retry-http429-v2.cherri", "v2"),
    ("qa-retry-transport-v2", "qa-retry-transport-v2.cherri", "v2"),
    ("qa-label-file-carrier-v2", "p15-label-probes/qa-label-file-carrier-v2.cherri", "v2"),
    ("qa-label-raw-empty-v2", "p15-label-probes/qa-label-raw-empty-v2.cherri", "v2"),
    ("qa-label-raw-grouping-v2", "p15-label-probes/qa-label-raw-grouping-v2.cherri", "v2"),
    ("qa-file-materialization-diagnostic-v2", "qa-file-materialization-diagnostic-v2.cherri", "v2"),
    ("qa-n03-text-equality-diagnostic-v2", "qa-n03-text-equality-diagnostic-v2.cherri", "v2"),
    ("qa-optional-forwarding-diagnostic-v2", "qa-optional-forwarding-diagnostic-v2.cherri", "v2"),
    ("qa-native-boundary-diagnostic-v2", "qa-native-boundary-diagnostic-v2.cherri", "v2"),
    ("qa-export-unicode-diagnostic-v2", "qa-export-unicode-diagnostic-v2.cherri", "v2"),
    ("qa-export-unicode-v2", "p15-export-probes/qa-export-unicode-v2.cherri", "v2"),
    ("qa-export-empty-v2", "p15-export-probes/qa-export-empty-v2.cherri", "v2"),
    ("qa-export-thousand-v2", "p15-export-probes/qa-export-thousand-v2.cherri", "v2"),
    ("qa-export-redacted-v2", "p15-export-probes/qa-export-redacted-v2.cherri", "v2"),
    ("qa-export-unsupported-file-v2", "p15-export-probes/qa-export-unsupported-file-v2.cherri", "v2"),
    ("qa-export-text-file-v2", "p15-export-probes/qa-export-text-file-v2.cherri", "v2"),
    ("qa-functional-v2", "qa-functional-v2.cherri", "v2"),
    ("qa-functional-legacy", "qa-functional-legacy.cherri", "legacy"),
    ("qa-scroll-editor-v2", "qa-scroll-editor-v2.cherri", "v2"),
    ("qa-scroll-editor-legacy", "qa-scroll-editor-legacy.cherri", "legacy"),
    ("qa-rtl-hebrew-v2", "qa-rtl-hebrew-v2.cherri", "v2"),
    ("qa-bool-list-v2", "qa-bool-list-v2.cherri", "v2"),
    ("qa-short-circuit-v2", "qa-short-circuit-v2.cherri", "v2"),
    ("qa-short-circuit-local-v2", "qa-short-circuit-local-v2.cherri", "v2"),
    ("qa-f01-diag-counter-v2", "qa-f01-diag-counter-v2.cherri", "v2"),
    ("qa-f01-diag-predicate-v2", "qa-f01-diag-predicate-v2.cherri", "v2"),
    ("qa-f01-diag-control-v2", "qa-f01-diag-control-v2.cherri", "v2"),
    ("qa-f01-diag-logic-v2", "qa-f01-diag-logic-v2.cherri", "v2"),
    ("qa-require-zero-diag-v2", "qa-require-zero-diag-v2.cherri", "v2"),
    ("qa-label-file-controls-v2", "qa-label-file-controls-v2.cherri", "v2"),
    ("qa-export-file-control-v2", "qa-export-file-control-v2.cherri", "v2"),
    ("qa-export-build-only-v2", "qa-export-build-only-v2.cherri", "v2"),
    ("qa-export-parse-only-v2", "qa-export-parse-only-v2.cherri", "v2"),
    ("qa-nil-clear-v2", "qa-nil-clear-v2.cherri", "v2"),
    ("qa-collect-empty-split-v2", "qa-collect-empty-split-v2.cherri", "v2"),
    ("qa-export-file-bytes-v2", "qa-export-file-bytes-v2.cherri", "v2"),
    ("qa-file-name-binding-v2", "qa-file-name-binding-v2.cherri", "v2"),
    ("qa-file-name-extension-v2", "qa-file-name-extension-v2.cherri", "v2"),
    ("qa-f04-yield-list-v2", "qa-f04-yield-list-v2.cherri", "v2"),
    ("qa-f04-yield-empty-list-v2", "qa-f04-yield-empty-list-v2.cherri", "v2"),
    ("qa-f04-record-empty-v2", "qa-f04-record-empty-v2.cherri", "v2"),
    ("qa-f01-ifresult-let-v2", "qa-f01-ifresult-let-v2.cherri", "v2"),
    ("qa-f01-ifresult-var-v2", "qa-f01-ifresult-var-v2.cherri", "v2"),
    ("qa-require-v2", "qa-require-v2.cherri", "v2"),
    ("qa-measure-v2", "qa-measure-v2.cherri", "v2"),
    ("qa-measure-release-v2", "qa-measure-release-v2.cherri", "v2"),
    ("qa-functional-v2-uninstrumented", "qa-functional-v2-uninstrumented.cherri", "v2"),
    ("qa-functional-legacy-uninstrumented", "qa-functional-legacy-uninstrumented.cherri", "legacy"),
]

manifest_entries = []
fixture_rejections = []
compiled_actions_by_id = {}

def native_strings(value):
    if isinstance(value, str):
        yield value
    elif isinstance(value, dict):
        for key, item in value.items():
            yield from native_strings(key)
            yield from native_strings(item)
    elif isinstance(value, list):
        for item in value:
            yield from native_strings(item)

for fid, fname, dialect in fixture_specs:
    profile = "release" if fid == "qa-measure-release-v2" else "debug"
    trace_expectation = "ABSENT_RELEASE" if profile == "release" else "REQUIRED_NDJSON"
    fpath = os.path.join(fixtures_dir, fname)
    with open(fpath, "rb") as f:
        src_bytes = f.read()
    src_sha256 = hashlib.sha256(src_bytes).hexdigest()
    optional_forwarding_source = verify_optional_forwarding_source(root_dir, fid, src_bytes)

    build_src = os.path.join(builds_dir, fname)
    os.makedirs(os.path.dirname(build_src), exist_ok=True)
    shutil.copyfile(fpath, build_src)
    out_base = os.path.join(builds_dir, fid)
    out_shortcut = out_base + ".shortcut"
    cmd = [cherri_bin, build_src, "--skip-sign", f"--output={out_shortcut}", f"--build-profile={profile}"]
    if dialect == "legacy":
        cmd.append("--legacy")
    proc = subprocess.run(cmd, cwd=builds_dir, capture_output=True, text=True)
    if fid in EXPECTED_REJECTIONS:
        checked = subprocess.run([cherri_bin, "--check=" + build_src, "--json"],
                                 cwd=builds_dir, capture_output=True, text=True)
        rejection = verify_expected_rejection(fid, src_bytes.decode("utf-8"), proc, checked,
            [out_shortcut, os.path.splitext(build_src)[0] + ".shortcut",
             os.path.splitext(build_src)[0] + ".plist"])
        rejection.update(fileName=fname, dialect=dialect, sourceSha256=src_sha256)
        fixture_rejections.append(rejection)
        continue
    if proc.returncode != 0:
        raise RuntimeError(f"Failed to compile {fid}: {proc.stdout}\n{proc.stderr}")

    with open(out_shortcut, "rb") as f:
        plist_bytes = f.read()
    plist_sha256 = hashlib.sha256(plist_bytes).hexdigest()
    doc = plistlib.loads(plist_bytes)
    actions = doc.get("WFWorkflowActions", [])
    compiled_actions_by_id[fid] = actions

    action_ids = [a.get("WFWorkflowActionIdentifier", "") for a in actions]
    release_diagnostics_absent = False
    release_body_verified = False
    if profile == "release":
        # Inspect the emitted graph independently of the CLI flag. An ignored
        # profile flag must not permit the absence of runtime trace evidence.
        release_diagnostics_absent = not any(
            "__cherri_debug_" in item or "__cherri_qa_trace" in item
            or item == "is.workflow.actions.gettimebetweendates"
            for item in native_strings(actions)
        )
        release_body_verified = (
            action_ids.count("is.workflow.actions.delay") == 1
            and action_ids.count("is.workflow.actions.dictionary") == 1
        )
        if not release_diagnostics_absent or not release_body_verified:
            raise RuntimeError("Release measure fixture retained diagnostics or lost/duplicated its native body")
    manifest_entries.append({
        "fixtureId": fid,
        "fileName": fname,
        "dialect": dialect,
        "buildProfile": profile,
        "runtimeTraceExpectation": trace_expectation,
        "httpProbeGraph": validate_fixture_graph(fid, actions),
        "jsonBodySource": verify_json_body_source(root_dir, fid, src_bytes),
        "jsonBodyGraph": verify_json_body_graph(root_dir, fid, actions),
        "jsonBodyCausalMutations": verify_json_body_mutations(root_dir, fid, actions),
        "exportUnicodeDiagnosticGraph": validate_export_unicode_graph(fid, actions),
        "fileMaterializationSource": verify_file_materialization_source(root_dir, fid, src_bytes),
        "n03TextEqualitySource": verify_n03_text_equality_source(root_dir, fid, src_bytes),
        "nestedListProbe": verify_nested_list_prepared(root_dir, fid, src_bytes, plist_bytes),
        "optionalForwardingSource": optional_forwarding_source,
        "optionalForwardingGraph": verify_optional_forwarding_unsigned(root_dir, artifact_dir, fid, out_shortcut, plist_sha256),
        "fileMaterializationGraph": verify_file_materialization_graph(fid, actions),
        "bindingDefaultsSource": verify_adapter_source(root_dir, fid, src_bytes),
        "bindingDefaultsGraph": verify_adapter_graph(fid, actions),
        "recordEmptyFeasibilitySource": verify_record_empty_source(root_dir, fid, src_bytes),
        "recordEmptyFeasibilityGraph": verify_record_empty_graph(fid, actions),
        "nativeBoundarySource": verify_boundary_source(root_dir, fid, src_bytes),
        "nativeBoundaryGraph": verify_boundary_graph(fid, actions),
        "debugLabelSource": verify_label_source(root_dir, fid, src_bytes),
        "debugLabelGraph": verify_label_graph(fid, actions),
        "debugLabelCausalMutations": verify_label_mutations(fid, actions) if verify_label_source(root_dir, fid, src_bytes) else None,
        "releaseDiagnosticActionsAbsent": release_diagnostics_absent,
        "releaseBodyShapeVerified": release_body_verified,
        "sourceSha256": src_sha256,
        "unsignedPlistSha256": plist_sha256,
        "nativeActionCount": len(actions),
        "actionIdentifiers": action_ids,
    })

compiler_manifest = {
    "schemaVersion": "1.5",
    "syntheticOnly": True,
    "compilerCommitSha": source_sha,
    "fixtures": manifest_entries,
    "fixtureRejections": fixture_rejections,
    "compileAttempts": len(manifest_entries) + len(fixture_rejections),
    "compiledArtifactCount": len(manifest_entries),
    "expectedDiagnosticRejectionCount": len(fixture_rejections),
}
with open(os.path.join(artifact_dir, "compiler-manifest.json"), "w", encoding="utf-8") as f:
    json.dump(compiler_manifest, f, indent=2)

# Structural classifier matching debug_stdlib.go (never naive substring heuristics)
def is_structural_inline_trace_text(actions, idx):
    if idx < 0 or idx + 1 >= len(actions):
        return False
    act = actions[idx]
    nxt = actions[idx + 1]
    if act.get("WFWorkflowActionIdentifier") != "is.workflow.actions.gettext":
        return False
    if nxt.get("WFWorkflowActionIdentifier") != "is.workflow.actions.appendvariable":
        return False
    nxt_params = nxt.get("WFWorkflowActionParameters", {})
    if nxt_params.get("WFVariableName") != "__cherri_qa_trace":
        return False
    act_uuid = act.get("WFWorkflowActionParameters", {}).get("UUID", "")
    inp = nxt_params.get("WFInput", {}).get("Value", {})
    if not act_uuid or inp.get("OutputUUID") != act_uuid:
        return False
    txt = act.get("WFWorkflowActionParameters", {}).get("WFTextActionText", {})
    if not isinstance(txt, dict) or txt.get("WFSerializationType") != "WFTextTokenString":
        return False
    atts = txt.get("Value", {}).get("attachmentsByRange", {})
    if len(atts) != 1:
        return False
    only_att = next(iter(atts.values()))
    return isinstance(only_att, dict) and only_att.get("Type") == "CurrentDate"

def is_structural_collector_envelope_text(actions, idx):
    if idx <= 0 or idx >= len(actions):
        return False
    prev = actions[idx - 1]
    act = actions[idx]
    if prev.get("WFWorkflowActionIdentifier") != "is.workflow.actions.text.combine":
        return False
    if act.get("WFWorkflowActionIdentifier") != "is.workflow.actions.gettext":
        return False
    prev_params = prev.get("WFWorkflowActionParameters", {})
    combine_uuid = prev_params.get("UUID", "")
    combine_var = prev_params.get("text", {}).get("Value", {}).get("VariableName", "")
    if not combine_uuid or combine_var != "__cherri_qa_trace":
        return False
    txt = act.get("WFWorkflowActionParameters", {}).get("WFTextActionText", {})
    if not isinstance(txt, dict) or txt.get("WFSerializationType") != "WFTextTokenString":
        return False
    s = txt.get("Value", {}).get("string", "")
    if "===CHERRI_QA_TRACE_NDJSON_V14===" not in s:
        return False
    atts = txt.get("Value", {}).get("attachmentsByRange", {})
    if len(atts) != 1:
        return False
    only_att = next(iter(atts.values()))
    return (
        isinstance(only_att, dict)
        and only_att.get("Type") == "ActionOutput"
        and only_att.get("OutputUUID") == combine_uuid
        and only_att.get("OutputName") == "Combined Text"
    )

def classify_actions(actions):
    source_ids = []
    inline_debug_ids = []
    collector_ids = []
    for idx, act in enumerate(actions):
        aid = act.get("WFWorkflowActionIdentifier", "")
        params = act.get("WFWorkflowActionParameters", {})
        vname = str(params.get("WFVariableName", ""))
        if vname in ("__cherri_debug_log", "__cherri_qa_trace") or vname.startswith("__cherri_debug_timer_"):
            inline_debug_ids.append({"nativeIndex": idx, "identifier": aid, "variable": vname})
            continue
        if aid == "is.workflow.actions.detect.date":
            aggs = params.get("WFInput", {}).get("Value", {}).get("Aggrandizements", [])
            if len(aggs) == 1 and aggs[0].get("PropertyUserInfo") == "WFFileCreationDate":
                inline_debug_ids.append({"nativeIndex": idx, "identifier": aid, "reason": "CreationDateCapture"})
                continue
        if aid == "is.workflow.actions.text.combine":
            cvar = params.get("text", {}).get("Value", {}).get("VariableName", "")
            if cvar in ("__cherri_debug_log", "__cherri_qa_trace"):
                collector_ids.append({"nativeIndex": idx, "identifier": aid})
                continue
        if aid == "is.workflow.actions.nothing" and idx + 1 < len(actions):
            nxt = actions[idx + 1]
            next_params = nxt.get("WFWorkflowActionParameters", {})
            if (nxt.get("WFWorkflowActionIdentifier") == "is.workflow.actions.setvariable"
                    and next_params.get("WFVariableName") == "__cherri_debug_log"
                    and "WFInput" not in params and "WFInput" not in next_params):
                inline_debug_ids.append({"nativeIndex": idx, "identifier": aid, "reason": "ClearLogsNothing"})
                continue
        if aid == "is.workflow.actions.gettext":
            if is_structural_inline_trace_text(actions, idx):
                inline_debug_ids.append({"nativeIndex": idx, "identifier": aid, "reason": "TraceEventLine"})
                continue
            if is_structural_collector_envelope_text(actions, idx):
                collector_ids.append({"nativeIndex": idx, "identifier": aid, "reason": "TraceExportEnvelope"})
                continue
            if params.get("WFTextActionText") == "" and idx + 1 < len(actions):
                nxt = actions[idx + 1]
                if (
                    nxt.get("WFWorkflowActionIdentifier") == "is.workflow.actions.setvariable"
                    and nxt.get("WFWorkflowActionParameters", {}).get("WFVariableName") == "__cherri_debug_log"
                ):
                    inline_debug_ids.append({"nativeIndex": idx, "identifier": aid, "reason": "ClearLogsEmptyText"})
                    continue
        source_ids.append(aid)
    return source_ids, inline_debug_ids, collector_ids

comparisons = []
for dialect in ("v2", "legacy"):
    inst_id = f"qa-functional-{dialect}"
    uninst_id = f"qa-functional-{dialect}-uninstrumented"
    inst_src, inst_dbg, inst_col = classify_actions(compiled_actions_by_id[inst_id])
    uninst_src, _, _ = classify_actions(compiled_actions_by_id[uninst_id])
    if inst_src != uninst_src:
        raise RuntimeError(f"SOURCE_ACTION projection mismatch for {dialect}: {inst_src} != {uninst_src}")
    comparisons.append({
        "dialect": dialect,
        "instrumentedFixtureId": inst_id,
        "uninstrumentedFixtureId": uninst_id,
        "uninstrumentedActionCount": len(compiled_actions_by_id[uninst_id]),
        "instrumentedTotalActionCount": len(compiled_actions_by_id[inst_id]),
        "projectedSourceActionCount": len(inst_src),
        "inlineDebugActionCount": len(inst_dbg),
        "collectorEventActionCount": len(inst_col),
        "sourceActionSequenceMatches": (inst_src == uninst_src),
        "sourceActionIdentifiers": inst_src,
        "inlineDebugActions": inst_dbg,
        "collectorActions": inst_col,
    })

with open(os.path.join(artifact_dir, "runtime", "instrumentation-vs-original.json"), "w", encoding="utf-8") as f:
    json.dump({"schemaVersion": "1.4", "comparisons": comparisons}, f, indent=2)
PY
FIXTURE_COMPILE_RC=$?
if [ "$FIXTURE_COMPILE_RC" -ne 0 ]; then exit "$FIXTURE_COMPILE_RC"; fi

# An unsupported preserved probe is a compiler rejection, never a runnable
# fixture or a successful Apple observation. Stop before signing/Simulator use.
python3 - "$ARTIFACT_DIR/compiler-manifest.json" "$TARGET_FIXTURE_ID" <<'PY'
import json
import sys
with open(sys.argv[1], encoding="utf-8") as stream:
    manifest = json.load(stream)
target = sys.argv[2]
rejected = [row for row in manifest.get("fixtureRejections", []) if row["fixtureId"] == target]
if rejected:
    print("BLOCKED_PLATFORM: " + target + ": E_TARGET_VALUE_SHAPE; no native artifact emitted", file=sys.stderr)
    sys.exit(3)
if sum(row["fixtureId"] == target for row in manifest["fixtures"]) != 1:
    raise SystemExit("selected fixture has no unique compiled artifact")
PY
TARGET_COMPILE_RC=$?
if [ "$TARGET_COMPILE_RC" -ne 0 ]; then exit "$TARGET_COMPILE_RC"; fi

# Verify only the explicitly selected paginate targets against immutable public
# synthetic bytes before simulator/signing work. Other fixtures do not acquire
# a network dependency on this page corpus.
case "$SAMPLE_ID" in
  qa-paginate-ordered|qa-paginate-cycle|qa-paginate-page-cap|qa-paginate-item-cap|qa-paginate-empty)
    python3 "$ROOT_DIR/tools/p10r15/verify_paginate_probe_data.py" --root "$ROOT_DIR" --report "$ARTIFACT_DIR/runtime/paginate-data-verification.json"
    PAGINATE_DATA_RC=$?
    if [ "$PAGINATE_DATA_RC" -ne 0 ]; then exit "$PAGINATE_DATA_RC"; fi
    ;;
esac


# ---------------------------------------------------------------------------
qa_acquisition_checkpoint COMPILED || exit 2

# Phase 1: Preflight & Runner / Simulator Discovery
# ---------------------------------------------------------------------------
echo "=== Phase 1: Runner & Simulator Preflight ==="
UNAME_OUT="$(uname -a)"
SW_VERS_OUT="$(sw_vers 2>&1 || true)"
XCODE_SELECT_OUT="$(xcode-select -p 2>&1 || true)"
XCODEBUILD_VER_OUT="$(xcodebuild -version 2>&1 || true)"

if ! command -v xcrun >/dev/null 2>&1; then
  GATE_RUNNER_AVAILABLE="BLOCKED_RUNNER"
else
  GATE_RUNNER_AVAILABLE="PASS"
fi

xcrun simctl list runtimes -j > "$ARTIFACT_DIR/sim-runtimes.json" 2>/dev/null || echo "{}" > "$ARTIFACT_DIR/sim-runtimes.json"
xcrun simctl list devicetypes -j > "$ARTIFACT_DIR/sim-devicetypes.json" 2>/dev/null || echo "{}" > "$ARTIFACT_DIR/sim-devicetypes.json"
xcrun simctl list devices available -j > "$ARTIFACT_DIR/sim-devices.json" 2>/dev/null || echo "{}" > "$ARTIFACT_DIR/sim-devices.json"

SIM_SELECTION="$(python3 - "$ARTIFACT_DIR" "$DEVICE_PROFILE" <<'PY'
import json
import subprocess
import sys

artifact_dir, profile = sys.argv[1], sys.argv[2]
with open(f"{artifact_dir}/sim-runtimes.json") as f:
    runtimes = json.load(f).get("runtimes", [])
with open(f"{artifact_dir}/sim-devicetypes.json") as f:
    devicetypes = json.load(f).get("devicetypes", [])
with open(f"{artifact_dir}/sim-devices.json") as f:
    devices_map = json.load(f).get("devices", {})

ios_runtimes = [r for r in runtimes if r.get("isAvailable") and ("iOS" in r.get("name", "") or "iPadOS" in r.get("name", ""))]
prefer_ipad = (profile != "iphone-fallback")

def runtime_rank(r):
    rid = r.get("identifier", "")
    ver = str(r.get("version", ""))
    devs = devices_map.get(rid, [])
    has_avail_ipad = any("iPad" in d.get("name", "") and d.get("isAvailable", True) for d in devs)
    not_iphone_specific = ("iPhone" not in rid)
    is_27_0 = 1 if ver.startswith("27.0") else 0
    return (
        1 if (has_avail_ipad if prefer_ipad else not has_avail_ipad) else 0,
        1 if (not_iphone_specific if prefer_ipad else True) else 0,
        is_27_0,
        ver,
    )

ios_runtimes.sort(key=runtime_rank, reverse=True)

chosen_rt = None
for r in ios_runtimes:
    if str(r.get("version", "")).startswith("27") or "27" in r.get("name", ""):
        chosen_rt = r
        break
if not chosen_rt and ios_runtimes:
    chosen_rt = ios_runtimes[0]

if not chosen_rt:
    print(json.dumps({"error": "NO_IOS_RUNTIME"}))
    sys.exit(0)

rt_id = chosen_rt["identifier"]
rt_devs = devices_map.get(rt_id, [])

chosen_dev = None
if prefer_ipad:
    for d in rt_devs:
        if "iPad mini" in d.get("name", "") and d.get("isAvailable", True):
            chosen_dev = d
            break
    if not chosen_dev:
        for d in rt_devs:
            if "iPad" in d.get("name", "") and d.get("isAvailable", True):
                chosen_dev = d
                break
    if not chosen_dev:
        ipad_types = [dt for dt in devicetypes if "iPad" in dt.get("name", "")]
        if ipad_types:
            dt_id = ipad_types[0]["identifier"]
            proc = subprocess.run(
                ["xcrun", "simctl", "create", "Cherri-QA-iPad", dt_id, rt_id],
                capture_output=True, text=True
            )
            if proc.returncode == 0 and proc.stdout.strip():
                chosen_dev = {
                    "udid": proc.stdout.strip(),
                    "name": ipad_types[0]["name"],
                    "deviceTypeIdentifier": dt_id,
                    "createdOnDemand": True,
                }

if not chosen_dev:
    for d in rt_devs:
        if d.get("isAvailable", True):
            chosen_dev = d
            break

if not chosen_dev:
    print(json.dumps({"error": "NO_SIMULATOR_DEVICE", "runtimeId": rt_id}))
    sys.exit(0)

print(json.dumps({
    "udid": chosen_dev["udid"],
    "deviceName": chosen_dev.get("name", "Unknown"),
    "runtimeId": rt_id,
    "runtimeName": chosen_rt.get("name", "Unknown"),
    "runtimeVersion": chosen_rt.get("version", "Unknown"),
    "isIPad": "iPad" in chosen_dev.get("name", ""),
}))
PY
)"

echo "Simulator Selection: $SIM_SELECTION"
UDID="$(echo "$SIM_SELECTION" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("udid",""))')"
DEVICE_NAME="$(echo "$SIM_SELECTION" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("deviceName",""))')"
RUNTIME_NAME="$(echo "$SIM_SELECTION" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("runtimeName",""))')"
RUNTIME_VERSION="$(echo "$SIM_SELECTION" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("runtimeVersion",""))')"

if [ -z "$UDID" ]; then
  GATE_SIMULATOR_BOOTED="BLOCKED_RUNNER"
else
  echo "Booting simulator UDID: $UDID ($DEVICE_NAME)"
  xcrun simctl boot "$UDID" 2>/dev/null || true
  if xcrun simctl bootstatus "$UDID" -b; then
    GATE_SIMULATOR_BOOTED="PASS"
  else
    GATE_SIMULATOR_BOOTED="BLOCKED_RUNNER"
  fi
fi

SHORTCUTS_BUNDLE_PRESENT="false"
if [ "$GATE_SIMULATOR_BOOTED" = "PASS" ]; then
  if ! python3 -B "$ROOT_DIR/tools/shortcuts-simulator-qa/simulator_listapps_observation.py" \
    --repo "$ROOT_DIR" \
    --root "${RUNNER_TEMP:-$ARTIFACT_DIR}/cherri-qa-session-boundaries-$RUN_ID-${GITHUB_RUN_ATTEMPT:-1}/preflight-listapps" \
    --udid "$UDID" --source-sha "$SOURCE_SHA" --run-id "$RUN_ID" \
    --attempt "${GITHUB_RUN_ATTEMPT:-1}" --fixture-id "$TARGET_FIXTURE_ID" \
    --output "$QA_SCRATCH_DIR/sim-listapps.plist"; then
    GATE_SHORTCUTS_APP_INSTALLED="BLOCKED_RUNNER"
    echo "SIMULATOR_LISTAPPS_OBSERVATION_BLOCKED" >&2
    exit 70
  fi
  if grep -q "com.apple.shortcuts" "$QA_SCRATCH_DIR/sim-listapps.plist" 2>/dev/null; then
    SHORTCUTS_BUNDLE_PRESENT="true"
    GATE_SHORTCUTS_APP_INSTALLED="PASS"
  else
    GATE_SHORTCUTS_APP_INSTALLED="BLOCKED_SIMULATOR_APP"
  fi
fi
python3 - "$ARTIFACT_DIR/sim-devices.json" "$ARTIFACT_DIR/sim-runtimes.json" <<'PY' 2>/dev/null || true
import json, sys
devs_map = json.load(open(sys.argv[1])).get("devices", {})
rts = {r.get("identifier"): r.get("version") for r in json.load(open(sys.argv[2])).get("runtimes", [])}
summary = []
for rid, devs in devs_map.items():
    for d in devs:
        if d.get("isAvailable", True) and "iPad" in d.get("name", ""):
            summary.append(f"{d.get('name')} (rt={rts.get(rid, rid)}, state={d.get('state')})")
print("Available iPad simulators:", ", ".join(summary) if summary else "none")
PY
rm -f "$QA_SCRATCH_DIR/sim-listapps.plist" "$ARTIFACT_DIR/sim-runtimes.json" "$ARTIFACT_DIR/sim-devicetypes.json" "$ARTIFACT_DIR/sim-devices.json"

# Liveness diagnostics have one monotonic budget and never replace the signed
# import, callback, or exact runtime oracle below.
simctl_getenv_ok() {
  python3 "$ROOT_DIR/tools/shortcuts-simulator-qa/simulator_session_recovery.py" probe \
    --udid "$1" --scratch "$QA_SCRATCH_DIR" >/dev/null 2>&1
}

wait_for_springboard_display() {
  python3 "$ROOT_DIR/tools/shortcuts-simulator-qa/simulator_session_recovery.py" display \
    --udid "$1" --scratch "$QA_SCRATCH_DIR" --display-budget "${2:-90}"
}

ensure_sim_session_ready() {
  python3 "$ROOT_DIR/tools/shortcuts-simulator-qa/simulator_session_recovery.py" recover \
    --udid "$1" --scratch "$QA_SCRATCH_DIR" --force-reboot "${2:-0}" \
    --receipt "$ARTIFACT_DIR/runtime/simulator-session-recovery.json" \
    --source-sha "$SOURCE_SHA" --run-id "$RUN_ID" --fixture-id "$TARGET_FIXTURE_ID"
}

# XCTest may finish before its simulator session has settled. Acquire the same
# device afresh for external host operations; a getenv result from a previous
# phase is not a current boot-state witness. A failed boundary remains a real
# prerequisite failure, while the existing finalizer retains incomplete evidence.
ensure_phase_session_ready() {
  local phase="$1"
  python3 -B "$ROOT_DIR/tools/shortcuts-simulator-qa/simulator_phase_boundary.py" \
    --repo "$ROOT_DIR" \
    --root "${RUNNER_TEMP:-$ARTIFACT_DIR}/cherri-qa-session-boundaries-$RUN_ID-${GITHUB_RUN_ATTEMPT:-1}/$phase" \
    --udid "$UDID" --source-sha "$SOURCE_SHA" --run-id "$RUN_ID" \
    --attempt "${GITHUB_RUN_ATTEMPT:-1}" --fixture-id "$TARGET_FIXTURE_ID" --phase "$phase"
}

simctl_openurl_resilient() {
  local target_udid="$1"
  local target_url="$2"
  local max_attempts="${3:-2}"
  local fallback_file="${4:-}"
  local allow_sim_restart="${5:-0}"
  local timeout_sec="${6:-60}"
  local attempt=1
  local out=""
  local rc=1
  while [ "$attempt" -le "$max_attempts" ]; do
    out="$(python3 - "$target_udid" "$target_url" "$timeout_sec" <<'PY'
import os, signal, subprocess, sys, tempfile
udid, url, t = sys.argv[1], sys.argv[2], int(sys.argv[3])
with tempfile.TemporaryFile(mode="w+b") as tf:
    p = subprocess.Popen(
        ["xcrun", "simctl", "openurl", udid, url],
        stdin=subprocess.DEVNULL,
        stdout=tf,
        stderr=subprocess.STDOUT,
        start_new_session=True,
    )
    try:
        rc = p.wait(timeout=t)
        tf.seek(0)
        sys.stdout.write(tf.read().decode("utf-8", errors="replace"))
        sys.exit(rc)
    except subprocess.TimeoutExpired:
        try:
            os.killpg(p.pid, signal.SIGKILL)
        except Exception:
            pass
        try:
            p.wait(timeout=3)
        except Exception:
            pass
        tf.seek(0)
        partial = tf.read().decode("utf-8", errors="replace").strip()
        sys.stdout.write(f"simctl openurl timed out after {t}s {partial}".strip())
        sys.exit(124)
PY
)"
    rc=$?
    if [ "$rc" -eq 0 ]; then
      printf '%s' "$out"
      return 0
    fi
    if [ "$allow_sim_restart" = "1" ] && echo "$out" | grep -Eqi "Connection interrupted|server died|NSMachErrorDomain|CoreSimulatorService|timed out"; then
      python3 - "$target_udid" <<'PY' >/dev/null 2>&1 || true
import os, signal, subprocess, sys
u = sys.argv[1]
for cmd in (
    ["open", "-g", "-a", "Simulator", "--args", "-CurrentDeviceUDID", u],
    ["xcrun", "simctl", "boot", u],
    ["xcrun", "simctl", "launch", u, "com.apple.shortcuts"],
):
    p = subprocess.Popen(cmd, stdin=subprocess.DEVNULL, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, start_new_session=True)
    try:
        p.wait(timeout=12)
    except subprocess.TimeoutExpired:
        try:
            os.killpg(p.pid, signal.SIGKILL)
        except Exception:
            pass
        try:
            p.wait(timeout=2)
        except Exception:
            pass
PY
    fi
    if [ "$attempt" -lt "$max_attempts" ]; then
      sleep 2
    fi
    attempt=$((attempt + 1))
  done
  if [ "$allow_sim_restart" = "1" ] && [ -n "$fallback_file" ] && [ -f "$fallback_file" ]; then
    python3 - "$fallback_file" <<'PY' >/dev/null 2>&1 || true
import os, signal, subprocess, sys
p = subprocess.Popen(["open", "-a", "Simulator", sys.argv[1]], stdin=subprocess.DEVNULL, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, start_new_session=True)
try:
    p.wait(timeout=12)
except subprocess.TimeoutExpired:
    try:
        os.killpg(p.pid, signal.SIGKILL)
    except Exception:
        pass
PY
  fi
  printf '%s' "$out"
  return "$rc"
}

if [ "$GATE_SHORTCUTS_APP_INSTALLED" = "PASS" ]; then
  python3 - "$UDID" <<'PY' >/dev/null 2>&1 || true
import subprocess, sys, time
u = sys.argv[1]
try:
    subprocess.run(["open", "-g", "-a", "Simulator", "--args", "-CurrentDeviceUDID", u], stdin=subprocess.DEVNULL, capture_output=True, timeout=15)
except Exception:
    pass
time.sleep(2)
PY
  if ! wait_for_springboard_display "$UDID" 60 && ! simctl_getenv_ok "$UDID"; then
    ensure_sim_session_ready "$UDID" 1
  fi
  python3 - "$UDID" <<'PY' >/dev/null 2>&1 || true
import subprocess, sys, time
u = sys.argv[1]
for cmd in (
    ["xcrun", "simctl", "launch", u, "com.apple.shortcuts"],
):
    try:
        subprocess.run(cmd, stdin=subprocess.DEVNULL, capture_output=True, timeout=30)
    except Exception:
        pass
time.sleep(3)
try:
    subprocess.run(["xcrun", "simctl", "terminate", u, "com.apple.shortcuts"], stdin=subprocess.DEVNULL, capture_output=True, timeout=20)
except Exception:
    pass
PY
fi

MASKED_UDID="$(echo "$UDID" | cut -c1-8)-REDACTED"
python3 - "$ARTIFACT_DIR/environment.json" "$UNAME_OUT" "$SW_VERS_OUT" "$XCODE_SELECT_OUT" "$XCODEBUILD_VER_OUT" "$MASKED_UDID" "$DEVICE_NAME" "$RUNTIME_NAME" "$RUNTIME_VERSION" "$SHORTCUTS_BUNDLE_PRESENT" "$SOURCE_SHA" <<'PY'
import json
import sys

out_path, uname_s, sw_vers_s, xcode_sel, xcode_ver, masked_udid, dev_name, rt_name, rt_ver, app_present, sha = sys.argv[1:12]
env_doc = {
    "schemaVersion": "1.4",
    "compilerCommitSha": sha,
    "uname": uname_s.strip(),
    "swVers": sw_vers_s.strip(),
    "xcodeSelectPath": xcode_sel.strip(),
    "xcodebuildVersion": xcode_ver.strip(),
    "simulator": {
        "maskedUdid": masked_udid,
        "deviceName": dev_name,
        "runtimeName": rt_name,
        "runtimeVersion": rt_ver,
        "isIPad": ("iPad" in dev_name),
        "shortcutsAppInstalled": (app_present == "true"),
        "shortcutsBundleId": "com.apple.shortcuts" if app_present == "true" else None,
    }
}
with open(out_path, "w", encoding="utf-8") as f:
    json.dump(env_doc, f, indent=2)
PY

# ---------------------------------------------------------------------------
# Phase 2: Import Decision Tree (HubSign Exact-File Signing + 4 Import Routes)
# ---------------------------------------------------------------------------
SIGNED_ARTIFACT_VALID_AEA1="false"
SIGNING_METHOD="NONE"
SIGNED_SHA256=""
SIGNED_BYTE_SIZE="0"
LEGACY_SIGNING_VERIFIED="false"

if [ "$PROBE_LEVEL" != "preflight" ] && [ "$GATE_SHORTCUTS_APP_INSTALLED" = "PASS" ]; then
  echo "=== Phase 2: Testing Import Decision Tree & HubSign Signing ==="
  GATE_FILE_IMPORT_ATTEMPTED="PASS"

  CHERRI_BIN="$ARTIFACT_DIR/builds/cherri"
  mkdir -p "$ARTIFACT_DIR/builds/signed"
  UNSIGNED_FILE="$ARTIFACT_DIR/builds/${TARGET_FIXTURE_ID}.shortcut"
  SIGNED_FILE="$ARTIFACT_DIR/builds/signed/${EXPECTED_SHORTCUT_NAME}.shortcut"
  SIGNED_CANDIDATE="$ARTIFACT_DIR/builds/${TARGET_FIXTURE_ID}-signed.shortcut"
  PRE_SIGNED_FIXTURE="$ROOT_DIR/tools/shortcuts-simulator-qa/fixtures/signed/${TARGET_FIXTURE_ID}.shortcut"
  LEGACY_UNSIGNED_FILE="$ARTIFACT_DIR/builds/${LEGACY_COUNTERPART_ID}.shortcut"
  LEGACY_SIGNED_CANDIDATE="$ARTIFACT_DIR/builds/${LEGACY_COUNTERPART_ID}-signed.shortcut"
  if [ "$UNSIGNED_FILE" = "$SIGNED_FILE" ]; then
    echo "ERROR: UNSIGNED_FILE and SIGNED_FILE must never alias ($UNSIGNED_FILE)" >&2
    exit 1
  fi
  if [ ! -s "$UNSIGNED_FILE" ]; then
    echo "ERROR: Compiled unsigned plist missing at $UNSIGNED_FILE" >&2
    exit 1
  fi

  ROUTE1_RC=-1
  ROUTE1_OUT=""
  ROUTE2_RC=-1
  ROUTE2_OUT=""
  ROUTE4_RC=-1
  ROUTE4_OUT=""

  run_negative_and_scheme_probes() {
    # Route 2: Unsigned .shortcut via simctl openurl (verify rejection of unsigned file)
    ROUTE2_OUT="$(simctl_openurl_resilient "$UDID" "file://$UNSIGNED_FILE" 1 "" 1 30)"
    ROUTE2_RC=$?
    sleep 2
    xcrun simctl io "$UDID" screenshot "$ARTIFACT_DIR/editor/route2-unsigned-openurl.png" 2>/dev/null || true
    xcrun simctl terminate "$UDID" com.apple.shortcuts >/dev/null 2>&1 || true

    # Route 4: Exploratory undocumented shortcuts://import-shortcut (explicitly tagged UNDOCUMENTED)
    ROUTE4_OUT="$(simctl_openurl_resilient "$UDID" "shortcuts://import-shortcut?name=CherriQAProbe" 1 "" 1 30)"
    ROUTE4_RC=$?
    sleep 1
    xcrun simctl terminate "$UDID" com.apple.shortcuts >/dev/null 2>&1 || true

    # Route 1: Documented shortcuts://create-shortcut URL scheme
    ROUTE1_OUT="$(simctl_openurl_resilient "$UDID" "shortcuts://create-shortcut" 1 "" 1 30)"
    ROUTE1_RC=$?
    sleep 2
    xcrun simctl io "$UDID" screenshot "$ARTIFACT_DIR/editor/route1-create-shortcut.png" 2>/dev/null || true
    if [ "$ROUTE1_RC" -eq 0 ]; then
      if [ "$GATE_SHORTCUTS_LAUNCHED" = "NOT_RUN" ]; then
        GATE_SHORTCUTS_LAUNCHED="PASS"
      fi
      if [ "$GATE_EDITOR_OPENED" = "NOT_RUN" ]; then
        GATE_EDITOR_OPENED="PASS"
      fi
    fi
    xcrun simctl terminate "$UDID" com.apple.shortcuts >/dev/null 2>&1 || true
  }

  # Route 3: Sign EXACT preflight-validated unsigned plist via Cherri HubSign (or CHERRI_SIGNING_SERVER if configured)
  rm -f "$SIGNED_CANDIDATE" "$SIGNED_FILE"
  SIGN_ARGS=(--hubsign)
  SIGN_CMD_LABEL="--hubsign"
  if [ -n "${CHERRI_SIGNING_SERVER:-}" ]; then
    SIGN_ARGS=(--signing-server="$CHERRI_SIGNING_SERVER")
    SIGN_CMD_LABEL="--signing-server=<redacted>"
  fi
  SIGN_OUT="$("$CHERRI_BIN" sign "$UNSIGNED_FILE" \
    --output="$SIGNED_CANDIDATE" \
    "${SIGN_ARGS[@]}" \
    --source-file="$TARGET_SOURCE_FILE" \
    --compiler-sha="$SOURCE_SHA" \
    --manifest-out="$ARTIFACT_DIR/builds/signing-provenance-target.json" \
    --no-ansi 2>&1)"
  SIGN_RC=$?

  if [ "$SIGN_RC" -eq 0 ] && [ -s "$SIGNED_CANDIDATE" ]; then
    HEADER_MAGIC="$(head -c 4 "$SIGNED_CANDIDATE" 2>/dev/null || true)"
    BYTE_COUNT="$(wc -c < "$SIGNED_CANDIDATE" | tr -d '[:space:]')"
    if [ "$HEADER_MAGIC" = "AEA1" ] && [ "${BYTE_COUNT:-0}" -ge 16 ]; then
      cp -f "$SIGNED_CANDIDATE" "$SIGNED_FILE"
      SIGNED_ARTIFACT_VALID_AEA1="true"
      SIGNING_METHOD="HUBSIGN_REMOTE_SERVICE"
      if [ -n "${CHERRI_SIGNING_SERVER:-}" ] && [[ "$CHERRI_SIGNING_SERVER" != *"shortcuty.app"* ]] && [[ "$CHERRI_SIGNING_SERVER" != *"hubsign.routinehub.services"* ]]; then
        SIGNING_METHOD="CUSTOM_REMOTE_SIGNING_SERVICE"
      fi
      SIGNED_SHA256="$(shasum -a 256 "$SIGNED_FILE" | awk '{print $1}')"
      SIGNED_BYTE_SIZE="$BYTE_COUNT"
    else
      rm -f "$SIGNED_CANDIDATE" "$SIGNED_FILE"
      SIGN_RC=1
      SIGN_OUT="HubSign returned invalid artifact header=$HEADER_MAGIC bytes=${BYTE_COUNT:-0}"
    fi
  fi

  # Also verify Legacy counterpart compiles and HubSigns cleanly using exact unsigned plist bytes
  rm -f "$LEGACY_SIGNED_CANDIDATE"
  LEGACY_SIGN_OUT=""
  LEGACY_SIGN_RC=1
  if [ -s "$LEGACY_UNSIGNED_FILE" ]; then
    LEGACY_SIGN_OUT="$("$CHERRI_BIN" sign "$LEGACY_UNSIGNED_FILE" \
      --output="$LEGACY_SIGNED_CANDIDATE" \
      "${SIGN_ARGS[@]}" \
      --source-file="$LEGACY_COUNTERPART_SOURCE" \
      --compiler-sha="$SOURCE_SHA" \
      --manifest-out="$ARTIFACT_DIR/builds/signing-provenance-legacy.json" \
      --no-ansi 2>&1)"
    LEGACY_SIGN_RC=$?
    if [ "$LEGACY_SIGN_RC" -eq 0 ] && [ -s "$LEGACY_SIGNED_CANDIDATE" ]; then
      LEG_MAGIC="$(head -c 4 "$LEGACY_SIGNED_CANDIDATE" 2>/dev/null || true)"
      LEG_SIZE="$(wc -c < "$LEGACY_SIGNED_CANDIDATE" | tr -d '[:space:]')"
      if [ "$LEG_MAGIC" = "AEA1" ] && [ "${LEG_SIZE:-0}" -ge 16 ]; then
        LEGACY_SIGNING_VERIFIED="true"
      else
        rm -f "$LEGACY_SIGNED_CANDIDATE"
        LEGACY_SIGN_RC=1
      fi
    fi
  fi

  # Diagnostic fallback ONLY if remote HubSign was unreachable or failed: record host CLI / pre-signed fixture behavior honestly
  if [ "$SIGNED_ARTIFACT_VALID_AEA1" != "true" ]; then
    if [ -x /usr/bin/shortcuts ]; then
      HOST_CLI_OUT="$(/usr/bin/shortcuts sign --mode anyone --input "$UNSIGNED_FILE" --output "$SIGNED_CANDIDATE" 2>&1)"
      HOST_CLI_RC=$?
      if [ "$HOST_CLI_RC" -eq 0 ] && [ -s "$SIGNED_CANDIDATE" ] && [ "$(head -c 4 "$SIGNED_CANDIDATE" 2>/dev/null || true)" = "AEA1" ]; then
        cp -f "$SIGNED_CANDIDATE" "$SIGNED_FILE"
        SIGNED_ARTIFACT_VALID_AEA1="true"
        SIGNING_METHOD="MACOS_HOST_SHORTCUTS_CLI"
        SIGNED_SHA256="$(shasum -a 256 "$SIGNED_FILE" | awk '{print $1}')"
        SIGNED_BYTE_SIZE="$(wc -c < "$SIGNED_FILE" | tr -d '[:space:]')"
      else
        SIGN_OUT="${SIGN_OUT} | macOS host CLI fallback failed: ${HOST_CLI_OUT}"
      fi
    fi
    if [ "$SIGNED_ARTIFACT_VALID_AEA1" != "true" ] && [ -s "$PRE_SIGNED_FIXTURE" ] && [ "$(head -c 4 "$PRE_SIGNED_FIXTURE" 2>/dev/null || true)" = "AEA1" ]; then
      cp -f "$PRE_SIGNED_FIXTURE" "$SIGNED_FILE"
      SIGNED_ARTIFACT_VALID_AEA1="true"
      SIGNING_METHOD="OWNER_PRE_SIGNED_SYNTHETIC_FIXTURE"
      SIGNED_SHA256="$(shasum -a 256 "$SIGNED_FILE" | awk '{print $1}')"
      SIGNED_BYTE_SIZE="$(wc -c < "$SIGNED_FILE" | tr -d '[:space:]')"
    fi
  fi

  # Write sanitized signing provenance summary (zero internal URLs, zero credentials, exact SHA-256 chain)
  python3 - "$ARTIFACT_DIR/runtime/signing-provenance.json" \
    "$ARTIFACT_DIR/builds/signing-provenance-target.json" \
    "$ARTIFACT_DIR/builds/signing-provenance-legacy.json" \
    "$SIGNED_ARTIFACT_VALID_AEA1" "$SIGNING_METHOD" "$SIGNED_SHA256" "$SIGNED_BYTE_SIZE" \
    "$LEGACY_SIGNING_VERIFIED" "$SOURCE_SHA" <<'PY'
import json, os, sys

out_path, tgt_p, leg_p, valid_s, method, sha, sz, leg_ok, commit_sha = sys.argv[1:10]
def load_opt(p):
    if os.path.exists(p):
        try:
            return json.load(open(p, encoding="utf-8"))
        except Exception:
            return None
    return None

doc = {
    "schemaVersion": "1.4",
    "compilerCommitSha": commit_sha,
    "signedArtifactValidAEA1": (valid_s == "true"),
    "signingMethod": method,
    "signedArtifactSha256": sha if sha else None,
    "signedByteSize": int(sz) if sz.isdigit() else 0,
    "legacyCounterpartSignedAEA1": (leg_ok == "true"),
    "targetProvenance": load_opt(tgt_p),
    "legacyProvenance": load_opt(leg_p),
}
with open(out_path, "w", encoding="utf-8") as f:
    json.dump(doc, f, indent=2)
PY

  if [ -n "${CHERRI_SIGNING_SERVER:-}" ]; then
    SIGN_OUT="${SIGN_OUT//$CHERRI_SIGNING_SERVER/<redacted-signing-server>}"
  fi
  ROUTE3_RC="$SIGN_RC"
  if [ "$SIGNED_ARTIFACT_VALID_AEA1" = "true" ]; then
    ROUTE3_OUT="AEA1 signed artifact verified ($SIGNED_BYTE_SIZE bytes, method=$SIGNING_METHOD, legacyCounterpart=$LEGACY_SIGNING_VERIFIED); deferred simctl openurl to coordinated XCUITest handshake."
  else
    ROUTE3_OUT="Blocked signing: $SIGN_OUT"
  fi

  write_import_attempts_json() {
    python3 - "$ARTIFACT_DIR/import-attempts.json" "$ROUTE1_RC" "$ROUTE1_OUT" "$ROUTE2_RC" "$ROUTE2_OUT" "$SIGN_RC" "$SIGN_OUT" "$ROUTE3_RC" "$ROUTE3_OUT" "$ROUTE4_RC" "$ROUTE4_OUT" "$SIGNED_ARTIFACT_VALID_AEA1" "$SIGNING_METHOD" "$SIGNED_SHA256" "$SIGNED_BYTE_SIZE" "$LEGACY_SIGNING_VERIFIED" "$SIGN_CMD_LABEL" <<'PY'
import json
import sys

out_path = sys.argv[1]
r1_rc, r1_out = int(sys.argv[2]), sys.argv[3]
r2_rc, r2_out = int(sys.argv[4]), sys.argv[5]
sign_rc, sign_out = int(sys.argv[6]), sys.argv[7]
r3_rc, r3_out = int(sys.argv[8]), sys.argv[9]
r4_rc, r4_out = int(sys.argv[10]), sys.argv[11]
aea1_valid, sign_method, signed_sha, signed_bytes, legacy_signed = (
    (sys.argv[12] == "true"),
    sys.argv[13],
    sys.argv[14],
    int(sys.argv[15]) if sys.argv[15].isdigit() else 0,
    (sys.argv[16] == "true"),
)
sign_cmd_label = sys.argv[17] if len(sys.argv) > 17 else "--hubsign"

attempts = {
    "schemaVersion": "1.4",
    "signedArtifactValidAEA1": aea1_valid,
    "signingMethod": sign_method,
    "signedArtifactSha256": signed_sha if signed_sha else None,
    "signedByteSize": signed_bytes,
    "legacyCounterpartSignedAEA1": legacy_signed,
    "routes": [
        {
            "routeId": "route_1_create_shortcut_scheme",
            "documented": True,
            "command": "xcrun simctl openurl <UDID> shortcuts://create-shortcut",
            "exitCode": r1_rc,
            "status": "PASS" if r1_rc == 0 else "BLOCKED_IMPORT",
            "observedOutput": r1_out[:512],
        },
        {
            "routeId": "route_2_unsigned_file_openurl",
            "documented": True,
            "command": "xcrun simctl openurl <UDID> file://<unsigned.shortcut>",
            "exitCode": r2_rc,
            "status": "UNSIGNED_REJECTED_EXPECTED_UI_MODAL" if r2_rc == 0 else "BLOCKED_SIGNING",
            "observedOutput": r2_out[:512],
        },
        {
            "routeId": "route_3_signed_shortcut_openurl",
            "documented": True,
            "command": f"cherri sign <unsigned.shortcut> {sign_cmd_label} && coordinated xcrun simctl openurl <UDID> file://<signed.shortcut>",
            "hostSignExitCode": sign_rc,
            "hostSignOutput": sign_out[:512],
            "openUrlExitCode": r3_rc,
            "status": "SIGNED_ARTIFACT_READY_FOR_COORDINATED_IMPORT" if aea1_valid else "BLOCKED_SIGNING",
            "observedOutput": r3_out[:512],
        },
        {
            "routeId": "route_4_exploratory_import_shortcut_scheme",
            "documented": False,
            "classification": "UNDOCUMENTED",
            "command": "xcrun simctl openurl <UDID> shortcuts://import-shortcut?name=CherriQAProbe",
            "exitCode": r4_rc,
            "status": "UNDOCUMENTED_PROBE",
            "observedOutput": r4_out[:512],
        }
    ]
}
with open(out_path, "w", encoding="utf-8") as f:
    json.dump(attempts, f, indent=2)
PY
  }

  if [ "$PROBE_LEVEL" = "import" ]; then
    run_negative_and_scheme_probes
    write_import_attempts_json
  fi

  if [ "$SIGNED_ARTIFACT_VALID_AEA1" != "true" ]; then
    GATE_COMPILER_ARTIFACT_IMPORTED="BLOCKED_SIGNING"
    GATE_SHORTCUT_EXISTS_IN_LIBRARY="BLOCKED_SIGNING"
  else
    GATE_COMPILER_ARTIFACT_IMPORTED="BLOCKED_IMPORT"
    GATE_SHORTCUT_EXISTS_IN_LIBRARY="BLOCKED_IMPORT"
  fi
fi

# ---------------------------------------------------------------------------
# Phase 3 & 4: Build & Execute XCTest / XCUIAutomation Suite + Receiver App
# ---------------------------------------------------------------------------
if [ "$PROBE_LEVEL" = "editor" ] || [ "$PROBE_LEVEL" = "runtime" ] || [ "$PROBE_LEVEL" = "full" ]; then
  if [ "$GATE_SHORTCUTS_APP_INSTALLED" = "PASS" ]; then
    echo "=== Phase 3 & 4: Running XCTest UI Automation & Callback Receiver ==="
    case "${QA_OBSERVE_PROJECT_PREREQUISITE:-false}" in
      true)
        # One original prepare call with bounded encrypted observations. A
        # failed observed call cannot fall back, repeat or consume a skip token.
        python3 "$ROOT_DIR/tools/shortcuts-simulator-qa/project_prerequisite_observation.py" \
          --repo "$ROOT_DIR" \
          --root "$RUNNER_TEMP/cherri-qa-project-prerequisite-${RUN_ID}-${GITHUB_RUN_ATTEMPT:-1}" \
          --qa-root "$ARTIFACT_DIR" --source-sha "$SOURCE_SHA" --run-id "$RUN_ID" \
          --attempt "${GITHUB_RUN_ATTEMPT:-1}" --fixture-id "$TARGET_FIXTURE_ID" \
          --bash "$BASH" --github-output "$GITHUB_OUTPUT" || exit 2
        ;;
      false)
        python3 "$ROOT_DIR/tools/shortcuts-simulator-qa/project_prerequisite.py" \
          --repo "$ROOT_DIR" --root "$ARTIFACT_DIR" --source-sha "$SOURCE_SHA" \
          --run-id "$RUN_ID" --fixture-id "$TARGET_FIXTURE_ID" --bash "$BASH" || exit 2
        ;;
      *) echo "ERROR: invalid project prerequisite observation mode" >&2; exit 2 ;;
    esac

    # A GitHub rerun reuses RUN_ID. A fresh random suffix prevents old callbacks
    # from a previous attempt or a retained simulator container satisfying this run.
    EXEC_RANDOM_NONCE="$(python3 -c 'import secrets; print(secrets.token_hex(16))')"
    if ! [[ "$EXEC_RANDOM_NONCE" =~ ^[0-9a-f]{32}$ ]]; then
      echo "ERROR: could not generate a fresh runtime nonce" >&2
      exit 2
    fi
    EXEC_RUN_NONCE="${TARGET_FIXTURE_ID}-${RUN_ID}-${EXEC_RANDOM_NONCE}"
    case "$TARGET_FIXTURE_ID" in
      qa-debug-label-*)
        python3 "$ROOT_DIR/tools/shortcuts-simulator-qa/debug_label_preview.py" prepare \
          --root "$ARTIFACT_DIR" --fixture "$TARGET_FIXTURE_ID" --commit "$SOURCE_SHA" \
          --run-id "$RUN_ID" --attempt "${GITHUB_RUN_ATTEMPT:-1}" --nonce "$EXEC_RUN_NONCE" || exit 2
        ;;
    esac
    SHORTCUT_INPUT_VALUE="CHERRI_QA_INPUT"
    if [ "$HTTP_PROBE_ENABLED" = "true" ]; then
      # Static source bytes are never rewritten: only reviewed loopback port/nonce
      # enter through ShortcutInput. Keep this finite server alive through Phase 4B.
      http_probe serve --root "$ARTIFACT_DIR" --fixture "$TARGET_FIXTURE_ID" --nonce "$EXEC_RUN_NONCE" --commit "$SOURCE_SHA" >"$ARTIFACT_DIR/runtime/http-probe-server.log" 2>&1 &
      HTTP_PROBE_PID=$!
      HTTP_PROBE_STARTED="true"
      if ! SHORTCUT_INPUT_VALUE="$(http_probe input --root "$ARTIFACT_DIR" --fixture "$TARGET_FIXTURE_ID" --nonce "$EXEC_RUN_NONCE" --commit "$SOURCE_SHA")"; then
        # The helper emits an exact bounded startup schema; never print raw
        # server stdout, exception messages, paths, headers or response bodies.
        http_probe startup-report --root "$ARTIFACT_DIR" --fixture "$TARGET_FIXTURE_ID" --nonce "$EXEC_RUN_NONCE" --commit "$SOURCE_SHA"
        stop_http_probe
        echo "ERROR: bounded HTTP probe did not bind its reviewed loopback endpoint" >&2
        exit 2
      fi
    fi
    if [ "$STORAGE_CROSS_RUN_ENABLED" = "true" ]; then
      python3 "$ROOT_DIR/tools/shortcuts-simulator-qa/storage_cross_run.py" prepare --root "$ARTIFACT_DIR" --nonce "$EXEC_RUN_NONCE" --commit "$SOURCE_SHA" --udid "$UDID" || { echo "ERROR: storage phase preparation rejected before native launch" >&2; exit 2; }
      SHORTCUT_INPUT_VALUE="$(python3 "$ROOT_DIR/tools/shortcuts-simulator-qa/storage_cross_run.py" input --root "$ARTIFACT_DIR" --phase WRITE)" || { echo "ERROR: storage phase preparation rejected before native launch" >&2; exit 2; }
      EXPECTED_SEMANTIC_OUTPUT="CHERRI_QA_STORAGE_CROSS_RUN_RESULT:WRITE:TYPE_OK:VALUE_OK"
    fi
    RUN_URL="$(python3 - "$EXPECTED_SHORTCUT_NAME" "$EXEC_RUN_NONCE" "$TARGET_FIXTURE_ID" "$SHORTCUT_INPUT_VALUE" <<'PY'
import sys, urllib.parse
name, run_nonce, fid, input_text = sys.argv[1:5]
q = urllib.parse.quote
x_success = f"cherriqa://success?runId={q(run_nonce)}&fixtureId={q(fid)}&shortcutName={q(name)}&source=shortcuts-x-callback"
x_cancel = f"cherriqa://cancel?runId={q(run_nonce)}&fixtureId={q(fid)}&shortcutName={q(name)}&source=shortcuts-x-callback"
x_error = f"cherriqa://error?runId={q(run_nonce)}&fixtureId={q(fid)}&shortcutName={q(name)}&source=shortcuts-x-callback"
print(f"shortcuts://x-callback-url/run-shortcut?name={q(name)}&input=text&text={q(input_text, safe='')}&x-source=CherriSimulatorQAReceiver&x-success={q(x_success, safe='')}&x-cancel={q(x_cancel, safe='')}&x-error={q(x_error, safe='')}")
PY
)"
    RUN_URL_MINIMAL="$(python3 - "$EXPECTED_SHORTCUT_NAME" "$EXEC_RUN_NONCE" "$TARGET_FIXTURE_ID" <<'PY'
import sys, urllib.parse
name, run_nonce, fid = sys.argv[1:4]
q = urllib.parse.quote
x_success = f"cherriqa://success?runId={q(run_nonce)}&fixtureId={q(fid)}&shortcutName={q(name)}&source=shortcuts-x-callback"
x_cancel = f"cherriqa://cancel?runId={q(run_nonce)}&fixtureId={q(fid)}&shortcutName={q(name)}&source=shortcuts-x-callback"
x_error = f"cherriqa://error?runId={q(run_nonce)}&fixtureId={q(fid)}&shortcutName={q(name)}&source=shortcuts-x-callback"
print(f"shortcuts://x-callback-url/run-shortcut?name={q(name)}&x-source=CherriSimulatorQAReceiver&x-success={q(x_success, safe='')}&x-cancel={q(x_cancel, safe='')}&x-error={q(x_error, safe='')}")
PY
)"
    RUN_URL_PLAIN="$(python3 - "$EXPECTED_SHORTCUT_NAME" <<'PY'
import sys, urllib.parse
print(f"shortcuts://run-shortcut?name={urllib.parse.quote(sys.argv[1])}")
PY
)"

    if [ "$HTTP_PROBE_ENABLED" = "true" ] || [ "$STORAGE_CROSS_RUN_ENABLED" = "true" ]; then
      # These fixtures need the exact endpoint input; a no-input fallback would
      # test a different URL and cannot establish native HTTP behavior.
      RUN_URL_MINIMAL=""
      RUN_URL_PLAIN=""
    fi
    export TEST_RUNNER_CHERRI_QA_SHARED_OUTPUT_DIR="$ARTIFACT_DIR/editor"
    export TEST_RUNNER_CHERRI_QA_MAX_EDITOR_PAGES="$MAX_EDITOR_PAGES"
    export TEST_RUNNER_CHERRI_QA_EXPECTED_SHORTCUT_NAME="$EXPECTED_SHORTCUT_NAME"
    export TEST_RUNNER_CHERRI_QA_EXPECT_SIGNED_IMPORT="$SIGNED_ARTIFACT_VALID_AEA1"
    export TEST_RUNNER_CHERRI_QA_EXPECT_RUNTIME_RUN="$SIGNED_ARTIFACT_VALID_AEA1"
    export TEST_RUNNER_CHERRI_QA_DEFER_RUNTIME_TO_PHASE4B="true"
    export TEST_RUNNER_CHERRI_QA_EXEC_RUN_NONCE="$EXEC_RUN_NONCE"
    export TEST_RUNNER_CHERRI_QA_SOURCE_SHA="$SOURCE_SHA"
    export TEST_RUNNER_CHERRI_QA_WORKFLOW_RUN_ID="$RUN_ID"
    export TEST_RUNNER_CHERRI_QA_WORKFLOW_RUN_ATTEMPT="${GITHUB_RUN_ATTEMPT:-1}"
    export TEST_RUNNER_CHERRI_QA_RUN_URL="$RUN_URL"
    export TEST_RUNNER_CHERRI_QA_RUN_URL_MINIMAL="$RUN_URL_MINIMAL"
    export TEST_RUNNER_CHERRI_QA_RUN_URL_PLAIN="$RUN_URL_PLAIN"

    stop_bg_pid() {
      local pid="${1:-}"
      if [ -n "$pid" ]; then
        kill "$pid" 2>/dev/null || true
        sleep 0.5
        kill -9 "$pid" 2>/dev/null || true
      fi
    }
    run_cmd_timeout() {
      local timeout_sec="$1"
      shift
      python3 - "$timeout_sec" "$@" <<'PY'
import os, signal, subprocess, sys
t = int(sys.argv[1])
cmd = sys.argv[2:]
p = subprocess.Popen(cmd, start_new_session=True)
try:
    sys.exit(p.wait(timeout=t))
except subprocess.TimeoutExpired:
    try:
        os.killpg(p.pid, signal.SIGKILL)
    except Exception:
        pass
    try:
        p.wait(timeout=5)
    except Exception:
        pass
    sys.stderr.write(f"command timed out after {t}s: {' '.join(cmd[:4])}\n")
    sys.exit(124)
PY
    }

    DERIVED_DATA_PATH="$QA_SCRATCH_DIR/DerivedData"
    XCRESULT_PATH="$QA_SCRATCH_DIR/CherriSimulatorQA.xcresult"
    XCRESULT_PART1_PATH="$QA_SCRATCH_DIR/CherriSimulatorQA-Part1.xcresult"
    rm -rf "$XCRESULT_PATH" "$XCRESULT_PART1_PATH" "$DERIVED_DATA_PATH" "$ARTIFACT_DIR/tests/"*.xcresult

    mkdir -p "$ARTIFACT_DIR/builds"
    run_cmd_timeout 600 xcodebuild build-for-testing \
      -project "$ROOT_DIR/tools/shortcuts-simulator-qa/XcodeProject/CherriSimulatorQA.xcodeproj" \
      -scheme CherriSimulatorQAReceiver \
      -derivedDataPath "$DERIVED_DATA_PATH" \
      -destination "platform=iOS Simulator,id=$UDID" \
      CODE_SIGNING_ALLOWED=NO \
      > "$ARTIFACT_DIR/builds/xcodebuild-build.log" 2>&1
    XCODEBUILD_BUILD_RC=$?
    echo "xcodebuild build-for-testing exit code: $XCODEBUILD_BUILD_RC"
    if [ "$XCODEBUILD_BUILD_RC" -ne 0 ] && [ -f "$ARTIFACT_DIR/builds/xcodebuild-build.log" ]; then
      sed "s/$UDID/<UDID>/g" "$ARTIFACT_DIR/builds/xcodebuild-build.log" | tail -n 30 || true
    fi

    ensure_sim_session_ready "$UDID"

    RAW_SIM_LOG="$ARTIFACT_DIR/runtime/raw-sim-log.txt"
    rm -f "$RAW_SIM_LOG"
    xcrun simctl spawn "$UDID" log stream --predicate '(process == "Shortcuts" || process == "BackgroundShortcutRunner" || process == "siriactionsd" || process == "SpringBoard" || subsystem == "com.apple.shortcuts" || subsystem == "com.apple.WorkflowKit" || process == "CherriSimulatorQAReceiver") AND NOT (subsystem == "com.apple.dt.xctest") AND NOT (subsystem == "com.apple.xpc.transaction")' --level info --style compact > "$RAW_SIM_LOG" 2>&1 &
    SIM_LOG_PID=$!

    rm -f "$ARTIFACT_DIR/editor/request-"* "$ARTIFACT_DIR/editor/ack-"* "$ARTIFACT_DIR/editor/pre-import-ack.txt" "$ARTIFACT_DIR/editor/handshake-stop.txt"

    # Stage 3.1: Run Receiver self-test + sanitizer + confirm-noop tests cleanly so XCTest exits before file:// import
    run_cmd_timeout 420 xcodebuild test-without-building \
      -project "$ROOT_DIR/tools/shortcuts-simulator-qa/XcodeProject/CherriSimulatorQA.xcodeproj" \
      -scheme CherriSimulatorQAReceiver \
      -derivedDataPath "$DERIVED_DATA_PATH" \
      -destination "platform=iOS Simulator,id=$UDID" \
      -only-testing:CherriSimulatorQAUITests/CherriSimulatorQAUITests/test01_ReceiverSelfCallbackAndContainerPersistence \
      -only-testing:CherriSimulatorQAUITests/CherriSimulatorQAUITests/test03_SanitizerRedactsSecretsAndNonAllowlistedStrings \
      -only-testing:CherriSimulatorQAUITests/CherriSimulatorQAUITests/test04ConfirmPendingRuntimeDialog \
      -resultBundlePath "$XCRESULT_PART1_PATH" \
      CODE_SIGNING_ALLOWED=NO \
      > "$ARTIFACT_DIR/builds/xcodebuild-part1.log" 2>&1
    XCODEBUILD_PART1_RC=$?
    echo "xcodebuild test part1 exit code: $XCODEBUILD_PART1_RC"
    if [ "$XCODEBUILD_PART1_RC" -ne 0 ] && [ -f "$ARTIFACT_DIR/builds/xcodebuild-part1.log" ]; then
      grep -Ev "DVTAssertions|nativeArchitecture|Apple Watch|SimDevice :|Object:[[:space:]]+<DVTiPhoneSimulator|Method:[[:space:]]+-nativeArchitecture|Thread:[[:space:]]+<_NSMainThread|feedbackassistant\.apple\.com|^[[:space:]]*\)[[:space:]]*$" "$ARTIFACT_DIR/builds/xcodebuild-part1.log" | sed "s/$UDID/<UDID>/g; s|/Users/[^/ ]*|/Users/<redacted>|g" | tail -n 25 || true
    fi

    # Save Part1 self-test callback store immediately in case Simulator resprings/migrates containers before Phase 5
    PART1_RECEIVER_DATA_DIR="$(xcrun simctl get_app_container "$UDID" org.cherrilang.qa.Receiver data 2>/dev/null || true)"
    if [ -n "$PART1_RECEIVER_DATA_DIR" ] && [ -f "$PART1_RECEIVER_DATA_DIR/Documents/callback-events.json" ]; then
      cp "$PART1_RECEIVER_DATA_DIR/Documents/callback-events.json" "$ARTIFACT_DIR/runtime/callback-events-part1.json" || true
    fi
    if [ "$XCODEBUILD_PART1_RC" -ne 0 ]; then
      if [ "$XCODEBUILD_PART1_RC" -eq 65 ] || ! simctl_getenv_ok "$UDID"; then
        echo "Part1 exited with rc=$XCODEBUILD_PART1_RC; forcing clean Simulator session recovery before Phase 3b..."
        ensure_sim_session_ready "$UDID" 1
      else
        echo "Part1 exited with rc=$XCODEBUILD_PART1_RC while CoreSimulator remains healthy; skipping destructive reboot."
      fi
      if [ -n "${SIM_LOG_PID:-}" ] && ! kill -0 "$SIM_LOG_PID" 2>/dev/null; then
        xcrun simctl spawn "$UDID" log stream --predicate '(process == "Shortcuts" || process == "BackgroundShortcutRunner" || process == "siriactionsd" || process == "SpringBoard" || subsystem == "com.apple.shortcuts" || subsystem == "com.apple.WorkflowKit" || process == "CherriSimulatorQAReceiver") AND NOT (subsystem == "com.apple.dt.xctest") AND NOT (subsystem == "com.apple.xpc.transaction")' --level info --style compact >> "$RAW_SIM_LOG" 2>&1 &
        SIM_LOG_PID=$!
      fi
    fi

    import_signed_via_localhost_http() {
      local target_udid="$1"
      local signed_path="$2"
      local sc_name="$3"
      python3 - "$target_udid" "$signed_path" "$sc_name" <<'PY'
import functools, http.server, os, shutil, signal, socketserver, subprocess, sys, tempfile, threading, urllib.parse

udid, spath, name = sys.argv[1:4]
serve_tmp = tempfile.mkdtemp(prefix="cherri-http-import-")
fname = f"{name}.shortcut"
staged_path = os.path.join(serve_tmp, fname)
shutil.copy2(spath, staged_path)
served = threading.Event()

class QuietHandler(http.server.SimpleHTTPRequestHandler):
    def end_headers(self):
        self.send_header("Content-Disposition", f'attachment; filename="{fname}"')
        super().end_headers()
    def do_GET(self):
        served.set()
        super().do_GET()
    def guess_type(self, path):
        return "application/x-apple-shortcut"
    def log_message(self, fmt, *args):
        pass

handler = functools.partial(QuietHandler, directory=serve_tmp)
httpd = socketserver.TCPServer(("127.0.0.1", 0), handler)
port = httpd.server_address[1]
t = threading.Thread(target=httpd.serve_forever, daemon=True)
t.start()
import_url = f"shortcuts://import-shortcut?url={urllib.parse.quote(f'http://127.0.0.1:{port}/{fname}', safe='')}&name={urllib.parse.quote(name, safe='')}"
p = subprocess.Popen(["xcrun", "simctl", "openurl", udid, import_url], stdin=subprocess.DEVNULL, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, start_new_session=True)
try:
    rc = p.wait(timeout=25)
    out = (p.stdout.read() or b"").decode("utf-8", errors="replace")
except subprocess.TimeoutExpired:
    try:
        os.killpg(p.pid, signal.SIGKILL)
    except Exception:
        pass
    rc = 124
    out = "localhost import-shortcut timed out after 25s"
if rc == 0:
    served.wait(10)
httpd.shutdown()
httpd.server_close()
shutil.rmtree(serve_tmp, ignore_errors=True)
print(f"rc={rc} served={served.is_set()} {out.strip()}")
sys.exit(rc)
PY
    }

    # Stage 3.2: Pre-XCTest outside-runner signed .shortcut import so simctl openurl file:// never contends with active AX/XCTest
    if [ "$SIGNED_ARTIFACT_VALID_AEA1" = "true" ] && [ -n "${SIGNED_FILE:-}" ] && [ -s "${SIGNED_FILE:-}" ]; then
      if ! ensure_phase_session_ready pre-import; then
        echo "Pre-import selected-device session boundary was not verified."
        exit 70
      fi
      echo "=== Phase 3b: Pre-XCTest outside-runner signed .shortcut import openurl ==="
      run_cmd_timeout 20 xcrun simctl launch "$UDID" com.apple.shortcuts >/dev/null 2>&1 || true
      sleep 4
      PRE_IMP_METHOD="file_primary"
      PRE_IMP_T0=$SECONDS
      PRE_IMP_RC=0
      PRE_IMP_OUT="$(simctl_openurl_resilient "$UDID" "file://$SIGNED_FILE" 1 "" 0 45)" || PRE_IMP_RC=$?
      PRE_IMP_DT=$((SECONDS - PRE_IMP_T0))
      if [ "$PRE_IMP_RC" -ne 0 ]; then
        TMP_SIGNED_DIR="$QA_SCRATCH_DIR/import-named"
        mkdir -p "$TMP_SIGNED_DIR"
        TMP_SIGNED_IMPORT="$TMP_SIGNED_DIR/${EXPECTED_SHORTCUT_NAME}.shortcut"
        cp -f "$SIGNED_FILE" "$TMP_SIGNED_IMPORT"
        PRE_IMP_METHOD="file_tmp"
        _t1=$SECONDS
        PRE_IMP2_RC=0
        PRE_IMP2_OUT="$(simctl_openurl_resilient "$UDID" "file://$TMP_SIGNED_IMPORT" 1 "" 0 30)" || PRE_IMP2_RC=$?
        _dt1=$((SECONDS - _t1))
        PRE_IMP_RC=$PRE_IMP2_RC
        PRE_IMP_DT=$((PRE_IMP_DT + _dt1))
        PRE_IMP_OUT="${PRE_IMP_OUT} | tmp=${PRE_IMP2_OUT}"
        rm -rf "$TMP_SIGNED_DIR"
      fi
      if [ "$PRE_IMP_RC" -ne 0 ]; then
        PRE_IMP_METHOD="localhost_import_scheme_signed_aea1"
        _t2=$SECONDS
        PRE_IMP3_RC=0
        PRE_IMP3_OUT="$(import_signed_via_localhost_http "$UDID" "$SIGNED_FILE" "$EXPECTED_SHORTCUT_NAME" 2>&1)" || PRE_IMP3_RC=$?
        _dt2=$((SECONDS - _t2))
        PRE_IMP_RC=$PRE_IMP3_RC
        PRE_IMP_DT=$((PRE_IMP_DT + _dt2))
        PRE_IMP_OUT="${PRE_IMP_OUT} | http_scheme=${PRE_IMP3_OUT}"
      fi
      sleep 2
      run_cmd_timeout 15 xcrun simctl io "$UDID" screenshot "$ARTIFACT_DIR/editor/pre-import-screen.png" >/dev/null 2>&1 || true
      PRE_SCREEN_BYTES=0
      if [ -f "$ARTIFACT_DIR/editor/pre-import-screen.png" ]; then
        PRE_SCREEN_BYTES="$(wc -c < "$ARTIFACT_DIR/editor/pre-import-screen.png" | tr -d '[:space:]')"
      fi
      if [ "${PRE_SCREEN_BYTES:-0}" -ge 1 ] && [ "${PRE_SCREEN_BYTES:-0}" -lt 120000 ]; then
        echo "Phase 3b detected boot/respring screen (${PRE_SCREEN_BYTES} B); waiting for SpringBoard settle before re-dispatching..."
        wait_for_springboard_display "$UDID" 90 || true
        run_cmd_timeout 20 xcrun simctl launch "$UDID" com.apple.shortcuts >/dev/null 2>&1 || true
        sleep 4
        _t3=$SECONDS
        PRE_IMP_RETRY_RC=0
        PRE_IMP_RETRY_OUT="$(simctl_openurl_resilient "$UDID" "file://$SIGNED_FILE" 1 "" 0 40)" || PRE_IMP_RETRY_RC=$?
        if [ "$PRE_IMP_RETRY_RC" -ne 0 ]; then
          PRE_IMP_RETRY2_OUT="$(import_signed_via_localhost_http "$UDID" "$SIGNED_FILE" "$EXPECTED_SHORTCUT_NAME" 2>&1)" || PRE_IMP_RETRY_RC=$?
          PRE_IMP_RETRY_OUT="${PRE_IMP_RETRY_OUT} | http_retry=${PRE_IMP_RETRY2_OUT}"
        fi
        PRE_IMP_DT=$((PRE_IMP_DT + SECONDS - _t3))
        if [ "$PRE_IMP_RETRY_RC" -eq 0 ]; then
          PRE_IMP_METHOD="${PRE_IMP_METHOD}_post_recovery_retry"
          PRE_IMP_RC=0
          PRE_IMP_OUT="${PRE_IMP_OUT} | post_recovery=ok"
        fi
        sleep 2
        run_cmd_timeout 25 xcrun simctl io "$UDID" screenshot "$ARTIFACT_DIR/editor/pre-import-screen.png" >/dev/null 2>&1 || true
      elif [ "${PRE_SCREEN_BYTES:-0}" -eq 0 ] && ! simctl_getenv_ok "$UDID"; then
        echo "Phase 3b detected unhealthy CoreSimulator (0 B + getenv failed); recovering session..."
        ensure_sim_session_ready "$UDID" 1
      fi
      printf 'rc=%s\nmethod=%s durationSec=%s %s\n' "$PRE_IMP_RC" "$PRE_IMP_METHOD" "$PRE_IMP_DT" "${PRE_IMP_OUT:0:180}" > "$ARTIFACT_DIR/editor/pre-import-ack.txt"
      echo "Phase 3b pre-import ($PRE_IMP_METHOD) rc=$PRE_IMP_RC durationSec=${PRE_IMP_DT}s out=$(printf '%s' "$PRE_IMP_OUT" | sed "s/$UDID/<UDID>/g" | tr '\n' ' ' | cut -c1-180)"
    fi
    (
      import_dispatched=0
      import_retried=0
      run_dispatched=0
      for _tick in $(seq 1 4500); do
        if [ "$import_dispatched" -eq 0 ] && [ -f "$ARTIFACT_DIR/editor/request-signed-import.txt" ]; then
          import_dispatched=1
          if [ "$SIGNED_ARTIFACT_VALID_AEA1" = "true" ] && [ -n "${SIGNED_FILE:-}" ] && [ -s "${SIGNED_FILE:-}" ]; then
            _t0=$SECONDS
            IMP_RC=0
            IMP_OUT="$(simctl_openurl_resilient "$UDID" "file://$SIGNED_FILE" 1 "" 0 45)" || IMP_RC=$?
            _dt=$((SECONDS - _t0))
            printf 'rc=%s\ndurationSec=%s %s\n' "$IMP_RC" "$_dt" "${IMP_OUT:0:220}" > "$ARTIFACT_DIR/editor/ack-signed-import.txt"
          else
            printf 'rc=1\nNO_VALID_AEA1\n' > "$ARTIFACT_DIR/editor/ack-signed-import.txt"
          fi
        fi
        if [ "$import_retried" -eq 0 ] && [ -f "$ARTIFACT_DIR/editor/request-signed-import-retry.txt" ]; then
          import_retried=1
          if [ "$SIGNED_ARTIFACT_VALID_AEA1" = "true" ] && [ -n "${SIGNED_FILE:-}" ] && [ -s "${SIGNED_FILE:-}" ]; then
            _t0=$SECONDS
            IMP2_RC=0
            IMP2_OUT="$(simctl_openurl_resilient "$UDID" "file://$SIGNED_FILE" 1 "" 0 35)" || IMP2_RC=$?
            if [ "$IMP2_RC" -ne 0 ]; then
              IMP2_HTTP_RC=0
              IMP2_HTTP_OUT="$(import_signed_via_localhost_http "$UDID" "$SIGNED_FILE" "$EXPECTED_SHORTCUT_NAME" 2>&1)" || IMP2_HTTP_RC=$?
              IMP2_RC=$IMP2_HTTP_RC
              IMP2_OUT="${IMP2_OUT} | http_scheme=${IMP2_HTTP_OUT}"
            fi
            _dt=$((SECONDS - _t0))
            printf 'rc=%s\ndurationSec=%s %s\n' "$IMP2_RC" "$_dt" "${IMP2_OUT:0:220}" > "$ARTIFACT_DIR/editor/ack-signed-import-retry.txt"
          fi
        fi
        if [ "$run_dispatched" -eq 0 ] && [ -f "$ARTIFACT_DIR/editor/request-shortcut-run.txt" ]; then
          run_dispatched=1
          _t0=$SECONDS
          RUN_RC=0
          RUN_OUT="$(simctl_openurl_resilient "$UDID" "$RUN_URL" 1 "" 0 35)" || RUN_RC=$?
          _dt=$((SECONDS - _t0))
          printf 'rc=%s\ndurationSec=%s %s\n' "$RUN_RC" "$_dt" "${RUN_OUT:0:220}" > "$ARTIFACT_DIR/editor/ack-shortcut-run.txt"
        fi
        [ -f "$ARTIFACT_DIR/editor/handshake-stop.txt" ] && break
        sleep 0.2
      done
    ) &
    HANDSHAKE_PID=$!
    cleanup_handshake() {
      if [ -n "${HANDSHAKE_PID:-}" ]; then
        touch "$ARTIFACT_DIR/editor/handshake-stop.txt" 2>/dev/null || true
        stop_bg_pid "$HANDSHAKE_PID"
        HANDSHAKE_PID=""
      fi
      if [ -n "${SIM_LOG_PID:-}" ]; then
        stop_bg_pid "$SIM_LOG_PID"
        SIM_LOG_PID=""
      fi
      if [ -n "${PHASE4_LOG_PID:-}" ]; then
        stop_bg_pid "$PHASE4_LOG_PID"
        PHASE4_LOG_PID=""
      fi
    }
    trap 'stop_http_probe; cleanup_handshake; rm -rf "${XCRESULT_PATH:-}" "${XCRESULT_PART1_PATH:-}" "${CONFIRM_XCRESULT:-}" "${QA_SCRATCH_DIR:-}"' EXIT

    # Xcode must currently advertise this same owned simulator as an eligible
    # destination. One non-reboot readiness refresh and test02 share the original
    # 900-second phase; the pre-issued import and handshake are not repeated.
    # This persisted monotonic origin precedes the unchanged 900-second test phase.
    qa_acquisition_checkpoint TEST02_STARTED || exit 2
    python3 "$ROOT_DIR/tools/shortcuts-simulator-qa/xcode_destination_readiness.py" \
      --repo "$ROOT_DIR" --scratch "$QA_SCRATCH_DIR" --artifacts "$ARTIFACT_DIR" \
      --udid "$UDID" --source-sha "$SOURCE_SHA" --run-id "$RUN_ID" \
      --fixture-id "$TARGET_FIXTURE_ID"
    XCODEBUILD_TEST_RC=$?
    echo "test02 destination/test phase exit code: $XCODEBUILD_TEST_RC"
    qa_acquisition_checkpoint TEST02_COMPLETED "$XCODEBUILD_TEST_RC" || exit 2
    if [ "$XCODEBUILD_TEST_RC" -ne 0 ] && [ -f "$ARTIFACT_DIR/builds/xcodebuild-test.log" ]; then
      grep -Ev "DVTAssertions|nativeArchitecture|Apple Watch|SimDevice :|Object:[[:space:]]+<DVTiPhoneSimulator|Method:[[:space:]]+-nativeArchitecture|Thread:[[:space:]]+<_NSMainThread|feedbackassistant\.apple\.com|^[[:space:]]*\)[[:space:]]*$" "$ARTIFACT_DIR/builds/xcodebuild-test.log" | sed "s/$UDID/<UDID>/g; s|/Users/[^/ ]*|/Users/<redacted>|g" | tail -n 35 || true
    fi

    cleanup_handshake
    trap 'stop_http_probe' EXIT

    DID_HANDSHAKE_RUN="false"
    if [ -f "$ARTIFACT_DIR/editor/ack-shortcut-run.txt" ]; then
      DID_HANDSHAKE_RUN="true"
    fi
    python3 - "$ARTIFACT_DIR/editor" "$UDID" "$ROOT_DIR" <<'PY'
import json, os, re, sys
ed, udid, root_dir = sys.argv[1:4]
def read_ack(name):
    p = os.path.join(ed, name)
    if not os.path.exists(p):
        return None
    raw = open(p, encoding="utf-8", errors="replace").read()
    if udid:
        raw = raw.replace(udid, "<UDID>")
    if root_dir:
        raw = raw.replace(root_dir, "<REPO_ROOT>")
    raw = re.sub(r"/Users/[^/\s]+", "/Users/<redacted>", raw)
    lines = raw.splitlines()
    rc = None
    if lines and lines[0].startswith("rc="):
        try:
            rc = int(lines[0][3:].strip())
        except Exception:
            rc = None
        msg = "\n".join(lines[1:]).strip()
    else:
        msg = raw.strip()
    return {"rc": rc, "message": msg[:256]}

doc = {
    "schemaVersion": "1.4",
    "preImportAttempted": os.path.exists(os.path.join(ed, "pre-import-ack.txt")),
    "preImportAck": read_ack("pre-import-ack.txt"),
    "importRequested": os.path.exists(os.path.join(ed, "request-signed-import.txt")),
    "importAck": read_ack("ack-signed-import.txt"),
    "importRetried": os.path.exists(os.path.join(ed, "request-signed-import-retry.txt")),
    "importRetryAck": read_ack("ack-signed-import-retry.txt"),
    "runRequested": os.path.exists(os.path.join(ed, "request-shortcut-run.txt")),
    "runAck": read_ack("ack-shortcut-run.txt"),
}
with open(os.path.join(ed, "handshake-status.json"), "w", encoding="utf-8") as f:
    json.dump(doc, f, indent=2)
PY
    rm -f "$ARTIFACT_DIR/editor/request-"* "$ARTIFACT_DIR/editor/ack-"* "$ARTIFACT_DIR/editor/pre-import-ack.txt" "$ARTIFACT_DIR/editor/handshake-stop.txt"

    PART1_JSON="$ARTIFACT_DIR/tests/xcresult-part1.json"
    PART2_JSON="$ARTIFACT_DIR/tests/xcresult-part2.json"
    rm -f "$PART1_JSON" "$PART2_JSON"
    # Metadata collection shares the original phase deadline; each original
    # stdout/stderr remains private and hash-bound even after a failed query.
    python3 "$ROOT_DIR/tools/shortcuts-simulator-qa/qa_terminal_receipts.py" collect \
      --repo "$ROOT_DIR" --root "$ARTIFACT_DIR" --source-sha "$SOURCE_SHA" \
      --run-id "$RUN_ID" --fixture-id "$TARGET_FIXTURE_ID" --udid "$UDID" \
      --scratch "$QA_SCRATCH_DIR"
    POST_TEST_INSPECTION_RC=$?
    python3 - "$ARTIFACT_DIR/tests/xcresult-summary.json" "$PART1_JSON" "$PART2_JSON" "$UDID" "$MASKED_UDID" <<'PY'
import json, os, sys
out_path, p1_path, p2_path, udid, masked = sys.argv[1:6]
docs = []
for p in (p1_path, p2_path):
    if os.path.exists(p) and os.path.getsize(p) > 0:
        try:
            docs.append(json.load(open(p, encoding="utf-8")))
        except Exception:
            pass
if len(docs) == 2:
    merged = dict(docs[1])
    merged["passedTests"] = docs[0].get("passedTests", 0) + docs[1].get("passedTests", 0)
    merged["failedTests"] = docs[0].get("failedTests", 0) + docs[1].get("failedTests", 0)
    merged["skippedTests"] = docs[0].get("skippedTests", 0) + docs[1].get("skippedTests", 0)
    merged["totalTestCount"] = docs[0].get("totalTestCount", 0) + docs[1].get("totalTestCount", 0)
    merged["expectedFailures"] = docs[0].get("expectedFailures", 0) + docs[1].get("expectedFailures", 0)
    merged["testFailures"] = list(docs[0].get("testFailures", [])) + list(docs[1].get("testFailures", []))
    merged["result"] = "Passed" if merged["failedTests"] == 0 and docs[0].get("result") == "Passed" and docs[1].get("result") == "Passed" else "Failed"
    raw = json.dumps(merged, indent=2)
elif len(docs) == 1:
    raw = json.dumps(docs[0], indent=2)
else:
    raw = ""
if udid and raw:
    raw = raw.replace(udid, masked)
if raw:
    with open(out_path, "w", encoding="utf-8") as f:
        f.write(raw + "\n")
PY
    rm -rf "$XCRESULT_PATH" "$XCRESULT_PART1_PATH" "$PART1_JSON" "$PART2_JSON"
    rm -f "$ARTIFACT_DIR/tests/xcodebuild-test.log"

    # Extract receiver callback events from app data container (or Part1 snapshot if Simulator resprung)
    RECEIVER_DATA_DIR="$(python3 "$ROOT_DIR/tools/shortcuts-simulator-qa/qa_terminal_receipts.py" receiver-path \
      --repo "$ROOT_DIR" --root "$ARTIFACT_DIR" --source-sha "$SOURCE_SHA" \
      --run-id "$RUN_ID" --fixture-id "$TARGET_FIXTURE_ID" 2>/dev/null || true)"
    if [ -n "$RECEIVER_DATA_DIR" ] && [ -f "$RECEIVER_DATA_DIR/Documents/callback-events.json" ]; then
      cp "$RECEIVER_DATA_DIR/Documents/callback-events.json" "$ARTIFACT_DIR/runtime/callback-events.json"
      GATE_CALLBACK_CAPTURED="PASS"
    elif [ -f "$ARTIFACT_DIR/runtime/callback-events-part1.json" ]; then
      cp "$ARTIFACT_DIR/runtime/callback-events-part1.json" "$ARTIFACT_DIR/runtime/callback-events.json"
      GATE_CALLBACK_CAPTURED="PASS"
    else
      # No native callback file was acquired. Preserve absence, not a fabricated empty event stream.
      GATE_CALLBACK_CAPTURED="FAIL_UI"
    fi
    rm -f "$ARTIFACT_DIR/runtime/callback-events-part1.json"

    # Evaluate import & editor capture gates from editor-action-coverage.json
    if [ -f "$ARTIFACT_DIR/editor/editor-action-coverage.json" ]; then
      LAUNCHED_VAL="$(python3 -c 'import json,sys; d=json.load(open(sys.argv[1])); print("PASS" if d.get("shortcutForegroundVerified") else "FAIL_UI")' "$ARTIFACT_DIR/editor/editor-action-coverage.json")"
      EDITOR_VAL="$(python3 -c 'import json,sys; d=json.load(open(sys.argv[1])); print("PASS" if d.get("editorOpened") else "FAIL_UI")' "$ARTIFACT_DIR/editor/editor-action-coverage.json")"
      LIB_EXISTS_VAL="$(python3 -c 'import json,sys; d=json.load(open(sys.argv[1])); print("true" if d.get("shortcutExistsInLibrary") else "false")' "$ARTIFACT_DIR/editor/editor-action-coverage.json")"
      CAPTURE_STATUS="$(python3 -c 'import json,sys; d=json.load(open(sys.argv[1])); print(d.get("captureStatus","PARTIAL_EDITOR_CAPTURE"))' "$ARTIFACT_DIR/editor/editor-action-coverage.json")"

      GATE_SHORTCUTS_LAUNCHED="$LAUNCHED_VAL"
      GATE_EDITOR_OPENED="$EDITOR_VAL"

      if [ "$SIGNED_ARTIFACT_VALID_AEA1" = "true" ] && [ "$LIB_EXISTS_VAL" = "true" ]; then
        GATE_COMPILER_ARTIFACT_IMPORTED="PASS"
        GATE_SHORTCUT_EXISTS_IN_LIBRARY="PASS"
      fi

      # Enforce strict prerequisite: EDITOR_CAPTURE_COMPLETE requires SHORTCUT_EXISTS_IN_LIBRARY == PASS
      if [ "$CAPTURE_STATUS" = "EDITOR_CAPTURE_COMPLETE" ] && [ "$GATE_SHORTCUT_EXISTS_IN_LIBRARY" = "PASS" ]; then
        GATE_ACTION_EDITOR_CAPTURED="EDITOR_CAPTURE_COMPLETE"
      elif [ "$GATE_EDITOR_OPENED" = "PASS" ]; then
        GATE_ACTION_EDITOR_CAPTURED="PARTIAL_EDITOR_CAPTURE"
      else
        GATE_ACTION_EDITOR_CAPTURED="FAIL_UI"
      fi
    fi

    # AX observations and native plist are independent inputs. Missing identity,
    # unknown values or ambiguous overlap remain NEEDS_REVIEW, never false PASS.
    if [ -n "${UNSIGNED_FILE:-}" ] && [ -s "${UNSIGNED_FILE:-}" ]; then
      python3 tools/shortcuts-simulator-qa/editor_inspector.py \
        --plist "$UNSIGNED_FILE" \
        --tree "$ARTIFACT_DIR/editor/editor-tree.json" \
        --coverage "$ARTIFACT_DIR/editor/editor-action-coverage.json" \
        --fixture "$EXPECTED_SHORTCUT_NAME" --run-id "$EXEC_RUN_NONCE" \
        --source-sha "$SOURCE_SHA" --out-dir "$ARTIFACT_DIR/editor"
      INSPECTOR_RC=$?
      # Preserve the historical artifact name as an alias to the stricter report.
      if [ -f "$ARTIFACT_DIR/editor/editor-comparison.json" ]; then
        cp "$ARTIFACT_DIR/editor/editor-comparison.json" "$ARTIFACT_DIR/editor/plist-vs-editor-comparison.json"
      fi
      if [ "$INSPECTOR_RC" -ne 0 ]; then
        echo "Editor Inspector: NEEDS_REVIEW (see editor-comparison.json; capture alone is not one-to-one verification)"
      fi
    fi

    # Phase 4: Correlated Runtime Execution of Imported Shortcut via x-callback-url
    PHASE4_SIM_LOG="$ARTIFACT_DIR/runtime/phase4-sim-log.txt"
    rm -f "$PHASE4_SIM_LOG"
    if [ "$GATE_SHORTCUT_EXISTS_IN_LIBRARY" = "PASS" ]; then
      GATE_SHORTCUT_RUN_ATTEMPTED="PASS"
      FOUND_NONCE="no"
      if [ -n "$RECEIVER_DATA_DIR" ] && [ -f "$RECEIVER_DATA_DIR/Documents/callback-events.json" ]; then
        cp "$RECEIVER_DATA_DIR/Documents/callback-events.json" "$ARTIFACT_DIR/runtime/callback-events.json"
        FOUND_NONCE="$(python3 -c 'import json,sys; evs=json.load(open(sys.argv[1])); print("yes" if any((not e.get("receiverSelfTest")) and e.get("runId")==sys.argv[2] for e in evs) else "no")' "$ARTIFACT_DIR/runtime/callback-events.json" "$EXEC_RUN_NONCE" 2>/dev/null || echo "no")"
      fi

      if [ "$FOUND_NONCE" != "yes" ]; then
        if ! ensure_phase_session_ready post-test-runtime; then
          echo "Post-XCTest selected-device session boundary was not verified."
          exit 70
        fi
        echo "=== Phase 4b: Post-XCTest ReceiverApp x-callback-url dispatch ($TARGET_FIXTURE_ID) ==="
        # Outside UITests.xctrunner (automationmode=0), launch ReceiverApp so UIApplication.shared.open passes sourceApplication appIdentifier
        xcrun simctl spawn "$UDID" log stream --predicate '(process == "Shortcuts" || process == "BackgroundShortcutRunner" || process == "CherriSimulatorQAReceiver" || subsystem == "com.apple.shortcuts" || subsystem == "com.apple.WorkflowKit" || (process == "lsd" && (eventMessage CONTAINS[c] "cherri" || eventMessage CONTAINS[c] "org.cherrilang"))) AND NOT (subsystem == "com.apple.dt.xctest") AND NOT (subsystem == "com.apple.xpc.transaction") AND NOT (subsystem == "com.apple.Accessibility") AND NOT (subsystem == "com.apple.defaults") AND NOT (category IN {"ToolKitIndexing", "ToolKitDatabase", "ToolKitScheduler", "BackgroundTaskScheduler", "ActionRegistry", "Daemon"})' --level debug --style compact > "$PHASE4_SIM_LOG" 2>&1 &
        PHASE4_LOG_PID=$!
        sleep 1
        TRIGGER_WRAPPER_URL="$(python3 - "$RUN_URL" <<'PY'
import sys, urllib.parse
print("cherriqa://trigger?target=" + urllib.parse.quote(sys.argv[1], safe=""))
PY
)"
        # Install Xcode-signed ReceiverApp FIRST, let LaunchServices index cherriqa://, then restart warm Shortcuts so Shortcuts gets fresh LSDatabase bindings
        run_cmd_timeout 15 xcrun simctl terminate "$UDID" org.cherrilang.qa.Receiver >/dev/null 2>&1 || true
        sleep 1
        RECEIVER_SIGNED_DD="$QA_SCRATCH_DIR/ReceiverSignedDD"
        rm -rf "$RECEIVER_SIGNED_DD"
        echo "=== ReceiverApp Xcode Simulator build + install ==="
        XCODE_RECV_RC=0
        run_cmd_timeout 90 xcodebuild build \
          -project "$ROOT_DIR/tools/shortcuts-simulator-qa/XcodeProject/CherriSimulatorQA.xcodeproj" \
          -scheme CherriSimulatorQAReceiver \
          -sdk iphonesimulator \
          -derivedDataPath "$RECEIVER_SIGNED_DD" \
          -destination "platform=iOS Simulator,id=$UDID" \
          CODE_SIGNING_ALLOWED=YES \
          CODE_SIGNING_REQUIRED=NO \
          CODE_SIGN_IDENTITY="-" \
          > "$ARTIFACT_DIR/builds/xcodebuild-receiver-signed.log" 2>&1 || XCODE_RECV_RC=$?
        echo "ReceiverApp signed xcodebuild rc=$XCODE_RECV_RC"
        RECEIVER_BUNDLE_PATH="$(find "$RECEIVER_SIGNED_DD/Build/Products" -maxdepth 3 -name "CherriSimulatorQAReceiver.app" -type d 2>/dev/null | head -n 1 || true)"
        if [ "$XCODE_RECV_RC" -eq 0 ] && [ -n "$RECEIVER_BUNDLE_PATH" ] && [ -d "$RECEIVER_BUNDLE_PATH" ]; then
          INST_OUT="$(xcrun simctl install "$UDID" "$RECEIVER_BUNDLE_PATH" 2>&1 || true)"
          [ -n "$INST_OUT" ] && printf '%s\n' "$INST_OUT" | sed "s/$UDID/<UDID>/g; s|/Users/[^/ ]*|/Users/<redacted>|g" | head -n 5 || true
        else
          echo "ReceiverApp Xcode signed build fallback -> plain ad-hoc codesign"
          [ -f "$ARTIFACT_DIR/builds/xcodebuild-receiver-signed.log" ] && tail -n 15 "$ARTIFACT_DIR/builds/xcodebuild-receiver-signed.log" | sed "s/$UDID/<UDID>/g; s|/Users/[^/ ]*|/Users/<redacted>|g" || true
          RECEIVER_BUILT_APP="$(find "$DERIVED_DATA_PATH/Build/Products" -maxdepth 3 -name "CherriSimulatorQAReceiver.app" -type d 2>/dev/null | head -n 1 || true)"
          RECEIVER_BUNDLE_PATH="${RECEIVER_BUILT_APP:-}"
          if [ -z "$RECEIVER_BUNDLE_PATH" ] || [ ! -d "$RECEIVER_BUNDLE_PATH" ]; then
            RECEIVER_BUNDLE_PATH="$(xcrun simctl get_app_container "$UDID" org.cherrilang.qa.Receiver app 2>/dev/null || true)"
          fi
          if [ -n "$RECEIVER_BUNDLE_PATH" ] && [ -d "$RECEIVER_BUNDLE_PATH" ]; then
            SIGNED_RECEIVER_TMP="$QA_SCRATCH_DIR/CherriSimulatorQAReceiver-signed.app"
            rm -rf "$SIGNED_RECEIVER_TMP"
            cp -R "$RECEIVER_BUNDLE_PATH" "$SIGNED_RECEIVER_TMP"
            CS_OUT="$(codesign --force --sign - --identifier org.cherrilang.qa.Receiver "$SIGNED_RECEIVER_TMP" 2>&1 || true)"
            INST_OUT="$(xcrun simctl install "$UDID" "$SIGNED_RECEIVER_TMP" 2>&1 || true)"
            rm -rf "$SIGNED_RECEIVER_TMP"
            [ -n "$CS_OUT" ] && printf '%s\n' "$CS_OUT" | sed "s/$UDID/<UDID>/g; s|/Users/[^/ ]*|/Users/<redacted>|g" | head -n 5 || true
            [ -n "$INST_OUT" ] && printf '%s\n' "$INST_OUT" | sed "s/$UDID/<UDID>/g; s|/Users/[^/ ]*|/Users/<redacted>|g" | head -n 5 || true
          fi
        fi
        sleep 5
        INSTALLED_RECEIVER_PATH="$(xcrun simctl get_app_container "$UDID" org.cherrilang.qa.Receiver app 2>/dev/null || true)"
        NEW_DATA_DIR="$(xcrun simctl get_app_container "$UDID" org.cherrilang.qa.Receiver data 2>/dev/null || true)"
        [ -n "$NEW_DATA_DIR" ] && RECEIVER_DATA_DIR="$NEW_DATA_DIR"
        if [ -n "$INSTALLED_RECEIVER_PATH" ] && [ -d "$INSTALLED_RECEIVER_PATH" ]; then
          codesign -dvv "$INSTALLED_RECEIVER_PATH" 2>&1 | sed "s/$UDID/<UDID>/g; s|/Users/[^/ ]*|/Users/<redacted>|g" | head -n 12 || true
          codesign -d --entitlements :- "$INSTALLED_RECEIVER_PATH" 2>&1 | sed "s/$UDID/<UDID>/g; s|/Users/[^/ ]*|/Users/<redacted>|g" | head -n 20 || true
          echo "=== ReceiverApp installed Info.plist ==="
          plutil -p "$INSTALLED_RECEIVER_PATH/Info.plist" 2>&1 | sed "s/$UDID/<UDID>/g; s|/Users/[^/ ]*|/Users/<redacted>|g" | head -n 45 || true
        fi
        echo "=== ReceiverApp simctl appinfo ==="
        xcrun simctl appinfo "$UDID" org.cherrilang.qa.Receiver 2>&1 | sed "s/$UDID/<UDID>/g; s|/Users/[^/ ]*|/Users/<redacted>|g" | head -n 35 || true
        # Restart Shortcuts cleanly AFTER ReceiverApp install so Shortcuts loads updated LaunchServices bindings for cherriqa:// while remaining warm before openURL
        run_cmd_timeout 15 xcrun simctl terminate "$UDID" com.apple.shortcuts >/dev/null 2>&1 || true
        sleep 2
        run_cmd_timeout 15 xcrun simctl launch "$UDID" com.apple.shortcuts >/dev/null 2>&1 || true
        sleep 4
        LAUNCH_RC=0
        LAUNCH_OUT="$( ( export SIMCTL_CHILD_CHERRI_TRIGGER_SHORTCUT_URL="$RUN_URL"; run_cmd_timeout 25 xcrun simctl launch "$UDID" org.cherrilang.qa.Receiver ) 2>&1 )" || LAUNCH_RC=$?
        echo "Phase4b simctl launch rc=$LAUNCH_RC out=$(printf '%s' "$LAUNCH_OUT" | sed "s/$UDID/<UDID>/g" | tr '\n' ' ' | cut -c1-200)"
        FALLBACK_TRIGGER_SENT="no"
        for poll_idx in $(seq 1 16); do
          if [ -n "$RECEIVER_DATA_DIR" ] && [ -f "$RECEIVER_DATA_DIR/Documents/callback-events.json" ]; then
            cp "$RECEIVER_DATA_DIR/Documents/callback-events.json" "$ARTIFACT_DIR/runtime/callback-events.json"
            FOUND_NONCE="$(python3 -c 'import json,sys; evs=json.load(open(sys.argv[1])); print("yes" if any((not e.get("receiverSelfTest")) and e.get("runId")==sys.argv[2] for e in evs) else "no")' "$ARTIFACT_DIR/runtime/callback-events.json" "$EXEC_RUN_NONCE" 2>/dev/null || echo "no")"
            if [ "$FOUND_NONCE" = "yes" ]; then
              run_cmd_timeout 15 xcrun simctl io "$UDID" screenshot "$ARTIFACT_DIR/editor/runtime-callback.png" >/dev/null 2>&1 || true
              break
            fi
          fi
          if [ "$poll_idx" -eq 5 ] && [ "$FALLBACK_TRIGGER_SENT" = "no" ]; then
            OPEN_OK="no"
            if [ -n "$RECEIVER_DATA_DIR" ] && [ -f "$RECEIVER_DATA_DIR/Documents/trigger-debug.txt" ] && grep -q "openCompletion=true" "$RECEIVER_DATA_DIR/Documents/trigger-debug.txt"; then
              OPEN_OK="yes"
            fi
            if [ "$OPEN_OK" != "yes" ]; then
              FALLBACK_TRIGGER_SENT="yes"
              TRIG_RC=0
              TRIG_OUT="$(run_cmd_timeout 15 xcrun simctl openurl "$UDID" "$TRIGGER_WRAPPER_URL" 2>&1)" || TRIG_RC=$?
              echo "Phase4b fallback cherriqa://trigger openurl rc=$TRIG_RC out=$(printf '%s' "$TRIG_OUT" | sed "s/$UDID/<UDID>/g" | tr '\n' ' ' | cut -c1-200)"
            fi
          fi
          sleep 1
        done

        if [ "$FOUND_NONCE" != "yes" ]; then
          run_cmd_timeout 15 xcrun simctl io "$UDID" screenshot "$ARTIFACT_DIR/editor/phase4-after-receiver-openurl.png" >/dev/null 2>&1 || true
          echo "=== Phase 4b-confirm: Checking for Shortcuts open/allow consent dialog via test-without-building ==="
          CONFIRM_XCRESULT="$QA_SCRATCH_DIR/CherriSimulatorQAConfirm.xcresult"
          rm -rf "$CONFIRM_XCRESULT"
          echo "1" > "$ARTIFACT_DIR/editor/confirm-dialog-mode.txt"
          TEST_RUNNER_CHERRI_QA_DIALOG_CONFIRM_ONLY=1 run_cmd_timeout 120 xcodebuild test-without-building \
            -project "$ROOT_DIR/tools/shortcuts-simulator-qa/XcodeProject/CherriSimulatorQA.xcodeproj" \
            -scheme CherriSimulatorQAReceiver \
            -derivedDataPath "$DERIVED_DATA_PATH" \
            -destination "platform=iOS Simulator,id=$UDID" \
            -only-testing:CherriSimulatorQAUITests/CherriSimulatorQAUITests/test04ConfirmPendingRuntimeDialog \
            -resultBundlePath "$CONFIRM_XCRESULT" \
            CODE_SIGNING_ALLOWED=NO \
            > "$ARTIFACT_DIR/builds/xcodebuild-confirm.log" 2>&1 || true
          if [ -f "$ARTIFACT_DIR/builds/xcodebuild-confirm.log" ]; then
            sed "s/$UDID/<UDID>/g; s|/Users/[^/ ]*|/Users/<redacted>|g" "$ARTIFACT_DIR/builds/xcodebuild-confirm.log" | tail -n 60 || true
          fi
          rm -rf "$CONFIRM_XCRESULT" "$ARTIFACT_DIR/editor/confirm-dialog-mode.txt"
          for poll_idx in $(seq 1 10); do
            if [ -n "$RECEIVER_DATA_DIR" ] && [ -f "$RECEIVER_DATA_DIR/Documents/callback-events.json" ]; then
              cp "$RECEIVER_DATA_DIR/Documents/callback-events.json" "$ARTIFACT_DIR/runtime/callback-events.json"
              FOUND_NONCE="$(python3 -c 'import json,sys; evs=json.load(open(sys.argv[1])); print("yes" if any((not e.get("receiverSelfTest")) and e.get("runId")==sys.argv[2] for e in evs) else "no")' "$ARTIFACT_DIR/runtime/callback-events.json" "$EXEC_RUN_NONCE" 2>/dev/null || echo "no")"
              if [ "$FOUND_NONCE" = "yes" ]; then
                if [ ! -f "$ARTIFACT_DIR/editor/runtime-callback.png" ]; then
                  run_cmd_timeout 15 xcrun simctl io "$UDID" screenshot "$ARTIFACT_DIR/editor/runtime-callback.png" >/dev/null 2>&1 || true
                fi
                break
              fi
            fi
            sleep 1
          done
        fi

        if [ "$FOUND_NONCE" != "yes" ] && [ -n "${RUN_URL_MINIMAL:-}" ]; then
          echo "=== Phase 4b-minimal: Trying minimal x-callback-url without input parameters ==="
          run_cmd_timeout 15 xcrun simctl terminate "$UDID" org.cherrilang.qa.Receiver >/dev/null 2>&1 || true
          run_cmd_timeout 15 xcrun simctl launch "$UDID" com.apple.shortcuts >/dev/null 2>&1 || true
          sleep 2
          MIN_RC=0
          MIN_OUT="$( ( export SIMCTL_CHILD_CHERRI_TRIGGER_SHORTCUT_URL="$RUN_URL_MINIMAL"; run_cmd_timeout 25 xcrun simctl launch "$UDID" org.cherrilang.qa.Receiver ) 2>&1 )" || MIN_RC=$?
          echo "Phase4b-minimal launch rc=$MIN_RC out=$(printf '%s' "$MIN_OUT" | sed "s/$UDID/<UDID>/g" | tr '\n' ' ' | cut -c1-200)"
          for poll_idx in $(seq 1 12); do
            if [ -n "$RECEIVER_DATA_DIR" ] && [ -f "$RECEIVER_DATA_DIR/Documents/callback-events.json" ]; then
              cp "$RECEIVER_DATA_DIR/Documents/callback-events.json" "$ARTIFACT_DIR/runtime/callback-events.json"
              FOUND_NONCE="$(python3 -c 'import json,sys; evs=json.load(open(sys.argv[1])); print("yes" if any((not e.get("receiverSelfTest")) and e.get("runId")==sys.argv[2] for e in evs) else "no")' "$ARTIFACT_DIR/runtime/callback-events.json" "$EXEC_RUN_NONCE" 2>/dev/null || echo "no")"
              if [ "$FOUND_NONCE" = "yes" ]; then
                run_cmd_timeout 15 xcrun simctl io "$UDID" screenshot "$ARTIFACT_DIR/editor/runtime-callback.png" >/dev/null 2>&1 || true
                break
              fi
            fi
            sleep 1
          done
        fi

        if [ -n "${PHASE4_LOG_PID:-}" ]; then
          stop_bg_pid "$PHASE4_LOG_PID"
          PHASE4_LOG_PID=""
        fi
        if [ -n "$RECEIVER_DATA_DIR" ] && [ -f "$RECEIVER_DATA_DIR/Documents/trigger-debug.txt" ]; then
          sed "s/$UDID/<UDID>/g" "$RECEIVER_DATA_DIR/Documents/trigger-debug.txt" > "$ARTIFACT_DIR/runtime/receiver-trigger-debug.txt" || true
          echo "=== ReceiverApp trigger-debug.txt ==="
          cat "$ARTIFACT_DIR/runtime/receiver-trigger-debug.txt" || true
        fi
        echo "=== Phase 4b finished (FOUND_NONCE=$FOUND_NONCE) ==="
      fi

      # Evaluate callback & NDJSON trace using strict oracle rules (never hardcode BLOCKED_IMPORT!)
      evaluate_runtime_callback() {
      RUNTIME_EVAL="$(python3 - "$ARTIFACT_DIR/runtime/callback-events.json" "$EXEC_RUN_NONCE" "$TARGET_FIXTURE_ID" "$EXPECTED_SEMANTIC_OUTPUT" "$ARTIFACT_DIR/runtime/runtime-trace.jsonl" "$ARTIFACT_DIR/compiler-manifest.json" "$SOURCE_SHA" "$ROOT_DIR" <<'PY'
import json, re, sys

cb_path, expected_nonce, expected_trace_run_id, expected_out, trace_out_path, manifest_path, expected_sha = sys.argv[1:8]
if expected_trace_run_id in ("qa-nested-list-control-v2", "qa-nested-list-candidate-v2"):
    from pathlib import Path
    assert len(sys.argv) == 9
    sys.path.insert(0, str(Path(sys.argv[8]) / "tools/shortcuts-simulator-qa"))
    from nested_list_runtime import emit
    emit(sys.argv[8], *sys.argv[1:8])
    sys.exit(0)
profile = "UNKNOWN"
trace_expectation = "REQUIRED_NDJSON"
release_build_verified = False
source_sha256 = ""
unsigned_sha256 = ""

def result(run_gate, trace_gate, reason):
    print(json.dumps({
        "runGate": run_gate, "traceGate": trace_gate, "reason": reason,
        "targetFixtureId": expected_trace_run_id, "compilerCommitSha": expected_sha,
        "buildProfile": profile, "traceExpectation": trace_expectation,
        "releaseBuildVerified": release_build_verified,
        "sourceSha256": source_sha256, "unsignedPlistSha256": unsigned_sha256,
    }))

try:
    manifest = json.load(open(manifest_path, encoding="utf-8"))
    assert re.fullmatch(r"[0-9a-f]{40}", expected_sha) and expected_nonce
    assert manifest.get("syntheticOnly") is True and manifest.get("compilerCommitSha") == expected_sha
    fixtures = manifest.get("fixtures")
    assert isinstance(fixtures, list) and all(isinstance(f, dict) for f in fixtures)
    entries = [f for f in fixtures if f.get("fixtureId") == expected_trace_run_id]
    assert len(entries) == 1
    fixture = entries[0]
    source_sha256 = fixture.get("sourceSha256", "")
    unsigned_sha256 = fixture.get("unsignedPlistSha256", "")
    assert isinstance(source_sha256, str) and re.fullmatch(r"[0-9a-f]{64}", source_sha256)
    assert isinstance(unsigned_sha256, str) and re.fullmatch(r"[0-9a-f]{64}", unsigned_sha256)
    profile = fixture.get("buildProfile")
    trace_expectation = fixture.get("runtimeTraceExpectation")
    if expected_trace_run_id == "qa-measure-release-v2":
        assert expected_out == "CHERRI_QA_MEASURE_RELEASE_RESULT:P15_MEASURE_RELEASE_OK"
        assert profile == "release" and trace_expectation == "ABSENT_RELEASE"
        assert fixture.get("releaseDiagnosticActionsAbsent") is True
        assert fixture.get("releaseBodyShapeVerified") is True
        release_build_verified = True
    else:
        assert profile == "debug" and trace_expectation == "REQUIRED_NDJSON"
    if expected_trace_run_id == "qa-optional-forwarding-diagnostic-v2":
        assert source_sha256 == "e6f195aadcd2d46f25c6fe1de5165798c18a40f4c1942785b9e39171b1cc2139"
        reviewed_source = {
            "status": "PASS_REVIEWED_PUBLIC_SOURCE_ONLY", "sourceSha256": source_sha256,
            "expectedContractSha256": "da1335a22389cbe97e72a706192cb6b93ece8eff380080fa0541dcb5cbc50cef",
            "observedFields": 11, "appleRuntime": "NOT_RUN", "formalAcceptanceGranted": False,
        }
        observed_source = fixture.get("optionalForwardingSource")
        assert isinstance(observed_source, dict) and set(observed_source) == set(reviewed_source)
        assert all(type(observed_source[key]) is type(value) and observed_source[key] == value
                   for key, value in reviewed_source.items())
        reviewed_graph = {
            "schemaVersion": "1.0", "fixtureId": expected_trace_run_id,
            "status": "PASS_NATIVE_STRUCTURE_ONLY", "sourceSha256": source_sha256,
            "expectedContractSha256": reviewed_source["expectedContractSha256"],
            "unsignedPlistSha256": unsigned_sha256, "nativeActionCount": 402,
            "appleRuntime": "NOT_RUN", "formalAcceptanceGranted": False,
        }
        observed_graph = fixture.get("optionalForwardingGraph")
        assert isinstance(observed_graph, dict) and set(observed_graph) == set(reviewed_graph) | {"testStdoutSha256", "testStderrSha256"}
        assert all(type(observed_graph[key]) is type(value) and observed_graph[key] == value
                   for key, value in reviewed_graph.items())
        assert all(isinstance(observed_graph[key], str) and re.fullmatch(r"[0-9a-f]{64}", observed_graph[key])
                   for key in ("testStdoutSha256", "testStderrSha256"))
except (OSError, ValueError, TypeError, AttributeError, AssertionError) as exc:
    result("FAIL_RUNTIME", "FAIL_RUNTIME", f"compiler manifest/runtime profile contract rejected: {type(exc).__name__}")
    sys.exit(0)
try:
    events = json.load(open(cb_path, encoding="utf-8"))
except Exception as exc:
    result("FAIL_RUNTIME", "FAIL_RUNTIME", f"failed to read callback-events.json: {exc}")
    sys.exit(0)

if not isinstance(events, list) or not all(isinstance(e, dict) for e in events):
    result("FAIL_RUNTIME", "FAIL_RUNTIME", "callback store is not an array of event objects")
    sys.exit(0)

matched = [e for e in events if not e.get("receiverSelfTest") and e.get("runId") == expected_nonce]
if len(matched) == 0:
    result("FAIL_RUNTIME", "FAIL_RUNTIME", f"no non-self-test callback for runId {expected_nonce}")
    sys.exit(0)
if len(matched) > 1:
    result("FAIL_RUNTIME", "FAIL_RUNTIME", f"duplicate callback replay ({len(matched)}) for runId {expected_nonce}")
    sys.exit(0)

ev = matched[0]
if ev.get("receiverSelfTest") is not False or ev.get("eventSource") != "shortcuts-x-callback" or ev.get("fixtureId") != expected_trace_run_id or ev.get("expectedShortcutName") != expected_trace_run_id:
    result("FAIL_RUNTIME", "FAIL_RUNTIME", "callback source or fixture identity mismatch")
    sys.exit(0)
if ev.get("status") != "SUCCESS":
    result("FAIL_RUNTIME", "FAIL_RUNTIME", f"callback status={ev.get('status')} error={ev.get('errorMessage')}")
    sys.exit(0)

summary_text = ev.get("resultSummary")
trace_text = ev.get("tracePayload")
# Codable omits nil optional tracePayload when no trace was returned. False,
# zero, lists and dictionaries are not accepted as an absent trace.
if trace_text is None:
    trace_text = ""
if not isinstance(summary_text, str) or not isinstance(trace_text, str):
    result("FAIL_RUNTIME", "FAIL_RUNTIME", "callback result/trace must be text")
    sys.exit(0)
if expected_out and summary_text.strip() != expected_out:
    result("FAIL_RUNTIME", "FAIL_RUNTIME", "callback result does not exactly match expected output")
    sys.exit(0)

if release_build_verified:
    if trace_text != "":
        result("FAIL_RUNTIME", "FAIL_RUNTIME", "unexpected trace in the explicit Release fixture")
    else:
        # An empty file is absence evidence, never a fabricated collector event.
        open(trace_out_path, "w", encoding="utf-8").close()
        result("PASS", "NOT_APPLICABLE_RELEASE", "verified Release build, exact fresh body/result callback and absent trace")
    sys.exit(0)

# Validate NDJSON trace lines
raw_lines = [l.strip() for l in trace_text.splitlines() if l.strip() and l.strip() != "===CHERRI_QA_TRACE_NDJSON_V14==="]
valid_records = []
has_begin = False
has_end = False
inline_count = 0
last_step_sequence = -1
trace_err = None

for idx, line in enumerate(raw_lines):
    try:
        rec = json.loads(line)
    except (ValueError, TypeError):
        trace_err = f"line {idx+1} invalid JSON"
        break
    if not isinstance(rec, dict) or rec.get("schemaVersion") != "1.4":
        trace_err = f"line {idx+1} invalid trace schema"
        break
    if has_end:
        trace_err = f"line {idx+1} occurs after collector_end"
        break
    kind = rec.get("kind")
    stage = rec.get("stage")
    if kind == "COLLECTOR_EVENT":
        if rec.get("runId") != expected_trace_run_id:
            trace_err = f"collector runId missing or mismatched at line {idx+1}"
            break
        if stage == "collector_begin" and idx == 0 and not has_begin:
            has_begin = True
        elif stage == "collector_end" and has_begin and inline_count > 0:
            has_end = True
        else:
            trace_err = f"invalid collector order/stage at line {idx+1}"
            break
        valid_records.append(rec)
    elif kind == "INLINE_DEBUG_ACTION":
        step = rec.get("stepSequence")
        stamp = rec.get("timestamp")
        aid = rec.get("actionIdentifier")
        value_type = rec.get("valueType")
        if (not has_begin or stage not in ("before", "after", "error", "end")
                or not isinstance(stamp, str) or not stamp.strip()
                or type(step) is not int or step < 0
                or not isinstance(aid, str) or not aid.startswith("is.workflow.actions.")
                or not isinstance(value_type, str) or not value_type):
            trace_err = f"invalid INLINE_DEBUG_ACTION record at line {idx+1}"
            break
        # The fixture stream is linear. Duplicate/regressing records indicate
        # replay or misordered export, not evidence of an additional action.
        if step <= last_step_sequence:
            trace_err = f"duplicate or regressing stepSequence at line {idx+1}"
            break
        last_step_sequence = step
        inline_count += 1
        valid_records.append(rec)
    else:
        trace_err = f"unknown trace record kind at line {idx+1}"
        break

if not trace_err and (not has_begin or not has_end or inline_count == 0):
    trace_err = f"incomplete trace stream (begin={has_begin}, end={has_end}, inlineCount={inline_count})"

if trace_err:
    result("PASS", "FAIL_RUNTIME", trace_err)
else:
    with open(trace_out_path, "w", encoding="utf-8") as tf:
        for r in valid_records:
            tf.write(json.dumps(r, ensure_ascii=False) + "\n")
    result("PASS", "PASS", "verified correlated callback output and NDJSON trace")
PY
)"
      }
      evaluate_runtime_callback
      if [ "$STORAGE_CROSS_RUN_ENABLED" = "true" ]; then
        printf '%s\n' "$RUNTIME_EVAL" > "$ARTIFACT_DIR/runtime/storage-writer-oracle.json"
        if [ -f "$ARTIFACT_DIR/runtime/runtime-trace.jsonl" ]; then
          cp "$ARTIFACT_DIR/runtime/runtime-trace.jsonl" "$ARTIFACT_DIR/runtime/storage-writer-trace.jsonl"
        fi
        if python3 "$ROOT_DIR/tools/shortcuts-simulator-qa/storage_cross_run.py" writer-ready --root "$ARTIFACT_DIR"; then
          # This is a second external invocation, not a nested runShortcut().
          # A fresh nonce and a single dispatch prohibit writer/reader replay.
          EXEC_RUN_NONCE="$(python3 "$ROOT_DIR/tools/shortcuts-simulator-qa/storage_cross_run.py" reader-nonce --root "$ARTIFACT_DIR")" || { echo "ERROR: storage phase preparation rejected before native launch" >&2; exit 2; }
          EXPECTED_SEMANTIC_OUTPUT="CHERRI_QA_STORAGE_CROSS_RUN_RESULT:READ:TYPE_OK:VALUE_OK"
          rm -f "$ARTIFACT_DIR/runtime/runtime-trace.jsonl"
          CHERRI_QA_READER_CONFIRM_PROJECT="$ROOT_DIR/tools/shortcuts-simulator-qa/XcodeProject/CherriSimulatorQA.xcodeproj" \
          CHERRI_QA_READER_CONFIRM_DERIVED_DATA="$DERIVED_DATA_PATH" \
          CHERRI_QA_READER_CONFIRM_RESULT="$QA_SCRATCH_DIR/CherriStorageReaderConfirm.xcresult" \
            python3 "$ROOT_DIR/tools/shortcuts-simulator-qa/storage_cross_run.py" dispatch-reader --root "$ARTIFACT_DIR" --udid "$UDID" --receiver-data "$RECEIVER_DATA_DIR" || true
          evaluate_runtime_callback
          printf '%s\n' "$RUNTIME_EVAL" > "$ARTIFACT_DIR/runtime/storage-reader-oracle.json"
        fi
        # A writer-only PASS never satisfies the two-invocation requirement.
        if ! python3 "$ROOT_DIR/tools/shortcuts-simulator-qa/storage_cross_run.py" report --root "$ARTIFACT_DIR"; then
          RUNTIME_EVAL="$(printf '%s' "$RUNTIME_EVAL" | python3 -c 'import json,sys; d=json.load(sys.stdin); d.update(runGate="FAIL_RUNTIME",traceGate="FAIL_RUNTIME",reason="two separate storage invocations did not both satisfy strict callback/trace/order"); print(json.dumps(d))')"
        fi
      fi
      GATE_SHORTCUT_RUN_COMPLETED="$(echo "$RUNTIME_EVAL" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("runGate","FAIL_RUNTIME"))')"
      GATE_TRACE_CAPTURED="$(echo "$RUNTIME_EVAL" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("traceGate","FAIL_RUNTIME"))')"
      RUNTIME_EVAL_REASON="$(echo "$RUNTIME_EVAL" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("reason",""))')"
    else
      # Do NOT fake runtime execution from an unimported shortcut; propagate exact upstream blocker
      GATE_SHORTCUT_RUN_ATTEMPTED="$GATE_SHORTCUT_EXISTS_IN_LIBRARY"
      GATE_SHORTCUT_RUN_COMPLETED="$GATE_SHORTCUT_EXISTS_IN_LIBRARY"
      GATE_TRACE_CAPTURED="$GATE_SHORTCUT_EXISTS_IN_LIBRARY"
      RUNTIME_EVAL_REASON="Simulator Shortcuts library import blocked ($GATE_SHORTCUT_EXISTS_IN_LIBRARY); runtime execution skipped fail-closed."
    fi

    if [ "${GATE_SHORTCUT_RUN_COMPLETED:-}" != "PASS" ] && { [ -f "${PHASE4_SIM_LOG:-}" ] || [ -f "${RAW_SIM_LOG:-}" ]; }; then
      LOG_SRC="${PHASE4_SIM_LOG:-}"
      if [ ! -s "$LOG_SRC" ]; then
        LOG_SRC="${RAW_SIM_LOG:-}"
      fi
      python3 - "$UDID" "$ROOT_DIR" "$LOG_SRC" "$ARTIFACT_DIR/runtime/shortcuts-log-excerpt.txt" <<'PY' || true
import re, sys
udid, root_dir, in_path, out_path = sys.argv[1:5]
lines = open(in_path, encoding="utf-8", errors="replace").read().splitlines()
forbidden = re.compile(r"(?i)(cherri-shortcut-audit-20261110|\b[0-3][0-9]\.plist\b|BEGIN [A-Z ]*PRIVATE KEY|ghp_[A-Za-z0-9_]+|github_pat_[A-Za-z0-9_]+|sk-[A-Za-z0-9]{16,}|Bearer\s+[A-Za-z0-9._\-]+|AppleID|iCloudToken)")
noise = (
    "ToolKitIndexing", "ToolKitDatabase", "approvedForPublicDrawerToDisplayActionIdentifier",
    "ToolDatabaseWriter", "CFBundle:strings", "CFBundle:resources",
    "com.apple.Accessibility", "com.apple.defaults:User Defaults",
    "com.apple.AppleMediaServices", "com.apple.clouddocs", "com.apple.accounts",
    "com.apple.foundation.filecoordination",
    "com.apple.cloudkit.sim", "BackBoardServices", "FrontBoardServices:Workspace",
    "com.apple.dt.xctest", "com.apple.xpc.transaction", "XC_kAXXCAttribute",
    "com.apple.runningboard:monitor", "com.apple.siri:Client",
    "com.apple.launchservices:datasep", "application record search init",
    "com.apple.appintents:Connection", "com.apple.shortcuts:AppIntentsMetadata",
    "Realizing settings extension", "SceneExtension", "FBSScene", "UIScene",
    "UIKeyboard", "RBSAssertion", "com.apple.runningboard", "CoreAnalytics",
    "container_create_or_lookup_app_group_path",
    "com.apple.UIKit", "com.apple.BackBoard", "com.apple.FrontBoard",
    "com.apple.GenerativeModels", "com.apple.BacklightServices",
    "com.apple.BoardServices", "libMobileGestalt.dylib", "com.apple.xpc:connection",
    "com.apple.xpc:session", "com.apple.KeyboardArbiter", "com.apple.TextInputUI",
    "com.apple.useractivity", "com.apple.containermanager", "com.apple.network:",
    "com.apple.CloudKit", "com.apple.cloudkit", "com.apple.launchservices:datasep",
    "ToolEmbeddingDatabase", "RunningBoardServices", "CoreSpotlight", "SpotlightSync",
    "com.apple.ManagedConfiguration", "com.apple.purplebuddy", "PrototypeTools",
    "com.apple.calls.callkit", "com.apple.UIIntelligenceSupport",
    "com.apple.foundation.filecoordination", "Updating Key-Value Observers",
    "Loading Preferences From User Session CFPrefsD", "CoreAnalytics",
    "com.apple.CoreAnalytics", "com.apple.shortcuts:Files",
    "com.apple.PlugInKit", "com.apple.shortcuts:ActionRegistry"
)
uuid_re = re.compile(r"[0-9A-Fa-f]{8}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{12}")
users_re = re.compile(r"/Users/[^/\s]+")
filtered = []
exec_lines = []
exec_spam = (
    "processParameterStates", "requiresRemoteExecution",
    "dismissPresentedContent", "Not reaching out to UI for dismissal",
    "Adding 0 user activities", "Starting image proxy injection",
    "ImageProxyInjection", "Reset extension request"
)
exec_keywords = (
    "CherriReceiver", "BackgroundShortcutRunner", "WFOutputAction",
    "WorkflowExecution", "RunningLifecycle", "XCallback", "x-callback",
    "taint", "appIdentifier", "destination app", "could not be determined",
    "cherriqa", "VoiceShortcutClient", "WFXCallback", "openURL",
    "OpenURL", "WFLibraryRunCoordinator", "Shortcuts:URL", "WFWorkflow",
    "WFActionError", "contentDestination", "performDataAccessChecks",
    "originator", "org.cherrilang", "Security", "LaunchServices", "ShortcutRunner",
    "Interchange", "bindings", "lsd["
)
for ln in lines:
    if udid:
        ln = ln.replace(udid, "<UDID>")
    if root_dir:
        ln = ln.replace(root_dir, "<REPO_ROOT>")
    ln = users_re.sub("/Users/<redacted>", ln)
    ln = uuid_re.sub("<UUID>", ln)
    if forbidden.search(ln):
        continue
    if "CherriReceiver" not in ln and "org.cherrilang" not in ln and any(n in ln for n in noise):
        continue
    filtered.append(ln[:320])
    if any(k in ln for k in exec_keywords) and "ToolEmbedding" not in ln and not any(sp in ln for sp in exec_spam):
        exec_lines.append(ln[:320])
with open(out_path, "w", encoding="utf-8") as f:
    kept = filtered[:250] + ["--- [truncated middle] ---"] + filtered[-350:] if len(filtered) > 650 else filtered
    f.write("\n".join(kept) + ("\n" if kept else ""))
exec_path = out_path.replace("shortcuts-log-excerpt.txt", "phase4-execution-log.txt")
exec_kept = exec_lines[:250] + ["--- [truncated middle] ---"] + exec_lines[-350:] if len(exec_lines) > 650 else exec_lines
with open(exec_path, "w", encoding="utf-8") as ef:
    ef.write("\n".join(exec_kept) + ("\n" if exec_kept else ""))
print(f"scrubbed Phase4/sim log excerpt ({in_path}): {len(filtered)} lines -> kept {len(kept)}, execLines={len(exec_lines)}")
for ln in exec_kept[:120]:
    print("PHASE4_EXEC:", ln)
preview = filtered[:60] + (["---"] + filtered[-40:] if len(filtered) > 60 else [])
for ln in preview:
    print("PHASE4LOG:", ln)
PY
    fi
    rm -f "${RAW_SIM_LOG:-}" "${PHASE4_SIM_LOG:-}"

    if declare -F run_negative_and_scheme_probes >/dev/null 2>&1; then
      run_negative_and_scheme_probes
      write_import_attempts_json
    fi
  fi
fi

# Keep private build originals for hash-bound receipt validation. The existing
# explicit publisher quarantines them; no build/raw path is uploaded.
rm -rf "$ARTIFACT_DIR/tests/"*.log "$ARTIFACT_DIR/tests/"*.xcresult "${QA_SCRATCH_DIR:-}" "${RAW_SIM_LOG:-}" "${PHASE4_SIM_LOG:-}"

# Bind the actual current invocation before the normal assertion emitter.
# Storage cross-run may have advanced to its reader nonce after test02.
python3 "$ROOT_DIR/tools/shortcuts-simulator-qa/qa_terminal_receipts.py" output-context \
  --repo "$ROOT_DIR" --root "$ARTIFACT_DIR" --source-sha "$SOURCE_SHA" \
  --run-id "$RUN_ID" --fixture-id "$TARGET_FIXTURE_ID" --nonce "${EXEC_RUN_NONCE:-}" || exit 2

# Write output-assertions.json and runtime-trace.jsonl
python3 - "$ARTIFACT_DIR" "$GATE_SHORTCUT_RUN_COMPLETED" "$GATE_CALLBACK_CAPTURED" "$GATE_TRACE_CAPTURED" "${RUNTIME_EVAL_REASON:-}" "$TARGET_FIXTURE_ID" "$EXPECTED_SEMANTIC_OUTPUT" "${EXEC_RUN_NONCE:-}" "${RUNTIME_EVAL:-}" "$ARTIFACT_DIR/compiler-manifest.json" "$SOURCE_SHA" "$ROOT_DIR" <<'PY'
import json
import os
import re
import sys

artifact_dir, run_status, cb_status, trace_status, reason, fid, expected_out, expected_nonce, oracle_json = sys.argv[1:10]
try:
    oracle = json.loads(oracle_json)
    if not isinstance(oracle, dict):
        oracle = {}
except (ValueError, TypeError):
    oracle = {}

# An import failure can prevent the callback oracle from running. Preserve
# independently validated compiler facts without inventing oracle PASS gates.
# A partial/invalid manifest never produces a partially trusted provenance tuple.
if not oracle and len(sys.argv) >= 12:
    manifest_path, expected_sha = sys.argv[10:12]
    try:
        manifest = json.load(open(manifest_path, encoding="utf-8"))
        assert re.fullmatch(r"[0-9a-f]{40}", expected_sha)
        assert manifest.get("syntheticOnly") is True
        assert manifest.get("compilerCommitSha") == expected_sha
        fixtures = manifest.get("fixtures")
        assert isinstance(fixtures, list) and all(isinstance(f, dict) for f in fixtures)
        entries = [f for f in fixtures if f.get("fixtureId") == fid]
        assert len(entries) == 1
        fixture = entries[0]
        source_hash = fixture.get("sourceSha256")
        unsigned_hash = fixture.get("unsignedPlistSha256")
        assert isinstance(source_hash, str) and re.fullmatch(r"[0-9a-f]{64}", source_hash)
        assert isinstance(unsigned_hash, str) and re.fullmatch(r"[0-9a-f]{64}", unsigned_hash)
        profile = fixture.get("buildProfile")
        trace_expectation = fixture.get("runtimeTraceExpectation")
        release_verified = fid == "qa-measure-release-v2"
        if release_verified:
            assert expected_out == "CHERRI_QA_MEASURE_RELEASE_RESULT:P15_MEASURE_RELEASE_OK"
            assert profile == "release" and trace_expectation == "ABSENT_RELEASE"
            assert fixture.get("releaseDiagnosticActionsAbsent") is True
            assert fixture.get("releaseBodyShapeVerified") is True
        else:
            assert profile == "debug" and trace_expectation == "REQUIRED_NDJSON"
        oracle = {"targetFixtureId": fid, "compilerCommitSha": expected_sha,
                  "sourceSha256": source_hash, "unsignedPlistSha256": unsigned_hash,
                  "buildProfile": profile, "traceExpectation": trace_expectation,
                  "releaseBuildVerified": release_verified}
    except (OSError, ValueError, TypeError, AttributeError, AssertionError):
        oracle = {}

cb_path = os.path.join(artifact_dir, "runtime", "callback-events.json")
events = []
if os.path.exists(cb_path):
    try:
        events = json.load(open(cb_path, encoding="utf-8"))
    except Exception:
        events = []

if not isinstance(events, list) or not all(isinstance(e, dict) for e in events):
    events = []
self_cb_ok = any(e.get("receiverSelfTest") is True and e.get("eventSource") == "receiver-self-test" and e.get("status") == "SUCCESS" for e in events)
correlated = [e for e in events if e.get("receiverSelfTest") is False
              and expected_nonce and e.get("runId") == expected_nonce
              and e.get("eventSource") == "shortcuts-x-callback"
              and e.get("fixtureId") == fid and e.get("expectedShortcutName") == fid]
# Reporting must never relax the production oracle's result. An unrelated,
# stale or rejected SUCCESS event is not a verified compiled-Shortcut callback.
shortcut_cb_ok = (run_status == "PASS" and len(correlated) == 1
                  and correlated[0].get("status") == "SUCCESS"
                  and isinstance(correlated[0].get("resultSummary"), str)
                  and correlated[0]["resultSummary"].strip() == expected_out)
release_trace_absent = (len(correlated) == 1
                        and (correlated[0].get("tracePayload") is None
                             or correlated[0].get("tracePayload") == ""))
release_contract = (fid == "qa-measure-release-v2"
                    and expected_out == "CHERRI_QA_MEASURE_RELEASE_RESULT:P15_MEASURE_RELEASE_OK"
                    and oracle.get("targetFixtureId") == fid
                    and oracle.get("buildProfile") == "release"
                    and oracle.get("traceExpectation") == "ABSENT_RELEASE"
                    and oracle.get("releaseBuildVerified") is True
                    and oracle.get("runGate") == "PASS"
                    and oracle.get("traceGate") == "NOT_APPLICABLE_RELEASE"
                    and trace_status == "NOT_APPLICABLE_RELEASE"
                    and release_trace_absent)
normal_contract = (fid != "qa-measure-release-v2" and trace_status == "PASS"
                   and oracle.get("targetFixtureId") == fid
                   and oracle.get("buildProfile") == "debug"
                   and oracle.get("traceExpectation") == "REQUIRED_NDJSON"
                   and oracle.get("runGate") == "PASS" and oracle.get("traceGate") == "PASS")
runtime_contract_verified = shortcut_cb_ok and (release_contract or normal_contract)
if fid == "qa-measure-release-v2" and not release_contract:
    shortcut_cb_ok = False
observed_status = correlated[0].get("status") if len(correlated) == 1 else None
if observed_status not in ("SUCCESS", "ERROR", "CANCELLED"):
    observed_status = "UNKNOWN"

assertions = {
    "schemaVersion": "1.5",
    "targetFixtureId": fid,
    "compilerCommitSha": oracle.get("compilerCommitSha", "UNKNOWN"),
    "compiledSourceSha256": oracle.get("sourceSha256", ""),
    "unsignedPlistSha256": oracle.get("unsignedPlistSha256", ""),
    "buildProfile": oracle.get("buildProfile", "UNKNOWN"),
    "traceExpectation": oracle.get("traceExpectation", "REQUIRED_NDJSON"),
    "releaseBuildVerified": oracle.get("releaseBuildVerified") is True,
    "runtimeContractVerified": runtime_contract_verified,
    "traceObserved": "ABSENT" if release_contract else ("NDJSON_VERIFIED" if normal_contract else "UNVERIFIED"),
    "expectedRunId": expected_nonce,
    "expectedSemanticOutput": expected_out,
    "shortcutRunCompletedGate": run_status,
    "callbackCapturedGate": cb_status,
    "traceCapturedGate": trace_status,
    "receiverSelfCallbackVerified": self_cb_ok,
    "compiledShortcutCallbackVerified": shortcut_cb_ok,
    "compiledShortcutRuntimeExecuted": shortcut_cb_ok,
    "observedCorrelatedCallbackStatus": observed_status,
    "typedValueCoverage": "UNKNOWN_AT_TRACE_EVENT",
    "perActionCoverageVerified": False,
    "diagnosticReason": reason,
    "notes": "Receiver self-test and compiled Shortcut callbacks are separate. Verified runtime requires the strict oracle plus exact source, fixture, fresh nonce and output. ERROR/CANCELLED are observed outcomes, never substituted for SUCCESS; their causes remain unverified. Inline trace events do not establish typed per-action coverage."
}
if fid in ("qa-nested-list-control-v2", "qa-nested-list-candidate-v2"):
    from pathlib import Path
    assert len(sys.argv) == 13
    sys.path.insert(0, str(Path(sys.argv[12]) / "tools/shortcuts-simulator-qa"))
    from nested_list_runtime import apply_assertions
    apply_assertions(assertions, oracle, sys.argv[12], artifact_dir, sys.argv[11], expected_out)
with open(os.path.join(artifact_dir, "runtime", "output-assertions.json"), "w", encoding="utf-8") as f:
    json.dump(assertions, f, indent=2)

trace_jsonl_path = os.path.join(artifact_dir, "runtime", "runtime-trace.jsonl")
# A missing storage reader trace is evidence bound by the earlier phase report.
# Preserve that absence after a timeout; a collector fallback would change its
# hash without observing a native reader callback and block truthful publication.
if not os.path.exists(trace_jsonl_path) and fid not in ("qa-storage-text-cross-run-v2", "qa-nested-list-control-v2", "qa-nested-list-candidate-v2"):
    with open(trace_jsonl_path, "w", encoding="utf-8") as f:
        if fid == "qa-measure-release-v2":
            # No collector is executed by the Release fixture.
            sys.exit(0)
        f.write(json.dumps({
            "schemaVersion": "1.4",
            "kind": "COLLECTOR_EVENT",
            "stage": "collector_end",
            "runId": fid,
            "status": trace_status,
            "omittedReason": reason if trace_status != "PASS" else None,
        }) + "\n")
PY

# A successful exact callback does not substitute for requested visible labels.
# This fresh-run receipt is additive; original values, trace and callback remain required.
case "$TARGET_FIXTURE_ID" in
  qa-debug-label-*)
    python3 "$ROOT_DIR/tools/shortcuts-simulator-qa/debug_label_preview.py" finalize \
      --root "$ARTIFACT_DIR" --fixture "$TARGET_FIXTURE_ID" --commit "$SOURCE_SHA" \
      --run-id "$RUN_ID" --attempt "${GITHUB_RUN_ATTEMPT:-1}" || exit 2
    ;;
esac

if [ "$HTTP_PROBE_STARTED" = "true" ]; then
  stop_http_probe
  http_probe diagnose --root "$ARTIFACT_DIR" --fixtures-dir "$ROOT_DIR/tools/shortcuts-simulator-qa/fixtures" --fixture "$TARGET_FIXTURE_ID" --nonce "$EXEC_RUN_NONCE" --commit "$SOURCE_SHA"
  HTTP_PROBE_DIAGNOSTIC_RC=$?
fi

# Legacy pattern scan only; raster safety is NOT established by this scan.
# The separate publication gate quarantines all raster/archives/raw inputs.
PRIVACY_SCAN_OUT="$(python3 - "$ARTIFACT_DIR" "$UDID" <<'PY'
import json, os, sys
sys.path.insert(0, os.path.join(os.getcwd(), "tools", "shortcuts-simulator-qa"))
from prepare_public_artifacts import scan_public_privacy
print(json.dumps(scan_public_privacy(sys.argv[1], sys.argv[2],
                                    os.environ.get("CHERRI_SIGNING_SERVER", "").strip())))
PY
)"

# Write summary.json and docs/report.md with strict prerequisite invariant verification
python3 - "$ARTIFACT_DIR" "$RUN_ID" "$SOURCE_SHA" "$REQUIRE_RUNTIME" \
  "$GATE_RUNNER_AVAILABLE" \
  "$GATE_SIMULATOR_BOOTED" \
  "$GATE_SHORTCUTS_APP_INSTALLED" \
  "$GATE_SHORTCUTS_LAUNCHED" \
  "$GATE_EDITOR_OPENED" \
  "$GATE_FILE_IMPORT_ATTEMPTED" \
  "$GATE_COMPILER_ARTIFACT_IMPORTED" \
  "$GATE_SHORTCUT_EXISTS_IN_LIBRARY" \
  "$GATE_ACTION_EDITOR_CAPTURED" \
  "$GATE_SHORTCUT_RUN_ATTEMPTED" \
  "$GATE_SHORTCUT_RUN_COMPLETED" \
  "$GATE_CALLBACK_CAPTURED" \
  "$GATE_TRACE_CAPTURED" \
  "$GATE_PHYSICAL_DEVICE_TESTED" "$UDID" <<'PY'
import json
import os
import re
import sys

artifact_dir, run_id, sha, req_rt = sys.argv[1:5]
raw_udid = sys.argv[19] if len(sys.argv) > 19 else ""
gates = {
    "RUNNER_AVAILABLE": sys.argv[5],
    "SIMULATOR_BOOTED": sys.argv[6],
    "SHORTCUTS_APP_INSTALLED": sys.argv[7],
    "SHORTCUTS_LAUNCHED": sys.argv[8],
    "EDITOR_OPENED": sys.argv[9],
    "FILE_IMPORT_ATTEMPTED": sys.argv[10],
    "COMPILER_ARTIFACT_IMPORTED": sys.argv[11],
    "SHORTCUT_EXISTS_IN_LIBRARY": sys.argv[12],
    "ACTION_EDITOR_CAPTURED": sys.argv[13],
    "SHORTCUT_RUN_ATTEMPTED": sys.argv[14],
    "SHORTCUT_RUN_COMPLETED": sys.argv[15],
    "CALLBACK_CAPTURED": sys.argv[16],
    "TRACE_CAPTURED": sys.argv[17],
    "PHYSICAL_DEVICE_TESTED": sys.argv[18],
}

# Enforce prerequisite invariants fail-closed
if gates["COMPILER_ARTIFACT_IMPORTED"] != "PASS":
    gates["SHORTCUT_EXISTS_IN_LIBRARY"] = gates["COMPILER_ARTIFACT_IMPORTED"]
if gates["SHORTCUT_EXISTS_IN_LIBRARY"] != "PASS":
    if gates["ACTION_EDITOR_CAPTURED"] == "EDITOR_CAPTURE_COMPLETE":
        gates["ACTION_EDITOR_CAPTURED"] = "PARTIAL_EDITOR_CAPTURE"
    gates["SHORTCUT_RUN_ATTEMPTED"] = gates["SHORTCUT_EXISTS_IN_LIBRARY"]
    gates["SHORTCUT_RUN_COMPLETED"] = gates["SHORTCUT_EXISTS_IN_LIBRARY"]
    gates["TRACE_CAPTURED"] = gates["SHORTCUT_EXISTS_IN_LIBRARY"]
if gates["SHORTCUT_RUN_COMPLETED"] != "PASS" and gates["TRACE_CAPTURED"] in ("PASS", "NOT_APPLICABLE_RELEASE"):
    gates["TRACE_CAPTURED"] = "FAIL_RUNTIME"

try:
    assertions = json.load(open(os.path.join(artifact_dir, "runtime", "output-assertions.json"), encoding="utf-8"))
    if not isinstance(assertions, dict):
        assertions = {}
except (OSError, ValueError):
    assertions = {}
release_absence_verified = (
    assertions.get("targetFixtureId") == "qa-measure-release-v2"
    and assertions.get("compilerCommitSha") == sha
    and assertions.get("buildProfile") == "release"
    and assertions.get("traceExpectation") == "ABSENT_RELEASE"
    and assertions.get("traceObserved") == "ABSENT"
    and assertions.get("releaseBuildVerified") is True
    and assertions.get("runtimeContractVerified") is True
)
normal_trace_verified = (gates["TRACE_CAPTURED"] == "PASS"
                         and assertions.get("targetFixtureId") != "qa-measure-release-v2"
                         and assertions.get("compilerCommitSha") == sha
                         and assertions.get("buildProfile") == "debug"
                         and assertions.get("traceExpectation") == "REQUIRED_NDJSON"
                         and assertions.get("runtimeContractVerified") is True)
trace_requirement_met = (normal_trace_verified
                         or (gates["TRACE_CAPTURED"] == "NOT_APPLICABLE_RELEASE" and release_absence_verified))
if gates["TRACE_CAPTURED"] in ("PASS", "NOT_APPLICABLE_RELEASE") and not trace_requirement_met:
    gates["TRACE_CAPTURED"] = "FAIL_RUNTIME"

if (
    gates["SHORTCUT_RUN_COMPLETED"] == "PASS"
    and trace_requirement_met
    and gates["ACTION_EDITOR_CAPTURED"] == "EDITOR_CAPTURE_COMPLETE"
):
    readiness = "READY_SIMULATOR_VERIFIED"
elif gates["COMPILER_ARTIFACT_IMPORTED"] == "BLOCKED_SIGNING":
    readiness = "PARTIAL_READY_WITH_BLOCKED_SIGNING"
else:
    readiness = "PARTIAL_READY"

# The same explicit text boundary is used by the independent publisher.
# Raw original logs remain quarantined and available for exact receipt checks.
sys.path.insert(0, os.path.join(os.getcwd(), "tools", "shortcuts-simulator-qa"))
from prepare_public_artifacts import scan_public_privacy
privacy = scan_public_privacy(artifact_dir, raw_udid,
                             os.environ.get("CHERRI_SIGNING_SERVER", "").strip())
privacy_violations = privacy["violations"]
privacy_passed = privacy["passed"]
inspector_status = "NOT_RUN"
inspector_path = os.path.join(artifact_dir, "editor", "editor-comparison.json")
if os.path.exists(inspector_path):
    try:
        inspector_status = json.load(open(inspector_path, encoding="utf-8")).get("status", "NEEDS_REVIEW")
    except (OSError, ValueError):
        inspector_status = "NEEDS_REVIEW"
if readiness == "READY_SIMULATOR_VERIFIED" and inspector_status != "PASS":
    readiness = "RUNTIME_VERIFIED_EDITOR_NEEDS_REVIEW"

inspection_status = "NOT_RUN"
inspection_path = os.path.join(artifact_dir, "runtime", "post-test-inspection.json")
checkpoint_path = os.path.join(artifact_dir, "builds", "qa-acquisition-test02_started.json")
if os.path.exists(checkpoint_path):
    inspection_status = "INCOMPLETE"
    try:
        from qa_terminal_receipts import context, unique_json, validate_inspection
        expected = context(artifact_dir, sha, run_id, assertions.get("targetFixtureId", ""))
        inspection = unique_json(open(inspection_path, "rb").read())
        validate_inspection(inspection, artifact_dir, expected)
        inspection_status = inspection["status"]
    except (OSError, ValueError, TypeError, KeyError):
        inspection_status = "REJECTED"
    if inspection_status != "COLLECTED" and readiness.startswith(("READY_", "RUNTIME_VERIFIED_")):
        readiness = "RUNTIME_VERIFIED_COLLECTION_INCOMPLETE"

summary = {
    "schemaVersion": "1.4",
    "runId": run_id,
    "compilerCommitSha": sha,
    "requireRuntime": (req_rt == "true"),
    "targetFixtureId": assertions.get("targetFixtureId", "UNKNOWN"),
    "buildProfile": assertions.get("buildProfile", "UNKNOWN"),
    "traceExpectation": assertions.get("traceExpectation", "REQUIRED_NDJSON"),
    "traceRequirementMet": trace_requirement_met,
    "runtimeContractVerified": assertions.get("runtimeContractVerified") is True,
    "releaseReadiness": readiness,
    "privacyGatePassed": privacy_passed,
    "privacyScope": "EXPLICIT_SYNTHETIC_TEXT_PATHS_ONLY",
    "pixelRedactionVerified": False,
    "editorInspectorStatus": inspector_status,
    "privacyViolations": privacy_violations,
    "gates": gates,
    "postTestInspectionStatus": inspection_status,
    "acquisitionIncomplete": inspection_status in ("INCOMPLETE", "REJECTED"),
}
if assertions.get("targetFixtureId") in ("qa-nested-list-control-v2", "qa-nested-list-candidate-v2"):
    observed = (assertions.get("runtimeObservationVerified") is True
                and trace_requirement_met and gates["SHORTCUT_RUN_COMPLETED"] == "PASS")
    readiness = "DIAGNOSTIC_OBSERVATION_COMPLETE" if observed else "DIAGNOSTIC_OBSERVATION_INCOMPLETE"
    summary.update(runtimeContractMode="NESTED_LIST_OBSERVATION_ONLY", diagnosticOnly=True,
                   runtimeObservationVerified=observed, semanticContractVerified=False,
                   nativeListTypeProved=False, formalAcceptanceGranted=False, releaseReadiness=readiness)
with open(os.path.join(artifact_dir, "summary.json"), "w", encoding="utf-8") as f:
    json.dump(summary, f, indent=2)

lines = [
    f"# Cherri iPadOS 27 Simulator QA Report (`{run_id}`)",
    "",
    f"- **Compiler Commit SHA:** `{sha}`",
    f"- **Release Readiness:** `{readiness}`",
    f"- **Require Runtime Mode:** `{req_rt}`",
    f"- **Privacy Gate Passed:** `{privacy_passed}`",
    "",
    "## Progressive Sub-Gate Outcomes",
    "",
    "| Sub-Gate | Status |",
    "|---|---|",
]
for k, v in gates.items():
    lines.append(f"| `{k}` | `{v}` |")

report_md = "\n".join(lines) + "\n"
with open(os.path.join(artifact_dir, "docs", "report.md"), "w", encoding="utf-8") as f:
    f.write(report_md)

step_summary = os.environ.get("GITHUB_STEP_SUMMARY")
if step_summary:
    with open(step_summary, "a", encoding="utf-8") as sf:
        sf.write(report_md)
PY

echo "=== QA Run Complete: $ARTIFACT_DIR/summary.json ==="
cat "$ARTIFACT_DIR/summary.json"

if [ "$REQUIRE_RUNTIME" = "true" ]; then
  python3 - "$ARTIFACT_DIR/summary.json" <<'PY'
import json, sys

summary = json.load(open(sys.argv[1], encoding="utf-8"))
gates = summary.get("gates", {})
release_absence = (summary.get("targetFixtureId") == "qa-measure-release-v2"
                   and summary.get("buildProfile") == "release"
                   and summary.get("traceExpectation") == "ABSENT_RELEASE"
                   and summary.get("runtimeContractVerified") is True
                   and summary.get("traceRequirementMet") is True
                   and gates.get("TRACE_CAPTURED") == "NOT_APPLICABLE_RELEASE")
normal_trace = (summary.get("targetFixtureId") != "qa-measure-release-v2"
                and summary.get("buildProfile") == "debug"
                and summary.get("traceExpectation") == "REQUIRED_NDJSON"
                and summary.get("traceRequirementMet") is True
                and gates.get("TRACE_CAPTURED") == "PASS")
if (gates.get("SHORTCUT_RUN_COMPLETED") != "PASS"
        or summary.get("runtimeContractVerified") is not True
        or not (normal_trace or release_absence)):
    print("ERROR: require_runtime=true but exact runtime/trace contract was not verified", file=sys.stderr)
    sys.exit(1)
PY
  REQUIRE_RUNTIME_RC=$?
  if [ "$REQUIRE_RUNTIME_RC" -ne 0 ]; then exit "$REQUIRE_RUNTIME_RC"; fi
fi

# Observation completeness is separate from the unchanged strict runtime gate.
# In particular, an ERROR/cancel diagnostic never makes require_runtime pass.
if [ "$HTTP_PROBE_DIAGNOSTIC_RC" -ne 0 ]; then
  echo "ERROR: bounded HTTP diagnostic evidence was incomplete or rejected" >&2
  exit "$HTTP_PROBE_DIAGNOSTIC_RC"
fi

# Required acquisition data is separate from any independently verified callback.
if [ "$POST_TEST_INSPECTION_RC" -ne 0 ]; then
  echo "ERROR: post-test metadata collection was incomplete; originals are retained" >&2
  exit "$POST_TEST_INSPECTION_RC"
fi
