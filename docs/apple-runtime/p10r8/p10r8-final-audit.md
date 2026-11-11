# Cherri P10R8 — Complete Native Fidelity & Dual Pipeline (Legacy + V2) Audit Report

**Date:** 2026-11-11  
**Branch:** `agent/cherri-p10r8-complete-native-fidelity-20261111`  
**Implementation Commit:** `5cdc4e97f813c84f9a040173899f555536c15bbf` (`compilerWorktreeDirty: false`)  
**Compiler Audit Binary SHA-256:** `0b34dd933087ebe0508f9d19c8945ebde602e07446a7b61f45562621e4f38d8b`  
**Corpus:** 38 real-world Apple Shortcuts (`001`–`038`), **27,652** total source actions  
**Independent Oracle:** `tools/p10r7/independent_fidelity_oracle.py` (`p10r8-fidelity-oracle-20261111.1`)  
**Audit Harness:** `tools/p10r7/run_38_audit.py` (`p10r8-audit-harness-20261111.1`)  
**Verification Mode:** `PRIVATE_CORPUS_VERIFIED_LOCALLY`

---

## 1. Executive Verdict

P10R8 completes lossless native wire fidelity across **both** compilation pipelines (**Legacy** `#workflowheader` + `#declare` + `rawAction(..., rawPlist(...))` and **Cherri v2** `native.workflow(native.plist(...), exact: true)` + `native.action(..., parameters: native.plist(...))`), while preserving existing semantic behavior for user-authored Cherri and `--import=`:

- **Zero Action Bloat / Zero Drops:** `27,652 -> 27,652` compiled actions (`totalActionDelta: 0`, `familyAdded: {}`, `familyRemoved: {}`).
- **38 / 38 Strict Action, Wire, Variable, Header & Third-Party Parity (Legacy + V2):** All 38 shortcuts pass `STRICT_ACTION_SEQUENCE_PASS`, `ACTION_PARAMETER_WIRE_PASS`, `VARIABLE_GRAPH_PASS`, `WORKFLOW_HEADER_PASS`, and `SOURCE_CONTAINER_EQUIVALENCE_PASS`, plus `11/11` applicable shortcuts passing `THIRD_PARTY_DESCRIPTOR_WIRE_PASS` (`27` `NOT_APPLICABLE`).
- **Genuine V2 Pipeline Without Legacy Fallback:** `V2_COMPILE_PASS` rose from `1/38` (P10R7) to **`38/38`** compiled through the real V2 frontend/lowering/backend path (`cherri import` defaults to V2 lossless; `cherri compile` executes V2 `ExactRawAction`), matching Legacy output 1:1.
- **Honest Control-Flow & Device Integrity (`37/38` Full Pass, `0/38` Device Run):** Only specimen `035` fails `CONTROL_FLOW_PASS` / `FULL_WORKFLOW_SEMANTIC_PASS` at `WFWorkflowActions[723].WFControlFlowMode` (`subcategory: SOURCE_CONTROL_FLOW_INVALID`, source container contains `CONTROL_FLOW_ORPHAN_END`), preserved verbatim 1:1 without inventing or dropping actions. `APPLE_RUNTIME_EXECUTED` remains truthfully `0/38` (`BLOCKED_DEVICE`).

---

## 2. P10R7 Baseline vs. P10R8 Final Matrix (`38` Specimens)

