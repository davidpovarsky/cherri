# P10R11 Pre-Overlay Action Parity Matrix

## Executive Summary

- **Total Corpus Size:** 38 private Shortcut specimens (27,652 actions total).
- **Natural Verified Actions:** `10,871 / 27,652 (39.31%)` in both Legacy and Cherri v2 dialects.
- **Pre-Overlay Semantic Wire Parity:** `10,871 / 10,871 (100.0%)` — all functional parameters, types, condition codes, control modes, and nested values match exactly before import metadata restoration.
- **Post-Overlay Native Wire Parity:** `10,871 / 10,871 (100.0%)` — full byte-for-byte and structural plist match after verified metadata restoration.
- **Compiler Binary SHA-256:** `875c297660e1eeb8ce7a46c8755957f78a4be5434be9440ad8e151a88220136d`

---

## 1. Metadata Taxonomy & Key Ownership

In P10R11, the legacy heuristic `strings.HasPrefix(k, "Show")` was permanently eliminated and replaced by an explicit, verified key ownership allowlist (`verifiedImportMetadataKeys`):

| Category | Keys | Behavior / Rationale |
|---|---|---|
| **Identity Keys** | `UUID`, `CustomOutputName`, `GroupingIdentifier` | Graph-sensitive identity linkage. Restored only via verified multi-pass semantic hash match. |
| **Presentation / UI Keys** | `ShowHeaders`, `ShowWhenRun`, `Show-WFInput`, `Show-WFURLActionURL`, `Show-text` | Strictly enumerated Apple UI presentation fields. Non-functional for action semantics. |
| **Import Presence Keys** | `WFSerializationType` | Preserved in import mode when absent in legacy sources (2,487 `setVariable`, 64 `appendVariable`). |
| **Functional / Semantic Keys** | `WFInput`, `WFURL`, `WFHTTPHeaders`, `WFJSONValues`, `WFTextActionText`, `WFCondition`, `WFControlFlowMode`, etc. | Must be exact pre-overlay for natural actions. Cannot be restored by metadata overlay. |
| **Disallowed Unverified Keys** | `ShowSecurity`, `ShowSecret`, `ShowCommand`, `WFPrivateFutureParam` | **Rejected fail-closed**: forces fallback to native raw action. Never hidden in provenance. |

---

## 2. Key Action Wire Fixes

### A. Case `014:208` — `fileRequest` & `ShowHeaders`
- In P10R10, `fileRequest` omitted `ShowHeaders` during pre-overlay compile and relied on `Show*` prefix heuristic restore.
- In P10R11, `showHeaders` was added as a true canonical optional parameter in `actions/web.cherri` across `downloadURL`, `formRequest`, `jsonRequest`, and `fileRequest`.
- Pre-overlay compilation of `fileRequest(..., showHeaders: true)` natively produces `ShowHeaders: true`.
- User edits to `showHeaders` (e.g. from `true` to `false` or vice versa) compile natively and are never overwritten by stale provenance.

### B. `getValue` (`is.workflow.actions.getvalueforkey`)
- In P10R10, `getValue` hardcoded `"WFGetDictionaryValueType": "Value"`, causing discrepancies when original plists omitted this default.
- In P10R11, `actions/basic.cherri` was updated to make `?get: 'WFGetDictionaryValueType'` an optional parameter.
- When omitted in source, `getValue` does not emit the redundant key, matching Apple's native wire pre-overlay.

### C. `WFSerializationType` Variance in Legacy Variables
- In the original corpus, 2,487 `setVariable` actions and 64 `appendVariable` actions omit `WFSerializationType`.
- Cherri import mode preserves this exact absence without breaking normal authored code behavior.
- Normal authored Legacy code continues to emit valid serialization types, maintaining 100% upstream pure compatibility.
