# Cherri P10R7 — Delta Root Cause Map & Structural Diagnostics

**Date:** 2026-11-10  
**Phase:** P10R7 — True Shortcut Fidelity, Independent Oracle, Safe Lowering & Evidence Integrity  
**Evaluation Scope:** 38 Canonical Specimens (`001`–`038`)  
**Verified Source Actions:** 27,652  
**Verified P10R6 Rebuilt Actions:** 34,355  
**Net Action Delta:** +6,703  

---

## 1. Action Family Root Cause Mapping

Every single delta observed in the P10R6 corpus audit has been traced to exact source files and line numbers in the Cherri compiler and decompiler:

| Delta Family | Net Delta | Primary Locations | Root Cause Mechanism | Remediation Strategy |
|---|---:|---|---|---|
| `is.workflow.actions.nothing` | **+6,538** | `parser.go:1511`<br>`parser.go:1562`<br>`parser.go:1576`<br>`decompile.go:2586` | Compiler synthetically injects `nothing` actions before/after every control-flow block closing brace `}` and menu item to clear magic variable pipeline, while `decompile.go:skipDecompAction` strips real `nothing` actions adjacent to block ends. | Eliminate automatic compiler-injected `nothing` padding on control-flow closures; decompile explicit `nothing()` actions losslessly. |
| `is.workflow.actions.appendvariable` | **−254** | `shortcutgen.go:172-178`<br>`shortcutgen.go:225-230`<br>`decompile.go:994-995` | `decompile.go` emits `@var += ...`. In `shortcutgen.go`, `makeVariableAction` checks `!isArrayVariable(t.ident)` and falls back to `setvariable` or arithmetic/string concatenation when `@var` is not pre-declared as an array, eliminating all 254 append operations. | Preserve `appendvariable` operations losslessly via `rawAction` fallback or dedicated append lowering. |
| `is.workflow.actions.setvariable` | **+656** | `shortcutgen.go:173`<br>`shortcutgen.go:190`<br>`decompile.go:1285` | Expanded due to `@var += ...` lowering fallback to `setvariable`, plus variable synthesis for intermediate inlined expression captures. | Resolving `appendvariable` restores 254 actions; inlining preservation balances remaining set operations. |
| `is.workflow.actions.number` | **−147** | `decompile.go:979-983`<br>`decompile.go:1134-1189` | Standalone `number` actions with unreferenced UUIDs are collapsed into `currentVariableValue = decompValue(number)` and inlined as literals into downstream actions. | Emit standalone `number(val)` DSL actions or preserve via `rawAction` when not referenced as magic variables. |
| `is.workflow.actions.dictionary` | **−45** | `decompile.go:984-985`<br>`decompile.go:1670-1689` | Standalone `dictionary` actions unreferenced by UUID are inlined into downstream consuming action parameters as raw JSON literals. | Decompile standalone dictionaries as `dictionary(...)` actions when they occupy discrete workflow nodes. |
| `is.workflow.actions.getvalueforkey` | **−31** | `decompile.go:1004-1010`<br>`parser.go:651-667`<br>`shortcutgen.go:210-216` | Decompiler translates `getvalueforkey` into subscript syntax `dict['key']`. The parser attaches `WFDictionaryValueVariableAggrandizement` to the variable token rather than emitting a standalone action. | Preserve standalone `getvalueforkey` as `getValue(...)` action when present as a discrete action in the workflow. |
| `is.workflow.actions.calculateexpression` | **−15** | `decompile.go:992-993`<br>`decompile.go:1194-1213` | Decompiler inlines simple arithmetic expressions into `a + b`, which lowers to `is.workflow.actions.math` (+5) or literal values. | Retain `calculateexpression` or register formal lowering equivalence proof. |
| `is.workflow.actions.conditional` | **−1** | `decompile.go:1450-1520`<br>`parser.go:1530-1560` | Edge-case control-flow branch handling in complex multi-nested shortcuts (e.g. specimen 035). | Audit block grouping and matching grouping IDs. |

