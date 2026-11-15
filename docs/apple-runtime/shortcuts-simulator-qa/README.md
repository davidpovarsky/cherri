# Cherri iOS / iPadOS 27 Shortcuts Simulator QA Infrastructure

This directory documents Cherri's automated iOS/iPadOS 27 Simulator QA harness for Apple's `Shortcuts.app` (`com.apple.shortcuts`), executed on GitHub Actions `xcode-27` runners (`macOS 27.0.1`, `Xcode 27.1`, `iOS/iPadOS 27.2 Simulator`).

---

## 1. Architecture & Components

| Component | Path | Responsibility |
|---|---|---|
| **Orchestrator Script** | [ios27_shortcuts_simulator_qa.sh](file:///C:/Users/DAVID/Code/cherri-p10r13-simulator-debug-fidelity-20261115/scripts/ios27_shortcuts_simulator_qa.sh) | Builds Cherri, compiles 7 synthetic fixtures (`v2` + `legacy`, instrumented + uninstrumented), boots an iPadOS/iOS 27.2 simulator, executes the 4-route import decision tree, runs `xcodebuild test`, extracts `callback-events.json`, and writes the 14-gate `summary.json`. |
| **Synthetic Fixtures** | `tools/shortcuts-simulator-qa/fixtures/*.cherri` | Curated, zero-PII synthetic shortcuts (`qa-functional-v2`, `qa-functional-legacy`, `qa-scroll-editor-v2`, `qa-scroll-editor-legacy`, `qa-rtl-hebrew-v2`, and uninstrumented baselines). |
| **Callback Receiver App** | [ReceiverApp.swift](file:///C:/Users/DAVID/Code/cherri-p10r13-simulator-debug-fidelity-20261115/tools/shortcuts-simulator-qa/XcodeProject/ReceiverApp/ReceiverApp.swift) | Registers the `cherriqa://` custom URL scheme (`org.cherrilang.qa.Receiver`) and persists structured callback events (`runId`, `status`, `resultSummary`, `tracePayload`, `timestampISO8601`) to `Documents/callback-events.json`. |
| **XCUIAutomation Suite** | [CherriSimulatorQAUITests.swift](file:///C:/Users/DAVID/Code/cherri-p10r13-simulator-debug-fidelity-20261115/tools/shortcuts-simulator-qa/XcodeProject/UITests/CherriSimulatorQAUITests.swift) | Uses public `XCUIApplication(bundleIdentifier: "com.apple.shortcuts")`, `XCUIElementSnapshot.dictionaryRepresentation`, and `XCUIScreen.main.screenshot()` to verify foreground state, discover the editor scroll container, scroll multi-page action canvases with multi-signal bottom detection, and verify `cherriqa://` callback delivery. |
| **GitHub Actions Workflow** | [cherri-ios27-shortcuts-simulator-qa.yml](file:///C:/Users/DAVID/Code/cherri-p10r13-simulator-debug-fidelity-20261115/.github/workflows/cherri-ios27-shortcuts-simulator-qa.yml) | Runs on `xcode-27` (`workflow_dispatch` + scoped `push`), enforces a pre-upload privacy/sanitization gate, and uploads 7-day retention artifacts (`cherri-ios27-simulator-qa-<run_id>`). |

---

## 2. Progressive 14-Gate Truthfulness Model

The harness never collapses simulator execution into a single binary pass/fail. Every run records 14 independent progressive sub-gates in `summary.json`:

1. `RUNNER_AVAILABLE` — `xcode-27` macOS 27.0.1 + Xcode 27.1 + `xcrun simctl` available.
2. `SIMULATOR_BOOTED` — iOS/iPadOS 27.2 Simulator booted and verified via `simctl bootstatus -b`.
3. `SHORTCUTS_APP_INSTALLED` — `com.apple.shortcuts` bundle verified in `simctl listapps`.
4. `SHORTCUTS_LAUNCHED` — `Shortcuts.app` launched and verified in `runningForeground` state.
5. `EDITOR_OPENED` — Editor view opened via `shortcuts://create-shortcut` and UI automation.
6. `FILE_IMPORT_ATTEMPTED` — All 4 import routes executed and logged in `import-attempts.json`.
7. `COMPILER_ARTIFACT_IMPORTED` — Whether a compiled `.shortcut` artifact was accepted by the Simulator's Shortcuts importer (`BLOCKED_SIGNING` on unauthenticated CI runners because `/usr/bin/shortcuts sign` requires an active iCloud login).
8. `SHORTCUT_EXISTS_IN_LIBRARY` — Whether the compiled shortcut appears in the Simulator's Shortcuts library (`BLOCKED_SIGNING` when import is blocked by Apple's AEA1 signing requirement).
9. `ACTION_EDITOR_CAPTURED` — `PASS` when all synthetic anchors (`CHERRI_QA_ANCHOR_TOP`/`MID`/`BOTTOM`) are captured in the editor; `PARTIAL_EDITOR_CAPTURE` when the editor accessibility tree and multi-page screenshots are captured on an unimported/empty shortcut canvas.
10. `SHORTCUT_RUN_ATTEMPTED` — Whether `shortcuts://x-callback-url/run-shortcut` was invoked on an imported shortcut (`BLOCKED_SIGNING` if the shortcut could not be imported).
11. `SHORTCUT_RUN_COMPLETED` — Whether the imported shortcut completed execution and returned `x-success` (`BLOCKED_SIGNING` when blocked by import signing).
12. `CALLBACK_CAPTURED` — Whether `CherriSimulatorQAReceiver` captured and persisted a `cherriqa://` URL callback to `Documents/callback-events.json` (`PASS`).
13. `TRACE_CAPTURED` — Whether runtime `INLINE_DEBUG_ACTION` / `COLLECTOR_EVENT` JSONL trace lines from an imported shortcut were captured (`BLOCKED_SIGNING` when blocked by import signing).
14. `PHYSICAL_DEVICE_TESTED` — Always `BLOCKED_DEVICE` for Simulator runs; Simulator evidence never substitutes for physical iPadOS 27 hardware verification (`APPLE_RUNTIME_EXECUTED`).
