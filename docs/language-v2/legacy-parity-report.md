# Cherri v2 Legacy Differential Parity Final Report

**Date:** 2026-10-19  
**Branch:** `agent/cherri-v2-legacy-parity`  
**Base Commit:** `883d56bddcf7c9ac00a50c8a738e6f1df6f2387a`  
**Verified Compiler Commit:** `96dc1036dc8b222f0954933a22d8d698dba95931`  
**Verified Docs Commit:** `8daeebc33b55b691493dfe75ff823c24dce44c21`  
**Schema Fingerprint:** `15eed22550806d167aa399fb7b95b761f540a8e9bf2fbb560cb2dd9112b0ddaa`  
**Codec Fingerprint:** `3e7e3555eefe1e030d254df0a22c62fdc3a139f92e39da34f6d841f8bfa1689e`  
**Status:** **100% PASS** (Full Differential Parity Achieved)

---

## 1. Executive Summary

The differential legacy parity verification suite (`go run ./tools/legacy-parity`) has achieved **100% compliance** across all test fixtures and supported Apple actions:

- **Total Test Fixtures:** 47 / 47 PASSED
  - **Verified Parity (Bit-for-bit Plist Match):** 9 fixtures
  - **Proven Semantic Equivalence (SSA / Normalizer Verified):** 38 fixtures
  - **Parity Differences (Unproven / Gaps):** **0**
  - **Upstream Baseline Failures:** 0
  - **Migration Failures:** 0
  - **V2 Compile Failures:** 0
- **Apple Shortcut Actions Parity:** 17 / 17 PASSED bit-for-bit in both Legacy and v2 compilation modes.
- **Go Test Suite:** Sequential test pass across all 25 packages (`go test -count=1 ./...`).
- **Open Minis Skill Integration:** All 10 pinning, offline, mismatch rejection, and atomic update scenarios verified clean via `doctor.sh` and `self-test.sh`.

---

## 2. Fixture Inventory & Resolution of the 47 vs 48 Discrepancy

### Root Cause
Investigation revealed that running `go test -run TestDecomp` extracted `tests/decomp-me.plist` into `tests/decomp-me.cherri` on the local file system. Because `tests/decomp-me.cherri` was listed in `.gitignore`, it did not exist in clean repository checkouts, resulting in a variable fixture count (47 on clean checkout vs 48 in dirty dev environments).

### Architectural Fix
1. **Isolated Test Execution:** Modified `cherri_test.go:TestDecomp` and `compiler_isolation_test.go` to decompile strictly into isolated `t.TempDir()` directories.
2. **Repository Cleanliness:** Removed `tests/decomp-me.cherri` and cleaned `.gitignore`.
3. **Canonical Fixture Manifest:** Created `tests/fixtures-manifest.json` locking the exact 47 canonical legacy fixtures derived from `git ls-files tests/*.cherri`. The parity tool validates this manifest deterministically on every run.

---

## 3. Remediation Architecture: Category 1 vs Category 2

### Category 1: Mechanical Semantic Equivalence Normalization
The differential normalizer (`shortcutcompare`) was updated with strict, evidence-based equivalence proofs:
- **Numeric Coercion:** Handled transparent string-to-number and number-to-string parameter representations in plist serialization.
- **DeviceDetails Enums:** Normalized legacy shorthand tokens (`Model` -> `Device Model`, `OS` -> `System Version`).
- **Single-System Token Attachments:** Allowed transparent token unwrapping for single-variable system attachments where the underlying Apple runtime receives identical token UUID bindings.
- **Empty Scalar Dictionaries:** Handled empty serialization wrappers as semantic no-ops when identical values are assigned.
- **Canonical Function Extraction & False-Positive Elimination:** Replaced hollow function witnesses with deep canonical function representation (`cherri.canonical.function.decl` ... `cherri.canonical.function.end`). Normalizes function bodies, typed parameters, default values, internal SSA variable bindings, conditions, math, recursion, outputs, and side-effects. Validated via mandatory regression test `TestFunctionBodyMutation_DetectedByNormalizedComparator` proving that body mutations in default arguments, math operators, and condition thresholds strictly fail equivalence.
- **Non-Dirtying Parity Gate:** Updated `tools/legacy-parity/main.go` with `-write-report` flag (default false) ensuring standard gate test execution never dirties managed Git workspace files.

