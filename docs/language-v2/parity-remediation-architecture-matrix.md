# Cherri Legacy ↔ v2 Differential Parity Remediation Matrix

**Author:** איתן (Principal Architect)  
**Status:** AUTHORITATIVE ARCHITECTURE MATRIX  
**Work Branch:** `agent/cherri-v2-legacy-parity`  
**Reference Document:** `docs/ai-team/handoffs/cherri-legacy-v2-parity-final-gate-followup-2026-10-19.md`  

---

## 1. Fixture Inventory & 47 vs 48 Reconciliation

### 1.1 Discrepancy Root Cause
- **Tracked Fixtures in Git:** 47 fixtures.
- **Checked-in Parity Report Count:** 48 fixtures (`legacy-parity-results.json`).
- **Mechanism:** `TestDecomp` in `cherri_test.go:144-162` decompiles `tests/decomp-me.plist` and writes `tests/decomp-me.cherri` directly to `tests/`. Because `.gitignore` line 23 ignored `/tests/decomp-me.cherri`, the file remained untracked.
- **Behavioral Impact:** On a clean checkout before running tests, `filepath.Glob` finds 47 files (19 differences). Once `go test` runs, `decomp-me.cherri` is left on disk, `filepath.Glob` finds 48 files, and the duplicate fixture adds 1 identical failure (20 differences).

### 1.2 Permanent Architecture Remediation
1. **`cherri_test.go:TestDecomp`**: Direct output to `filepath.Join(t.TempDir(), "decomp-me.cherri")`.
2. **`.gitignore`**: Remove `/tests/decomp-me.cherri`.
3. **`tools/legacy-parity/main.go`**: Consume a canonical, tracked `tests/fixtures-manifest.json` containing the 47 canonical test fixtures.

---

## 2. Categorization & Remediation Architecture

### Category 1: Mechanically Provable Semantic Equivalence (Normalizer Updates)

| Fixture | Legacy Wire Behavior | v2 Wire Behavior | Equivalence Invariant | Remediation Location & Witness |
| :--- | :--- | :--- | :--- | :--- |
| **`numbers.cherri`**, **`empty-types.cherri`**, **`types.cherri`** | Number literals emitted as string types (`"5"`, `"0"`, `"42"`) in `WFMathOperand`, `WFInput`, `WFNumberActionNumber`. | Emitted as native float numbers (`5.0`, `0.0`, `42.0`). | Apple Shortcuts action parameter runtime automatically coerces string numbers to numeric floats in mathematical and numeric inputs. | `shortcutcompare/compare.go:toNumeric`: parse string to float64 when compared against numeric float in numeric action keys. |
| **`control-flow-output.cherri`**, **`getas-typecast.cherri`** | `Aggrandizements[0].PropertyName`: `"Model"`, `"OS"`. | `Aggrandizements[0].PropertyName`: `"Device Model"`, `"System Version"`. | In Apple Shortcuts' `WFPropertyVariableAggrandizement` dictionary for `DeviceDetails`, `"Model"` and `"Device Model"` are synonyms, as are `"OS"` and `"System Version"`. | `shortcutcompare/normalize.go`: canonicalize `DeviceDetails` property names (`Model` -> `Device Model`, `OS` -> `System Version`). |
| **`globals.cherri`** | Assigning system tokens (`ShortcutInput`, `Clipboard`) emits `is.workflow.actions.setvariable`. | Directly references system token attachments without intermediate `setvariable`. | Single-write assignment from a system token and downstream reference is isomorphic to direct token attachment referencing. | `shortcutcompare/normalize.go:extractOutputUUID`: extend forwardMap to forward single-assignment system tokens (`ShortcutInput`, `Clipboard`). |
| **`nil.cherri`** | Emits empty dictionary `{}` (`map[]`) for `nil` argument in menu item or repeat parameter. | Emits empty string `""` or omits parameter key. | In Apple Shortcuts plist schema, an empty dictionary `{}` for a scalar string/number slot is an invalid legacy artifact treated as empty/nil. | `shortcutcompare/normalize.go`: normalize `{}` in scalar parameter positions to omitted/empty. |

---

### Category 2: Real Compiler / Lowerer / Migration Defects (Action for Noam | DEV)

