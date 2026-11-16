# Cherri iOS / iPadOS 27 Shortcuts Simulator QA Infrastructure

This directory documents Cherri's automated iOS/iPadOS 27 Simulator QA harness for Apple's `Shortcuts.app` (`com.apple.shortcuts`), executed on GitHub Actions `xcode-27` runners (`macOS 27.0.1`, `Xcode 27.1`, `iOS/iPadOS 27.0` `iPad mini (A17 Pro)`).

---

## 1. Architecture & Components

| Component | Path | Responsibility |
|---|---|---|
| **Orchestrator Script** | [ios27_shortcuts_simulator_qa.sh](../../../scripts/ios27_shortcuts_simulator_qa.sh) | Builds Cherri, compiles 7 synthetic fixtures (`v2` + `legacy`, instrumented + uninstrumented), signs synthetic V2/Legacy `.shortcut` files as exact raw bytes via `RoutineHub HubSign` (`HUBSIGN_REMOTE_SERVICE`), boots an iPadOS 27 simulator, executes the 4-route import decision tree, runs `xcodebuild test` (`CherriSimulatorQAReceiver`), evaluates correlated `x-callback-url` runtime callbacks + NDJSON traces, and writes the 14-gate `summary.json`. |
| **Signing & Provenance** | [signing.go](../../../signing.go) | Signs exact unsigned binary plist bytes via `HubSign` / RoutineHub (`SignUnsignedShortcutFileWithProvenance`), validates non-empty `AEA1` magic output, rejects XML/JSON/non-`AEA1`/timeouts fail-closed, and records SHA-256 provenance without leaking credentials or private shortcuts. |
| **Go Oracle & Privacy Scanner** | [simulator_qa_oracle.go](../../../simulator_qa_oracle.go) | Enforces the 14-gate prerequisite state machine (`ValidateSimulatorQASummary`), validates NDJSON runtime trace streams (`ValidateRuntimeTraceNDJSON`), correlates `x-callback-url` nonces with replay protection (`ValidateCorrelatedRuntimeCallback`), and scans text + binary PNG/xcresult artifacts for secrets/PII (`ScanArtifactTreeForPrivacyViolations`). |
| **Synthetic Fixtures** | `tools/shortcuts-simulator-qa/fixtures/*.cherri` | Curated, zero-PII synthetic shortcuts (`qa-functional-v2`, `qa-functional-legacy`, `qa-scroll-editor-v2`, `qa-scroll-editor-legacy`, `qa-rtl-hebrew-v2`, and uninstrumented baselines). |
| **Callback Receiver App** | [ReceiverApp.swift](../../../tools/shortcuts-simulator-qa/XcodeProject/ReceiverApp/ReceiverApp.swift) & [Info.plist](../../../tools/shortcuts-simulator-qa/XcodeProject/ReceiverApp/Info.plist) | Registers the `cherriqa://` custom URL scheme (`org.cherrilang.qa.Receiver`, preserved via `INFOPLIST_FILE` + `GENERATE_INFOPLIST_FILE: "NO"` in `project.yml`) and persists structured callback events (`runId`, `eventSource`, `receiverSelfTest`, `status`, `resultSummary`, `tracePayload`, `timestampISO8601`) to `Documents/callback-events.json`. |
| **XCUIAutomation Suite** | [CherriSimulatorQAUITests.swift](../../../tools/shortcuts-simulator-qa/XcodeProject/UITests/CherriSimulatorQAUITests.swift) | Uses public `XCUIApplication(bundleIdentifier: "com.apple.shortcuts")`, `XCUIElementSnapshot.dictionaryRepresentation`, and `XCUIScreen.main.screenshot()` to confirm `.shortcut` import consent sheets, verify `All Shortcuts` library presence, open and scroll real editor canvases with multi-signal bottom detection, confirm runtime share privacy alerts (`test04ConfirmPendingRuntimeDialog`), and redact non-allowlisted UI strings (`sanitizeUIString`). |
| **GitHub Actions Workflow** | [cherri-ios27-shortcuts-simulator-qa.yml](../../../.github/workflows/cherri-ios27-shortcuts-simulator-qa.yml) | Runs on `xcode-27` (`workflow_dispatch` with `require_runtime` + scoped `push`), runs Go oracle & HubSign negative/positive contract tests first, enforces a text + binary PNG pre-upload privacy gate, and uploads 7-day retention artifacts (`cherri-ios27-simulator-qa-<run_id>`). |

