# Cherri iOS / iPadOS 27 Shortcuts Simulator QA — Verified CI Evidence (`P10R13`)

- **Primary Full-Fixture Run ID:** `43733946853` (`cherri-ios27-simulator-qa-43733946853`)
- **iPad mini (A20 Pro) Run ID:** `43731642051` (`cherri-ios27-simulator-qa-43731642051`)
- **Compiler Commit SHA:** `616f14eecd4d81a020677ecab4a3d37fc24c2625`
- **Runner:** GitHub Actions `xcode-27` (`macOS 27.0.1`, Build `26A434`, `Xcode 27.1` Build `27A9275`, `ARM64`)

---

## 1. Progressive 14-Gate Verification Table

| # | Sub-Gate | Status | Verified Evidence |
|---:|---|---|---|
| 1 | `RUNNER_AVAILABLE` | **`PASS`** | `macOS 27.0.1 (26A434)` + `Xcode 27.1 (27A9275)` + `xcrun simctl` verified in `environment.json`. |
| 2 | `SIMULATOR_BOOTED` | **`PASS`** | Booted `iPad mini (A20 Pro)` (`iOS 27.2`, Build `24B8117`, run `43731642051`) and `iPhone Duo` (`iOS 27.2`, Build `24B91016s`, run `43733946853`) via `xcrun simctl boot` + `bootstatus -b`. |
| 3 | `SHORTCUTS_APP_INSTALLED` | **`PASS`** | `com.apple.shortcuts` confirmed installed in `simctl listapps` on both `iOS 27.2` Simulator runtimes. |
| 4 | `SHORTCUTS_LAUNCHED` | **`PASS`** | `XCUIApplication(bundleIdentifier: "com.apple.shortcuts")` launched and verified `state == .runningForeground` (`shortcutForegroundVerified: true`). |
| 5 | `EDITOR_OPENED` | **`PASS`** | `xcrun simctl openurl <UDID> shortcuts://create-shortcut` (`exitCode: 0`) opened the Shortcuts editor view (`editorOpened: true`). |
| 6 | `FILE_IMPORT_ATTEMPTED` | **`PASS`** | All 4 routes in the import decision tree executed and recorded in `import-attempts.json`. |
| 7 | `COMPILER_ARTIFACT_IMPORTED` | **`BLOCKED_SIGNING`** | Route 2 (`xcrun simctl openurl <UDID> file://<unsigned.shortcut>`) is rejected by iOS/iPadOS 27.2 `Shortcuts.app` because Apple requires AEA1-signed `.shortcut` files; Route 3 (`/usr/bin/shortcuts sign --mode anyone`) fails on headless GitHub Actions runners with `exitCode: 1`: `"Error: In order to do this, you must be signed into iCloud."` |
| 8 | `SHORTCUT_EXISTS_IN_LIBRARY` | **`BLOCKED_SIGNING`** | Because unauthenticated CI runners cannot AEA1-sign `.shortcut` archives via iCloud, the compiled shortcut cannot be added to the Simulator's Shortcuts library. |
| 9 | `ACTION_EDITOR_CAPTURED` | **`PARTIAL_EDITOR_CAPTURE`** | `CherriSimulatorQAUITests` discovered the live Shortcuts scroll container (`ScrollView` `475x1158` on `iPad mini (A20 Pro)`; `CollectionView` `466x678` on `iPhone Duo`), captured 3 scrolled viewport pages (`page-0001.png`..`page-0003.png`) + `editor-tree.json` via `XCUIElementSnapshot.dictionaryRepresentation`, and detected bottom-of-canvas via `MULTI_SIGNAL_STALL_AT_PAGE_3:IDENTICAL_VISIBLE_LABEL_FRAME_DIGEST+ZERO_LAST_CARD_Y_DELTA`. Marked `PARTIAL_EDITOR_CAPTURE` because custom compiled action cards require signed import. |
| 10 | `SHORTCUT_RUN_ATTEMPTED` | **`BLOCKED_SIGNING`** | Dependent on `SHORTCUT_EXISTS_IN_LIBRARY`; not faked when the compiled shortcut is blocked from library import. |
| 11 | `SHORTCUT_RUN_COMPLETED` | **`BLOCKED_SIGNING`** | Dependent on `SHORTCUT_EXISTS_IN_LIBRARY`; not faked when the compiled shortcut is blocked from library import. |
| 12 | `CALLBACK_CAPTURED` | **`PASS`** | `CherriSimulatorQAReceiver` (`org.cherrilang.qa.Receiver`) handled `cherriqa://success?runId=receiver-self-test&status=SUCCESS&resultSummary=CHERRI_QA_RECEIVER_OK` in Simulator and persisted `Documents/callback-events.json` (`256 B`). |
| 13 | `TRACE_CAPTURED` | **`BLOCKED_SIGNING`** | Compile-time instrumentation (`INLINE_DEBUG_ACTION` + `COLLECTOR_EVENT`) is verified in `runtime/instrumentation-vs-original.json` (`sourceActionSequenceMatches: true`), while live runtime trace emission requires signed shortcut import. |
| 14 | `PHYSICAL_DEVICE_TESTED` | **`BLOCKED_DEVICE`** | Simulator execution does not substitute for physical iPadOS 27 hardware verification (`APPLE_RUNTIME_EXECUTED=BLOCKED_DEVICE`). |

