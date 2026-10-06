# Upstream Boundary Manifest — Language v2.0

**Baseline:** `559abceadda1dbf0cd7d7deb1a26315b8ac44c3f`
**Upstream:** `electrikmilk/cherri` (`a66db15b7f247f3121726c2a2b72becfeef3d96b`)
**Divergence:** 30 upstream ahead, 104 fork ahead

## Classification Categories

- **UPSTREAM_UNCHANGED**: Files originating from upstream, kept as-is
- **UPSTREAM_HOOKED**: Files with narrow fork-specific hooks added
- **FORK_OWNED**: Files created entirely by this fork
- **GENERATED**: Files generated from upstream definitions + semantic facets
- **EXCLUDED_FROM_IMPORT**: Upstream reference material not imported into active docs

## Upstream-Controlled Files (Preserved Unchanged)

### Action Definition Files (`actions/*.cherri`)
These 25 files are upstream declaration inputs. The language-v2 build-time adapter
reads them; they are NOT converted to new syntax.

| File | Classification | Notes |
|------|---------------|-------|
| `actions/a11y.cherri` | UPSTREAM_UNCHANGED | Read by schema adapter |
| `actions/basic.cherri` | UPSTREAM_UNCHANGED | Read by schema adapter |
| `actions/calendar.cherri` | UPSTREAM_UNCHANGED | Read by schema adapter |
| `actions/contacts.cherri` | UPSTREAM_UNCHANGED | Read by schema adapter |
| `actions/crypto.cherri` | UPSTREAM_UNCHANGED | Read by schema adapter |
| `actions/device.cherri` | UPSTREAM_UNCHANGED | Read by schema adapter |
| `actions/documents.cherri` | UPSTREAM_UNCHANGED | Read by schema adapter |
| `actions/dropbox.cherri` | UPSTREAM_UNCHANGED | Read by schema adapter |
| `actions/images.cherri` | UPSTREAM_UNCHANGED | Read by schema adapter |
| `actions/intelligence.cherri` | UPSTREAM_UNCHANGED | Read by schema adapter |
| `actions/location.cherri` | UPSTREAM_UNCHANGED | Read by schema adapter |
| `actions/mac.cherri` | UPSTREAM_UNCHANGED | Read by schema adapter |
| `actions/math.cherri` | UPSTREAM_UNCHANGED | Read by schema adapter |
| `actions/media.cherri` | UPSTREAM_UNCHANGED | Read by schema adapter |
| `actions/music.cherri` | UPSTREAM_UNCHANGED | Read by schema adapter |
| `actions/network.cherri` | UPSTREAM_UNCHANGED | Read by schema adapter |
| `actions/pdf.cherri` | UPSTREAM_UNCHANGED | Read by schema adapter |
| `actions/photos.cherri` | UPSTREAM_UNCHANGED | Read by schema adapter |
| `actions/settings.cherri` | UPSTREAM_UNCHANGED | Read by schema adapter |
| `actions/sharing.cherri` | UPSTREAM_UNCHANGED | Read by schema adapter |
| `actions/shortcuts.cherri` | UPSTREAM_UNCHANGED | Read by schema adapter |
| `actions/storage.cherri` | UPSTREAM_UNCHANGED | Read by schema adapter |
| `actions/text.cherri` | UPSTREAM_UNCHANGED | Read by schema adapter |
| `actions/translation.cherri` | UPSTREAM_UNCHANGED | Read by schema adapter |
| `actions/web.cherri` | UPSTREAM_UNCHANGED | Read by schema adapter |

### Standard Library Function Source
| File | Classification | Notes |
|------|---------------|-------|
| `stdfunc.cherri` | UPSTREAM_UNCHANGED | Input for generated new-language equivalent |

### Core Compiler Files (Upstream-Derived)
These files originate upstream and contain fork hooks. Listed in order of sensitivity.

| File | Classification | Fork Hooks | Reason |
|------|---------------|------------|--------|
| `parser.go` | UPSTREAM_HOOKED | TBD: narrow v2 dispatch point | Route to new parser when available |
| `action.go` | UPSTREAM_HOOKED | Fork action definitions added | Extended action registry |
| `actions_std.go` | UPSTREAM_HOOKED | Fork complex actions added | Extended standard actions |
| `compiler_state.go` | UPSTREAM_HOOKED | Fork state fields if needed | Reset boundary |
| `shortcutgen.go` | UPSTREAM_HOOKED | Fork codec additions | Value generation |
| `decompile.go` | UPSTREAM_HOOKED | Fork decompiler entries | Decompilation support |
| `main.go` | UPSTREAM_HOOKED | CLI routing | Command dispatch |
| `args.go` | UPSTREAM_HOOKED | Fork CLI args | Additional flags |
| `shortcut.go` | UPSTREAM_UNCHANGED | — | Plist structure |
| `token.go` | UPSTREAM_UNCHANGED | — | Token types |
| `functions.go` | UPSTREAM_UNCHANGED | — | Function dispatch |
| `variables.go` | UPSTREAM_UNCHANGED | — | Variable handling |
| `includes.go` | UPSTREAM_UNCHANGED | — | Include mechanism |
| `packages.go` | UPSTREAM_UNCHANGED | — | Package mechanism |
| `references.go` | UPSTREAM_UNCHANGED | — | Reference handling |
| `output.go` | UPSTREAM_UNCHANGED | — | Output formatting |
| `copy_paste.go` | UPSTREAM_UNCHANGED | — | Copy/paste preprocessing |
| `raw_actions.go` | UPSTREAM_UNCHANGED | — | Raw action support |
| `search.go` | UPSTREAM_UNCHANGED | — | Action search |
| `signing.go` | UPSTREAM_UNCHANGED | — | Signing support |
| `glyphs.go` | UPSTREAM_UNCHANGED | — | Glyph data |
| `automations.go` | UPSTREAM_UNCHANGED | — | Automation triggers |
| `embedded.go` | UPSTREAM_UNCHANGED | — | Embedded assets |
| `version.go` | UPSTREAM_UNCHANGED | — | Version constant |
| `docs.go` | UPSTREAM_UNCHANGED | — | CLI docs output |
| `import.go` | UPSTREAM_UNCHANGED | — | Import handling |

