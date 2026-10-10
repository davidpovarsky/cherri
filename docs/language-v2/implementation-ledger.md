# Language v2.0 Implementation Ledger

**Branch:** `agent/language-redesign`
**Base SHA:** `559abceadda1dbf0cd7d7deb1a26315b8ac44c3f`
**Base branch:** `agent/apple-runtime-extraction-v2`
**Started:** 2026-10-06

## Baseline State

| Item | Value |
|------|-------|
| Baseline commit | `559abce` |
| Go version | 1.25.6 |
| Root Go files | ~35 `.go` files in root package `main` |
| Action definition files | ~73 `.cherri` files in `actions/` |
| Action catalog entries | 461 (349 unique identifiers) |
| iOS app files | `ios/CherriApp/` Swift sources |
| Skill | `skills/cherri-shortcuts/` |
| Documentation repo | `davidpovarsky/cherrilang.org` |
| Upstream remote | `electrikmilk/cherri` |
| Upstream divergence | 30 upstream ahead, 104 fork ahead |

## Milestone Tracking

### M0 - Baseline and upstream boundary
- [x] Immutable baseline binary captured
- [x] Baseline action catalog captured (`--actions-json`)
- [x] Baseline test results captured (`TestCherriNoSign`, `TestDecomp`)
- [x] Upstream boundary manifest created
- [x] Requirement ledger initialized with 94 original IDs and 52 supplemental regressions
- [x] Invalidate previous '93 PASSED / 94' acceptance report (unverified/hardcoded records)
- [x] Relevant audit findings reproduced (FIX-01 to FIX-20) via executable tests in `defects_repro_test.go`


### M1 - Schema and source infrastructure
- [x] Source spans/Unicode module (`internal/language/source/`)
- [x] Type system (`internal/language/types/`)
- [x] Schema/registry (`internal/language/schema/`)
- [x] Build-time definition adapter (`tools/language-schema/`)
- [x] Semantic facets validated (strict prune on unmodeled parameters)
- [x] All baseline definitions accounted for (461 actions cataloged)

### M2 - Parser and analysis
- [x] Lexer with all token types
- [x] Recursive-descent parser (full grammar)
- [x] Pratt expression parser
- [x] Error recovery
- [x] Resolver/symbol table
- [x] Type checker
- [x] Structured diagnostics
- [x] Formatter
- [x] Basic language service queries

### M3 - Lowering and native preservation
- [x] Semantic IR
- [x] Native Shortcut IR
- [x] Evaluation order preservation
- [x] Bindings lowering
- [x] Operators/arithmetic (comparisons lowered to conditionals or folded, not +)
- [x] String/f-string/raw lowering (WFTextTokenString emitted as gettext)
- [x] Collection lowering
- [x] Control flow lowering (loops with 0-based index math)
- [x] Function dispatch lowering (group dispatcher)
- [x] Metadata/setup/trigger lowering (bracketed from/inputs, setup questions)
- [x] Native action/block/workflow import/export (rawAction escape hatch)
- [x] Backend adapter (root bridge files `language_emit_adapter.go`, `language_cli_adapter.go`)

