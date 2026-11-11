# Cherri P10R7 — Lowering Rule & Serialization Fidelity Catalog

**Phase:** P10R7 (True Shortcut Fidelity, Independent Oracle & Safe Lowering)  
**Scope:** Documented lowering rules verified across 38 real-world workflows + 505-action unit/round-trip suite, separating **verified safe rules** from **residual wire/lowering gaps** requiring follow-up work.

---

## 1. Verified Lowering & Preservation Rules Implemented in P10R7

| Rule ID | Component & Symbol | Transformation / Invariant | Verification Evidence |
| --- | --- | --- | --- |
| **LR-01** | `parser.go` (`firstChar`) | Reset `char = 0` at start of `firstChar()` so a leading blank newline `\n` in a decompiled file never skips line `0` into `lineIdx = 1` on pass 2 (prevented false duplicate include errors). | `TestForkActionRoundTrips/rawAction/text-token-string-preserves-braces`, 38/38 `LEGACY_COMPILE_PASS` |
| **LR-02** | `actions_std.go` (`checkMissingStandardInclude`) | Never emit `#include <actions/basic>` during decompilation (`if actionInclude != "basic"`), matching `decompile.go` auto-include semantics. | `38/38 DECOMPILE_PASS`, zero duplicate `basic` include failures |
| **LR-03** | `decompile.go` (`decompStructuredValue`, `decompObjectValue`, `decompAttachmentString`) + `compiler_state.go` | Track `decompilingRawTextToken` (reset in `resetCompilerStateFully`) so `WFTextTokenString` values inside `rawAction(...)` preserve `{...}` attachment references verbatim without double backslash escaping. | `TestForkActionRoundTrips/rawAction/text-token-string-preserves-braces`; `019` fixed (`VARIABLE_GRAPH_PASS: 10/38`) |
| **LR-04** | `decompile.go` (`decompVariable`) | Route `is.workflow.actions.setvariable` with `WFInput == nil && WFVariableName != nil` through `makeRawAction` (`rawAction`) instead of `@var = nil`, eliminating synthetic `nothing()` padding while keeping symbol registration in `parser.go`. | `TestForkActionRoundTrips/setvariable/no-wfinput-preserves-rawAction` |
| **LR-05** | `shortcutgen.go` (`menuItemValueAndTitle`), `shared_control.go` (`BuildMenuStartItems`, `BuildMenuCaseWithTitle`), `internal/language/backend/control.go` | Preserve interpolated `{var}` tokens inside `choosefrommenu` (`WFMenuItems` and `WFMenuItemAttributedTitle`) as native `WFTextTokenString` dicts while keeping readable `WFMenuItemTitle`, routed through shared control-flow helpers. | `TestForkActionRoundTrips/menuItem/interpolated-variable-token-string` |
| **LR-06** | `raw_actions.go` (`handleRawActionParams` vs `handleRawParams`), `language_backend_adapter.go`, `decompile.go` (`plistCanonicalEqual`) | Scope flat `IntentAppDefinition` preservation strictly to `rawAction(...)` (legacy & v2) while keeping upstream `actions/contacts.cherri` `collectAdditionalParams()` (`handleRawParams`) and `schemaFingerprint` untouched; compare `IntentAppDefinition` canonically via `plistCanonicalEqual` so `call`/`facetimeCall` decompile natively (`505/505`) while rich real-world `IntentAppDefinition` dicts fall back losslessly to `rawAction`. | `TestForkActionRoundTrips/rawAction/intent-app-definition-stays-flat`, `TestP10_VerifyAndRecord505CatalogExecution` (`505/505`), `verify_consumer_compatibility.py` |
| **LR-07** | `shortcutgen.go` (`isDictAggrandizement`) | Guard `Action` type assertions when determining `WFDictionaryValueVariableAggrandizement` vs `WFPropertyVariableAggrandizement` to eliminate panics and misclassified property lookups. | `38/38 DECOMPILE_PASS`, `38/38 LEGACY_COMPILE_PASS` |
| **LR-08** | `parser.go` (`collectEndStatement`, `collectMenuItem`, `checkConditionalTypes`, `collectRepeat`, `collectRepeatEach`, `collectTypeValues`) | Remove unconditional `addNothing()` padding; support `#define inputs none` (`__NONE__`); preserve array-backed action checks and optional index/item binders without injecting bloat. | Net corpus delta reduced `+6,703 -> +6` (`-99.91%`); `21/38 STRICT_ACTION_SEQUENCE_PASS` |
| **LR-09** | `shortcut.go` | Mark empty `WFQuickActionSurfaces`, `WFWorkflowOutputContentItemClasses`, `WFWorkflowNoInputBehavior`, and `WFWorkflowTriggers` with `plist:",omitempty"` so unused headers are omitted instead of emitted as noisy defaults. | Reduced extra header noise (`0` unexpected extra header keys across 38 shortcuts) |

