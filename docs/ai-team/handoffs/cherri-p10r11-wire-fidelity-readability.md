# Handoff: Cherri P10R11 — True Action-Wire Parity, Editable Import & Provenance Truth

**Date:** 2026-11-12  
**Branch:** `agent/cherri-p10r11-action-wire-semantic-truth-20261112`  
**Working Directory:** `C:\Users\DAVID\Code\cherri-p10r11-action-wire-semantic-truth-20261112`  
**Execution Mode:** `NORMAL` (`[MAIN_SESSION]`)  
**Assigned Roles:** `[איתן | Architecture]`, `[נועם | Engineering]`, `[רוני | QA]`, `[תומר | Integration]`

---

## 1. Executive Summary & Verification State

P10R11 completes the transition from speculative metadata heuristics to **true action-wire parity and editable hybrid import**.

### Verified Gates
- **Corpus Wire Parity (`38/38` Specimens):**
  - `lossless`: 38/38 Legacy & 38/38 V2.
  - `hybrid`: 38/38 Legacy & 38/38 V2.
- **Pre-Overlay Semantic Wire Parity:** `10,871 / 10,871 (100.0%)` natural verified actions match Apple wire *before* metadata restoration.
- **Post-Overlay Native Wire Parity:** `10,871 / 10,871 (100.0%)` match after verified metadata restoration.
- **50 Production-Path Adversarial Tests (`TestP11_WIRE_01` .. `TestP11_WIRE_50`):** All 50 pass.
- **Full Differential Parity:** All 47 fixtures in `tests/*.cherri` and 17 Apple built-in actions pass.
- **CI Test Discovery:** All 44 top-level regression tests pass across all families (`CORE`, `CODEC`, `CTRL`, `IOS`, `META`, `LANG`, `GEN`).
- **Release Gates:** 55/56 PASS (1 `BLOCKED_PLATFORM` on Windows).
- **Physical Device Status:** `APPLE_RUNTIME_EXECUTED = BLOCKED_DEVICE`.

---

## 2. Key Code Changes

1. **`actions/web.cherri`:**
   Added canonical `bool ?showHeaders: 'ShowHeaders'` to `downloadURL`, `formRequest`, `jsonRequest`, and `fileRequest`.
2. **`actions/basic.cherri`:**
   Updated `getValue` definition removing hardcoded static `"WFGetDictionaryValueType": "Value"`, making `?get: 'WFGetDictionaryValueType'` optional.
3. **`import_provenance.go`:**
   - Replaced `strings.HasPrefix(k, "Show")` with strict allowlist `verifiedImportMetadataKeys`.
   - Implemented `deterministicSemanticActionHash` normalizing integer types (`int`, `int64`, `uint64`), slices, and maps.
   - Enforced 3-pass restoration (in-place match, reorder match, legacy fallback), preventing stale metadata cross-binding.
4. **`shortcutgen.go`:**
   Added `t.typeof == AddTo` (`+=`) check in variable input generation, eliminating spurious `OutputUUID` emissions.
5. **`internal/language/schema/generated_registry.go`:**
   Regenerated with new schema fingerprint `8fca1c4c849e2a8517a2277c9e68269932140d193f953e515595b44a98bb343a`.
6. **`skills/cherri-shortcuts/compatibility-manifest.json`:**
   Updated to match new schema fingerprint.

---

## 3. Asana & Project Context

Asana project `1219603572114432` is read-only. No write operations were performed. Proposed status updates are staged in `docs/ai-team/handoffs/p10r11-suggested-asana-updates.md`.
