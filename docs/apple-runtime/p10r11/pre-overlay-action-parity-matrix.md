# P10R11 Pre-Overlay Action Parity Matrix & Semantic Truth Ledger

**Audit Date:** 2026-11-12  
**Compiler Binary SHA-256:** `875c297660e1eeb8ce7a46c8755957f78a4be5434be9440ad8e151a88220136d`  
**No-Patch Audit Binary SHA-256:** `e384bcb514a0d4b22e699712411ee20b069e3639bfaf0cca5926e761ed7f7b35`  
**Anti-Tautology Verification:** `PASSED` (Tested on injected value mutation, bool vs int coercion, missing keys, and unwhitelisted `ShowSecret` / `ShowSecurity` parameters).

---

## 1. Executive Summary & 38×2 Parity Verification

Across **all 38 real shortcuts in the private test corpus**, tested in both **Legacy** and **Cherri v2** dialects (**76 independent import-and-compile cycles**):

| Audit Layer | Compared Actions | Exact Matches | Divergences | Parity Rate |
| :--- | :---: | :---: | :---: | :---: |
| **Layer 1: Real Native Source → Post-Overlay Final Output** | **55,304** (27,652 × 2) | **55,304** | **0** | **100.0%** (76/76 full trees exact) |
| **Layer 2: Real Native Source → Pre-Overlay Action Identifiers** | **22,918** (11,459 × 2) | **22,918** | **0** | **100.0%** |
| **Layer 3: Pre-Overlay Semantic Parameters (Before Restore)** | **22,918** (11,459 × 2) | **22,918** | **0** | **100.0%** |
| **Layer 3: Canonical `ShowHeaders` Pre-Overlay Equality** | **2** (Specimen 014 × 2) | **2** | **0** | **100.0%** |
| **Layer 3: Post-Overlay Full Action Parameters Equality** | **22,918** (11,459 × 2) | **22,918** | **0** | **100.0%** |

---

## 2. Disjoint Action Accounting (Per Dialect Denominator = 27,652)

| Category | Actions (Per Dialect) | Actions (38 × 2 Total) | Share |
| :--- | :---: | :---: | :---: |
| **`NATURAL_VERIFIED`** | **11,459** | **22,918** | 41.44% |
| **`EXACT_NATIVE_FALLBACK`** | **16,192** | **32,384** | 58.56% |
| **`SOURCE_INVALID`** (`035:723`) | **1** | **2** | 0.00% |
| **TOTAL** | **27,652** | **55,304** | **100.0%** |

*Note on P10R10 vs P10R11 grammar:*
- Under P10R10 baseline grammar: 10,871 Natural + 16,780 Fallbacks + 1 Invalid = 27,652.
- Under P10R11 expanded natural grammar (with canonical `ShowHeaders` on `downloadURL`/`fileRequest` and optional `get` on `getValue`): 11,459 Natural (+588 actions promoted) + 16,192 Fallbacks + 1 Invalid = 27,652. Both sums are mathematically disjoint and reconcile to the identical denominator.

---

## 3. Strict Key Ownership & Provenance Architecture

1. **Strict Metadata Allowlist (`isImportProvenanceMetaKey`):**
   Only non-semantic metadata keys are allowed to be restored from import provenance:
   `UUID`, `CustomOutputName`, `GroupingIdentifier`, `ShowWhenRun`, `Show-WFInput`, `Show-WFURLActionURL`, `Show-text`.
2. **Canonical Editable Semantic Parameters:**
   `ShowHeaders` is **not** in the metadata allowlist. It is an explicitly defined canonical parameter (`bool ?showHeaders: 'ShowHeaders'`) in `actions/web.cherri`. It is emitted natively prior to provenance overlay and verified pre-restore.
3. **Legacy `WFSerializationType` Variance:**
   2,487 `setVariable` and 64 `appendVariable` instances omit `WFSerializationType` in `WFInput`. This omission is preserved in import mode, while normal authored code emits standard serialized token structures.
4. **Deterministic Semantic Action Digest:**
   `deterministicSemanticActionHash` hashes strictly the action identifier and semantic parameters (normalizing integer representations to `i:%d`). If any action is reordered, edited, duplicated, or removed, metadata cannot be misattributed to the wrong node.
