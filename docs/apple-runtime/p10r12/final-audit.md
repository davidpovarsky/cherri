# P10R12 Verified Control Regions & Typed Complex Parameters — Final Audit

**Audited:** `2026-11-15T05:11:58Z`  
**Baseline Start SHA (P10R11):** `e8f3cd98e6a74f0c429edb80ae342b6d2b18812c`  
**Compiler Binary SHA-256:** `7407548dd6a768c86414ff1495a14d3cfb2ee669bd1e6bd72a08e59a01fe8bad`  
**No-Patch Oracle SHA-256:** `87b029d8c7db0845a3b343b9ab63a7fcd9175e82a50c634ad22ea7395870e911`  
**Schema Fingerprint (`skills/cherri-shortcuts/compatibility-manifest.json`):** `f55851491e52ec9d06e486420f83d4faef0e31292d411235c01bf104bc48045d`  
**Physical Apple Hardware:** **`APPLE_RUNTIME_EXECUTED=BLOCKED_DEVICE`** (no physical iOS/iPadOS execution claimed)

---

## 1. Executive Summary

- **Disjoint Corpus Accounting (Per Dialect, Denominator = `27,652`):**
  - **`NATURAL_VERIFIED`:** **`22,490`** (`81.33%` — up **`+11,031`** from P10R11's `11,459` / `41.44%`)
  - **`EXACT_NATIVE_FALLBACK`:** **`5,161`** (`18.66%` — `UNREPRESENTABLE_PARAM`: `4,273`, `CONTROL_REGION`: `692`, `THIRD_PARTY_NO_CANONICAL_DEF`: `137`, `AMBIGUOUS_VARIANT`: `46`, `UNKNOWN_NATIVE_ID`: `13`)
  - **`SOURCE_INVALID`:** **`1`** (`035:723` orphan `End If`, preserved losslessly)
- **Whole Balanced Control Regions:** **`3,820 / 4,071`** (`93.83%`) natural across `conditional` (`3,300`), `choosefrommenu` (`163`), `repeat.each` (`230/230`), and `repeat.count` (`127/127`), with **`0` unclosed regions** and **`0` orphan markers**.
- **Complex Typed Parameters:** `46,110` complex shape observations across `164` distinct promoted natural action IDs (`WFTextTokenAttachment`: `10,029`, `list`: `4,690`, `WFTextTokenString_with_attachments`: `4,418`, `Aggrandizements`: `4,337`, `WFDictionaryFieldValue`: `89`, `WFContentPredicateTableTemplate`: `55`).
- **38 × 2 Independent Multi-Layer Oracle Verification:**
  - `Layer 1` Full recursive typed plist match: **`76 / 76 (100.0%)`** (`0` errors, `0` Legacy/V2 mode parity diffs)
  - `Layer 2` Pre-overlay action IDs: **`44,980 / 44,980 (100.0%)`**
  - `Layer 3` Pre-overlay semantic params (before provenance restore): **`44,980 / 44,980 (100.0%)`**
  - `Layer 3` Post-overlay native wire: **`44,980 / 44,980 (100.0%)`**
  - Stage-3 demotions: **`0`** | Unresolved candidates: **`0`** | Anti-tautology checks: **`9 / 9 PASS`**
- **P10R12 Go Regression Tests Discovered:** **`55`** tests across ``p12_baseline_truth_test.go` (12), `p12_complex_params_test.go` (17), `p12_control_regions_test.go` (26)`, plus existing `TestP11_WIRE_01..50` (`50`), `TestSEM*`, `TestHYB*`, and SSoT gates.
- **CLI vs `go test` Determinism & Forward-Ref Fix:** Isolated `args.Args["import"]` inside `runDecompilePasses` (`semantic_import_planner.go`) and added `rejectUnseededForwardActionOutputCandidates` so unseeded forward `ActionOutput` consumers (such as `027:1556 -> 1557`) fail closed cleanly at Stage 1 without false candidates or CLI/`go test` divergence (`TestP12_PAR_13`).
- **Source Footprint Reduction:** Legacy `.cherri` total across 38 shortcuts dropped from `85.83 MB` (P10R11) to **`80.35 MB`** (`-6.38%`), and Cherri v2 `.cherri` dropped from `86.18 MB` to **`80.49 MB`** (`-6.60%`).
