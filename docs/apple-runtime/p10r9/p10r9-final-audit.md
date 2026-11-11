# Cherri P10R9 — Unified Engine, Hybrid Readable Import & Durable Project Governance Final Audit

**Date:** 2026-11-11  
**Branch:** `agent/cherri-p10r9-unified-engine-hybrid-import-20261111`  
**Base Commit (P10R8):** `20c5c750b4c147727ab7ff59c4177b54a8461188`  
**Implementation Commit:** `b91c1ca0834a6799213ac6829d36e61e9c8075e5`  
**Corpus:** 38 real-world Apple Shortcuts (`001`–`038`), **27,652** total source actions  
**Independent Oracle:** `tools/p10r7/independent_fidelity_oracle.py` (`p10r8-fidelity-oracle-20261111.1`)  
**Execution Mode:** `NORMAL` (`[MAIN_SESSION]`)

---

## 1. Executive Verdict

P10R9 consolidates **Legacy Cherri** and **Cherri v2** onto a single shared forward-and-reverse engine and introduces **Hybrid Readable Import (`--fidelity=hybrid`)** across both language modes without regressing any P10R8 lossless guarantee:

- **One Authoritative Action Source & 100% Surface Parity:** All `505` catalog entries (`497` callable actions + `8` compiler constructs) derive from `actions/*.cherri` + `actions_std.go` via `EnsureCanonicalStandardActions()`, with `100%` parity verified across `--actions-json`, `schema.DefaultRegistry()`, `currentMobileActionCatalog()` (`CherriActionCatalog`), and `DefaultReverseActionRegistry()` (`496` standard actions across `381` unique Apple identifiers: `303` single-variant, `78` multi-variant).
- **38 / 38 Lossless & Hybrid Import + Unsigned Compile Parity (Legacy + V2):** All `38` real-world corpus shortcuts (`27,652` actions) pass both `lossless` and `hybrid` import and unsigned compilation in both `legacy` and `v2` modes, achieving `100%` full decoded plist tree equality (`plistValuesStrictlyEqual`) and passing all wire gates in `independent_fidelity_oracle.py`.
- **1,463 Verified Natural Action Emissions Across 28 Callables:** Across the 38-shortcut corpus, `PlanHybridImport()` verified and emitted `1,463` natural Cherri calls (`comment`, `stop`, `text`, `alert`, `speak`, `dismissSiri`, `wait`, `waitToReturn`, `vibrate`, `show`, `showWebpage`, `createFolder`, `nothing`, `setBrightness`, `setVolume`, `returnToHomescreen`, `showNotification`, `toggleAppearance`, `define`, `lightMode`, `output`, `reboot`, `setAirdropReceiving`, `shutdown`, `confirm`, `toggleBluetooth`, `toggleSilenceUnknownCallers`, `toggleWifi`) while falling back per-action to `rawAction(...)` / `native.action(...)` on `26,188` complex/control-region actions and `1` source-invalid action (`035` index `723` `CONTROL_FLOW_ORPHAN_END`).
- **30 / 30 Adversarial Hybrid Tests (`HYB01`–`HYB30`) + Dual SSoT Propagation + 1,000 Sequential Mixed Compiles:** All 30 mandatory adversarial tests, both declarative and Go-handler SSoT propagation scenarios, and 1,000 sequential mixed compiles pass with zero state leakage.

---

## 2. P10R8 Baseline vs. P10R9 Final Matrix (`38` Specimens, `27,652` Actions)

