# Cherri P10R7 — Final Semantic Fidelity & Independent Oracle Audit Report

**Date:** 2026-11-11  
**Branch:** `agent/cherri-p10r7-verified-semantic-fidelity-20261110`  
**Compiler Audit Binary SHA-256:** `316e4bc858c2daddc658dd95fa645f3fceeab83d0d2ce5da45ed02916b0c2e2c`  
**Corpus:** 38 real-world Apple Shortcuts (`001`–`038`), **27,652** total source actions  
**Independent Oracle:** `tools/p10r7/independent_fidelity_oracle.py` (`oracleVersion`: `p10r7-independent-oracle-20261110.1`)

---

## 1. Executive Verdict

P10R7 replaced the broken tautological P10R6 sequence/semantic pass logic with a strict, non-tautological 13-gate independent oracle, closed six real compiler/decompiler fidelity defects verified by permanent regression tests, and reconciled all P10R6 and P10R7 ledger totals across private and sanitized artifacts:

- **No Crash / Full Decode & Legacy Compilation Maintained:** `38/38` raw JSON/plist decode, `38/38` decompile, `38/38` `--legacy` compile.
- **Action Bloat Eliminated (`+6,703 -> +6`, `-99.91%`):** Net action delta dropped from **`+6,703`** (`27,652 -> 34,355`) to **`+6`** (`27,652 -> 27,658`).
- **Strict Sequence Parity Raised `3/38 -> 21/38`:** `21` of `38` real-world workflows now match source action counts and `WFWorkflowActionIdentifier` sequences 1:1 (`CONTROL_FLOW_PASS` likewise rose `3/38 -> 21/38`).
- **Honest Non-Tautological Reporting (`0/38` Full Wire Parity, `BLOCKED_DEVICE` Hardware Gate):** Even on the `21` sequence-matched workflows, the independent strict oracle honestly flags residual parameter-wire differences (`ACTION_PARAMETER_WIRE_PASS: 0/38`), variable graph identifier differences (`VARIABLE_GRAPH_PASS: 10/38`), header differences (`WORKFLOW_HEADER_PASS: 4/38`), third-party sequence shifts (`THIRD_PARTY_DESCRIPTOR_WIRE_PASS: 2/11 PASS, 9/11 FAIL, 27 NOT_APPLICABLE`), and `FULL_WORKFLOW_SEMANTIC_PASS: 0/38`. `V2_COMPILE_PASS` remains separated (`1/38`, specimen `032`). `APPLE_RUNTIME_EXECUTED` remains truthfully `0/38` (`BLOCKED_DEVICE`).

---

## 2. Final 13-Gate Matrix Summary (`38` Specimens)

| Gate Name | P10R6 Baseline (Reconciled) | P10R7 Final | Honest Interpretation |
| --- | :---: | :---: | --- |
| `RAW_JSON_DECODE_PASS` | 38 / 38 | **38 / 38** | All source containers decoded cleanly |
| `BPLIST_OR_XML_DECODE_PASS` | 38 / 38 | **38 / 38** | All binary/XML plists parsed cleanly |
| `DECOMPILE_PASS` | 38 / 38 | **38 / 38** | All 38 `.cherri` sources generated |
| `LEGACY_COMPILE_PASS` | 38 / 38 | **38 / 38** | All 38 compile under `cherri --legacy --skip-sign` |
| `V2_COMPILE_PASS` | 1 / 38 (`032`) | **1 / 38 (`032`)** | Isolated from legacy counts; v2 covers Phase 1–5 subsets |
| `STRICT_ACTION_SEQUENCE_PASS` | 3 / 38 | **21 / 38** | Exact length & `WFWorkflowActionIdentifier` 1:1 parity (`+18`) |
| `CONTROL_FLOW_PASS` | 3 / 38 | **21 / 38** | Exact `WFControlFlowMode` & bijective `GroupingIdentifier` (`+18`) |
| `VARIABLE_GRAPH_PASS` | — | **10 / 38** | Fixed `019` (`rawAction` `{...}` token braces) |
| `WORKFLOW_HEADER_PASS` | — | **4 / 38** | `002, 010, 019, 038`; `34` fail on version/content-class/question headers |
| `THIRD_PARTY_DESCRIPTOR_WIRE_PASS` | — (coarse check only) | **2 / 11** (`27 N/A`) | All `164` descriptors counted 1:1; `004, 023` pass wire check, `9` fail sequence index shift |
| `ACTION_PARAMETER_WIRE_PASS` | 0 / 38 | **0 / 38** | Strict parameter equality catches `CustomOutputName`, `OutputName` sanitization, default keys, aggrandizements |
| `FULL_WORKFLOW_SEMANTIC_PASS` | 0 / 38 | **0 / 38** | Requires all sequence, wire, variable, control-flow, header, and descriptor gates |
| `APPLE_RUNTIME_EXECUTED` | 0 / 38 (`BLOCKED_DEVICE`) | **0 / 38 (`BLOCKED_DEVICE`)** | Physical iPadOS 27 hardware not attached |