---

## 2. Known Residual Lowering Gaps (Honest Accounting — Remaining Work)

These behaviors were observed directly under `independent_fidelity_oracle.py` and explain why `ACTION_PARAMETER_WIRE_PASS` is currently `0/38` and `STRICT_ACTION_SEQUENCE_PASS` is `21/38`:

1. **`uninitVars` Pre-Declarations (`+46 setvariable`, `+44 net nothing`):**
   - Remaining `11` of the `17` sequence-mismatched workflows trigger `uninitVars` when a variable name is read inside an earlier branch or parameter before the first linear `setvariable` action appears in flat order. Cherri's legacy compiler requires declaration before use; eliminating this delta safely requires either a `@var` symbol pre-declaration mode without emitting runtime `nothing + setvariable` actions or raw reference resolution.
2. **Inline Dictionary Subscript Folding (`-83 getvalueforkey`):**
   - Merging standalone `is.workflow.actions.getvalueforkey` into Cherri `dict['key']` syntax (`11` specimens) collapses separate `getvalueforkey` actions when referenced downstream (e.g., if Cherri later lowers an inline subscript into a synthesized `getvalueforkey` or folds multiple reads differently). Disabling unsafe folds when `UUID` / ordering matters would recover sequence parity on `002, 014, 016, 031, 038`.
3. **`calculateexpression` -> `math` (`-11 / +11` in `012, 022`):**
   - `decompExpression` parses `is.workflow.actions.calculateexpression` as arithmetic expressions that compile back as `is.workflow.actions.math`. Preserving `calculateexpression` fidelity (especially scientific/symbolic expressions) should route non-binary or explicit `calculateexpression` actions natively or via `rawAction`.
4. **`CustomOutputName` & Identifier Sanitization (`1,421` extra `CustomOutputName`, `1,098` `OutputName` shifts on 21 aligned specimens):**
   - Even when action count and sequence match 1:1 (`21/38`), Cherri's decompiler assigns symbolic variable/constant names (`@var`, `const Name = action(...)`), which emit `CustomOutputName` where Apple Shortcuts left unnamed Magic Variable UUIDs or used display titles containing spaces/punctuation that `sanitizeIdentifier` alters.
5. **Default Key Materialization & Aggrandizement Wire Shapes (`348` `WFSerializationType`, `115/75` `Aggrandizements`, `95` `Show-text`, `71` `WFGetDictionaryValueType`, `58` `WFURLActionURL`):**
   - Cherri's standard actions write explicit `WFSerializationType`, boolean `Show-*` defaults, and `WFTextTokenString` vs string scalars differently from older/newer iOS Shortcuts versions.
6. **Workflow Header Metadata (`WORKFLOW_HEADER_PASS: 4/38`):**
   - `WFWorkflowMinimumClientVersion` (`25` diffs), `WFWorkflowOutputContentItemClasses` (`19` missing empty array `[]` due to `omitempty` when source plist explicitly stored empty `[]`), `WFWorkflowImportQuestions` (`10`), and `WFWorkflowInputContentItemClasses` (`9`).
7. **Legacy-First Compilation (`V2_COMPILE_PASS: 1/38`):**
   - Only `032` compiles through Cherri v2 today; real-world decompilation targets `--legacy` (`38/38`) because `--import` emits legacy syntax (`@var`, `rawAction`, `#define`, etc.) and v2 lowering covers Phase 1–5 subsets.
8. **Malformed Source Specimen Edge Case (`035: conditional -1`):**
   - Source specimen `035` contains a duplicate closing `End If` (`WFControlFlowMode=2`) referencing an already-closed `GroupingIdentifier`; Cherri refuses to emit malformed unbalanced control flow (documented source artifact rather than a compiler defect).
