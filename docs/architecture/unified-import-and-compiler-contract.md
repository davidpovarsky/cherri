# Unified Import & Compiler Contract (P10R9)

**Date:** 2026-11-11  
**Status:** Active Architectural Contract (P10R9)  
**Owners:** `[איתן | Architecture]`, `[נועם | Engineering]`, `[רוני | QA]`

---

## 1. Purpose & Non-Negotiable Principles

Cherri operates as **one shared Apple Shortcuts compiler, decompiler, and action catalog engine** with two explicit surface syntax frontends (**Legacy** and **Cherri v2**).

1. **Single Source of Truth (SSoT):**
   - Built-in action definitions live in `actions/*.cherri` and `actions_std.go` (`EnsureCanonicalStandardActions()`).
   - No parallel action catalog, hand-maintained V2 registry, or duplicate plist emitter may exist.
   - `tools/language-schema` derives `internal/language/schema/generated_registry.go` directly from `cherri --actions-json`, and `tools/language-schema --verify-generated` enforces zero drift in CI.
2. **One Shared Native Document & Codec (`import_fidelity.go`):**
   - Every import path (`cherri import`, `cherri --import=...`, `ExecuteImportPipeline`, `decompileForMobileWithOptions`) decodes raw `.shortcut` / `.plist` / `.json` inputs through `DecodeNativeShortcutDocument(b []byte) (*NativeShortcutDocument, error)`.
   - `DecodeNativeShortcutDocument` enforces strict schema validation before any rendering or decompilation:
     - Rejects signed `AEA1` containers (`HYB18`).
     - Rejects trailing tokens/garbage after root JSON objects (`HYB20`).
     - Validates `root.WFWorkflowActions` is an array when present (`HYB19`).
     - Validates every `WFWorkflowActions[i]` element is a dictionary with a non-empty string `WFWorkflowActionIdentifier` and an optional dictionary `WFWorkflowActionParameters` (`HYB21`).
     - Validates control-flow block pairing (`WFControlFlowMode` 0/1/2 and `GroupingIdentifier` stack) and builds the producer/consumer `UUID` dependency graph.
3. **One Shared Backend & Action Emission (`action.go`, `language_backend_adapter.go`):**
   - Both Legacy (`shortcutgen.go` -> `makeAction`) and V2 (`lower.go` -> `CanonicalBackendSession.EmitResolvedCall`) evaluate canonical action definitions through `ActionEmissionContext` / `getActionParameters` in `action.go` and shared control-flow/token codecs in `shared_codec.go` and `shared_control.go`.
   - `EmitNativeWorkflow` in `language_emit_adapter.go` remains quarantined and unreachable from production `CompileSourceToPlist()`.
4. **Fail-Closed Mode Isolation:**
   - V2 compilation never falls back to the Legacy compiler on syntax or semantic errors (`HYB27`).
   - Semantic V2 import (`--fidelity=semantic --mode=v2`) validates migration diagnostics and V2 AST parsing via `MigrateSemanticLegacySourceToV2Strict`, failing closed on invalid output instead of silently emitting Legacy syntax (`HYB28`).

---

## 2. Unified Import Pipeline Architecture

```mermaid
flowchart TD
    In["Unsigned Input (.shortcut / .plist / .json)"] --> Decode["DecodeNativeShortcutDocument (import_fidelity.go)"]
    Decode --> Doc["NativeShortcutDocument + NativeActionRecord[]"]
    Doc --> Fidelity{"--fidelity"}
    Fidelity -->|"lossless"| Lossless["RenderLosslessLegacySourceFromDocument / RenderLosslessV2SourceFromDocument"]
    Fidelity -->|"hybrid"| Planner["PlanHybridImport (hybrid_import.go) + DefaultReverseActionRegistry (reverse_action_registry.go)"]
    Fidelity -->|"semantic"| Semantic["decompile() + MigrateSemanticLegacySourceToV2Strict"]
    Planner --> Verify["Strict Candidate Verification (Legacy + V2 in-memory re-compile)"]
    Verify --> Gate["Full-Workflow Verification Gate"]
    Gate --> OutHybrid["RenderHybridLegacySource / RenderHybridV2Source"]
```

