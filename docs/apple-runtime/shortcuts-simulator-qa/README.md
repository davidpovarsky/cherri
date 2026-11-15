# Cherri iOS / iPadOS 27 Shortcuts Simulator QA Infrastructure

This directory documents Cherri's automated iOS/iPadOS 27 Simulator QA harness for Apple's `Shortcuts.app` (`com.apple.shortcuts`), executed on GitHub Actions `xcode-27` runners (`macOS 27.0.1`, `Xcode 27.1`, `iOS/iPadOS 27.2 Simulator`).

---

## 1. Architecture & Components

| Component | Path | Responsibility |
|---|---|---|
| **Orchestrator Script** | [ios27_shortcuts_simulator_qa.sh](file:///C:/Users/DAVID/Code/cherri-p10r14-signed-shortcut-runtime-20261115/scripts/ios27_shortcuts_simulator_qa.sh) | Builds Cherri, compiles 7 synthetic fixtures (`v2` + `legacy`, instrumented + uninstrumented), boots an iPadOS/iOS 27.2 simulator, executes the 4-route import decision tree, runs `xcodebuild test`, evaluates correlated `x-callback-url` runtime callbacks + NDJSON traces, and writes the 14-gate `summary.json`. |
| **Go Oracle & Privacy Scanner** | [simulator_qa_oracle.go](file:///C:/Users/DAVID/Code/cherri-p10r14-signed-shortcut-runtime-20261115/simulator_qa_oracle.go) | Enforces the 14-gate prerequisite state machine (`ValidateSimulatorQASummary`), validates NDJSON runtime trace streams (`ValidateRuntimeTraceNDJSON`), correlates `x-callback-url` nonces with replay protection (`ValidateCorrelatedRuntimeCallback`), and scans text + binary PNG/xcresult artifacts for secrets/PII (`ScanArtifactTreeForPrivacyViolations`). |
| **Synthetic Fixtures** | `tools/shortcuts-simulator-qa/fixtures/*.cherri` | Curated, zero-PII synthetic shortcuts (`qa-functional-v2`, `qa-functional-legacy`, `qa-scroll-editor-v2`, `qa-scroll-editor-legacy`, `qa-rtl-hebrew-v2`, and uninstrumented baselines). |
| **Callback Receiver App** | [ReceiverApp.swift](file:///C:/Users/DAVID/Code/cherri-p10r14-signed-shortcut-runtime-20261115/tools/shortcuts-simulator-qa/XcodeProject/ReceiverApp/ReceiverApp.swift) & [Info.plist](file:///C:/Users/DAVID/Code/cherri-p10r14-signed-shortcut-runtime-20261115/tools/shortcuts-simulator-qa/XcodeProject/ReceiverApp/Info.plist) | Registers the `cherriqa://` custom URL scheme (`org.cherrilang.qa.Receiver`) and persists structured callback events (`runId`, `eventSource`, `receiverSelfTest`, `status`, `resultSummary`, `tracePayload`, `timestampISO8601`) to `Documents/callback-events.json`. |
| **XCUIAutomation Suite** | [CherriSimulatorQAUITests.swift](file:///C:/Users/DAVID/Code/cherri-p10r14-signed-shortcut-runtime-20261115/tools/shortcuts-simulator-qa/XcodeProject/UITests/CherriSimulatorQAUITests.swift) | Uses public `XCUIApplication(bundleIdentifier: "com.apple.shortcuts")`, `XCUIElementSnapshot.dictionaryRepresentation`, and `XCUIScreen.main.screenshot()` to handle `.shortcut` import consent sheets, distinguish the `All Shortcuts` library sidebar from the iPadOS 27 split-view editor pane (`evaluateEditorState`), scroll multi-page action canvases with multi-signal bottom detection, and redact non-allowlisted UI strings (`sanitizeUIString`). |
| **GitHub Actions Workflow** | [cherri-ios27-shortcuts-simulator-qa.yml](file:///C:/Users/DAVID/Code/cherri-p10r14-signed-shortcut-runtime-20261115/.github/workflows/cherri-ios27-shortcuts-simulator-qa.yml) | Runs on `xcode-27` (`workflow_dispatch` with `require_runtime` + scoped `push`), runs Go oracle unit tests first, enforces a text + binary PNG pre-upload privacy gate, and uploads 7-day retention artifacts (`cherri-ios27-simulator-qa-<run_id>`). |

---

## 2. Progressive 14-Gate Truthfulness Model (`schemaVersion: "1.4"`)

The harness never collapses simulator execution into a single binary pass/fail. Every run records 14 independent progressive sub-gates in `summary.json`, validated by `ValidateSimulatorQASummary`:

1. `RUNNER_AVAILABLE` — `xcode-27` macOS 27.0.1 + Xcode 27.1 + `xcrun simctl` available.
2. `SIMULATOR_BOOTED` — iOS/iPadOS 27.2 Simulator booted and verified via `simctl bootstatus -b`.
3. `SHORTCUTS_APP_INSTALLED` — `com.apple.shortcuts` bundle verified in `simctl listapps`.
4. `SHORTCUTS_LAUNCHED` — `Shortcuts.app` launched and verified in `runningForeground` state.
5. `EDITOR_OPENED` — Split-view / modal Shortcut editor pane (`Other["editor"]` / `NavigationBar["New Shortcut"]` / `NavigationBar[<expectedShortcutName>]`) verified open.
6. `FILE_IMPORT_ATTEMPTED` — All 4 import routes executed and logged in `import-attempts.json`.
7. `COMPILER_ARTIFACT_IMPORTED` — Whether an AEA1-signed `.shortcut` artifact was accepted by the Simulator's Shortcuts importer (`BLOCKED_SIGNING` on unauthenticated CI runners because `/usr/bin/shortcuts sign` requires an active iCloud login).
8. `SHORTCUT_EXISTS_IN_LIBRARY` — Whether the compiled shortcut (`expectedShortcutName`) appears in the Simulator's `All Shortcuts` library (`BLOCKED_SIGNING` when import is blocked by Apple's AEA1 signing requirement).
9. `ACTION_EDITOR_CAPTURED` — `EDITOR_CAPTURE_COMPLETE` only when `SHORTCUT_EXISTS_IN_LIBRARY == PASS`, `editorIdentityVerified == true`, and both top (`ANCHOR_CARD_01` / `ANCHOR_RTL_01`) and bottom (`ANCHOR_CARD_28` / `ANCHOR_RTL_01`) anchors are captured; `PARTIAL_EDITOR_CAPTURE` when the editor pane is open and captured on an empty/unimported shortcut canvas.
10. `SHORTCUT_RUN_ATTEMPTED` — Whether `shortcuts://x-callback-url/run-shortcut` was invoked on an imported shortcut with a per-run nonce (`BLOCKED_SIGNING` if the shortcut could not be imported).
11. `SHORTCUT_RUN_COMPLETED` — Whether the imported shortcut completed execution and returned a correlated `x-success` callback (`receiverSelfTest == false`, matching `runId`, expected `resultSummary`).
12. `CALLBACK_CAPTURED` — Whether `CherriSimulatorQAReceiver` captured and persisted a `cherriqa://` URL callback to `Documents/callback-events.json` (`PASS`).
13. `TRACE_CAPTURED` — Whether valid NDJSON runtime trace records (`===CHERRI_QA_TRACE_NDJSON_V14===`, `INLINE_DEBUG_ACTION` lines, and `COLLECTOR_END` summary) from an imported shortcut were captured (`BLOCKED_SIGNING` when blocked by import signing).
14. `PHYSICAL_DEVICE_TESTED` — Always `BLOCKED_DEVICE` for Simulator runs; Simulator evidence never substitutes for physical iPadOS 27 hardware verification (`APPLE_RUNTIME_EXECUTED`).

---

## 3. Evidence Files

- [p10r14-latest-evidence.json](file:///C:/Users/DAVID/Code/cherri-p10r14-signed-shortcut-runtime-20261115/docs/apple-runtime/shortcuts-simulator-qa/p10r14-latest-evidence.json) / [p10r14-latest-evidence.md](file:///C:/Users/DAVID/Code/cherri-p10r14-signed-shortcut-runtime-20261115/docs/apple-runtime/shortcuts-simulator-qa/p10r14-latest-evidence.md) — Latest verified P10R14 evidence (`runId: 43751936080`).
- [latest-evidence.json](file:///C:/Users/DAVID/Code/cherri-p10r14-signed-shortcut-runtime-20261115/docs/apple-runtime/shortcuts-simulator-qa/latest-evidence.json) / [latest-evidence.md](file:///C:/Users/DAVID/Code/cherri-p10r14-signed-shortcut-runtime-20261115/docs/apple-runtime/shortcuts-simulator-qa/latest-evidence.md) — Canonical pointer to the latest verified run.
