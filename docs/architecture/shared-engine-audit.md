# Cherri v2 — Shared Shortcuts Engine Architectural Audit

## 1. Executive Summary & Baselines

This document establishes the authoritative architecture, inventory, ownership split, and repair plan for consolidating Cherri Language v2 and the original Cherri compiler around **one shared, type-safe Apple Shortcuts emission engine**.

### Immutable Baseline References
- **Origin Baseline:** `fec09af2a587d7fe4d91254cdf0d34d0b56c7c00` (branch `agent/canonical-backend-production-cutover`)
- **Upstream Baseline:** `a66db15b7f247f3121726c2a2b72becfeef3d96b` (`https://github.com/electrikmilk/cherri`, branch `main`)
- **Active Working Branch:** `agent/shared-shortcuts-engine-v2-repair`
- **Docs Baseline:** `4218b152f6aea5e8e17912a8b4a8924ffd819f00` (`cherrilang.org`, branch `agent/canonical-backend-recovery-docs`)
- **Skill Compiler Source Pin:** `69281baab6401dd674e16beddeba62add2d1135d`

---

## 2. Inventory & Census Results

Calculated directly from live compiler metadata and action definitions:
- **Total Action Callables in Catalog:** 461
- **Unique Apple Identifiers:** 349
- **Multi-Name Identifier Groups:** 76 (Apple identifiers shared by multiple Cherri callables)
- **Fork-Added Action Names (20):**
  - **13 Declarative in `actions/*.cherri`:**
    - `getVariable`, `getType` (`actions/basic.cherri`)
    - `getUpcomingEvents`, `getTimeBetweenDates`, `addEvent`, `addReminder` (`actions/calendar.cherri`)
    - `getDirections`, `getTravelTime`, `getDistance` (`actions/location.cherri`)
    - `searchiTunes`, `createPlaylist` (`actions/music.cherri`)
    - `selectContacts` (`actions/contacts.cherri`)
    - `getAppStoreDetail` (`actions/shortcuts.cherri`)
  - **7 Added in `actions_std.go`:**
    - `setSilentMode`, `searchSpotlight`, `createRemindersList`, `startStopwatch`, `stopStopwatch`, `playAudiobook`, `openBook`
- **Fork-Relocated Actions (2):**
  - `run` (moved from declarative to complex Go definition in `actions_std.go` to support custom `WFWorkflow` target dictionary structure)
  - `transcribeText` (moved to complex Go definition to support custom AppIntent descriptor)
- **Fork-Modified Actions (10):**
  - `runJavaScriptOnWebpage` (added optional `WFInput`), `saveFile` (added optional `folder`), `openApp` (slideOver support), `base64Encode` / `base64Decode` (lineBreakMode support), `combineImages` (identifier correction), `getFileDetail` (input key correction), `calculate` (modulus / scientific operand), `prompt` (numeric bounds), `filterFiles` (limit enabled parameter).

---

## 3. Ownership Classification (Categories A, B, C, D)

| Category | Definition | Components / Paths |
|---|---|---|
| **A — Reuse Original Directly** | Action definitions, validation, parameter/key logic, App Intent descriptors, and serialization where existing implementation is correct and re-entrant. | `actions/*.cherri`, `actions_std.go`, `actionDefinition`, `makeActionParams`, `getActionParameters`, `checkAction`. |
| **B — Shared Pure Extracted Component** | Core codecs where legacy relied on global mutable state or tokens, or where v2 introduced deterministic, typed improvements (UTF-16 ranges, dictionaries, control flow segments, workflow headers). | `shared_codec.go` (references, text tokens, UTF-16 ranges, dictionary items), control flow segment builder, workflow header default initializer. |
| **C — Genuinely New v2 Component** | New *language semantics* rather than duplicate plist formats: AST, lexical scope, type checking, language service/LSP, 0-based indexing lowering, dynamic lazy `&&`/`\|\|`, `yield`, function dispatch envelope. | `internal/language/syntax/`, `analysis/`, `types/`, `service/`, `lower.go` (semantic composition only). |
| **D — Explicit Native Escape** | Lossless Apple action escape for unmodeled or proprietary actions. Explicit, validated, never implicit fallback for recognized actions. | `native.action`, `rawAction` via `EmitRawAction` / `NativeActionNode`. |

---

## 4. Confirmed Defects and Action Plan