| Gate Name | P10R7 Final | P10R8 Final (Legacy) | P10R8 Final (V2) | Notes |
| --- | :---: | :---: | :---: | --- |
| `RAW_JSON_DECODE_PASS` | 38 / 38 | **38 / 38** | — | Raw JSON source decoded & normalized identically |
| `BPLIST_OR_XML_DECODE_PASS` | 38 / 38 | **38 / 38** | — | Binary/XML plists parsed cleanly |
| `DECOMPILE_PASS` | 38 / 38 | **38 / 38** | — | `--fidelity=lossless` on `--import=` and `cherri import` |
| `LEGACY_COMPILE_PASS` / `V2_COMPILE_PASS` | 38 / 38 / 1 / 38 | **38 / 38** | **38 / 38** | Independent `--legacy` & `cherri compile` V2 builds |
| `STRICT_ACTION_SEQUENCE_PASS` | 21 / 38 | **38 / 38** | **38 / 38** | `delta = 0` on all 38 workflows |
| `ACTION_PARAMETER_WIRE_PASS` | 0 / 38 | **38 / 38** | **38 / 38** | Exact nested dicts, arrays, booleans, ints/floats, `<data>`, empty `{}` vs absent |
| `VARIABLE_GRAPH_PASS` | 10 / 38 | **38 / 38** | **38 / 38** | Bijective producer UUIDs & exact preserved source dangling references |
| `CONTROL_FLOW_PASS` | 21 / 38 | **37 / 38** | **37 / 38** | `035` fails closed on source orphan `End If` (`WFWorkflowActions[723]`) |
| `WORKFLOW_HEADER_PASS` | 4 / 38 | **38 / 38** | **38 / 38** | Preserves client versions, icons, import questions, empty vs omitted arrays |
| `THIRD_PARTY_DESCRIPTOR_WIRE_PASS` | 2 / 11 (`27 N/A`) | **11 / 11** (`27 N/A`) | **11 / 11** | All third-party descriptors preserved at exact indices and wire types |
| `FULL_WORKFLOW_SEMANTIC_PASS` | 0 / 38 | **37 / 38** | **37 / 38** | Only `035` fails closed on `SOURCE_CONTROL_FLOW_INVALID` |
| `SOURCE_CONTAINER_EQUIVALENCE_PASS` | — | **38 / 38** | — | Strict equivalence across source JSON & plist containers |
| `APPLE_RUNTIME_EXECUTED` | 0 / 38 (`BLOCKED_DEVICE`) | **0 / 38 (`BLOCKED_DEVICE`)** | — | Physical iPadOS 27 device not attached |

---

## 3. Architectural Changes & Fidelity Safeguards

1. **Shared Lossless Container & Header Codec (`import_fidelity.go`):** Added canonical plist/JSON decoding (`decodeShortcutRawMap`), lossless XML-with-strict-roundtrip / `b64:` binary-plist literal encoding (`encodePlistValueToLiteral`, `decodeRawPlistValueString`), workflow header/action splitting, verbatim header merging (`mergeWorkflowHeaderMap`), and both Legacy & V2 lossless source renderers.
2. **Legacy & V2 Pipeline Support:**
   - **Legacy (`token.go`, `parser.go`, `action.go`, `raw_actions.go`, `output.go`, `compiler_state.go`):** Added `#workflowheader`, `#declare`, and `rawPlist(...)`; preserved present-empty `{}` vs absent `WFWorkflowActionParameters`; fixed `collectDictionary` empty `{}` return (`map[string]any{}`); isolated per-compile state cleanly in `resetCompilerState()` and `ExecuteImportPipeline` (`import.go`).
   - **V2 (`internal/language/ir/ir.go`, `internal/language/lower/lower.go`, `language_backend_adapter.go`, `language_backend_session.go`):** Added `native.workflow(..., exact: true)` and exact `native.plist(...)` / `ExactRawAction` lowering without parameter/header coercion, plus parser progress guard in `internal/language/syntax/parser.go`.
   - **CLI Routing (`args.go`, `main.go`, `import.go`, `language_cli_adapter.go`):** Added `--fidelity` (`lossless` vs `semantic`); `cherri import` defaults to `lossless` (V2 by default, `--mode legacy` supported), while legacy `--import=` defaults to `semantic` (`--fidelity=lossless` opt-in).
3. **Hardened Independent Oracle (`tools/p10r7/independent_fidelity_oracle.py`, `test_independent_oracle_mutations.py`):** Expanded negative mutation coverage (`M37`–`M60`, `test_preserved_source_dangling_reference_consistency` -> `72/72` passing mutation checks) and enforced consistency for source-present dangling `OutputUUID` references only when `VARIABLE_GRAPH_PASS` confirms 1:1 count and UUID match without a producer.
4. **Sanitized Regression Tests (`p10r8_fidelity_test.go`) & Ledger Sync (`docs/apple-runtime/p10/*`):** Verified XML/binary-plist/JSON parity, `b64:` binary fallback on XML-illegal control bytes, `#declare`, empty `{}` preservation, in-process isolation, and `55/56` local P10 release gates (`R22` `BLOCKED_PLATFORM` on non-macOS host; `0` failures).
