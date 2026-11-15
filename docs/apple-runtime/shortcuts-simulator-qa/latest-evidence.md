# Cherri iOS / iPadOS 27 Shortcuts Simulator QA — Verified CI Evidence (`P10R14`)

- **Primary Run ID (`xcode-27`):** `43751936080` (`cherri-ios27-simulator-qa-43751936080`)
- **Intermediate Run ID (`xcode-27`):** `43749990966` (`cherri-ios27-simulator-qa-43749990966`)
- **Cross-Platform Build & Test Run ID:** `43749990977`
- **Compiler Commit SHA:** `66e50265d95497a32781d33e088a66dbdd55898a` (Implementation: `cfe0564038ee5376692350daac33200c4b91e9d7`, Provenance: `3b73f43f13e7a11d677f26fe503a6db93e48beb4`)
- **Runner:** GitHub Actions `xcode-27` (`macOS 27.0.1`, Build `26A434`, `Xcode 27.1` Build `27A9275`, `ARM64`)
- **Simulator Device:** `iPad mini (A20 Pro)` (`iOS 27.2`, Build `24B8117`, Masked UDID `4DF1AA75-REDACTED`)

---

## 1. Progressive 14-Gate Verification Table (`summary.json`)

| # | Sub-Gate | Status | Verified Evidence |
|---:|---|---|---|
| 1 | `RUNNER_AVAILABLE` | **`PASS`** | `macOS 27.0.1 (26A434)` + `Xcode 27.1 (27A9275)` + `xcrun simctl` verified in `environment.json`. |
| 2 | `SIMULATOR_BOOTED` | **`PASS`** | Booted `iPad mini (A20 Pro)` (`iOS 27.2`, Build `24B8117`, Masked UDID `4DF1AA75-REDACTED`) via `xcrun simctl boot` + `bootstatus -b`. |
| 3 | `SHORTCUTS_APP_INSTALLED` | **`PASS`** | `com.apple.shortcuts` confirmed installed in `simctl listapps` on `iOS 27.2` Simulator runtime. |
| 4 | `SHORTCUTS_LAUNCHED` | **`PASS`** | `XCUIApplication(bundleIdentifier: "com.apple.shortcuts")` launched and verified `state == .runningForeground` (`shortcutForegroundVerified: true`). |
| 5 | `EDITOR_OPENED` | **`PASS`** | `CherriSimulatorQAUITests.evaluateEditorState` verified that tapping `main.button.newshortcut` opened the iPadOS 27 split-view editor pane (`Other["editor"]` + `NavigationBar["New Shortcut"]` + `ScrollView` `475x1158` at `x: 281`). |
| 6 | `FILE_IMPORT_ATTEMPTED` | **`PASS`** | All 4 routes in the import decision tree executed and recorded in `import-attempts.json` (`schemaVersion: "1.4"`). |
| 7 | `COMPILER_ARTIFACT_IMPORTED` | **`BLOCKED_SIGNING`** | Route 2 (`xcrun simctl openurl <UDID> file://<unsigned.shortcut>`) is rejected by iOS/iPadOS 27.2 `Shortcuts.app` because Apple requires AEA1-signed `.shortcut` files; Route 3 (`/usr/bin/shortcuts sign --mode anyone`) fails on headless GitHub Actions runners with `exitCode: 1`: `"Error: In order to do this, you must be signed into iCloud."` |
| 8 | `SHORTCUT_EXISTS_IN_LIBRARY` | **`BLOCKED_SIGNING`** | Because unauthenticated CI runners cannot AEA1-sign `.shortcut` archives via iCloud, the compiled shortcut (`qa-functional-v2`) does not exist in `All Shortcuts` (`shortcutExistsInLibrary: false`). |
| 9 | `ACTION_EDITOR_CAPTURED` | **`PARTIAL_EDITOR_CAPTURE`** | `CherriSimulatorQAUITests` verified `editorOpened: true`, captured 3 viewport pages (`page-0001.png`..`page-0003.png`) + sanitized `editor-tree.json` + `screenshots-manifest.json` on the `ScrollView` (`475x1158` at `x: 281`), and recorded `PARTIAL_EDITOR_CAPTURE` because `editorIdentityVerified: false` (`expectedShortcutName: "qa-functional-v2"` is blocked by AEA1 signing). |
| 10 | `SHORTCUT_RUN_ATTEMPTED` | **`BLOCKED_SIGNING`** | Enforced by the 14-gate state machine (`ValidateSimulatorQASummary`): requires `SHORTCUT_EXISTS_IN_LIBRARY == PASS`. |
| 11 | `SHORTCUT_RUN_COMPLETED` | **`BLOCKED_SIGNING`** | Removed the P10R13 hardcoded `BLOCKED_IMPORT` overwrite in `scripts/ios27_shortcuts_simulator_qa.sh`; when `SHORTCUT_EXISTS_IN_LIBRARY == PASS`, evaluates the live correlated `x-callback-url` event (`runId == <fixtureId>-<runId>`, `receiverSelfTest == false`). On headless CI without an iCloud-signed fixture, truthfully reports `BLOCKED_SIGNING`. |
| 12 | `CALLBACK_CAPTURED` | **`PASS`** | Tracked `tools/shortcuts-simulator-qa/XcodeProject/ReceiverApp/Info.plist` in git (`!**/ReceiverApp/Info.plist` in `.gitignore`). `xcodebuild test` passed `3/3` XCTest methods (`passedTests: 3, failedTests: 0`), and `CherriSimulatorQAReceiver` persisted `Documents/callback-events.json` (`runId: "receiver-self-test"`, `receiverSelfTest: true`, `status: "SUCCESS"`, `resultSummary: "CHERRI_QA_RECEIVER_OK"`). |
| 13 | `TRACE_CAPTURED` | **`BLOCKED_SIGNING`** | Replaced P10R13's invalid `"eventsJsonl":"\uFFFC"` wrapper with a valid NDJSON stream (`schemaVersion: "1.4"`, `traceTransport: "ndjson-stream"`, `collectorSentinel: "===CHERRI_QA_TRACE_NDJSON_V14==="`), UTF-16 code-unit offsets (`utf16CodeUnitCount`), and JSON string escaping (`escapeJSONStringContent`). Validated by unit tests (`TestP14_*`); live simulator emission awaits AEA1-signed shortcut import (`BLOCKED_SIGNING`). |
| 14 | `PHYSICAL_DEVICE_TESTED` | **`BLOCKED_DEVICE`** | Simulator execution never substitutes for physical iPadOS 27 hardware verification (`APPLE_RUNTIME_EXECUTED=BLOCKED_DEVICE`). |