---

## 2. Import Decision Tree Evidence (`import-attempts.json`)

| Route ID | Documented | Command | Exit Code | Status | Observed Output / Apple Constraint |
|---|---|---|---:|---|---|
| `route_1_create_shortcut_scheme` | `true` | `xcrun simctl openurl <UDID> shortcuts://create-shortcut` | `0` | **`PASS`** | Opens `Shortcuts.app` directly into a new Shortcut editor canvas. |
| `route_2_unsigned_file_openurl` | `true` | `xcrun simctl openurl <UDID> file://<unsigned.shortcut>` | `0` | **`ATTEMPTED_UI_CHECK_REQUIRED`** | `simctl openurl` dispatches the file URL (`exitCode: 0`), but iOS/iPadOS 27.2 `Shortcuts.app` refuses to import unsigned `.shortcut` plists. |
| `route_3_macos_host_sign_then_openurl` | `true` | `/usr/bin/shortcuts sign --mode anyone --input ... --output ...` | `1` | **`BLOCKED_SIGNING`** | `Error: In order to do this, you must be signed into iCloud.` (Apple requires an active iCloud session on the macOS host to issue AEA1 signatures). |
| `route_4_exploratory_import_shortcut_scheme` | `false` (`UNDOCUMENTED`) | `xcrun simctl openurl <UDID> shortcuts://import-shortcut?name=CherriQAProbe` | `0` | **`UNDOCUMENTED_PROBE`** | Exploratory scheme probe; does not import unsigned local files on iOS 27.2. |

---

## 3. Compiled Synthetic Fixtures & Instrumentation Parity (`compiler-manifest.json`)

| Fixture ID | Dialect | Source SHA-256 | Unsigned `.shortcut` SHA-256 | Native Action Count | Source Sequence Matches Uninstrumented |
|---|---|---|---|---:|---|
| `qa-functional-v2` | `v2` | `c18a0d6f4f8fc339...` | `2633d1392e063e83...` | `30` (`13` src + `15` inline + `2` collector) | **`true` (`13/13`)** |
| `qa-functional-legacy` | `legacy` | `d12d6e3d770dbe8a...` | `70918c2b233bf8f0...` | `31` (`14` src + `15` inline + `2` collector) | **`true` (`14/14`)** |
| `qa-scroll-editor-v2` | `v2` | `e8991ab412f0bbe8...` | `f5212e210351646f...` | `35` | — |
| `qa-scroll-editor-legacy` | `legacy` | `7fda9c4c5286e109...` | `d064710fda26242e...` | `36` | — |
| `qa-rtl-hebrew-v2` | `v2` | `dc380bd398377012...` | `f8d00926161df32e...` | `14` | — |
| `qa-functional-v2-uninstrumented` | `v2` | `bb30fa2191c34e6f...` | `b9d8ff0653ec8f29...` | `13` | Baseline |
| `qa-functional-legacy-uninstrumented` | `legacy` | `e0ec58034022dc6b...` | `29fa8f57b665963e...` | `14` | Baseline |
