# Cherri P10R14 — Team Handoff & Verification Summary

- **Branch:** `agent/cherri-p10r14-signed-shortcut-runtime-20261115`
- **Base Commit (P10R13):** `11428e8db52c46ee3d616aa4b923d06cac0e48df`
- **Key P10R14 Commits:**
  - `cfe0564038ee5376692350daac33200c4b91e9d7` — Core P10R14 implementation (oracle state machine, NDJSON trace stream, UTF-16 ranges, compile-time trace manifest, `ReceiverApp/Info.plist`, UI sanitizer, workflow `require_runtime` + binary privacy scanner)
  - `3b73f43f13e7a11d677f26fe503a6db93e48beb4` — Non-circular `docs/fork-action-provenance.json` update
  - `66e50265d95497a32781d33e088a66dbdd55898a` — iPadOS 27 split-view Shortcuts editor pane detection & UI chrome allowlist
  - `8ff3566f806382e92785f6f240c200f72add569e` — Exact-file fail-closed HubSign signing (`RoutineHub HubSign`), SHA-256 provenance, and coordinated import/run handshake
  - `924eb7f34d2f9f4784a15511047ac86acd3d4eff` — Automatic IPv4 `tcp4` fallback on IPv6 `no-route-to-host` for HubSign requests
  - `0fde61ec4793a9dc8455003484c185419f2ff211` — Xcode Simulator-signed ReceiverApp & `test04ConfirmPendingRuntimeDialog` foreground AX guard
  - `3dd3af189ca56da3c979767fa33609354e7b8470` — Preserved `${EXPECTED_SHORTCUT_NAME}.shortcut` import naming on fallback & switched `xcodebuild build` to `-scheme CherriSimulatorQAReceiver`
  - `b696eb0205cd620d34f1f39f52144b8bb62106d6` — Removed destructive `info:` block in `project.yml` so `xcodegen generate` preserves `CFBundleURLTypes` (`cherriqa://`)
- **Execution Mode:** `NORMAL` (`[MAIN_SESSION]`)
- **Active Roles Executed in Sequence:** `[תומר | INT] [MAIN_SESSION]`, `[איתן | ARCH] [MAIN_SESSION]`, `[נועם | DEV] [MAIN_SESSION]`, `[מיה | UX] [MAIN_SESSION]`, `[דנה | SEC] [MAIN_SESSION]`, `[רוני | QA] [MAIN_SESSION]`

---

## 1. Delivered Milestones & Root Fixes

1. **Fail-Closed HubSign Signing + Provenance (`[נועם | DEV]` & `[דנה | SEC] [MAIN_SESSION]`):**
   - Wired exact binary plist signing via `RoutineHub HubSign` (`signing.go`, `scripts/ios27_shortcuts_simulator_qa.sh`) with IPv4 `tcp4` fallback, enforcing non-empty `AEA1` magic output and fail-closed rejection on HTTP errors, XML/JSON error payloads, and timeouts.
   - Records source SHA-256, unsigned plist SHA-256, and signed `AEA1` SHA-256 for both V2 (`qa-functional-v2`, `qa-rtl-hebrew-v2`) and Legacy (`qa-functional-legacy`) synthetic fixtures without sending private corpus files or CI iCloud logins.
2. **Resolved `WFActionErrorDomain Code=5` URL Scheme Stripping (`[נועם | DEV]` & `[תומר | INT] [MAIN_SESSION]`):**
   - Identified root cause in `tools/shortcuts-simulator-qa/XcodeProject/project.yml`: `info: path: ReceiverApp/Info.plist` without `properties:` caused `xcodegen generate` to overwrite `ReceiverApp/Info.plist` and delete `CFBundleURLTypes` (`cherriqa://`), leaving `LaunchServices` with `No bindings found for scheme cherriqa`.
   - Fixed via `INFOPLIST_FILE: ReceiverApp/Info.plist` + `GENERATE_INFOPLIST_FILE: "NO"` and a post-`xcodegen` `plistlib` guard.
3. **Real iPadOS 27 Simulator Import, Editor Capture, Live `x-callback-url` Execution & NDJSON Trace (`[מיה | UX]` & `[רוני | QA] [MAIN_SESSION]`):**
   - `43941751610` (`require_runtime=true`, `sample_id=all`): Signed `qa-functional-v2` (`24,995` B) & `qa-functional-legacy` (`25,107` B), imported into `All Shortcuts`, captured 7 full editor pages (`33` native actions, all 3 anchors `ANCHOR_CARD_01`/`15`/`28`, `23` card titles & `11` parameter values matched), confirmed Apple's Privacy output sheet, received real `x-success` callback (`qa-functional-v2-43941751610`, `CHERRI_QA_RESULT:ok:count=2:mode=v2`) + `5` live NDJSON trace lines, **`READY_SIMULATOR_VERIFIED`** (`4/4` XCTest pass).
   - `43945569711` (`require_runtime=true`, `sample_id=qa-rtl-hebrew`): Signed `qa-rtl-hebrew-v2` (`24,163` B), imported (`qa-rtl-hebrew-v2, 18 actions`), captured 5 editor pages (`ANCHOR_RTL_01_פתיחה`, Hebrew Text & Dictionary `"שפה": "עברית"`, `"כיוון": "RTL"`), received real `x-success` callback (`qa-rtl-hebrew-v2-43945569711`, `CHERRI_QA_RTL_RESULT:ok:hebrew`) + `4` live NDJSON trace lines, **`READY_SIMULATOR_VERIFIED`** (`4/4` XCTest pass).

---

## 2. Verified Outcomes & Remaining Scope

- **GitHub Actions `Cherri iOS/iPadOS 27 Shortcuts Simulator QA` (`xcode-27`):**
  - Run [`43941751610`](https://github.com/davidpovarsky/cherri/actions/runs/43941751610) (`qa-functional-v2`, `require_runtime=true`): **SUCCESS** (`READY_SIMULATOR_VERIFIED`, `13/14` PASS, `PHYSICAL_DEVICE_TESTED=BLOCKED_DEVICE`, `privacyGatePassed: true`)
  - Run [`43945569711`](https://github.com/davidpovarsky/cherri/actions/runs/43945569711) (`qa-rtl-hebrew-v2`, `require_runtime=true`): **SUCCESS** (`READY_SIMULATOR_VERIFIED`, `13/14` PASS, `PHYSICAL_DEVICE_TESTED=BLOCKED_DEVICE`, `privacyGatePassed: true`)
  - Push Run [`43936624262`](https://github.com/davidpovarsky/cherri/actions/runs/43936624262): **SUCCESS** (`READY_SIMULATOR_VERIFIED`)
- **38-Shortcut Corpus Regression & Fidelity Audit (`TestP13_CorpusFidelityAndSourceSizeAudit` on `b696eb0`):** **PASS** — `total=27652 natural=23353 (84.45%) fallback=4298 invalid=1 demoted=0`, `76/76` dual-dialect parity (`TestP11_WIRE_01`).
- **Remaining Open Gates (Do Not Close):**
  - `PHYSICAL_DEVICE_TESTED=BLOCKED_DEVICE` (no physical iPad verification in CI).
  - Asana `1219604454669950` (Remote Builder plugin release on torah-social Linux ARM64 remains open pending plugin release verification).
  - Live Simulator coverage of standalone `Bool`, `list`, interactive user-cancel, and runtime-error callbacks (currently verified via Go unit oracle matrix `TestP14_GateStateMachineAndAntiFalsePassNegativeMatrix` and Receiver URL handler rather than dedicated live Simulator runs).
