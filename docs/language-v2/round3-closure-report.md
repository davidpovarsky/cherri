# Cherri v2 — Round 3.1 Closure Report

## 1. Overview & Repository Identities

- **Branch:** `agent/shared-shortcuts-engine-v2-round3-closure`
- **Base Round 3 Release SHA (`R3_RELEASE`):** `e0bb44f4b8905229d30bce7163d2b73bbaee0c8c`
- **Base Round 3 Code SHA (`R3_CODE`):** `373339e105093bf12c503e245dbb765aeb26914f`
- **New Frozen Code SHA (`C_CODE`):** `1949d1dd0f2bd31d7cd2c1e95fb9943f9f058555`
- **Pinned Docs SHA (`D_DOCS`):** `cd8284c300cd3d169f022a3cdd8d65efaf7fe056`
- **Schema Fingerprint:** `cd5c0b8eaeabc28a9b0bc0002cb742b4bd725a97bb88ab5aa7d408983c7825a4`
- **Codec Fingerprint:** `3e7e3555eefe1e030d254df0a22c62fdc3a139f92e39da34f6d841f8bfa1689e`
- **Action Catalog:** 461 actions (unmodified, preserved 100%)

---

## 2. Scope & Resolved Issues (R3.1-A through R3.1-D)

### R3.1-A: Strict Target-Specific XCUITest Menu Selection
- **Root Cause:** In Round 3, `run_ui_helper` in `scripts/ios27_runtime_poc.sh` sent blind AppleScript `Return` (key code 36) and `Space` (key code 49) to the Simulator window. In Menu Run 1 (Alpha), this activated the menu dialog before XCTest launched, triggering the next action (`setClipboard`), which displayed SpringBoard's clipboard permission dialog `Allow “CherriMenuRuntimePOC” to copy to the clipboard?`. XCTest launched, searched for "Alpha", found nothing, fell through to candidate matching, and clicked "Allow", producing a false-positive tap log.
- **Fix in `UITests.swift`:**
  - Added strict menu-selection branch for stages starting with `menu_select_` (`menu_select_alpha`, `menu_select_beta`).
  - Target validated strictly against allowlist `["Alpha", "Beta"]`.
  - Searches for exact accessible label in SpringBoard and Shortcuts hierarchies.
  - On tap, emits structured marker:
    `CHERRI_EXACT_MENU_TAP target=<target> actual_label=<label> app=<app> kind=<kind> stage=<stage> result=tapped`
  - Never falls through to `Allow`, `Run`, `Add`, or candidate buttons. If exact target is not located, fails immediately via `XCTFail` and returns.
- **Fix in `scripts/ios27_runtime_poc.sh`:**
  - Suppressed blind AppleScript Return/Space keystrokes during `menu_select_*` stages.
  - Separated menu selection from clipboard permission handling (`run_permission_menu`).

### R3.1-B: Fail-Closed Runtime Shell & Reliable Evidence Verifier
- **Loophole Removed:** Completely deleted `if [ "$MENU_RUNTIME_STATUS" = "BLOCKED" ]; then ... exit 0; fi` from `scripts/ios27_runtime_poc.sh`. If menu interaction or any required gate fails, the script exits with code 1.
- **Structured Selection Proofs:** Script parses XCUITest logs and generates target-specific proof JSONs:
  - `artifacts/menu-alpha-selection-proof.json`
  - `artifacts/menu-beta-selection-proof.json`
  - `artifacts/menu-alpha-clipboard-result.txt`
  - `artifacts/menu-beta-clipboard-result.txt`
- **Evidence Verifier (`tools/ios-runtime-verifier`):** Built standalone Go tool and test suite (`verifier.go`, `verifier_test.go`) implementing 6 mandatory positive and negative test controls:
  1. Alpha clipboard correct but log shows "Allow" tapped without exact marker -> **FAIL** (tested against actual saved Round 3 log fixture).
  2. Alpha exact tap logged but clipboard contains `CHERRI_MENU_BETA` -> **FAIL**.
  3. Beta passed but Alpha missing -> **FAIL**.
  4. Corrupted log / mismatched stage / failed XCTest -> **FAIL**.
  5. BLOCKED status / missing directory artifacts -> **FAIL**.
  6. Both exact markers present with correct fresh clipboards -> **PASS**.
- Verifier gate is automatically run by `scripts/ios27_runtime_poc.sh` before exiting.

### R3.1-C: Acceptance Reporting Baseline
- Re-ran local acceptance runner via `go run ./tools/language-acceptance --phase local --out artifacts/round3-closure/local-acceptance-results.json`.
- Verified local baseline counts:
  - **Total Cases:** 94
  - **Passed Cases:** 72
  - **Skipped / Pending Cases:** 22
  - **Failed Cases:** 0
  - **Closure Complete:** `false`
- Distinctions clearly stated:
  - Local checks passed: 72/94.
  - Pending external tiers: 22 skipped (including `ci`, `ios-runtime`, `ios-ui`, `ai-eval`).
  - Unsupported features remain open: `native.plist`, `native.ref`, `native.workflow`.
  - External model evaluation `AI01` remains not configured.

### R3.1-D: CI Test Discovery Accuracy & Process Failure Propagation
- In `tools/ci-test-discovery/main.go`:
  - Replaced hardcoded `ALL 43` string with dynamic `len(requiredTestNames)` (44).
  - Factored test runner into `RunCommandAndVerify`, checking child process `cmd.Wait()` error and propagating exit failure even if matching output strings appeared.
  - Added unit tests `TestVerifyEvents_Exact44Count` and `TestRunCommandAndVerify_NegativeChildProcessFailure` in `main_test.go`.

---

## 3. Local Verification Results

- `go build ./...`: PASS (exit code 0)
- `go test -v ./tools/ci-test-discovery/...`: PASS (5/5 tests, including child failure propagation and exact 44 count)
- `go test -v ./tools/ios-runtime-verifier/...`: PASS (6/6 tests, including negative Round 3 regression fixture)
- `go run ./tools/ci-test-discovery -run '^Test(CORE|CODEC|CTRL|META|LANG|IOS|GEN)'`: PASS (44/44 regression matrix verified, gate PASS)
- `go test -run TestDecomp -v ./...`: PASS
- `go run ./tools/language-acceptance --phase local --out artifacts/round3-closure/local-acceptance-results.json`: PASS (72 passed, 22 skipped, 0 failed)