### 4.1 Defect 1: Nondeterministic Apple-ID Fallback in `lookupCanonicalAction`
- **Location:** `language_registry_adapter.go:lookupCanonicalAction`
- **Cause:** When `DefinitionID` and `VariantID` are not matched, iterates `stdActions` and `actions` maps checking `fullID == call.AppleIdentifier`. Because 76 Apple IDs have multiple variants, Go map iteration order randomly picks a variant. Temporarily mutates global `currentAction`.
- **Fix:** Strictly resolve by stable canonical definition identity. If explicit variant is requested, validate within the variant group. If unknown definition, fail with an actionable error. Remove Apple-ID map search fallback for named calls.

### 4.2 Defect 2: Parameter Label Drops in `lowerCall`
- **Location:** `internal/language/lower/lower.go:lowerCall` (~2958)
- **Cause:** Iterates `call.NamedArgs`; if `param, exists = actionSchema.ParameterByLabel(nArg.Label)` returns false, the argument is silently ignored.
- **Fix:** Reject unknown parameter labels and duplicate labels with diagnostic errors. Preserve distinctness of `nil`, omitted, `false`, `0`, and empty strings.

### 4.3 Defect 3: Dictionary Materialization & Parameter Type Discrepancies
- **Location:** `language_backend_adapter.go:convertCallArgToActionArg`, `lower.go:lowerBinding`
- **Cause:** `convertCallArgToActionArg(ValDict)` emits `[]WFDictionaryFieldValueItem`, which conflicts with actions expecting `map[string]any`. `lowerBinding` treats `let` and `var` maps differently.
- **Fix:** Unify typed dictionary values. Materialize literal maps as native Dictionary actions with Apple-compliant `WFItems` structure. Handle four supported dictionary approaches correctly.

### 4.4 Defect 4: Control Flow Duplication & Segment Incompleteness
- **Location:** `lower.go` (manual `NativeActionNode`), `language_backend_adapter.go:BeginControl/EndControl`
- **Cause:** Three separate ways of building conditional, menu, and loop actions exist. `BeginControl` does not handle `Else` or `MenuCase`.
- **Fix:** Extract an authoritative typed control flow builder (`IfStart`, `Else`, `IfEnd`, `RepeatCountStart`, `RepeatEachStart`, `LoopEnd`, `MenuStart`, `MenuCase`, `MenuEnd`). Unify both `Lowerer` and `CanonicalBackendSession`.

### 4.5 Defect 5: Hardcoded Workflow Client Version & Trigger Support
- **Location:** `language_backend_adapter.go:NewCanonicalBackendSession`
- **Cause:** Starts with hardcoded `WFWorkflowClientVersion: "4711"`. `ShortcutDecl.Triggers` is parsed but not emitted.
- **Fix:** Unified workflow header initializer using canonical `Shortcut` defaults. Emit supported triggers or return explicit diagnostics for unsupported automation triggers.

### 4.6 Defect 6: iOS Bridge Substring Routing & Preview Source Rewrite
- **Location:** `ios_bridge.go:compileForMobile`, `ios/CherriApp/App/WorkspaceView.swift:syncPreviewEditToSource`
- **Cause:** `ios_bridge.go` uses substring search (`"Ask"`, `"@"`, `"const"`) to detect legacy, and falls back to legacy compiler on v2 error. `WorkspaceView.swift` overwrites user source with decompiled legacy syntax after visual edit.
- **Fix:** Explicit compilation mode end-to-end (`v2` vs `legacy`). Never fall back to legacy on v2 error. Use AST source editing (`ApplyVisualSourceEdit`); if unsupported, keep original source unchanged and notify the user.

### 4.7 Defect 7: Schema Generation Stringification
- **Location:** `tools/language-schema/main.go`
- **Cause:** `fmt.Sprint(v)` for nested `StaticParameters` turns nested maps into string representations like `"map[Value:map[..."`.
- **Fix:** Deterministic recursive Go literal encoder for nested static parameters.

---

## 5. Four Supported Dictionary Approaches

From analysis of real-world Shortcut evidence (`אפשרויות מילון.json`):
1. **Approach A (JSON Text -> Get Dictionary):** `gettext` -> `is.workflow.actions.detect.dictionary` -> `setvariable`. Supported via existing `getDictionary(jsonText)`.
2. **Approach B (Native Dictionary Action):** `is.workflow.actions.dictionary` with `WFItems` -> `setvariable`. Supported via literal `{ "key": value }`.
3. **Approach C (JSON Text -> Set Name *.json):** `gettext` -> `is.workflow.actions.setitemname` (`WFName = "name.json"`) -> `setvariable`. Distinct from dictionary creation.
4. **Approach D (JSON Text -> Reference Coercion):** `gettext` -> `setvariable` with `WFCoercionVariableAggrandizement` (`CoercionItemClass: "WFDictionaryContentItem"`). Coercion reference without `detect.dictionary`.
