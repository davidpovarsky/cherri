# CHERRI P10R10 — Unified Semantic Importer, Provenance Preservation & Fail-Closed Verifier

- **Branch**: `agent/cherri-p10r10-semantic-import-unification-20261111` (branched from `37563df53e8ac0432bd4cd125b9489f32d2f631d`)
- **Implementation Commit:** `8e91ea9004e37581bf05c9752d39fe66dc0b4697`
- **Execution Mode**: `NORMAL` (`[MAIN_SESSION]`)
- **Fidelity Oracle**: `p10r8-fidelity-oracle-20261111.1`
- **Apple Hardware Execution (`APPLE_RUNTIME_EXECUTED`)**: `BLOCKED_DEVICE` (no iOS/macOS hardware signing or device runtime claim fabricated)

## 1. Architectural Summary

P10R10 replaces isolated single-action guessing in `--fidelity=hybrid` with a single unified semantic + native fallback pipeline (`planUnifiedSemanticHybridImport` in `semantic_import_planner.go`, invoked by `PlanHybridImport` / `PlanHybridImportWithRegistry` in `hybrid_import.go`):

1. **Shared Semantic Interpretation (`decompile.go` reuse):**
   - Calls `prepareDecompileSemanticPasses()` (`loadBasicStandardActions()`, `loadAllStandardActionsForDecompile()`, `resetParse()`, `firstChar()`, `initNamedVars()`, `mapVariables()`, `mapSplitActions()`, `mapIdentifiers()`, `mapControlFlowOutputs()`).
   - Reuses `matchAction()`, `decompAggrandizements()`, `utf16OffsetToRuneIndex()`, and `sanitizeIdentifier()` from `decompile.go` alongside `DefaultReverseActionRegistry()` (`reverse_action_registry.go`) and `DecodeNativeShortcutDocument()` (`import_fidelity.go`).
   - Both Legacy and Cherri v2 consume the single shared `HybridImportPlan` and diverge only when rendering Legacy vs V2 syntax.

2. **Structured Import Provenance (`import_provenance.go`):**
   - Preserves original `UUID`, `CustomOutputName`, `GroupingIdentifier`, variable bindings, parameter overrides, and workflow header metadata via `__CherriImportProvenance` encoded inside `#workflowheader`.
   - `extractAndStripImportProvenance()` strips import provenance metadata before plist serialization in both Legacy (`shortcutgen.go`, `import_fidelity.go`, `shared_codec.go`, `variables.go`) and Cherri v2 (`internal/language/analysis/analyzer.go`, `internal/language/lower/lower.go`, `internal/language/syntax/parser.go`, `internal/language/backend/control.go`, `language_backend_adapter.go`) so `__CherriImportProvenance` never leaks into compiled `.plist` / `.shortcut` outputs.
   - `applyImportProvenanceToCompiledActions()` restores action identity and reference integrity post-compile across complete multi-action workflows.

3. **Fail-Closed Whole-Workflow Verifier (`semantic_import_planner.go` & `hybrid_import.go`):**
   - `verifyAndStabilizePlan()` compiles complete Legacy and V2 sources, maps source lines to action indexes, demotes mismatched natural units/regions (`demoteUnit`, `enforceAtomicRegions`), and recompiles iteratively until zero plist delta remains against `doc.RawRoot`.
   - `RenderAndVerifyHybrid()` and `VerifyWholeWorkflowAgainstOriginal()` re-verify rendered output after demotions, avoid mutating caller plans on failure, and return explicit errors on compilation failures, metadata drift, or unresolved differences (zero silent pass on broken headers, missing UUIDs, shifted action indexes, or parameter type drift).

## 2. Verified P10R10 Test & Corpus Evidence

### 2.1 `TestSEM01`–`TestSEM40` & `TestHYB01`–`TestHYB30`
- Suite in `p10r10_semantic_hybrid_test.go` covers:
  - Semantic decompiler helper reuse, dual Legacy/V2 single-plan rendering, complex variable chains, `setVariable`/`getVariable`, dictionary/list expressions, aggrandizements & `.dictionary['k']` coercions (`WFCoercionVariableAggrandizement` + `WFDictionaryValueVariableAggrandizement`), `If`/`Otherwise`/`End If`, `Repeat`, `Repeat Each`, `Menu`, rich text tokens & attachments, `AppIntent` native/natural paths, provenance stripping across Legacy/V2, negative/adversarial mutations (header drift, missing UUID, shifted index, incompatible param types), and caller plan immutability on fail-closed errors.

### 2.2 Independent 38-Shortcut Corpus Audit (`docs/apple-runtime/p10r10/`)

| Surface / Baseline | Import Pass | Compile Pass | Full Tree Parity | Natural Readable Actions |
|---|---:|---:|---:|---:|
| Original Semantic (Legacy) | 35/38 | 35/38 | 0/38 | Best-effort (no wire parity) |
| Original Semantic (V2) | 14/38 | 1/38 | 0/38 | Best-effort (no wire parity) |
| P10R8 Lossless (Legacy & V2) | 38/38 | 38/38 | 38/38 | 0 / 27,652 (0.00%) |
| P10R9 Hybrid Baseline (Legacy & V2) | 38/38 | 38/38 | 38/38 | 1,463 / 27,652 (5.29%) |
| **P10R10 Unified Hybrid (Legacy & V2)** | **38/38** | **38/38** | **38/38** | **10,871 / 27,652 (39.31%)** |

- **Exact corpus classification counts (`27,652` total actions):**
  - `KNOWN_DISAMBIGUATED` (`NATURAL_VERIFIED`): `10,871` (`39.31%`)
  - `CONTROL_REGION` (`EXACT_NATIVE`): `10,160`
  - `UNREPRESENTABLE_PARAM` (`EXACT_NATIVE`): `6,424`
  - `THIRD_PARTY_NO_CANONICAL_DEF` (`EXACT_NATIVE`): `137`
  - `AMBIGUOUS_VARIANT` (`EXACT_NATIVE`): `46`
  - `UNKNOWN_NATIVE_ID` (`EXACT_NATIVE`): `13`
  - `SOURCE_INVALID` (`SOURCE_INVALID_EXACT_ONLY`): `1` (pre-existing orphan/malformed `035:723`)
- **Negative mutation Oracle checks (`negativeMutationsAllCaught`):** `true` (`header_version_drift`, `bool_instead_of_int`, `extra_action_param_key`).
- **Hybrid edit-isolation checks (`editIsolationAllPassed: false`):**
  - `012` (index `0`), `027` (index `524`), `035` (index `5`): `MODIFIED_EXISTING_NATURAL_COMMENT` — Legacy & V2 edit/compile `True`/`True`.
  - `001`: `NO_TOPLEVEL_COMMENT_FOUND` (`Compile=False`) because fixture `001` has no top-level natural `comment` action in its top-level natural set.
- **Top remaining Native fallback opportunities for future phases:** `is.workflow.actions.conditional` (`8880`), `choosefrommenu` (`976`), `getvalueforkey` (`704`), `dictionary` (`418`), `text.replace` (`404`), `gettext` (`364`), `runworkflow` (`316`), `setvariable` (`274`), `text.split` (`271`), `repeat.each` (`264`).

## 3. Provenance & Governance Policy
- Implementation artifacts and tests are committed first on the branch; `docs/fork-action-provenance.json` is then updated in a separate follow-up provenance commit recording the exact `firstForkCommit` SHA (non-circular ordering).
- All private audit corpora remain completely outside version control.