---

## 3. Root-Cause Compiler, Parser & Decompiler Fixes Verified

1. **`rawAction` `WFTextTokenString` Brace Preservation (`decompile.go`, `compiler_state.go`):** Added `decompilingRawTextToken` flag (reset cleanly in `resetCompilerStateFully`) so `WFTextTokenString` dictionaries inside `rawAction(...)` keep `{Ref}` attachment syntax verbatim instead of double-escaping.
2. **No-Input `setvariable` Fidelity (`decompile.go`, `parser.go`):** Emits `rawAction("is.workflow.actions.setvariable", { "WFVariableName": "..." })` when `WFInput == nil`, eliminating `@var = nil` + synthetic `nothing()` pairs while keeping variable registration in `parser.go`.
3. **Menu `{var}` Interpolation & Shared Control Helpers (`shortcutgen.go`, `shared_control.go`, `internal/language/backend/control.go`):** Preserves interpolated tokens inside `WFMenuItems` and `WFMenuItemAttributedTitle` as real `WFTextTokenString` dicts via `menuItemValueAndTitle` + shared `BuildMenuStartItems` / `BuildMenuCaseWithTitle`.
4. **Parser Leading-Newline & Basic Include Fixes (`parser.go`, `actions_std.go`):** Reset `char = 0` in `firstChar()` so leading `\n` lines never corrupt line-index tracking on pass 2 (`include` duplication crash fixed), and avoid emitting `#include <actions/basic>`.
5. **`IntentAppDefinition` SSoT Isolation (`raw_actions.go`, `language_backend_adapter.go`, `decompile.go`):** Preserves plain `IntentAppDefinition` dictionaries in `rawAction` (legacy & v2) without altering `action.go`'s `collectAdditionalParams()` (`schemaFingerprint` unchanged at `787e9842...`), and uses `plistCanonicalEqual` so `call` / `facetimeCall` keep `505/505` native round-trip parity.
6. **Control-Flow Bloat Removal & Safe Subscript/Aggrandizement Guards (`parser.go`, `shortcutgen.go`, `shortcut.go`):** Removed unconditional empty-branch `addNothing()` injection (`collectEndStatement`, `collectMenuItem`), added `#define inputs none` (`__NONE__`), guarded `isDictAggrandizement`, and added `omitempty` on optional workflow header slices/maps.

---

## 4. Oracle & Reconciliation Anti-Tamper Verification

1. **Independent Oracle Negative Mutation Suite (`tools/p10r7/test_independent_oracle_mutations.py`):** **`37 / 37` PASS** (`test_baseline_passes` + `M01`–`M36`, including dropped/reordered actions, value/type mutations, broken UUID graphs, malformed control-flow nesting, corrupted workflow headers, wrapped/dropped AppIntent descriptors, M31–M33 stale/reconciliation/provenance checks, M34–M35 lowered-action accountability, and M36 `BLOCKED_DEVICE` integrity).
2. **Private <-> Sanitized Cross-Check & Tamper Guard (`tools/p10r7/reconcile_private_and_sanitized_reports.py`):** Recomputed `p10r7-sanitized-diff-matrix.json`, corrected scrambled rows in `p10r7-baseline-reconciliation.md`, reconciled P10R6 prose/matrix totals (`27,652` source, `34,355` compiled, `3/38` sequence, `0/38` semantic, fixing `018`/`021` `3,383->3,381` and removing `28,103`/`35,016` errors), verified 1:1 private <-> sanitized hash/count/gate parity, and confirmed 6/6 tamper mutations fail closed.
3. **Privacy Guarantee:** Private shortcuts (`001`–`038`), raw plist values, URLs, and personal tokens remain outside version control in `cherri-shortcut-audit-20261110`; sanitized repository JSON/Markdown files contain only hashes, counts, action identifiers, header/parameter key names, and anonymized failure categories.

---

## 5. Honest Residual Roadmap After P10R7

To close the remaining gap from `21/38` strict sequence parity and `0/38` parameter-wire parity toward `38/38` full semantic fidelity:
1. Replace runtime `@Var = nil` (`uninitVars`) `nothing + setvariable` injection (`+46 setvariable`, `+44 net nothing`) with symbol-table forward declaration or scoped `rawAction` variable references.
2. Disable unsafe `getvalueforkey -> dict['k']` folding (`-83`) when downstream references or action indices depend on the standalone `getvalueforkey` action.
3. Preserve `is.workflow.actions.calculateexpression` (`-11`) natively without lossy translation to `math` (`+11`).
4. Preserve original Apple `CustomOutputName` absence / verbatim display names (`OutputName`) and exact header metadata (`WFWorkflowMinimumClientVersion`, empty `WFWorkflowOutputContentItemClasses: []` presence, `WFWorkflowImportQuestions`).
5. Validate real `.shortcut` execution on attached physical iPadOS 27 hardware once available (`APPLE_RUNTIME_EXECUTED`).