| Fixture | Defect Symptom | Exact Root Cause | Architectural Requirement | Action Owner |
| :--- | :--- | :--- | :--- | :--- |
| **`repeats.cherri`**, **`control-flow-output.cherri`**, **`menus.cherri`**, **`nil.cherri`** | Action count inflated; all downstream action indices shifted across entire files. | `lower.go:2170-2185` injects `is.workflow.actions.math` (`- 1`) to make `i` 0-based. Cherri's language contract and tests assert 1-based (`@firstIndex == 1`). | Do NOT inject `- 1` math action for `repeat i for N`. Bind `stmt.IndexVar` directly to native `Repeat Index`. | **נועם (DEV)** |
| **`arrays.cherri`**, **`empty-types.cherri`**, **`types.cherri`**, **`variables.cherri`** | `is.workflow.actions.list` plist parameter validation fails. | `lower.go` emits `WFItems` missing `WFItemType: 0` for string items, and wraps values in `WFTextTokenString` rather than plain strings. | In `lower.go` / `backend`, emit `{ "WFItemType": 0, "WFValue": item }` for list items matching Shortcuts schema. | **נועם (DEV)** |
| **`conditionals.cherri`**, **`decomp-expected.cherri`** | Action count inflated (185 vs 232); complex boolean trees synthesized. | Compound `&&` and `||` expressions lower to synthesized numbers (`1.0`/`0.0`) with nested `if/else` actions instead of native Shortcuts compound `WFConditions`. | Lower top-level `if cond1 && cond2` / `if cond1 || cond2` directly into native Shortcuts `WFConditions` filter templates (`WFActionParameterFilterPrefix`: 1 for AND, 0 for OR). | **נועם (DEV)** |
| **`conditionals.cherri`**, **`date-comparisons.cherri`** | Range comparisons fail to match native conditional wire representation. | `migrate.go:437-440` rewrites `<>` (Between) to `&&` (`>=` and `<=`), inflating actions instead of preserving native condition code `1003` (Between). | Preserve `<>` as `Between` (condition code 1003) with `WFNumberValue` and `WFAnotherNumber` / `WFDate` and `WFAnotherDate`. | **נועם (DEV)** |
| **`conditionals.cherri`** | `if legacy` comparison flag lost. | `migrate.go:415-418` strips `legacy` keyword instead of preserving `WFConditionalLegacyComparisonBehavior: true`. | Preserve `legacy` conditional keyword and emit `WFConditionalLegacyComparisonBehavior: true`. | **נועם (DEV)** |
| **`date-comparisons.cherri`** | `CurrentDate isToday` lowers to external action. | `migrate.go:441-445` rewrites `isToday` to `isToday(CurrentDate)` instead of native condition code `1002` (`Is Today`). | Emit native conditional predicate with condition code `1002`. | **נועם (DEV)** |
| **`dictionary.cherri`** | `setValue` action missing or misaligned in action sequence. | `setValue(@dict, "name", "Bob")` dropped or lowered differently when embedded in interpolation. | Lower `setValue` faithfully to `is.workflow.actions.setvalueforkey` with output attachment. | **נועם (DEV)** |
| **`arrays.cherri`**, **`variables.cherri`** | Array mutation via `+=` generates new list rather than appending. | `@mutable += "z"` lowers to list recreation rather than `is.workflow.actions.appendvariable`. | Lower `+=` on mutable variables to `is.workflow.actions.appendvariable`. | **נועם (DEV)** |
| **`functions.cherri`**, **`standard.cherri`** | Function headers and envelope differ from legacy inline header. | Functions in v2 use the `_cherri_magic = "__cherri_fn_envelope_v2__"` envelope and `Run Shortcut`. | **Preserve per Contract B2**. Provide dedicated equivalence witness in comparator. | **איתן + נועם** |
| **`web.cherri`** | Parameter defaults mismatch in web request actions. | `WFHTTPMethod`, headers, and empty JSON body parameters serialized differently. | Align action definition parameters with canonical Apple Shortcuts defaults. | **נועם (DEV)** |

---

## 3. Delegation & Execution Order

1. **תומר (INT):**
   - Implement `cherri_test.go:TestDecomp` temp dir fix.
   - Remove `/tests/decomp-me.cherri` from `.gitignore`.
   - Create canonical `tests/fixtures-manifest.json` (47 fixtures).
   - Implement Category 1 comparator normalizers in `shortcutcompare/normalize.go` and `compare.go`.
2. **נועם (DEV):**
   - Implement Category 2 compiler/lowerer/migrator fixes in `lower.go` and `migrate.go` per the matrix above.
   - Run targeted unit tests and full suite: `go test -count=1 ./...`.
3. **רוני (QA):**
   - Independently verify the full differential gate: `go run ./tools/legacy-parity`.
   - Execute adversarial mutation testing (inject intentional parameter and ordering bugs).
   - Verify CI step passes on Linux ARM64, macOS, and Nix.