---

## 2. Progressive 14-Gate Truthfulness Model (`schemaVersion: "1.4"`)

The harness never collapses simulator execution into a single binary pass/fail. Every run records 14 independent progressive sub-gates in `summary.json`, validated by `ValidateSimulatorQASummary`:

1. `RUNNER_AVAILABLE` — `xcode-27` macOS 27.0.1 + Xcode 27.1 + `xcrun simctl` available.
2. `SIMULATOR_BOOTED` — iPadOS 27.0 Simulator booted and verified via `simctl bootstatus -b`.
3. `SHORTCUTS_APP_INSTALLED` — `com.apple.shortcuts` bundle verified in `simctl listapps`.
4. `SHORTCUTS_LAUNCHED` — `Shortcuts.app` launched and verified in `runningForeground` state.
5. `EDITOR_OPENED` — Real imported Shortcut editor pane verified open (`editorIdentityVerified: true`).
6. `FILE_IMPORT_ATTEMPTED` — All 4 import routes executed and logged in `import-attempts.json`.
7. `COMPILER_ARTIFACT_IMPORTED` — Whether an `AEA1`-signed `.shortcut` artifact (via `RoutineHub HubSign`) was accepted by the Simulator's Shortcuts importer (`PASS` on `43941751610` & `43945569711`).
8. `SHORTCUT_EXISTS_IN_LIBRARY` — Whether the compiled shortcut (`expectedShortcutName`) appears in the Simulator's `All Shortcuts` library (`PASS`).
9. `ACTION_EDITOR_CAPTURED` — `EDITOR_CAPTURE_COMPLETE` only when `SHORTCUT_EXISTS_IN_LIBRARY == PASS`, `editorIdentityVerified == true`, and required anchors (`ANCHOR_CARD_01`/`15`/`28` or `ANCHOR_RTL_01_פתיחה`) are captured across pages; `PARTIAL_EDITOR_CAPTURE` otherwise.
10. `SHORTCUT_RUN_ATTEMPTED` — Whether `shortcuts://x-callback-url/run-shortcut` was invoked on an imported shortcut with a per-run nonce (`PASS`).
11. `SHORTCUT_RUN_COMPLETED` — Whether the imported shortcut completed execution and returned a correlated `x-success` callback (`receiverSelfTest == false`, matching `runId`, expected `resultSummary`) (`PASS`).
12. `CALLBACK_CAPTURED` — Whether `CherriSimulatorQAReceiver` captured and persisted `cherriqa://` URL callbacks to `Documents/callback-events.json` (`PASS`).
13. `TRACE_CAPTURED` — Whether valid NDJSON runtime trace records (`collector_begin`, `INLINE_DEBUG_ACTION` lines with live `apple-currentdate` timestamps, `collector_end`) from an imported shortcut were captured (`PASS`).
14. `PHYSICAL_DEVICE_TESTED` — Always `BLOCKED_DEVICE` for Simulator runs; Simulator evidence never substitutes for physical iPadOS 27 hardware verification (`APPLE_RUNTIME_EXECUTED`).

---

## 3. Evidence Files

- [p10r14-latest-evidence.json](./p10r14-latest-evidence.json) / [p10r14-latest-evidence.md](./p10r14-latest-evidence.md) — Latest verified P10R14 evidence (`runId: 43941751610` functional + `43945569711` Hebrew/RTL).
- [latest-evidence.json](./latest-evidence.json) / [latest-evidence.md](./latest-evidence.md) — Canonical pointer to the latest verified run.
