# Cherri Legacy to v2 Feature Parity Matrix

This document provides the authoritative, contract-verified feature parity mapping between Cherri Legacy and Cherri v2.0. It documents how every legacy construct is preserved, compiled, or losslessly migrated, along with implementation provenance and verification evidence.

---

## 1. Executive Summary

| Category | Legacy Capabilities | Cherri v2 Implementation | Parity Status | Verification Test / Gate |
| :--- | :--- | :--- | :--- | :--- |
| **Compiler Modes** | In-process compiler | Legacy preserved via `--legacy`; v2 pipeline default | **VERIFIED** | CLI flags, unit suites |
| **Migration Engine** | N/A (single version) | Syntax-aware AST migration; fails closed; zero data loss | **VERIFIED** | `migrate_test.go`, Parity Runner |
| **New Apple Actions** | 17 newly added actions | Fully functional in both Legacy and v2 | **100% VERIFIED** | `tools/legacy-parity` (17/17 PASS) |
| **Preprocessor Directives** | `#define`, `#include`, `#question`, `#trigger`, `#ref` | Canonical v2 constructs (`shortcut`, `setup`, `trigger`, metadata) | **VERIFIED** | Parity runner (48/48 fixtures) |
| **Copy / Paste Blocks** | `copy name { ... }` / `paste name` | Compile-time macro expansion | **VERIFIED** | `TestAnalyzerCopyPaste`, `TestMigrateCopyPaste` |
| **Automations & Triggers** | `#trigger <family> <args>` | 7 families supported (`trigger <family>(...)`) | **VERIFIED** | `TestLowerTriggersWithParameters`, acceptance `T01` |
| **Globals & Context** | `@Ask`, `@CurrentDate`, `@Clipboard`, etc. | Canonical global symbols (`Ask`, `CurrentDate`, `Clipboard`, etc.) | **VERIFIED** | `TestAnalyzerGlobals`, `G01` |
| **User Functions** | `function foo(...) { output(...) }` | First-class functions with `Run Shortcut` envelope & defaults | **VERIFIED** | `functions.cherri`, acceptance `FN01`-`FN04` |
| **Menus** | `menu "Prompt" { case "Option": ... }` | Supported AST and lowered native workflow actions | **VERIFIED** | `menus.cherri`, `TestMigrateMenus` |
| **Binary Operators** | `contains`, `beginsWith`, `endsWith`, `+` (string/number) | Full lexical, syntactic, and semantic type support | **VERIFIED** | `conditionals.cherri`, acceptance `N01`-`N03` |

---

## 2. Detailed Language Construct Mapping

### 2.1 Preprocessor Directives & Metadata

| Legacy Directive | Semantics | Cherri v2 Equivalent | Migration Strategy | Provenance Files |
| :--- | :--- | :--- | :--- | :--- |
| `#define name <str>` | Shortcut Name | `shortcut "<str>" { ... }` | Lifted to top-level `shortcut` block | `internal/language/migrate/migrate.go`, `ast.go` |
| `#define glyph <name>` | SF Symbol / Glyph | `shortcut "..." { glyph: <name> }` | Mapped via canonical `schema.Glyphs` | `metadata.go`, `migrate.go` |
| `#define color <name>` | Shortcut Palette Color | `shortcut "..." { color: <name> }` | Mapped via canonical `schema.Colors` | `metadata.go`, `migrate.go` |
| `#define version <n>` | Client Version | `shortcut "..." { clientVersion: <str> }` | Mapped via `schema.ClientVersions` | `metadata.go`, `migrate.go` |
| `#define inputs <list>` | Accepted Content Types | `shortcut "..." { inputTypes: [...] }` | Mapped via `schema.ContentItemClasses` | `metadata.go`, `migrate.go` |
| `#define outputs <list>` | Output Content Types | `shortcut "..." { outputTypes: [...] }` | Mapped via `schema.ContentItemClasses` | `metadata.go`, `migrate.go` |
| `#define noinput <action>`| Action on Empty Input | `shortcut "..." { noInput: <action> }` | Preserved in shortcut metadata block | `metadata.go`, `migrate.go` |
| `#question <name> "<p>" "<d>"` | Import / Setup Question | `setup <name>: Text { prompt: "<p>", default: "<d>" }` | Converted to typed `setup` declaration block | `syntax/parser.go`, `lower/lower.go` |
| `#include '<path>'` | File inclusion | Automatic stdlib / local inlining or `import "<path>" as <alias>` | Inlines local fixtures; stdlib actions are globally available in v2 | `migrate/migrate.go`, `lower/lower.go` |
| `#ref <name> <base64>` | Native reference token | `let <name> = native.ref(...)` | Base64-decoded into native metadata attachment | `migrate/migrate.go`, `lower/lower.go` |