---

## 2. Top-Level Workflow Header Diagnostics (0 / 38 Exact)

Direct comparison between source plists and rebuilt `.shortcut` files revealed systemic omission and overriding of workflow metadata:

1. **`WFWorkflowMinimumClientVersion`:**
   - *Status:* Missing or overwritten in all 38 files.
   - *Code Location:* `shortcutgen.go:27-28` (`BuildWorkflowHeader` hardcodes 900); completely unhandled in `decompile.go`.
   - *Impact:* High-version shortcuts (e.g. version 1113, 1400) lose their deployment target requirements.
2. **`WFWorkflowTypes`:**
   - *Status:* Discrepant in 38 / 38 files.
   - *Code Location:* `shortcutgen.go:1382-1386` (`generateWorkflowTypes` defaults to `["Watch", "WFWorkflowTypeShowInSearch"]`); `decompile.go:817` only matches 9 hardcoded string values.
   - *Impact:* Share sheet and quick action triggers fail to match original deployment configuration.
3. **`WFWorkflowInputContentItemClasses`:**
   - *Status:* Discrepant in 38 / 38 files.
   - *Code Location:* `shortcutgen.go:1358-1370` defaults to 17 legacy items when `inputs` is empty; `decompile.go:833` suppresses emission if class count is 20 or greater.
   - *Impact:* Universal input shortcuts ("Everything") get truncated to 17 classes; zero-input shortcuts get populated with 17 unwanted classes.
4. **`WFWorkflowNoInputBehavior`:**
   - *Status:* Parameter dictionaries partially omitted.
   - *Code Location:* `decompile.go:861-880`.
   - *Impact:* Custom error strings and ask-for-input configurations are dropped.
5. **`WFWorkflowTriggers`:**
   - *Status:* Completely omitted in rebuilt files when present in source.
   - *Code Location:* `decompile.go:811-880` has no trigger decompilation logic.

---

## 3. Parametric Wire Divergence in Identical-Sequence Specimens

Even in the 3 specimens where action identifiers matched 1:1 (`007`, `017`, `032`), parametric wire comparisons failed:

### Specimen `032` (7 actions):
- **Action 4 (`is.workflow.actions.gettext`):** `WFTextActionText` attachment changed from `ActionOutput` with `OutputName` to a named variable reference `MyVar`.
- **Action 5 (`is.workflow.actions.url`):** `WFURLActionURL` native type changed from a `dict` (`WFTextTokenString`) to a `list` (`[{'Value': ...}]`).
- **Action 6 (`is.workflow.actions.openurl`):** `WFInput` native type changed from `WFTextTokenAttachment` (`dict` with `OutputUUID`) to `WFTextTokenString` (`dict` with `attachmentsByRange`).

### Specimen `007` (10 actions):
- **Action 6 (`is.workflow.actions.getvalueforkey`):** Compiler injected `WFGetDictionaryValueType: "Value"` (a default parameter that was not present in the source plist).
- **Actions 1, 2, 4, 5, 7, 8, 9:** UUID reference mapping shifts and text token attachment structure differences.

---

## 4. Phase 2 Conclusion

The semantic deltas across the 38 specimens are not random compiler bugs; they stem from 3 distinct architectural flaws:
1. **Unwanted Compiler Padding:** Automatic insertion of `is.workflow.actions.nothing` around every block ending accounting for 97.5% (+6,538) of all extra actions.
2. **Destructive Variable Lowering:** Lowering `+=` to `setvariable` instead of preserving `appendvariable` (−254 actions).
3. **Metadata Loss:** Decompiler and header generator omitting or defaulting critical top-level keys (`WFWorkflowMinimumClientVersion`, `WFWorkflowInputContentItemClasses`, `WFWorkflowTypes`, `WFWorkflowNoInputBehavior`).

Eliminating these 3 flaws in Phase 3 is the direct path to true semantic fidelity.