## Fork-Owned Files

### Existing Fork Additions
| File/Directory | Purpose |
|---------------|---------|
| `actions_catalog.go` | Machine-readable catalog builder |
| `actions_catalog_test.go` | Catalog tests |
| `app_intent_test.go` | App Intent integration tests |
| `compiler_isolation_test.go` | State isolation regression tests |
| `decomp_includes_test.go` | Decompilation include tests |
| `fork_negative_test.go` | Negative test cases |
| `fork_provenance_test.go` | Provenance registry tests |
| `fork_roundtrip_test.go` | Round-trip verification tests |
| `ios_bridge.go` | iOS C/JSON bridge |
| `toolkit.go` | Apple ToolKit integration |
| `ios/` | iOS app (SwiftUI) |
| `tools/shortcut-corpus/` | Corpus analysis tool |
| `tools/shortcut-runtime/` | Runtime extraction tools |
| `skills/cherri-shortcuts/` | AI Skill |
| `docs/` (fork-specific) | Fork documentation |
| `scripts/` | Fork build/CI scripts |
| `.github/workflows/` | Fork CI workflows |

### New Language v2 Modules (To Be Created)
| Directory | Purpose |
|-----------|---------|
| `internal/language/source/` | Document IDs, spans, Unicode |
| `internal/language/syntax/` | Lexer, parser |
| `internal/language/types/` | Semantic type system |
| `internal/language/schema/` | Assembled action registry |
| `internal/language/analysis/` | Resolver, checker |
| `internal/language/ir/` | Semantic + Native IR |
| `internal/language/lower/` | Lowering to native actions |
| `internal/language/service/` | Shared analysis/completion API |
| `internal/language/protocol/` | JSON message schemas |
| `internal/language/lsp/` | LSP adapter |
| `tools/language-schema/` | Build-time definition ingestion |
| `tools/language-migrate/` | One-time legacy migration |
| `tools/language-acceptance/` | Acceptance matrix runner |
| `tests/language-v2/` | New language test fixtures |
| `docs/language-v2/` | Language documentation |

### Root Bridge Files (To Be Created)
| File | Purpose |
|------|---------|
| `language_registry_adapter.go` | Bind schema to existing serializers |
| `language_emit_adapter.go` | Bridge new IR to existing emitters |
| `language_cli_adapter.go` | Route CLI commands to new language |
| `language_bridge_adapter.go` | Route iOS bridge to new service |

## Generated Files (To Be Created)

| File | Input | Output |
|------|-------|--------|
| `internal/language/schema/generated_registry.go` | `actions/*.cherri` + `actions_std.go` + facets | Typed action schemas |
| `internal/language/schema/generated_stdfunc.go` | `stdfunc.cherri` | New-language stdfunc equivalents |
| `internal/language/schema/generated_metadata.go` | Generation run | Input hashes, versions |

## Semantic Facets

Facets are field-specific corrections/enrichments to upstream-derived definitions.
Location: `tools/language-schema/facets/`

Each facet:
- Keyed by stable definition identity
- References original definition field
- Contains only additions/overrides
- Includes provenance + expected upstream hash
- Validated during generation: upstream assumption mismatch → build error

## Upstream Sync Procedure

1. Fetch upstream: `git fetch upstream`
2. Inspect changes: `git diff HEAD...upstream/main -- actions/ stdfunc.cherri parser.go action.go actions_std.go`
3. Create sync branch: `git checkout -b sync/upstream-YYYYMMDD`
4. Merge upstream files: merge/cherry-pick upstream-owned changes
5. Regenerate schema: `go generate ./tools/language-schema/...`
6. Validate facets: generated schema tool reports assumption conflicts
7. Run migration tests: `go test -run TestLanguageV2 ./...`
8. Run existing tests: `go test -run TestCherriNoSign ./...`
9. Review and merge sync branch

## Documentation Repository Boundary

| Content | Location | Classification |
|---------|----------|---------------|
| Upstream language docs | `cherrilang.org/language/` | EXCLUDED_FROM_IMPORT |
| Fork active language guide | `cherrilang.org/fork-language/` | FORK_OWNED |
| Generated action docs | `cherrilang.org/fork-language/actions/` | GENERATED |
| Navigation/routing patches | `cherrilang.org` (small patches) | FORK_OWNED |