### 2.2 Trigger & Automation Directives

All 7 legacy trigger families are parsed, analyzed, migrated, and lowered into native workflow metadata and session triggers:

| Trigger Family | Legacy Syntax | Cherri v2 Canonical Declaration | Native Wire Representation |
| :--- | :--- | :--- | :--- |
| **Battery** | `#trigger battery risesAbove 50` | `trigger battery(risesAbove: 50)` | `WFWorkflowTriggerType: 1` + battery level payload |
| **Bluetooth** | `#trigger bluetooth "Device"` | `trigger bluetooth(device: "Device")` | `WFWorkflowTriggerType: 2` + bluetooth device name |
| **Charging** | `#trigger charging connected` | `trigger charging(connected: true)` | `WFWorkflowTriggerType: 3` + charging state bool |
| **Display** | `#trigger display sleep` | `trigger display(event: "sleep")` | `WFWorkflowTriggerType: 4` + sleep/wake event |
| **Screenshot** | `#trigger screenshot [finder, desktop]` | `trigger screenshot(locations: ["finder", "desktop"])` | `WFWorkflowTriggerType: 5` + target locations array |
| **StageManager** | `#trigger stageManager enabled` | `trigger stageManager(enabled: true)` | `WFWorkflowTriggerType: 6` + stage manager state bool |
| **Wi-Fi** | `#trigger wifi "SSID"` | `trigger wifi(network: "SSID")` | `WFWorkflowTriggerType: 7` + Wi-Fi network SSID |

### 2.3 Variables & Memory Model

| Legacy Construct | Legacy Semantics | Cherri v2 Construct | Semantic Guarantee |
| :--- | :--- | :--- | :--- |
| `const x = <val>` | Immutable binding | `let x = <val>` | Write-once semantics enforced by analyzer (`E_ASSIGN_IMMUTABLE`) |
| `@x = <val>` (single write) | Global/local variable | `let x = <val>` | Migrator analyzes write frequency; if assigned once, promotes to `let` |
| `@x = <val>` (multiple writes)| Reassigned variable | `var x = <val>` | Reassigned variable lowered to mutable variable with `appendvariable` or reassignment |
| `@x += <val>` | Append to list variable | `x += <val>` | Lowers to `is.workflow.actions.appendvariable` when targeting lists |
| `@x: type` | Uninitialized typed var | `var x: Type = <default>` | Emits type-safe initial empty value (`{}` for Map, `[]` for List, etc.) |

### 2.4 Control Flow & Functions

| Legacy Capability | Legacy Syntax | Cherri v2 Representation | Parity & Optimization Details |
| :--- | :--- | :--- | :--- |
| **User Functions** | `function foo(number n): text { output("{@n}") }` | `function foo(n: Number) -> Text { return f"{n}" }` | Compatible with legacy `Run Shortcut` envelope; preserves return type and arguments |
| **Hot Loop Warning** | N/A | `W_PERF` diagnostic | Warns if user functions are called in loops, helping avoid `Run Shortcut` overhead |
| **Value-Producing If** | N/A | `let x = if c { yield a } else { yield b }` | Functional expression blocks with full exhaustiveness checking |
| **Menus** | `menu "Prompt" { case "A": ... }` | `menu "Prompt" { case "A": ... }` | Full AST and native lowerer support for interactive menus |
| **Copy / Paste** | `copy name { ... }` / `paste name` | `copy name { ... }` / `paste name` | Compile-time macro expansion in analyzer before lowering |