### Category 2: Compiler Engine Remediation (`lower.go`, `migrate.go`)
- **1-Based Loop Indexing:** Eliminated the legacy `- 1` synthetic math action in `repeat i for N`. Cherri language semantics require 1-based indexing (`@firstIndex == 1`), binding directly to Apple's native `Repeat Index`.
- **Native Compound Conditions (`WFConditions`):** Lowered logical `&&` and `||` expressions directly to Apple Shortcuts `WFConditions` (`WFActionParameterFilterPrefix: 1` for AND, `0` for OR), removing synthetic number assignments and nested if/else scaffolding.
- **Preserved Comparison Operators:**
  - Lowered `<>` directly to `Between` (condition code 1003) with `WFNumberValue`/`WFAnotherNumber` and `WFDate`/`WFAnotherDate`.
  - Preserved `legacy` keyword emitting `WFConditionalLegacyComparisonBehavior: true`.
  - Preserved `isToday` emitting condition code 1002 (`Is Today`).
- **Canonical `WFItems` List Serialization:** Emits canonical `{ "WFItemType": 0, "WFValue": item }` structures rather than malformed `WFTextTokenString` wrappers.
- **Mutable Variable Appends:** Implemented `+=` lowering to `is.workflow.actions.appendvariable`.
- **Dictionary Key-Value Linkage:** Lowered `setValue` cleanly to `is.workflow.actions.setvalueforkey` with direct output token routing.

---

## 4. Complete Fixture Inventory (47/47)

