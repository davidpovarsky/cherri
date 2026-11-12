# P10R11 Action Wire Truth & Editable Hybrid — Independent Audit

## Final Executive Verdict

**Verdict:** `SEMANTIC_ACTION_WIRE_TRUTH_VERIFIED`

- **Compiler Binary SHA-256:** `875c297660e1eeb8ce7a46c8755957f78a4be5434be9440ad8e151a88220136d`
- **Corpus Parity Across 38 Specimen Files:**
  - `Lossless Legacy`: `38 / 38 (100%)`
  - `Lossless V2`: `38 / 38 (100%)`
  - `Hybrid Legacy`: `38 / 38 (100%)`
  - `Hybrid V2`: `38 / 38 (100%)`
- **Natural Readable Actions:** `10,871 / 27,652 (39.31%)`
- **Pre-Overlay Semantic Wire Parity:** `10,871 / 10,871 (100.0%)`
- **Post-Overlay Native Wire Parity:** `10,871 / 10,871 (100.0%)`
- **Adversarial Wire Tests (`P11-WIRE-01` .. `P11-WIRE-50`):** `50 / 50 PASS (100%)`
- **Targeted Edit Isolation:** `PASS` (`14 / 14` specific action & region edits isolated to expected fields only)
- **Negative Mutations Caught:** `TRUE` (72 Python oracle mutations, 11 ledger mutations, 8 consumer drift mutations)
- **Source Flow Validity:** `035:723` preserved as `SOURCE_CONTROL_FLOW_INVALID` without silent drop.
- **Hardware Status:** `APPLE_RUNTIME_EXECUTED: BLOCKED_DEVICE` (Mandatory disclosure: no unverified physical hardware claims).

---

## Key Technical Breakthroughs in P10R11

1. **Elimination of `Show*` Prefix Heuristic:**
   Replaced loose string prefix matching with strict, enumerated key ownership allowlist (`UUID`, `CustomOutputName`, `GroupingIdentifier`, `ShowHeaders`, `ShowWhenRun`, `Show-WFInput`, `Show-WFURLActionURL`, `Show-text`). Unknown parameters fail closed to raw action fallbacks.

2. **Canonical `ShowHeaders` Support:**
   Added `showHeaders: bool?` as a first-class canonical parameter on `downloadURL`, `formRequest`, `jsonRequest`, and `fileRequest`. Pre-overlay compilation generates native wire directly from source. User edits to `showHeaders` compile natively with zero stale provenance overwrite.

3. **Multi-Pass Deterministic Semantic Hashing:**
   Implemented `deterministicSemanticActionHash` normalizing integers, arrays, and dictionary values. In-place matching, reorder matching, and fallback handling prevent cross-binding or stale metadata transfer when actions are moved, reordered, inserted, or duplicated.

4. **Variable Action Native Serialization Truth:**
   Preserved legacy corpus omissions (2,487 `setVariable`, 64 `appendVariable`) without breaking standard authored code. Fixed `+=` (`AddTo`) output name assignment to prevent spurious `OutputUUID` emissions.