| Metric / Verification Gate | P10R8 Baseline | P10R9 Final (Lossless) | P10R9 Final (Hybrid) |
| :--- | :---: | :---: | :---: |
| **Legacy Import + Compile Pass** | `38 / 38` | **`38 / 38`** | **`38 / 38`** |
| **V2 Import + Compile Pass** | `38 / 38` | **`38 / 38`** | **`38 / 38`** |
| **Legacy vs. V2 Decoded Plist Tree Equality** | `38 / 38` | **`38 / 38`** | **`38 / 38`** |
| **`STRICT_ACTION_SEQUENCE_PASS`** | `38 / 38` (`27,652`) | **`38 / 38` (`27,652`)** | **`38 / 38` (`27,652`)** |
| **`ACTION_PARAMETER_WIRE_PASS`** | `38 / 38` | **`38 / 38`** | **`38 / 38`** |
| **`VARIABLE_GRAPH_PASS`** | `38 / 38` | **`38 / 38`** | **`38 / 38`** |
| **`WORKFLOW_HEADER_PASS`** | `38 / 38` | **`38 / 38`** | **`38 / 38`** |
| **`THIRD_PARTY_DESCRIPTOR_WIRE_PASS`** | `11 / 11` (`27 N/A`) | **`11 / 11` (`27 N/A`)** | **`11 / 11` (`27 N/A`)** |
| **`CONTROL_FLOW_PASS`** | `37 / 38` (`035` invalid) | **`37 / 38` (`035` invalid)** | **`37 / 38` (`035` invalid)** |
| **Verified Natural Action Calls (`NATURAL_VERIFIED`)** | `0` (`0.00%`) | `0` (`0.00%`) | **`1,463` (`5.29%`, `28` callables)** |
| **Exact Per-Action Native Fallbacks (`EXACT_NATIVE`)** | `27,652` | `27,652` | **`26,188` + `1` `SOURCE_INVALID`** |
| **`APPLE_RUNTIME_EXECUTED`** | `0 / 38` (`BLOCKED_DEVICE`) | **`0 / 38` (`BLOCKED_DEVICE`)** | **`0 / 38` (`BLOCKED_DEVICE`)** |

---

## 3. Benchmarks on Representative Specimens (`007`, `012`, `027`, `032`, `035`)

| Specimen | Actions | Hybrid Legacy Import (ms) | Hybrid Legacy Compile (ms) | Hybrid V2 Import (ms) | Hybrid V2 Compile (ms) | Legacy Source Bytes | V2 Source Bytes |
| :--- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| `007` | 10 | 157.38 | 104.37 | 43.36 | 31.51 | 538,357 | 538,585 |
| `012` | 2,474 | 1,111.03 | 596.90 | 298.09 | 207.51 | 1,908,311 | 1,955,739 |
| `027` | 2,398 | 985.68 | 548.61 | 322.29 | 249.42 | 1,755,138 | 1,800,970 |
| `032` | 7 | 125.14 | 83.82 | 26.51 | 20.09 | 333,023 | 333,151 |
| `035` | 2,155 | 2,868.91 | 2,070.12 | 693.31 | 513.02 | 7,381,822 | 7,423,338 |

---

## 4. Adversarial Suite (`HYB01`–`HYB30`) & Invariant Proofs

- **`TestHYB01`–`TestHYB30` (`p10r9_hybrid_test.go`):** 30/30 PASS covering natural emission, category `#include` synthesis, unknown parameter key fallback, `UUID` / `CustomOutputName` fallback, multi-variant disambiguation and ambiguity fallback, deterministic reverse matching across 100 iterations, AppIntent descriptor verification, third-party fallback, control-flow validity and orphan end (`035:723`) preservation, dangling reference preservation, empty vs omitted `{}` parameters, float vs int64 vs bool distinction, `<data>` base64 round-trip, XML-unsafe control char (`\x0b`) binary plist fallback, Hebrew/RTL/ZWJ/emoji preservation, malformed container rejection (`HYB18`–`HYB21`), `rawPlist` payload rejection (`HYB22`–`HYB25`), isolated compile probe safety (`HYB26`), Legacy vs V2 hybrid plist identity (`HYB27`), semantic V2 fail-closed validation (`HYB28`), explicit mobile mode routing (`HYB29`), and `035` 2,155-action hybrid parity (`HYB30`).
- **`TestP10R9_SSoTPropagation_DeclarativeAndGoHandler`:** Proves in an isolated temporary copy of the repository that adding both (A) a declarative action in `actions/basic.cherri` and (B) a Go-handler action in `actions_std.go` + running `go run ./tools/language-schema` automatically updates `--actions-json`, `schema.DefaultRegistry()`, `currentMobileActionCatalog()`, `DefaultReverseActionRegistry()`, forward Legacy/V2 compilation, and `--fidelity=hybrid` import in both Legacy and V2 without touching any other file.
- **`TestP10R9_SequentialMixedCompilesIsolation`:** Executes 1,000 sequential in-process compilations mixing Legacy semantic, V2 semantic, Legacy lossless, V2 lossless, Legacy hybrid, V2 hybrid, and intentional compile failures, verifying identical SHA-256 outputs and clean `compiler_state.go` resets after every iteration.
