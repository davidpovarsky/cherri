# Handoff: Cherri P10R12 — Verified Control Regions & Typed Complex Parameters

**Date:** 2026-11-15  
**Branch:** `agent/cherri-p10r12-control-regions-complex-params-20261113`  
**Starting Verified HEAD (P10R11):** `e8f3cd98e6a74f0c429edb80ae342b6d2b18812c`  
**Compiler Binary SHA-256 (38×2 Oracle):** `7407548dd6a768c86414ff1495a14d3cfb2ee669bd1e6bd72a08e59a01fe8bad`  
**No-Patch Oracle Binary SHA-256 (`-tags cherri_oracle_nopatch`):** `87b029d8c7db0845a3b343b9ab63a7fcd9175e82a50c634ad22ea7395870e911`  
**Schema Fingerprint (`skills/cherri-shortcuts/compatibility-manifest.json`):** `f55851491e52ec9d06e486420f83d4faef0e31292d411235c01bf104bc48045d`  
**Execution Mode:** `NORMAL` (`[MAIN_SESSION]`)  
**Physical Device Runtime Status:** **`APPLE_RUNTIME_EXECUTED=BLOCKED_DEVICE`**

---

## 1. Summary of Verified Outcomes

- **Disjoint Corpus Action Accounting (`38` Normalized Plists, `27,652` Actions Per Dialect):**
  - **`NATURAL_VERIFIED`:** **`22,490`** (`81.33%`) — up **`+11,031`** from P10R11's final `11,459` (`41.44%`).
  - **`EXACT_NATIVE_FALLBACK`:** **`5,161`** (`18.66%`):
    - `UNREPRESENTABLE_PARAM`: `4,273` (down from `5,836` in P10R11)
    - `CONTROL_REGION`: `692` (down from `10,160` in P10R11, `-93.19%`)
    - `THIRD_PARTY_NO_CANONICAL_DEF`: `137` (`137/137` native descriptor parity in Legacy and V2)
    - `AMBIGUOUS_VARIANT`: `46`
    - `UNKNOWN_NATIVE_ID`: `13`
  - **`SOURCE_INVALID`:** **`1`** (`035:723` orphan `End If`, preserved losslessly).
- **Whole Balanced Control Regions (`4,071` Complete Regions Across Corpus):**
  - **`3,820 / 4,071` (`93.83%`)** promoted atomically as natural Cherri syntax:
    - `conditional`: `3,300` natural (`382` top, `2,918` nested) / `226` fallback / `1` mixed internal (`035`)
    - `choosefrommenu`: `163` natural (`2` top, `161` nested) / `24` fallback
    - `repeat.each`: `230 / 230` (`100%`)
    - `repeat.count`: `127 / 127` (`100%`)
  - **`0` unclosed regions**, **`0` orphan markers** (`035:723` preserved as `SOURCE_INVALID`).
- **Typed Complex Parameter Reverse Codecs:**
  - `46,110` complex shape observations promoted across `164` distinct action identifiers (`plain_dict`: `22,490`, `WFTextTokenAttachment`: `10,029`, `list`: `4,690`, `WFTextTokenString_with_attachments`: `4,418`, `Aggrandizements`: `4,337`, `WFDictionaryFieldValue`: `89`, `WFContentPredicateTableTemplate`: `55`, `serialized:WFNumberSubstitutableState`: `2`).
- **Independent 38 × 2 Oracle Verification (`76` Subprocess Runs):**
  - `Layer 1` Full Recursive Typed Plist Match: **`76 / 76 (100.0%)`**, `errorRunCount = 0`, `modeParityDiffs = []`.
  - `Layer 2` & `Layer 3` Pre-Overlay Identifiers & Semantic Params: **`22,490 / 22,490`** per dialect (`44,980 / 44,980` total).
  - `Layer 3` Post-Overlay Native Wire Exact: **`22,490 / 22,490`** per dialect (`44,980 / 44,980` total).
  - Stage-3 Demotions: **`0`** | Unresolved `CANDIDATE`s: **`0`** | Anti-Tautology Suite: **`9 / 9 PASS`** (`AT-01`..`AT-09`).
- **CLI vs `go test` Determinism & Forward-Ref Fix (`semantic_import_planner.go`, `TestP12_PAR_13`):**
  - Isolated `args.Args["import"]` inside `runDecompilePasses` and added `rejectUnseededForwardActionOutputCandidates` so unseeded forward `ActionOutput` refs (`027:1556 -> 1557`) fail closed at Stage 1 cleanly in both CLI and `go test`.
- **Canonical Action Schema Updates (`actions_std.go`):**
  - Updated `prompt` (`is.workflow.actions.ask`: `multiline` `String -> Bool`, `nil` default handling, typed `WFAskActionDefaultAnswer*` keys without spurious `WFAskActionDefaultAnswer` / `urlAnswer`) and `generateImage` (`RawString` style handling); regenerated schema (`internal/language/schema/generated_registry.go`) and consumer compatibility evidence (`docs/apple-runtime/p10/`, `skills/cherri-shortcuts/compatibility-manifest.json`).
- **Measured Corpus File Sizes:**
  - Legacy `.cherri`: **`80,348,473 B`** (down **`-6.38%`** from P10R11's `85,826,939 B`).
  - V2 `.cherri`: **`80,485,763 B`** (down **`-6.60%`** from P10R11's `86,177,871 B`).

---

## 2. Public Report & Evidence Files

- [baseline-and-reasons.md](../apple-runtime/p10r12/baseline-and-reasons.md) / `.json`
- [control-region-coverage.md](../apple-runtime/p10r12/control-region-coverage.md) / `.json`
- [complex-parameter-coverage.md](../apple-runtime/p10r12/complex-parameter-coverage.md) / `.json`
- [natural-action-parity.md](../apple-runtime/p10r12/natural-action-parity.md) / `.json`
- [edit-isolation-matrix.md](../apple-runtime/p10r12/edit-isolation-matrix.md) / `.json`
- [source-size-benchmark.md](../apple-runtime/p10r12/source-size-benchmark.md) / `.json`
- [final-audit.md](../apple-runtime/p10r12/final-audit.md) / `.json`

---

## 3. Remaining Blockers & Safe Next Steps

**Implementation Commit (`<IMPL_SHA>`):** `bbb83df02e0707a222f49c745dcd04b136df7818`  
- **macOS CI Capacity & Physical Hardware Runtime:** Remote GitHub Actions macOS execution and physical iOS/iPadOS Shortcuts import/run validation remain external release blockers (`APPLE_RUNTIME_EXECUTED=BLOCKED_DEVICE`).
- **Provenance & Asana Closure State:** Recorded non-circular `firstForkCommit` entries (`prompt`, `generateImage`, and P10R12 milestone) in `docs/fork-action-provenance.json` and synced P10 ledgers (`docs/apple-runtime/p10/*`) in the dedicated provenance follow-up commit immediately prior to push, GitHub Actions verification, and live Asana sync.