---

## 3. Import Fidelity Modes & Guarantees

| Fidelity Mode | CLI Invocation | Wire Guarantee | Output Representation |
| :--- | :--- | :--- | :--- |
| **`lossless`** | `cherri import <file> [--fidelity=lossless] [--mode=v2\|legacy]` | **100% Exact Native Wire Parity** (`38/38` real corpus shortcuts) | Exact header (`native.workflow(..., exact: true)` / `#workflowheader`) + 1:1 raw actions (`native.action` / `rawAction`). |
| **`hybrid`** | `cherri import <file> --fidelity=hybrid [--mode=v2\|legacy]` | **100% Exact Native Wire Parity** (`38/38` real corpus shortcuts) | Exact header + verified readable named calls (`comment(...)`, `alert(...)`, `stop()`, `wait(...)`, `darkMode()`, `vibrate()`, etc.) mixed with exact native fallbacks where required. |
| **`semantic`** | `cherri --import=<file> [--fidelity=semantic] [--mode=v2\|legacy]` | **Best-Effort Semantic Reconstruction** | Legacy decompiler reconstruction (`#define`, `@var`, `if`/`menu`/`repeat` blocks); in `mode=v2`, strictly migrated and validated via `MigrateSemanticLegacySourceToV2Strict`. |

---

## 4. Deterministic Reverse Action Registry & Hybrid Planner Contract

1. **Deterministic Reverse Lookup (`reverse_action_registry.go`):**
   - `DefaultReverseActionRegistry()` indexes `EnsureCanonicalStandardActions()` by full `AppleIdentifier` with lexicographically sorted candidate lists.
   - `ResolveReverseActionCandidate(rec NativeActionRecord)` classifies every action into one of seven mutually exclusive reason codes:
     - `KNOWN_DISAMBIGUATED`: Unique canonical definition matched by `AppleIdentifier`, `AppIntentDescriptor`, `StaticParameters` (`appendParams`), and `ParameterKeys`, with no producer `UUID`/`CustomOutputName` or serialized attachment references.
     - `AMBIGUOUS_VARIANT`: Multiple canonical definitions share `AppleIdentifier` and wire parameters lack a distinguishing discriminator or tie on structural scores (`HYB06`).
     - `UNREPRESENTABLE_PARAM`: Action carries producer `UUID`/`CustomOutputName`, variable/attachment references, extra undocumented plist keys (`HYB08`), or non-literal parameter structures.
     - `CONTROL_REGION`: Control-flow marker (`WFControlFlowMode` / `GroupingIdentifier`).
     - `UNKNOWN_NATIVE_ID`: Uncatalogued `is.workflow.actions.*` identifier (`HYB05`).
     - `THIRD_PARTY_NO_CANONICAL_DEF`: Third-party app intent outside the built-in catalog (`HYB15`).
     - `SOURCE_INVALID`: Malformed source control-flow marker (e.g. `035` index `723` `CONTROL_FLOW_ORPHAN_END`, `HYB14`).
2. **Strict Candidate Verification & Full-Workflow Gate (`hybrid_import.go`):**
   - Every `KNOWN_DISAMBIGUATED` candidate is synthesized into both Legacy and V2 call syntax and compiled in-memory via `verifyHybridCandidatePair`.
   - Promotion to `NATURAL_VERIFIED` occurs **only** if both Legacy and V2 compile cleanly to a single `ShortcutAction` whose `WFWorkflowActionIdentifier`, `HadParametersKey`, and `WFWorkflowActionParameters` satisfy `plistValuesStrictlyEqual` against the original `NativeActionRecord`.
   - `RenderHybridLegacySource` and `RenderHybridV2Source` execute a final full-workflow verification gate against `doc.RawRoot`, demoting any action that would alter full-workflow wire equality.