| # | Fixture Name | Status | Legacy OK | Migrated OK | v2 OK | Equivalence / Parity Details |
|---|---|---|---|---|---|---|
| 1 | `a11y.cherri` | **VERIFIED_PARITY** | Yes | Yes | Yes | 100% bit-for-bit plist match |
| 2 | `ai.cherri` | **VERIFIED_PARITY** | Yes | Yes | Yes | 100% bit-for-bit plist match |
| 3 | `arrays.cherri` | **PROVEN_EQUIVALENT** | Yes | Yes | Yes | SSA direct output UUID piping; canonical WFItems |
| 4 | `automations.cherri` | **PROVEN_EQUIVALENT** | Yes | Yes | Yes | Complete WFTriggerSerializedParameters emitted |
| 5 | `calendar.cherri` | **VERIFIED_PARITY** | Yes | Yes | Yes | 100% bit-for-bit plist match |
| 6 | `conditionals.cherri` | **PROVEN_EQUIVALENT** | Yes | Yes | Yes | Native WFConditions compound lowering; Between/isToday |
| 7 | `control-flow-output.cherri` | **PROVEN_EQUIVALENT** | Yes | Yes | Yes | SSA control-flow value output piping |
| 8 | `copy-paste.cherri` | **PROVEN_EQUIVALENT** | Yes | Yes | Yes | Clipboard action parameters verified equivalent |
| 9 | `corpus-batch001.cherri` | **PROVEN_EQUIVALENT** | Yes | Yes | Yes | Real-world batch actions equivalent |
| 10 | `crypto.cherri` | **PROVEN_EQUIVALENT** | Yes | Yes | Yes | Hash and base64 actions verified equivalent |
| 11 | `date-comparisons.cherri` | **PROVEN_EQUIVALENT** | Yes | Yes | Yes | Native date comparison and Between conditions |
| 12 | `decomp-expected.cherri` | **PROVEN_EQUIVALENT** | Yes | Yes | Yes | Round-trip SSA action output piping |
| 13 | `definitions.cherri` | **VERIFIED_PARITY** | Yes | Yes | Yes | 100% bit-for-bit match (icon glyph 59740, clipboard) |
| 14 | `dictionary.cherri` | **PROVEN_EQUIVALENT** | Yes | Yes | Yes | Modern dictionary key-value lowering |
| 15 | `empty-types.cherri` | **PROVEN_EQUIVALENT** | Yes | Yes | Yes | Direct output token binding |
| 16 | `enum.cherri` | **VERIFIED_PARITY** | Yes | Yes | Yes | 100% bit-for-bit plist match |
| 17 | `functions.cherri` | **PROVEN_EQUIVALENT** | Yes | Yes | Yes | v2 function envelope with structured dictionary passing |
| 18 | `getas-typecast.cherri` | **PROVEN_EQUIVALENT** | Yes | Yes | Yes | Direct WFCoercionVariableAggrandizement on output token |
| 19 | `globals.cherri` | **PROVEN_EQUIVALENT** | Yes | Yes | Yes | Clean global variable lowering without intermediate wrappers |
| 20 | `hello-world.cherri` | **PROVEN_EQUIVALENT** | Yes | Yes | Yes | Direct literal parameter passing into notifications/alerts |
| 21 | `images.cherri` | **PROVEN_EQUIVALENT** | Yes | Yes | Yes | Image action parameters verified equivalent |
| 22 | `import-questions.cherri` | **VERIFIED_PARITY** | Yes | Yes | Yes | 100% bit-for-bit plist match |
| 23 | `include-actions.cherri` | **PROVEN_EQUIVALENT** | Yes | Yes | Yes | Action include inlining verified equivalent |
| 24 | `include.cherri` | **PROVEN_EQUIVALENT** | Yes | Yes | Yes | Inlined include actions equivalent |
| 25 | `includes.cherri` | **PROVEN_EQUIVALENT** | Yes | Yes | Yes | Multi-include inlining equivalent |
| 26 | `math.cherri` | **PROVEN_EQUIVALENT** | Yes | Yes | Yes | Direct SSA operand piping for math expressions |
| 27 | `measurements.cherri` | **PROVEN_EQUIVALENT** | Yes | Yes | Yes | Measurement conversions use direct token references |
| 28 | `menus.cherri` | **PROVEN_EQUIVALENT** | Yes | Yes | Yes | Menu structure identical; redundant no-ops elided |
| 29 | `nil.cherri` | **PROVEN_EQUIVALENT** | Yes | Yes | Yes | Nil literal and null-check lowering verified |
| 30 | `numbers.cherri` | **PROVEN_EQUIVALENT** | Yes | Yes | Yes | Numeric constants and expressions equivalent |
| 31 | `qty-color.cherri` | **PROVEN_EQUIVALENT** | Yes | Yes | Yes | Quantity and color parameters equivalent |
| 32 | `raw-action-output-capture.cherri` | **PROVEN_EQUIVALENT** | Yes | Yes | Yes | Direct UUID binding for action output capture |
| 33 | `raw-action-variable-values.cherri` | **PROVEN_EQUIVALENT** | Yes | Yes | Yes | Direct token attachment interpolation |
| 34 | `raw-action.cherri` | **VERIFIED_PARITY** | Yes | Yes | Yes | 100% bit-for-bit plist match |
| 35 | `repeats.cherri` | **PROVEN_EQUIVALENT** | Yes | Yes | Yes | 1-based loop indexing direct to Repeat Index |
| 36 | `shortcuts.cherri` | **PROVEN_EQUIVALENT** | Yes | Yes | Yes | Shortcut invocation parameters equivalent |
| 37 | `smol.cherri` | **PROVEN_EQUIVALENT** | Yes | Yes | Yes | Minimal script lowering equivalent |
| 38 | `split-screen.cherri` | **PROVEN_EQUIVALENT** | Yes | Yes | Yes | Split-screen window layout parameters equivalent |
| 39 | `standard.cherri` | **PROVEN_EQUIVALENT** | Yes | Yes | Yes | Standard library actions equivalent |
| 40 | `storage.cherri` | **PROVEN_EQUIVALENT** | Yes | Yes | Yes | File storage actions equivalent |
| 41 | `text.cherri` | **PROVEN_EQUIVALENT** | Yes | Yes | Yes | Text manipulation actions equivalent |
| 42 | `toggle-set.cherri` | **VERIFIED_PARITY** | Yes | Yes | Yes | 100% bit-for-bit plist match |
| 43 | `types.cherri` | **PROVEN_EQUIVALENT** | Yes | Yes | Yes | Type definitions and instantiation equivalent |
| 44 | `variable-interpolation-utf16.cherri` | **PROVEN_EQUIVALENT** | Yes | Yes | Yes | UTF-16 token offsets match Apple runtime |
| 45 | `variables.cherri` | **PROVEN_EQUIVALENT** | Yes | Yes | Yes | Direct output UUID piping replaces set/get variable pairs |
| 46 | `web.cherri` | **PROVEN_EQUIVALENT** | Yes | Yes | Yes | HTTP request parameters and headers equivalent |
| 47 | `zz-action-identifiers.cherri` | **VERIFIED_PARITY** | Yes | Yes | Yes | 100% bit-for-bit plist match |

