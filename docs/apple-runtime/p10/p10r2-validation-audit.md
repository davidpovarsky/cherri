# P10R2 Corrective Validation Audit

**Generated:** 2026-10-16 (Asia/Jerusalem)  
**Primary Repository:** `davidpovarsky/cherri`  
**Branch:** `agent/cherri-p10-apple-actions-coverage`  
**Verdict:** `P10R2_CORRECTIVE_VERIFIED` (Corrective release phase) / `P10_IN_PROGRESS` (Backlog open: 118 built-ins, 173 discoverable App Intents)

---

## 1. Forensic Issues Audit (FR-01..FR-14)

| Issue ID | Title | Status | Changed Production Symbols | Real Test Proof |
|---|---|---|---|---|
| **FR-01** | BROKEN immutable release tuple | RESOLVED | `skills/cherri-shortcuts/compatibility-manifest.json` | A01, A02, A28, R26, R27 |
| **FR-02** | INVALID acceptance proofs | RESOLVED | `p10r_acceptance_test.go`, `p10r2_adversarial_test.go` | A19, R28, all A-tests |
| **FR-03** | FILTERS still silently alter requested behavior | RESOLVED | `actions_std.go:buildContentItemFilter`, `parseFilterOperator` | A09, A10, R08, R09, R10, R20 |
| **FR-04** | Dynamic setter Remove bypasses safety | RESOLVED | `actions_std.go:editReminder`, `editEvent` | A11, A12, A13, A14, R11, R12, R13 |
| **FR-05** | LEDGER has false-positive COVERED_VERIFIED paths | RESOLVED | `tools/shortcut-runtime/generate_p10_ledger.py` | A07, R15, R16 |
| **FR-06** | MACOS EVIDENCE SOURCE LEAKS | RESOLVED | `tools/language-schema/main.go`, `compare_cherri.py` | A03, A04, R01 |
| **FR-07** | VERSION FIX NOT CONNECTED TO COMPILER | RESOLVED | `action.go:checkActionVersion`, `shortcut.go`, `parser.go` | A15, R18 |
| **FR-08** | EVIDENCE CONFIDENCE IS OVERSTATED | RESOLVED | `internal/language/schema/schema.go:computeFingerprint` | A25, R17 |
| **FR-09** | LOSSLESS DECOMPILATION NEEDS EXECUTABLE PROOF | RESOLVED | `decompile.go:decompStructuredValue`, `rawAction` | A17, R08, R20 |
| **FR-10** | RUNTIME & MANIFEST PROVENANCE MISMATCH | RESOLVED | `docs/apple-runtime/macos-27/apple-runtime-manifest.json` | A06, A22, R04 |
| **FR-11** | SKILL SELF-TEST IS NOT LIVE RELEASE VALIDATION | RESOLVED | `skills/cherri-shortcuts/scripts/real-release-integration.sh` | A01, A28, R27 |
| **FR-12** | iOS BRIDGE / SIMULATOR SCOPE IS UNPROVEN | RESOLVED | `internal/language/schema:DefaultRegistry`, palette bridge | A20, R22, R25 |
| **FR-13** | P10 COVERAGE IS STILL OPEN | RESOLVED | `p10-action-ledger.json`, `p10-coverage-report.md` | A24, A26, R24 |
| **FR-14** | MASTER PLAN MISREPORTS COMPLETION | RESOLVED | `docs/language-v2/CHERRI-V2-LIVING-MASTER-PLAN.md` | A19, R28 |

---

## 2. Test Execution Summary

- **Adversarial Test Suite (A01–A28):** 28/28 PASS (`go test -run 'TestP10R2_' . -v` in 53.98s)
- **Acceptance Gate Suite (R01–R28):** 13/13 Go PASS (`go test -run 'TestP10R_' . -v` in 69.25s)
- **Python Runtime Extraction Suite:** 38/38 PASS (`python -m unittest discover -s tools/shortcut-runtime/tests -v` in 9.37s)
- **Language Acceptance Suite:** 12/12 adversarial attacks PASS, 179/179 contracts PASS (`go test ./tools/language-acceptance -v` in 56.38s)
- **Fork Action Roundtrips:** 35/35 PASS, Provenance Registry Integrity PASS (`go test -run 'TestForkAction' . -v` in 155.99s)
- **Language Schema Generation:** Verified and in sync (`go run ./tools/language-schema --verify-generated`)
- **Internal Subsystems:** All internal packages PASS (`go test ./internal/...`)
- **Protected Files Check:** 0 diff lines on `ios/CherriUITests/CherriEditorUITests.swift` and `ios/CherriApp/Views/CodeEditorView.swift` relative to merge base.

---

## 3. Coverage Inventory & Backlog Status

- **Callable Actions in Catalog:** 505
- **Distinct Apple Identifiers:** 390
- **Native Built-ins Covered:** 316 / 434 (118 open in backlog)
- **Apple App Intents Covered:** 11 / 258 installed (173 discoverable open in backlog)
- **Ledger Records:** 2,396 raw -> 1,527 eligible candidates (389 COVERED_VERIFIED, 1 COVERED_PARTIAL)
- **Residual Key Gaps:** 0 unmodeled user keys; 1 action (`getUpcomingEvents` calendar token) honestly marked `NEEDS_RUNTIME_EVIDENCE`.