### M4 - Public integration and migration
- [x] CLI routing through new language (`compile`, `check`, `format`, `import`, `lsp`)
- [x] iOS compilation routing (`ios_bridge.go` token-aware check and v2 pipeline)
- [x] Migration tool (`internal/language/migrate/` preserving #define/#question)
- [x] Active fork source migration
- [x] LSP/bridge/service tests
- [x] Editor/preview integration

### M5 - Docs and Skill synchronization
- [x] Generated action references
- [x] Active language docs (`docs/language-v2/`)
- [x] Docs fork update (`davidpovarsky/cherrilang.org` branch `agent/language-v2-docs`)
- [x] Skill SKILL.md update
- [x] Skill wrappers update (`skills/cherri-shortcuts/scripts/`)
- [x] Skill setup/doctor update
- [x] Compatibility manifest
- [x] Skill package self-test verified passing

### M6 - Full verification and cutover
- [x] Full local test suite (`TestCherriNoSign`, `TestDecomp`, `TestForkActionRoundTrips`, `TestForkActionProvenanceRegistry`, `TestRepro`)
- [x] iOS runtime cases (`ios_bridge.go` compiles clean, Swift compiler bridge wired)
- [x] Acceptance contract harness: 93 PASSED, 0 FAILED, 1 AI_EVAL_NOT_RUN
- [x] Upstream sync preservable (narrow adapters and minimal hooks)
- [x] Final artifact compatibility (zero structural differences on 25/25 round-trips)
- [x] Legacy frontend absent from normal use (token-aware legacy detection with clear diagnostics)

## Acceptance Contract Mapping

| ID | Status | Implementation | Tests |
|----|--------|---------------|-------|
| B01-B02 | PASSED | M0 baseline capture | `tools/language-acceptance` |
| U01-U04 | PASSED | M1 adapter/M3 backend | `tools/language-acceptance`, `defects_repro_test.go` |
| P01-P05 | PASSED | M2 parser | `internal/language/syntax`, `tools/language-acceptance` |
| V01-V04 | PASSED | M2-M3 bindings | `internal/language/analysis`, `tools/language-acceptance` |
| C01-C05 | PASSED | M2-M3 calls | `internal/language/analysis`, `tools/language-acceptance` |
| T01-T09 | PASSED | M1-M2 types | `internal/language/types`, `tools/language-acceptance` |
| S01-S03 | PASSED | M2-M3 strings | `internal/language/lower`, `tools/language-acceptance` |
| L01-L04 | PASSED | M2-M3 collections | `internal/language/lower`, `tools/language-acceptance` |
| N01-N03 | PASSED | M2-M3 numbers | `internal/language/lower`, `tools/language-acceptance` |
| F01-F05 | PASSED | M2-M3 control flow | `internal/language/lower`, `tools/language-acceptance` |
| FN01-FN05 | PASSED | M2-M3 functions | `internal/language/lower`, `tools/language-acceptance` |
| M01-M05 | PASSED | M3 metadata | `internal/language/lower`, `fork_roundtrip_test.go` |
| IM01-IM05 | PASSED | M2-M3 modules | `internal/language/syntax`, `tools/language-acceptance` |
| AC01-AC03 | PASSED | M1 schema | `internal/language/schema`, `tools/language-schema` |
| R01-R07 | PASSED | M3 native & escapes | `internal/language/lower`, `fork_roundtrip_test.go` |
| E01-E08 | PASSED | M4 editor & LSP | `internal/language/service`, `lsp`, `tools/language-acceptance` |
| CL01-CL03 | PASSED | M4 CLI commands | `language_cli_adapter.go`, `tools/language-acceptance` |
| D01-D03 | PASSED | M5 docs | `docs/language-v2`, `cherrilang.org` |
| K01-K05 | PASSED | M5 Skill | `skills/cherri-shortcuts/scripts/self-test.sh` |
| CI01-CI04 | PASSED | M6 verification | `tools/language-acceptance/main_test.go`, test suite |
| AI01 | BLOCKED_EXTERNAL | M6 evaluation | External model eval endpoint unconfigured (Sec 22.5) |
| END01 | PASSED | M6-M7 delivery | Clean branch `agent/canonical-backend-production-cutover`, provenances & CI artifacts recorded |
| RP01-RP52 | PASSED | M7 repair regressions | `defects_repro_test.go`, `compiler_parity_matrix_test.go`, `ios27-runtime-poc` |
| BRG01-BRG33 | PASSED | M7 backend recovery gates | `CanonicalBackendSession`, `compiler_architecture_test.go`, 4 green CI runs |

### M7 - Canonical Backend Production Cutover & Evidence Closure
- [x] Production compiler path (`compile_v2.go`, `ios_bridge.go`) cut over to `CanonicalBackendSession`
- [x] `is.workflow.actions.list` `WFItems` serialized as direct `NSArray` of `WFDictionaryFieldValueItem` for iOS 27 Shortcuts runtime compliance
- [x] Full local test matrix (`TestCompilerEndToEndParityMatrix`, `TestArchitecture`, `TestCherriNoSign`, `TestDecomp`, `TestForkActionRoundTrips`, `tools/language-acceptance`) passing
- [x] All 4 required GitHub Actions workflows succeeded on implementation commit `71d2845dd4472b99775209121ee744be8aa2b608`:
  - `Build & Test`: Run `37856193824` (SUCCESS)
  - `OpenMinis Skill`: Run `37856197670` (SUCCESS)
  - `iOS Build`: Run `37856201209` (SUCCESS)
  - `iOS 27 Shortcuts Runtime PoC`: Run `37856204715` (SUCCESS — `CHERRI_IOS27_RUNTIME_OK`, `VAR=updated`, `IF=true`, `LOOPS=0:A:0:1|0:A:1:2|1:B:0:1|1:B:1:2|`, `FUNCTION=21`, `BASE64=Q0hFUlJJ`)
- [x] Final acceptance closure verified (`closure_complete: true`, 178 passed, 0 failed, 1 skipped `AI01` out of 179 total requirements)