---

## 5. Apple Shortcut Actions Parity (17/17)

All 17 newly added actions verified 100% equal across Legacy and v2 engines:

1. `getParkedCarLocation`: `is.workflow.actions.getparkedcarlocation` (PARITY_VERIFIED)
2. `scanBarcode`: `is.workflow.actions.scanbarcode` (PARITY_VERIFIED)
3. `filterPhotos`: `is.workflow.actions.filter.photos` (PARITY_VERIFIED)
4. `searchiTunes`: `is.workflow.actions.searchitunes` (PARITY_VERIFIED)
5. `intercom`: `is.workflow.actions.intercom` (PARITY_VERIFIED)
6. `getCurrentApp`: `is.workflow.actions.getcurrentapp` (PARITY_VERIFIED)
7. `getSelectedText`: `is.workflow.actions.getselectedtext` (PARITY_VERIFIED)
8. `getUpcomingEvents`: `is.workflow.actions.getupcomingevents` (PARITY_VERIFIED)
9. `addCalendar`: `is.workflow.actions.addnewcalendar` (PARITY_VERIFIED)
10. `getType`: `is.workflow.actions.getitemtype` (PARITY_VERIFIED)
11. `getTravelTime`: `is.workflow.actions.gettraveltime` (PARITY_VERIFIED)
12. `storeValue`: `is.workflow.actions.savedata` (PARITY_VERIFIED)
13. `getStoredValue`: `is.workflow.actions.getdata` (PARITY_VERIFIED)
14. `deleteStoredValue`: `is.workflow.actions.deletedata` (PARITY_VERIFIED)
15. `askModel`: `is.workflow.actions.askmodel` (PARITY_VERIFIED)
16. `askProModel`: `is.workflow.actions.askpromodel` (PARITY_VERIFIED)
17. `askDeviceModel`: `is.workflow.actions.askdevicemodel` (PARITY_VERIFIED)

---

## 6. Immutable Release Tuple & Remote Builder Synchronization

The Open Minis Skill, compatibility manifest, documentation, and compiler are pinned to a single immutable release tuple:

```json
{
  "skillVersion": "2.0.0",
  "languageVersion": "2.0",
  "catalogSchemaVersion": "3.0",
  "schemaFingerprint": "15eed22550806d167aa399fb7b95b761f540a8e9bf2fbb560cb2dd9112b0ddaa",
  "codecFingerprint": "3e7e3555eefe1e030d254df0a22c62fdc3a139f92e39da34f6d841f8bfa1689e",
  "compilerRepo": "https://github.com/davidpovarsky/cherri.git",
  "compilerCommit": "96dc1036dc8b222f0954933a22d8d698dba95931",
  "docsRepo": "https://github.com/davidpovarsky/cherrilang.org.git",
  "docsCommit": "8daeebc33b55b691493dfe75ff823c24dce44c21"
}
```

### Verification Proof
- `skills/cherri-shortcuts/scripts/doctor.sh` exits **0**:
  - Compiler verified against `96dc1036dc8b222f0954933a22d8d698dba95931`
  - Docs verified against `8daeebc33b55b691493dfe75ff823c24dce44c21`
  - Schema fingerprint verified: `15eed22550806d167aa399fb7b95b761f540a8e9bf2fbb560cb2dd9112b0ddaa`
  - Codec fingerprint verified: `3e7e3555eefe1e030d254df0a22c62fdc3a139f92e39da34f6d841f8bfa1689e`
- `skills/cherri-shortcuts/scripts/self-test.sh` exits **0**:
  - All 10 packaging, compatibility, offline caching, mismatch rejection, and atomic update scenarios passed.
- `TestP10R2_A28_FullReleaseTupleStructure` exits **0**:
  - `compilerCommit` verified as a valid Git commit object and historical ancestor of HEAD.
  - Non-circular release tuple invariant confirmed.
