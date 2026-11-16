# Cherri iOS / iPadOS 27 Shortcuts Simulator QA — Verified CI Evidence (`P10R14`)

- **Primary Runtime Run ID (`xcode-27`, `require_runtime=true`, `sample_id=all`):** [`43941751610`](https://github.com/davidpovarsky/cherri/actions/runs/43941751610) (`cherri-ios27-simulator-qa-43941751610`) — **`READY_SIMULATOR_VERIFIED`**
- **Hebrew/RTL Runtime Run ID (`xcode-27`, `require_runtime=true`, `sample_id=qa-rtl-hebrew`):** [`43945569711`](https://github.com/davidpovarsky/cherri/actions/runs/43945569711) (`cherri-ios27-simulator-qa-43945569711`) — **`READY_SIMULATOR_VERIFIED`**
- **Push-Triggered Simulator QA Run ID (`xcode-27`):** [`43936624262`](https://github.com/davidpovarsky/cherri/actions/runs/43936624262) (`cherri-ios27-simulator-qa-43936624262`) — **`READY_SIMULATOR_VERIFIED`**
- **Verified Compiler/Harness Commit SHA:** `b696eb0205cd620d34f1f39f52144b8bb62106d6` (HubSign integration & coordinated handshake commit: `8ff3566f806382e92785f6f240c200f72add569e`)
- **Runner:** GitHub Actions `xcode-27` (`macOS 27.0.1`, Build `26A434`, `Xcode 27.1` Build `27A9275`, `ARM64`)
- **Simulator Device:** `iPad mini (A17 Pro)` (`iOS 27.0`, Build `24A434`, Masked UDID `8721CBAE-REDACTED`)

---

## 1. Progressive 14-Gate Verification Table (`summary.json`)

| # | Sub-Gate | Status | Verified Evidence (`43941751610` + `43945569711`) |
|---:|---|---|---|
| 1 | `RUNNER_AVAILABLE` | **`PASS`** | `macOS 27.0.1 (26A434)` + `Xcode 27.1 (27A9275)` + `xcrun simctl` verified in `environment.json`. |
| 2 | `SIMULATOR_BOOTED` | **`PASS`** | Booted `iPad mini (A17 Pro)` (`iOS 27.0`, Build `24A434`, Masked UDID `8721CBAE-REDACTED`) via `xcrun simctl boot` + `bootstatus -b`. |
| 3 | `SHORTCUTS_APP_INSTALLED` | **`PASS`** | `com.apple.shortcuts` confirmed installed in `simctl listapps`. |
| 4 | `SHORTCUTS_LAUNCHED` | **`PASS`** | `XCUIApplication(bundleIdentifier: "com.apple.shortcuts")` launched and verified `state == .runningForeground` (`shortcutForegroundVerified: true`). |
| 5 | `EDITOR_OPENED` | **`PASS`** | `CherriSimulatorQAUITests` opened the real imported shortcut editor pane on iPadOS 27 (`editorIdentityVerified: true`, `ScrollView` `424x1133` at `(0, 0)`). |
| 6 | `FILE_IMPORT_ATTEMPTED` | **`PASS`** | All 4 import routes executed and recorded in `import-attempts.json` (`schemaVersion: "1.4"`). Route 2 unsigned `openurl` failed closed (`BLOCKED_SIGNING`); Route 3 signed exact-bytes HubSign + coordinated `simctl openurl` succeeded (`exitCode: 0`). |
| 7 | `COMPILER_ARTIFACT_IMPORTED` | **`PASS`** | Freshly compiled V2 and Legacy `.shortcut` plists were signed as exact raw bytes via `HUBSIGN_REMOTE_SERVICE` (`RoutineHub HubSign`) with `AEA1` magic header verification (`qa-functional-v2`: `24,995` B, SHA-256 `f1739727...`; `qa-functional-legacy`: `25,107` B, SHA-256 `1e4509ce...`; `qa-rtl-hebrew-v2`: `24,163` B, SHA-256 `2a88a0d1...`) and imported into Apple Shortcuts (`importConsentButtonTapped: true`, `import-consent-sheet.png`). |
| 8 | `SHORTCUT_EXISTS_IN_LIBRARY` | **`PASS`** | Independent UI enumeration confirmed `qa-functional-v2` (`library-after-import.png`) in `43941751610` and `qa-rtl-hebrew-v2, 18 actions` in `43945569711` inside `All Shortcuts`. |
| 9 | `ACTION_EDITOR_CAPTURED` | **`EDITOR_CAPTURE_COMPLETE`** | Full-scroll editor capture completed across `7` pages (`page-0001.png`..`page-0007.png`) on `qa-functional-v2` with all 3 anchors (`ANCHOR_CARD_01_TOP_HEADER`, `ANCHOR_CARD_15_MID_CENTER`, `ANCHOR_CARD_28_BOTTOM_FOOTER`), `33` native actions (`23` matched card titles, `11` matched parameter values in `plist-vs-editor-comparison.json`), and `5` pages (`ANCHOR_RTL_01_פתיחה`, `18` native actions) on `qa-rtl-hebrew-v2`. |
| 10 | `SHORTCUT_RUN_ATTEMPTED` | **`PASS`** | `CherriSimulatorQAReceiver` triggered `shortcuts://x-callback-url/run-shortcut` with unique per-run nonce (`qa-functional-v2-43941751610` / `qa-rtl-hebrew-v2-43945569711`), tapped `"Always Allow"` on Apple's Privacy share sheet (`phase4-before-confirm.png`), and logged live execution. |
| 11 | `SHORTCUT_RUN_COMPLETED` | **`PASS`** | Real `shortcuts-x-callback` event (`receiverSelfTest: false`) matched expected runId + semantic output: `CHERRI_QA_RESULT:ok:count=2:mode=v2` (`43941751610`) and `CHERRI_QA_RTL_RESULT:ok:hebrew` (`43945569711`). Visually verified in `runtime-callback.png`. |
| 12 | `CALLBACK_CAPTURED` | **`PASS`** | Fixed `xcodegen generate` (`project.yml` `INFOPLIST_FILE: ReceiverApp/Info.plist`, `GENERATE_INFOPLIST_FILE: "NO"`) so `CFBundleURLTypes` (`cherriqa://`) survives build generation. `xcodebuild test` passed `4/4` tests (`passedTests: 4, failedTests: 0`), capturing both self-test and live Shortcut `x-success` callbacks in `Documents/callback-events.json`. |
| 13 | `TRACE_CAPTURED` | **`PASS`** | Live Simulator execution emitted `5` valid NDJSON trace lines (`collector_begin`, `step-1-greeting` `before`/`after`, `workflow-complete` `end`, `collector_end`) on `qa-functional-v2` and `4` valid NDJSON lines (`collector_begin`, `rtl-opening`, `rtl-complete`, `collector_end`) on `qa-rtl-hebrew-v2`. |
| 14 | `PHYSICAL_DEVICE_TESTED` | **`BLOCKED_DEVICE`** | Simulator execution proves Simulator behavior on `iPadOS 27.0` (`iPad mini (A17 Pro)`); physical hardware verification remains a separate explicit owner gate (`APPLE_RUNTIME_EXECUTED=BLOCKED_DEVICE`). |

---

## 2. HubSign Integration & Root Fixes Completed in P10R14

1. **Exact-File Fail-Closed HubSign Signing (`signing.go`, `scripts/ios27_shortcuts_simulator_qa.sh`):**
   - Sends the exact compiled `.shortcut` binary plist bytes (never re-compiling via legacy/older binaries) to RoutineHub HubSign (`HUBSIGN_REMOTE_SERVICE`) with IPv4 `tcp4` fallback when IPv6 routing fails.
   - Fails closed on HTTP errors, XML/JSON error responses, empty outputs, timeouts, or non-`AEA1` magic bytes (`ValidateSignedShortcutBytes`), and records source, unsigned plist, and signed `AEA1` SHA-256 digests in `runtime/signing-provenance.json`.
   - Covered by positive and negative contract unit tests in `TestP14_ExactFileHubSignAndProvenanceContract`.
2. **Resolved `WFActionErrorDomain Code=5` Callback Failure (`tools/shortcuts-simulator-qa/XcodeProject/project.yml`):**
   - Root cause: `xcodegen generate` with `info: path: ReceiverApp/Info.plist` (without `properties:`) overwrote `ReceiverApp/Info.plist` and stripped `CFBundleURLTypes` (`cherriqa://`), causing `LaunchServices` (`BindingEvaluator::CreateWithURLScheme(cherriqa)`) to find no handler and Apple Shortcuts `WFOutputAction` to fail with `No bindings found for scheme cherriqa`.
   - Fix: Switched `project.yml` to `INFOPLIST_FILE: ReceiverApp/Info.plist` + `GENERATE_INFOPLIST_FILE: "NO"` and added a post-`xcodegen` `plistlib` assertion in `scripts/ios27_shortcuts_simulator_qa.sh`.
3. **Resolved Phase 4 UI Freeze & Privacy Dialog Hangs (`CherriSimulatorQAUITests.swift`, `scripts/ios27_shortcuts_simulator_qa.sh`):**
   - Guarded AX queries inside `test04ConfirmPendingRuntimeDialog` behind `.runningForeground`, removed manual `codesign` breaks that caused `ReceiverApp` launch failures (`NSPOSIXErrorDomain code=3`), switched `xcodebuild build` to `-scheme CherriSimulatorQAReceiver`, and preserved `${EXPECTED_SHORTCUT_NAME}.shortcut` on temp/HTTP fallback paths.
4. **38x2 Dual-Dialect Corpus Regression & Fidelity Audit (`TestP13_CorpusFidelityAndSourceSizeAudit`):**
   - Preserved `76/76` dual-dialect parity and `23,353 / 27,652` natural actions (`84.45%`, `fallback=4298`, `invalid=1`, `demoted=0`) on `b696eb0205cd620d34f1f39f52144b8bb62106d6`.
