# P10R5 — Consumer Compatibility Matrix

- **Generator**: `tools/p10r3/verify_consumer_compatibility.py`
- **Compiler HEAD at generation**: `7a6f41f825459c002a0284f822b1e30d6ca0f9c4`
- **Verdict**: `COMPATIBLE_WITH_EXTERNAL_BLOCKERS`

> `NOT_AVAILABLE_LOCALLY` and `BLOCKED_EXTERNAL` are never counted as `PASS`.

| Component | Source Commit | Schema Fingerprint | Codec Fingerprint | Tested Against | Status |
| :--- | :--- | :--- | :--- | :--- | :--- |
| Compiler | `7a6f41f82545` | `6eb0ef0129bb` | `3e7e3555eefe` | live `go run . --capabilities-json` | `PASS` |
| Documentation | `d8ad5e604c9e` | `6eb0ef0129bb` | `3e7e3555eefe` | compiler 7a6f41f82545 schema 6eb0ef0129bb | `PASS` |
| Skill | `17ba3ca3387f` | `6eb0ef0129bb` | `3e7e3555eefe` | docs d8ad5e604c9e | `PASS` |
| Remote Builder | `17ba3ca3387f` | `6eb0ef0129bb` | `3e7e3555eefe` | skill compatibility-manifest.json tuple | `BLOCKED_EXTERNAL` |
| iOS Bridge | `7a6f41f82545` | `6eb0ef0129bb` | `3e7e3555eefe` | schema.DefaultRegistry() (same registry as compiler) | `PASS` |
| iOS UI | `7a6f41f82545` | `6eb0ef0129bb` | `3e7e3555eefe` | iOS Bridge C ABI | `PASS` |
| Preview | `42ee7656e0cc` | `6eb0ef0129bb` | `3e7e3555eefe` | ios/WebPreview lock + generic catalog metadata renderer | `PASS` |
| Pure+Pack | `141b9cc96cf1` | `6eb0ef0129bb` | `3e7e3555eefe` | P10 catalog @ 7a6f41f82545 | `PASS` |

## Check Details

### Compiler — `PASS`
- [x] schemaFingerprint is 64-hex
- [x] codecFingerprint is 64-hex
- [x] codecFingerprint independent of schemaFingerprint
- [x] actionCount == 505 (got 505)
- [x] languageVersion == 2.0

### Documentation — `PASS`
- [x] cherri docs/language-v2/actions-reference.md fingerprint == compiler
- [x] cherrilang.org fork-language/actions.md fingerprint == compiler (got 6eb0ef0129bba916af7cfdeae85bdbb2e42245390ccaa1137670aeb46cb62cce)
- [x] smoke_test_snippets.py PINNED_CHERRI_COMMIT (17ba3ca3387f204a0fd80b092624e21f311db8d4) is an ancestor of compiler HEAD

### Skill — `PASS`
- [x] skill schemaFingerprint == compiler (got 6eb0ef0129bba916af7cfdeae85bdbb2e42245390ccaa1137670aeb46cb62cce)
- [x] skill codecFingerprint == compiler (got 3e7e3555eefe1e030d254df0a22c62fdc3a139f92e39da34f6d841f8bfa1689e)
- [x] skill languageVersion == compiler
- [x] skill catalogSchemaVersion == compiler
- [x] skill compilerCommit (17ba3ca3387f204a0fd80b092624e21f311db8d4) is an ancestor of HEAD
- [x] skill docsCommit (d8ad5e604c9e3585876ae95b0d1f628381a33c42) == docs HEAD (d8ad5e604c9e3585876ae95b0d1f628381a33c42)

### Remote Builder — `BLOCKED_EXTERNAL`
- [x] builder input compilerCommit is an ancestor of HEAD
- _Note_: Remote Linux ARM64 builder host is external; only its input tuple is verified locally. Not counted as PASS.

### iOS Bridge — `PASS`
- [x] ios_bridge.go exports CherriActionCatalog
- [x] ios_bridge.go returns registry-derived SchemaFingerprint
- [x] mobile_compiler.go consumes schema.DefaultRegistry()

### iOS UI — `PASS`
- [x] CherriCompiler.swift calls CherriActionCatalog()
- [x] CherriCompiler.swift decodes schemaFingerprint from bridge
- [x] no hardcoded fingerprint literals in CherriCompiler.swift (0 found)
- _Note_: Swift build/runtime verification requires macOS/Xcode (macOS CI job); static contract checked here.

### Preview — `PASS`
- [x] package.json pins preview-shortcut to a 40-hex commit (github:davidpovarsky/preview-shortcut#42ee7656e0cc07b00a097ff4d17d90c8eb8b6efd)
- [x] package-lock.json resolved commit == package.json pin
- [x] pinned preview commit exists in preview-shortcut repository

### Pure+Pack — `PASS`
- [x] Pure+Pack commit == 141b9cc96cf1df3f9d1593548dc49ae8a46a256a
- [x] Pure+Pack commit object present
- [x] Pure Original commit object present (read-only)
- [x] pinned definition-source reproduction byte-parity with 141b9cc (0 drift errors)
