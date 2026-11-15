# P10R12 Baseline Reconciliation & Fallback Reason Accounting

**Generated:** `2026-11-15T05:11:58Z`  
**Baseline Start SHA:** `e8f3cd98e6a74f0c429edb80ae342b6d2b18812c` (P10R11)  
**Compiler Binary SHA-256:** `7407548dd6a768c86414ff1495a14d3cfb2ee669bd1e6bd72a08e59a01fe8bad`  
**No-Patch Oracle SHA-256:** `87b029d8c7db0845a3b343b9ab63a7fcd9175e82a50c634ad22ea7395870e911`  
**Denominator:** **27,652** total original actions across **38** normalized corpus shortcuts (tested in both `legacy` and `v2`, **76** total runs).

---

## 1. Truthful Progression Across Milestones (Per Dialect)

| Category | P10R9 | P10R10 | P10R11 (Final) | P10R12 (Measured) | P10R12 Share |
|---|---:|---:|---:|---:|---:|
| **`NATURAL_VERIFIED`** | 1,463 | 10,871 | 11,459 | **22,490** | **81.33%** |
| **`EXACT_NATIVE_FALLBACK`** | 26,188 | 16,780 | 16,192 | **5,161** | **18.66%** |
| **`SOURCE_INVALID`** (`035:723`) | 1 | 1 | 1 | **1** | **0.00%** |
| **Total Reconciled** | **27,652** | **27,652** | **27,652** | **27,652** | **100.00%** |

### Reconciling P10R11 (`11,459` Final vs `11,480` Preliminary) & P10R12 Lifecycle (`Gate A`)
- In P10R11, `TestP11_WIRE_01` cited `11,480` preliminary planner candidates while fresh per-specimen CLI runs produced `11,459` final verified natural actions (`-21` delta caused by un-reset candidate cache/state across sequential calls and unseparated candidate-vs-final tracking).
- In P10R12 (`Gate A`), `semantic_import_planner.go` separates explicit lifecycle transitions (`NOT_CANDIDATE`, `SOURCE_INVALID`, `CANDIDATE` -> `VERIFIED` / `DEMOTED`) and isolates CLI/test state (`TestP12_BASE_02`, `TestP12_BASE_02B`).
- Across all **76** independent runs (`38` Legacy + `38` V2), **`demotedActionCount = 0`** and unresolved **`CANDIDATE = 0`**: all **`22,490`** candidates promoted to `NATURAL_VERIFIED`, with `0` drift between fresh CLI runs and sequential `PlanHybridImport` loops.

---

## 2. Disjoint Fallback Reason Breakdown (Per Dialect)

| Reason Code | P10R11 Final | P10R12 Measured | Net Change | Notes |
|---|---:|---:|---:|---|
| `KNOWN_DISAMBIGUATED` (`NATURAL_VERIFIED`) | 11,459 | **22,490** | **+11,031** | Whole balanced control regions + typed complex parameters |
| `CONTROL_REGION` | 10,160 | **692** | **-9,468** | Individual markers in unsupported operator/menu variants |
| `UNREPRESENTABLE_PARAM` | 5,836 | **4,273** | **-1,563** | Remaining complex/nested/forward-ref parameter shapes |
| `THIRD_PARTY_NO_CANONICAL_DEF` | 137 | **137** | 0 | Preserved verbatim with 137/137 descriptor fidelity |
| `AMBIGUOUS_VARIANT` | 46 | **46** | 0 | Fail-closed disambiguation preserved |
| `UNKNOWN_NATIVE_ID` | 13 | **13** | 0 | Obsolete/unmodeled Apple identifiers preserved verbatim |
| `SOURCE_INVALID` | 1 | **1** | 0 | `035:723` orphan `End If` marker (`WFControlFlowMode=2`) |
| **Total Actions** | **27,652** | **27,652** | **0** | **`22,490 + 692 + 4,273 + 137 + 46 + 13 + 1 = 27,652`** |