---

## 2. Six P10R13 Findings Resolved & Verified in P10R14

1. **Removed Hardcoded `BLOCKED_IMPORT` Overwrite in `scripts/ios27_shortcuts_simulator_qa.sh`:**
   - Inside `if [ "$GATE_SHORTCUT_EXISTS_IN_LIBRARY" = "PASS" ]`, `GATE_SHORTCUT_RUN_COMPLETED` and `GATE_TRACE_CAPTURED` are now computed from the correlated `x-callback-url` event (`runId == <fixtureId>-<runId>`, `receiverSelfTest == false`, `status == "SUCCESS"`, semantic `resultSummary`, and NDJSON trace validation) rather than being unconditionally overwritten with `BLOCKED_IMPORT`.
2. **Real Editor Verification + Regex/Allowlist Sanitizer in `CherriSimulatorQAUITests.swift`:**
   - Replaced the naive `searchFields.count > 0 || collectionViews.count > 0` check with `evaluateEditorState`, which distinguishes the `All Shortcuts` library sidebar from the iPadOS 27 split-view editor pane (`Other["editor"]` + `NavigationBar["New Shortcut"]` / `NavigationBar[expectedShortcutName]`) and checks `editorIdentityVerified` + top/bottom anchors before claiming `EDITOR_CAPTURE_COMPLETE`.
   - Replaced 256-char prefix truncation with `sanitizeUIString`, applying regex secret/PII/UUID/path redaction (`[REDACTED_SECRET_OR_PII]`) and strict UI allowlisting (`[REDACTED_NON_ALLOWLISTED_LEN_<N>]`).
3. **Valid NDJSON Trace Stream (`schemaVersion: "1.4"`) + UTF-16 Offsets (`debug_stdlib.go`):**
   - Replaced the invalid multiline quoted `"eventsJsonl":"\uFFFC"` wrapper with an unquoted NDJSON stream (`===CHERRI_QA_TRACE_NDJSON_V14===` sentinel followed by raw JSONL inline events and a single-line JSON `COLLECTOR_END` summary record).
   - Fixed `WFTextTokenString` attachment range calculation to use UTF-16 code units (`utf16CodeUnitCount`) and escaped embedded quotes, backslashes, control characters, and literal `U+FFFC` (`\ufffc`) via `escapeJSONStringContent`.
4. **Compile-Time Manifest + Structural Trace Action Classification (`debug_stdlib.go`):**
   - Added `recordedTraceActions` manifest tracking during compilation plus strict structural checks (`isStructuralInlineTraceTextAction`, `isStructuralCollectorEnvelopeTextAction`) in `ClassifyShortcutTraceActions`, preventing user-authored `Text` actions that mention debug substrings (`__cherri_qa_trace`, `COLLECTOR_EVENT`, `TraceExportEnvelope`) from being misclassified.
5. **Tracked `ReceiverApp/Info.plist` & Restored `3/3` XCTest Execution on `xcode-27`:**
   - Added `!/tools/shortcuts-simulator-qa/XcodeProject/ReceiverApp/Info.plist` after `*.plist` in `.gitignore` and committed `ReceiverApp/Info.plist`.
   - On `xcode-27` run `43751936080`, `xcodebuild test` built `CherriSimulatorQAReceiver` and passed all `3/3` XCTest methods (`test01_ReceiverSelfCallbackAndContainerPersistence`, `test02_ShortcutsImportLibraryAndActionEditorCoverage`, `test03_SanitizerRedactsSecretsAndNonAllowlistedStrings`), turning `CALLBACK_CAPTURED` from `FAIL_UI` to `PASS`.
6. **Workflow `require_runtime` Gate & Binary PNG/Artifact Privacy Scanner:**
   - Added `require_runtime` (`boolean`, default `false`) to `.github/workflows/cherri-ios27-shortcuts-simulator-qa.yml` and `--require-runtime` to `scripts/ios27_shortcuts_simulator_qa.sh`.
   - Added pre-upload text + binary PNG/xcresult scanning (`ScanArtifactTreeForPrivacyViolations` in Go and inline Python pre-upload step in the workflow).
