# P10R11 Action Wire Truth & Editable Hybrid — Independent Audit & Closure Report

## Final Executive Verdict

**Verdict:** `P10R11_LOCAL_VERIFIED_MACOS_BLOCKED`

- **Compiler Binary SHA-256:** `875c297660e1eeb8ce7a46c8755957f78a4be5434be9440ad8e151a88220136d`
- **No-Patch Audit Binary SHA-256:** `e384bcb514a0d4b22e699712411ee20b069e3639bfaf0cca5926e761ed7f7b35`
- **Measured Corpus Action Total:** **27,652** across 38 normalized originals (`62,980,131` bytes physical on disk).
- **Disjoint Action Classification:**
  - **Baseline (P10R10):** 10,871 Natural Verified (39.31%) + 16,780 Fallbacks (60.68%) + 1 Source Invalid (0.00%) = 27,652.
  - **Expanded (P10R11):** 11,459 Natural Verified (41.44%) + 16,192 Fallbacks (58.56%) + 1 Source Invalid (0.00%) = 27,652.
- **Independent 38×2 Multi-Layer Audit (76 complete runs):**
  - `Layer 1: Real Native Source → Post-Overlay Output`: **76 / 76 (100.0%)** complete recursive typed plist-tree exact matches (`bool` != `int`).
  - `Layer 2: Real Native Source → Pre-Overlay Identifiers`: **22,918 / 22,918 (100.0%)** exact action identifier matches.
  - `Layer 3: Pre-Overlay Semantic Parameters (Before Restore)`: **22,918 / 22,918 (100.0%)** exact semantic key-value matches (zero semantic divergence).
  - `Layer 3: Canonical ShowHeaders Pre-Overlay Equality`: **2 / 2 (100.0%)** exact in specimen 014 (Legacy & V2).
- **Adversarial Wire Tests (`P11-WIRE-01` .. `P11-WIRE-50`):** `50 / 50 PASS (100%)`
  - `TestP11_WIRE_01`: action-by-action pre- and post-overlay verification across all 38 files; fails if corpus missing when requested, reports `NOT_RUN_PRIVATE_CORPUS` when public.
  - `TestP11_WIRE_39`: actual action move across control boundary into repeat loop verified; GroupingIdentifier balanced, UUID metadata preserved via semantic hash, unbalanced boundary fails closed.
  - `TestP11_WIRE_07`: exact `014:208` action (`fileRequest` with `ShowHeaders=true` source truth) verified; mutations `true -> false` and removal (`absent`) verified across both dialects.
- **Negative Mutations & Anti-Tautology:** `PASSED` (72 Python oracle mutations, 11 ledger consistency checks, negative plist assertions).
- **CI Status:**
  - `Linux (Job 131298939329)`: **SUCCESS** (14/14 steps, full Go suite, discovery gate, decomp, parity, oracle, release gates).
  - `Nix (Job 131299034894)`: **SUCCESS** (package built).
  - `macOS (Job 131298934920)`: **MACOS_CI=BLOCKED_RUNNER_CAPACITY** (cancelled after 15 min without runner acquisition; `steps: []`; no code or test failure demonstrated).
- **Hardware Status:** `APPLE_RUNTIME_EXECUTED: BLOCKED_DEVICE` (Mandatory disclosure: Windows development environment, no unverified Apple physical hardware execution claimed).

---

## Technical Architecture & Policy Corrections

1. **Elimination of `Show*` Prefix Heuristic & Strict Key Ownership:**
   - Previous loose prefix rule (`strings.HasPrefix(k, "Show")`) removed.
   - Strict allowlist of import metadata keys (`verifiedImportMetadataKeys` in `import_provenance.go`): `UUID`, `CustomOutputName`, `GroupingIdentifier`, `ShowWhenRun`, `Show-WFInput`, `Show-WFURLActionURL`, `Show-text`.
   - Unknown parameters (e.g. `ShowSecret`, `ShowSecurity`, `WFPrivateFutureParam`) fail closed to raw native action fallbacks.
2. **Canonical `ShowHeaders` Parameter:**
   - `ShowHeaders` is **not** an import metadata key.
   - It is a first-class canonical semantic parameter (`bool ?showHeaders: 'ShowHeaders'`) in `actions/web.cherri` for `downloadURL`, `formRequest`, `jsonRequest`, and `fileRequest`.
   - Emits native wire directly pre-overlay; user edits compile without stale provenance restoration.
3. **Legacy `WFSerializationType` Variance:**
   - Preserves absent `WFSerializationType` in `WFInput` for 2,487 `setVariable` and 64 `appendVariable` instances in import mode.
   - Normal authored code retains standard serialized token structures.
   - Fixed `+=` (`AddTo`) output name assignment in `shortcutgen.go:makeVariableInput` to eliminate spurious `OutputUUID` emissions.
4. **Deterministic Semantic Action Digest:**
   - Multi-pass restoration (`deterministicSemanticActionHash`) unifies integer types (`int`, `int64`, `uint64`) to `i:%d`.
   - In-place match, reorder match, and fallback passes prevent stale metadata transfer when actions are moved, reordered, or duplicated.
