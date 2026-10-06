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
- [ ] Immutable baseline binary captured
- [ ] Baseline action catalog captured (`--actions-json`)
- [ ] Baseline test results captured (`TestCherriNoSign`, `TestDecomp`)
- [ ] Upstream boundary manifest created
- [ ] Requirement ledger initialized
- [ ] Relevant audit findings reproduced

### M1 - Schema and source infrastructure
- [ ] Source spans/Unicode module (`internal/language/source/`)
- [ ] Type system (`internal/language/types/`)
- [ ] Schema/registry (`internal/language/schema/`)
- [ ] Build-time definition adapter (`tools/language-schema/`)
- [ ] Semantic facets validated
- [ ] All baseline definitions accounted for

### M2 - Parser and analysis
- [ ] Lexer with all token types
- [ ] Recursive-descent parser (full grammar)
- [ ] Pratt expression parser
- [ ] Error recovery
- [ ] Resolver/symbol table
- [ ] Type checker
- [ ] Structured diagnostics
- [ ] Formatter
- [ ] Basic language service queries

### M3 - Lowering and native preservation
- [ ] Semantic IR
- [ ] Native Shortcut IR
- [ ] Evaluation order preservation
- [ ] Bindings lowering
- [ ] Operators/arithmetic
- [ ] String/f-string/raw lowering
- [ ] Collection lowering
- [ ] Control flow lowering
- [ ] Function dispatch lowering
- [ ] Metadata/setup/trigger lowering
- [ ] Native action/block/workflow import/export
- [ ] Backend adapter (root bridge files)

### M4 - Public integration and migration
- [ ] CLI routing through new language
- [ ] iOS compilation routing
- [ ] Migration tool
- [ ] Active fork source migration
- [ ] LSP/bridge/service tests
- [ ] Editor/preview integration

### M5 - Docs and Skill synchronization
- [ ] Generated action references
- [ ] Active language docs
- [ ] Docs fork update
- [ ] Skill SKILL.md update
- [ ] Skill wrappers update
- [ ] Skill setup/doctor update
- [ ] Compatibility manifest
- [ ] Skill package build

### M6 - Full verification and cutover
- [ ] Full local test suite
- [ ] iOS runtime cases
- [ ] iOS app build
- [ ] LSP transcript tests
- [ ] Docs/Skill gates
- [ ] Upstream sync rehearsal
- [ ] Final artifact compatibility
- [ ] Legacy frontend absent from normal use

## Acceptance Contract Mapping

| ID | Status | Implementation | Tests |
|----|--------|---------------|-------|
| B01 | IN_PROGRESS | M0 baseline capture | — |
| B02 | IN_PROGRESS | M0 catalog capture | — |
| U01-U04 | PLANNED | M1 adapter/M3 backend | — |
| P01-P05 | PLANNED | M2 parser | — |
| V01-V04 | PLANNED | M2-M3 bindings | — |
| C01-C05 | PLANNED | M2-M3 calls | — |
| T01-T09 | PLANNED | M1-M2 types | — |
| S01-S03 | PLANNED | M2-M3 strings | — |
| L01-L04 | PLANNED | M2-M3 collections | — |
| N01-N03 | PLANNED | M2-M3 numbers | — |
| F01-F05 | PLANNED | M2-M3 control flow | — |
| FN01-FN05 | PLANNED | M2-M3 functions | — |
| M01-M05 | PLANNED | M3 metadata | — |
| IM01-IM05 | PLANNED | M2-M3 modules | — |
| AC01-AC03 | PLANNED | M1 schema | — |
| R01-R07 | PLANNED | M3 native | — |
| E01-E08 | PLANNED | M4 editor | — |
| CL01-CL03 | PLANNED | M4 CLI | — |
| D01-D03 | PLANNED | M5 docs | — |
| K01-K05 | PLANNED | M5 Skill | — |
| CI01-CI04 | PLANNED | M6 verification | — |
| AI01 | PLANNED | M6 evaluation | — |
| END01 | PLANNED | M6 delivery | — |
