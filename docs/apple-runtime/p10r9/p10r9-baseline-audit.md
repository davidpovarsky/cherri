# P10R9 Phase 0 — Baseline Audit & Corpus Action Taxonomy

**Date:** 2026-11-11  
**Base Commit (P10R8):** `bbf2fc922ffb7e810d862e3f6ab94b9b871537c4` (`bbf2fc922ffb7e810d862e3f6ab94b9b871577c4`)  
**Upstream Merge Base:** `951c0bb3c34ef4e3d6cb2ce9a1bff35071c9a7a2`  
**Baseline Binary SHA-256:** `c47951dfebd79c591f6a0511ad4cf055b9510c512372261ebeefe8eeec28c91f`  
**Execution Mode:** `[איתן | Architecture] [MAIN_SESSION]` / `[רוני | QA] [MAIN_SESSION]`

---

## 1. Executive Summary

Phase 0 establishes the deterministic pre-modification baseline before introducing the P10R9 shared `NativeShortcutDocument` / `NativeActionRecord` model, deterministic reverse action registry, and shared hybrid readable import planner (`--fidelity=hybrid`).

1. **Lossless Baseline Invariant (`38/38`):**
   - All `38` real-world corpus specimens (`001`–`038`, totaling `27,652` native Shortcut actions) pass lossless import and unsigned compilation in both **Legacy** (`--import=... --import-fidelity=lossless --mode=legacy`) and **V2** (`cherri import --fidelity=lossless --mode=v2`) modes.
   - All `38/38` compiled outputs pass the independent Python `plistlib` structural equality oracle (`tools/p10r7/independent_fidelity_oracle.py`).
   - `37/38` specimens have valid control-flow block pairing (`WFControlFlowMode` 0/1/2 and `GroupingIdentifier`); specimen `035` contains one source-side defect (`CONTROL_FLOW_ORPHAN_END` at action index `723` out of `1,858` actions), which is preserved verbatim in lossless mode.
2. **Canonical Action Registry Baseline (`505` Actions):**
   - `cherri --actions-json` exposes `505` canonical definitions derived from the single shared source of truth (`actions/*.cherri` + `actions_std.go`).
3. **Pre-Hybrid Corpus Action Taxonomy (`27,652` Actions):**
   - `KNOWN_DISAMBIGUATED`: `1,589` actions (`5.75%`) — unique canonical definition with pure literal parameters and no producer `UUID`, `CustomOutputName`, or `GroupingIdentifier`.
   - `AMBIGUOUS_VARIANT`: `229` actions (`0.83%`) — multiple canonical Cherri overloads map to the same Apple `WFWorkflowActionIdentifier` and require static discriminator + parameter-key disambiguation plus strict re-emission verification.
   - `UNREPRESENTABLE_PARAM`: `15,113` actions (`54.65%`) — actions carrying producer `UUID`s, `CustomOutputName`s, magic variable tokens (`WFTextTokenAttachment` / `WFTextTokenString`), or complex serialized parameter payloads that require exact native fallback to guarantee bit-for-bit parity.
   - `CONTROL_REGION`: `10,570` actions (`38.22%`) — control-flow block markers (`conditional`, `choosefrommenu`, `repeat.count`, `repeat.each`) carrying `WFControlFlowMode` and `GroupingIdentifier`.
   - `THIRD_PARTY_NO_CANONICAL_DEF`: `137` actions (`0.50%`) — third-party app intents outside the built-in catalog.
   - `UNKNOWN_NATIVE_ID`: `13` actions (`0.05%`) — uncatalogued native `is.workflow.actions.*` identifiers.
   - `SOURCE_INVALID`: `1` action (`0.004%`) — specimen `035` action index `723` (`CONTROL_FLOW_ORPHAN_END`).

---

## 2. Baseline Import & Compile Performance Benchmarks

Measured on the unmodified P10R8 baseline binary across the five largest/most complex corpus specimens (`007`, `012`, `027`, `032`, `035`):

| Specimen | Actions | Mode | Import Time (ms) | Compile Time (ms) | Imported Source (bytes) | Compiled XML `.plist` (bytes) | Compiled `.shortcut` (bytes) |
| :--- | ---: | :--- | ---: | ---: | ---: | ---: | ---: |
| `007` | 2,163 | Legacy Lossless | 77.15 | 153.37 | 538,357 | 537,299 | 531,945 |
| `007` | 2,163 | V2 Lossless | 58.69 | 68.92 | 538,585 | 539,046 | 531,945 |
| `012` | 3,841 | Legacy Lossless | 312.22 | 657.40 | 1,936,932 | 1,672,113 | 405,698 |
| `012` | 3,841 | V2 Lossless | 274.65 | 305.24 | 1,986,440 | 2,272,759 | 405,698 |
| `027` | 3,730 | Legacy Lossless | 257.11 | 508.15 | 1,793,757 | 1,536,823 | 328,270 |
| `027` | 3,730 | V2 Lossless | 242.93 | 261.62 | 1,841,745 | 1,981,220 | 328,270 |
| `032` | 620 | Legacy Lossless | 52.33 | 117.90 | 333,652 | 327,899 | 640,185 |
| `032` | 620 | V2 Lossless | 48.65 | 55.49 | 333,820 | 329,004 | 640,185 |
| `035` | 1,858 | Legacy Lossless | 573.87 | 2,258.52 | 7,404,728 | 7,174,058 | 5,892,051 |
| `035` | 1,858 | V2 Lossless | 603.09 | 630.86 | 7,447,856 | 7,910,481 | 5,892,051 |

---

## 3. Baseline Findings & Architectural Observations

1. **Shared Lossless Decode Need (`import_fidelity.go`):**
   - P10R8 introduced `DecodeLosslessImportInput`, `RenderLosslessLegacySource`, and `RenderLosslessV2Source`, which must now be unified around a richer `NativeShortcutDocument` and `NativeActionRecord` model that also performs strict JSON/plist schema validation (`HYB18`–`HYB21`), control-flow stack analysis, and UUID producer/consumer tracking.
2. **Non-Deterministic Map Iteration in `decompile.go:matchAction`:**
   - `decompile.go:matchAction` iterates over the `actions` Go map without sorting keys, which can produce non-deterministic overload selection when multiple Cherri definitions share the same Apple identifier (`HYB07`). P10R9 replaces this with deterministic sorted key iteration and introduces `reverse_action_registry.go` for deterministic reverse candidate lookup.
3. **0-Argument Action Wire Parity (`mergeWorkflowHeaderMap`):**
   - In Apple Shortcuts, 0-parameter actions (`is.workflow.actions.exit`, `dismisssiri`, `waittoreturn`, `vibrate`, `nothing`) carry `WFWorkflowActionParameters: {}` in the native plist. In P10R8, `mergeWorkflowHeaderMap` only emitted `WFWorkflowActionParameters: {}` when `exactActionParamsPresence[i]` was explicitly set by `rawAction`/`native.action`. Extending `exactActionParamsPresence` to track explicit omission on raw actions while emitting `WFWorkflowActionParameters: {}` for natural 0-argument calls enables bit-for-bit hybrid promotion for 0-argument actions (`stop()`, `vibrate()`, `waitToReturn()`, `dismissSiri()`, `nothing()`), verified by strict candidate re-compilation.
4. **Machine-Readable Artifact:**
   - Full sanitized per-specimen baseline metrics and action taxonomy counts are recorded in `docs/apple-runtime/p10r9/p10r9-baseline-matrix.json`.
