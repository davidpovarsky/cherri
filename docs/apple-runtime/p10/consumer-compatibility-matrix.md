# P10R5 — Consumer Compatibility Matrix

- **Generator**: `tools/p10r3/verify_consumer_compatibility.py`
- **Compiler HEAD at generation**: `6daaf525b131266f648d527176dfe8e74b4bd881`
- **Verdict**: `COMPATIBLE_WITH_EXTERNAL_BLOCKERS`

> `NOT_AVAILABLE_LOCALLY` and `BLOCKED_EXTERNAL` are never counted as `PASS`.

| Component | Source Commit | Schema Fingerprint | Codec Fingerprint | Tested Against | Status |
| :--- | :--- | :--- | :--- | :--- | :--- |
| Compiler | `6daaf525b131` | `8fca1c4c849e` | `3e7e3555eefe` | live `go run . --capabilities-json` | `PASS` |
| Documentation | — | `8fca1c4c849e` | `3e7e3555eefe` | cherri in-repo reference only | `NOT_AVAILABLE_LOCALLY` |
| Skill | `1204e1f7dfac` | `8fca1c4c849e` | `3e7e3555eefe` | docs a38b62b819e4 | `PASS` |
| Remote Builder | `1204e1f7dfac` | `8fca1c4c849e` | `3e7e3555eefe` | skill compatibility-manifest.json tuple | `BLOCKED_EXTERNAL` |
| iOS Bridge | `6daaf525b131` | `8fca1c4c849e` | `3e7e3555eefe` | schema.DefaultRegistry() (same registry as compiler) | `PASS` |
| iOS UI | `6daaf525b131` | `8fca1c4c849e` | `3e7e3555eefe` | iOS Bridge C ABI | `PASS` |
| Preview | `42ee7656e0cc` | `8fca1c4c849e` | `3e7e3555eefe` | ios/WebPreview lock + generic catalog metadata renderer | `PASS` |
| Pure+Pack | `141b9cc96cf1` | `8fca1c4c849e` | `3e7e3555eefe` | P10 catalog @ 6daaf525b131 | `PASS` |

## Check Details

### Compiler — `PASS`
- [x] schemaFingerprint is 64-hex
- [x] codecFingerprint is 64-hex
- [x] codecFingerprint independent of schemaFingerprint
- [x] actionCount == 505 (got 505)
- [x] languageVersion == 2.0

### Documentation — `NOT_AVAILABLE_LOCALLY`
- [x] cherri docs/language-v2/actions-reference.md fingerprint == compiler
- _Note_: cherrilang.org checkout not present at C:\nonexistent; site copy and smoke pin not verified.

### Skill — `PASS`
- [x] skill schemaFingerprint == compiler (got 8fca1c4c849e2a8517a2277c9e68269932140d193f953e515595b44a98bb343a)
- [x] skill codecFingerprint == compiler (got 3e7e3555eefe1e030d254df0a22c62fdc3a139f92e39da34f6d841f8bfa1689e)
- [x] skill languageVersion == compiler
- [x] skill catalogSchemaVersion == compiler
- [x] skill compilerCommit (1204e1f7dfac7af8df0813d1514fac629765af69) is an ancestor of HEAD

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
- [x] deterministic generator byte-parity with 141b9cc (0 drift errors)