---

## 3. Newly Added Apple Actions Parity Verification

All 17 newly added Apple actions were tested and confirmed 100% operational in both Legacy (`cherri <file> --legacy --skip-sign`) and v2 (`cherri compile <file> --skip-sign`):

| # | Action Callable | Category | Apple Action Identifier | Legacy Status | v2 Status | Verified |
| :-: | :--- | :--- | :--- | :-: | :-: | :-: |
| 1 | `getParkedCarLocation` | `location` | `is.workflow.actions.getparkedcarlocation` | **OK** | **OK** | PASS |
| 2 | `scanBarcode` | `images` | `is.workflow.actions.scanbarcode` | **OK** | **OK** | PASS |
| 3 | `filterPhotos` | `photos` | `is.workflow.actions.filter.photos` | **OK** | **OK** | PASS |
| 4 | `searchiTunes` | `music` | `is.workflow.actions.searchitunes` | **OK** | **OK** | PASS |
| 5 | `intercom` | `device` | `is.workflow.actions.intercom` | **OK** | **OK** | PASS |
| 6 | `getCurrentApp` | `mac` | `is.workflow.actions.getcurrentapp` | **OK** | **OK** | PASS |
| 7 | `getSelectedText` | `mac` | `is.workflow.actions.getselectedtext` | **OK** | **OK** | PASS |
| 8 | `getUpcomingEvents` | `calendar` | `is.workflow.actions.getupcomingevents` | **OK** | **OK** | PASS |
| 9 | `addCalendar` | `calendar` | `is.workflow.actions.addnewcalendar` | **OK** | **OK** | PASS |
| 10 | `getType` | `basic` | `is.workflow.actions.gettypeaction` | **OK** | **OK** | PASS |
| 11 | `getTravelTime` | `location` | `is.workflow.actions.gettraveltime` | **OK** | **OK** | PASS |
| 12 | `storeValue` | `storage` | `is.workflow.actions.setstoredcontent` | **OK** | **OK** | PASS |
| 13 | `getStoredValue` | `storage` | `is.workflow.actions.getstoredcontent` | **OK** | **OK** | PASS |
| 14 | `deleteStoredValue` | `storage` | `is.workflow.actions.deletestoredcontent` | **OK** | **OK** | PASS |
| 15 | `askModel` | `intelligence` | `is.workflow.actions.askllm` | **OK** | **OK** | PASS |
| 16 | `askProModel` | `intelligence` | `is.workflow.actions.askllm` | **OK** | **OK** | PASS |
| 17 | `askDeviceModel` | `intelligence` | `is.workflow.actions.askllm` | **OK** | **OK** | PASS |

---

## 4. Test & Verification Infrastructure

- **Acceptance Contracts (`tools/language-acceptance`):**
  - All 71 local acceptance requirements pass (`Failed: 0`).
  - Schema fingerprint matches authoritative catalog `15eed22550806d167aa399fb7b95b761f540a8e9bf2fbb560cb2dd9112b0ddaa`.
- **Legacy Differential Parity Runner (`tools/legacy-parity`):**
  - 48 test fixtures scanned.
  - Zero migration failures (`Migration Fails: 0`).
  - Zero v2 compilation failures (`V2 Compile Fails: 0`).
  - 17/17 newly added Apple actions passed in both modes.
- **Unit and Decompiler Suites:**
  - `go test -run TestCherriNoSign -v .`: 46/46 fixtures PASS.
  - `go test -run TestDecomp -v .`: Decompilation and round-trip PASS.
  - `go test -v ./internal/language/...`: 100% PASS across syntax, analysis, lower, migrate, schema, lsp, and service packages.
