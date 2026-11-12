# P10R5 — Consumer Compatibility Matrix

- **Generator**: `tools/p10r3/verify_consumer_compatibility.py`
- **Compiler HEAD at generation**: `dd9bd664571d53cc558956481ef79e1760615063`
- **Verdict**: `INCOMPATIBLE`

> `NOT_AVAILABLE_LOCALLY` and `BLOCKED_EXTERNAL` are never counted as `PASS`.

| Component | Source Commit | Schema Fingerprint | Codec Fingerprint | Tested Against | Status |
| :--- | :--- | :--- | :--- | :--- | :--- |
| Compiler | `dd9bd664571d` | `8fca1c4c849e` | `3e7e3555eefe` | live `go run . --capabilities-json` | `PASS` |
| Documentation | `a38b62b819e4` | `8fca1c4c849e` | `3e7e3555eefe` | compiler dd9bd664571d schema 8fca1c4c849e | `FAIL` |
| Skill | `1204e1f7dfac` | `8fca1c4c849e` | `3e7e3555eefe` | docs a38b62b819e4 | `PASS` |
| Remote Builder | `1204e1f7dfac` | `8fca1c4c849e` | `3e7e3555eefe` | skill compatibility-manifest.json tuple | `BLOCKED_EXTERNAL` |
| iOS Bridge | `dd9bd664571d` | `8fca1c4c849e` | `3e7e3555eefe` | schema.DefaultRegistry() (same registry as compiler) | `PASS` |
| iOS UI | `dd9bd664571d` | `8fca1c4c849e` | `3e7e3555eefe` | iOS Bridge C ABI | `PASS` |
| Preview | `42ee7656e0cc` | `8fca1c4c849e` | `3e7e3555eefe` | ios/WebPreview lock + generic catalog metadata renderer | `PASS` |
| Pure+Pack | `141b9cc96cf1` | `8fca1c4c849e` | `3e7e3555eefe` | P10 catalog @ dd9bd664571d | `FAIL` |

## Check Details

### Compiler — `PASS`
- [x] schemaFingerprint is 64-hex
- [x] codecFingerprint is 64-hex
- [x] codecFingerprint independent of schemaFingerprint
- [x] actionCount == 505 (got 505)
- [x] languageVersion == 2.0

### Documentation — `FAIL`
- [x] cherri docs/language-v2/actions-reference.md fingerprint == compiler
- [ ] cherrilang.org fork-language/actions.md fingerprint == compiler (got 787e9842bbe92b14dbf195bf896c371cb622cd0b6178532b146dc1da6164cc46)
- [x] smoke_test_snippets.py PINNED_CHERRI_COMMIT (4f4d30f1b863e8d53cd3d56eb8ddcaffab7ce8f2) is an ancestor of compiler HEAD

### Skill — `PASS`
- [x] skill schemaFingerprint == compiler (got 8fca1c4c849e2a8517a2277c9e68269932140d193f953e515595b44a98bb343a)
- [x] skill codecFingerprint == compiler (got 3e7e3555eefe1e030d254df0a22c62fdc3a139f92e39da34f6d841f8bfa1689e)
- [x] skill languageVersion == compiler
- [x] skill catalogSchemaVersion == compiler
- [x] skill compilerCommit (1204e1f7dfac7af8df0813d1514fac629765af69) is an ancestor of HEAD
- [x] skill docsCommit (a38b62b819e48fea73dfd0636eb6de4dd08c2713) == docs HEAD (a38b62b819e48fea73dfd0636eb6de4dd08c2713)

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

### Pure+Pack — `FAIL`
- [x] Pure+Pack commit == 141b9cc96cf1df3f9d1593548dc49ae8a46a256a
- [x] Pure+Pack commit object present
- [x] Pure Original commit object present (read-only)
- [ ] deterministic generator byte-parity with 141b9cc (2 drift errors)
