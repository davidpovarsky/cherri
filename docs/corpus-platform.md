# Shortcut Corpus Platform

This document describes how Cherri expands from real Apple Shortcuts files
and which components share responsibility. It is the operating manual for
future corpus batches.

## Architecture

```text
real Shortcut evidence / corpus
        -> cherri --actions-json   (machine-readable catalog, shared source of truth)
        -> tools/shortcut-corpus   (ingest, sanitize, dedupe, classify, report)
        -> reports -> review -> action definitions in actions/*.cherri
        -> fork-specific changes recorded in docs/fork-action-provenance.json
        -> compiler / decompiler / docs / Skill / iOS palette / preview metadata
```

Cherri core is the only action knowledge base:

- `actions_catalog.go` builds one shared catalog consumed by:
  - CLI: `cherri --actions-json`
  - iOS bridge: `CherriActionCatalog` (same JSON schema)
  - corpus analyzer (`-catalog` or by invoking the CLI)
- `scripts/generate-action-docs.sh` renders documentation from the same
  definitions; never hand-edit generated signatures.
- Documentation repository default: `davidpovarsky/cherrilang.org`
  (fork of `electrikmilk/cherrilang.org`; override with `CHERRI_DOCS_REPO`).
- preview-shortcut (active fork `davidpovarsky/preview-shortcut`, pinned by
  commit in `ios/WebPreview/package.json`) accepts external metadata via its
  generic fallback path; custom renderers remain last resort.
- CodeEditorView shadow fork `davidpovarsky/CodeEditorView` exists but is not
  consumed yet.

## Ingesting a new batch

1. Place raw Shortcut files (JSON/plist) into an untracked inbox directory,
   e.g. `corpus-inbox/<batch-name>/`. Raw data must never be committed
   (`.gitignore` excludes `/corpus-inbox/`, `/analysis/`, state files).
2. Build the analyzer and generate the current catalog:

   ```sh
   go build -o dist/shortcut-corpus ./tools/shortcut-corpus
   go build -o dist/cherri .
   dist/cherri --actions-json > catalog.json
   ```

3. Analyze incrementally (repeatable `-in`; directories are recursive):

   ```sh
   dist/shortcut-corpus analyze \
     -in corpus-inbox/<batch-name> \
     -catalog catalog.json \
     -out analysis/<batch-name> \
     -state corpus-state.json
   ```

4. Review order:
   1. `analysis/<batch-name>/summary.md`
   2. `needs-review.json` (variants, unknowns, custom-implementation shapes)
   3. `new-actions.json` — open raw files only for these records
   4. `third-party-actions.json` — record app-intent surface area
5. Promote findings deliberately:
   - SAFE_CANDIDATE records get generated candidates under
     `analysis/<batch-name>/candidates/`; verify each signature against real
     Shortcuts behavior before converting into `actions/*.cherri`.
   - CUSTOM_IMPLEMENTATION_REQUIRED records need manual parameter
     construction/decompiler handling.
6. After definitions change, regenerate docs
   (`sh scripts/generate-action-docs.sh <dir> <categories>`) and commit the
   analyzer state only if desired — it contains sanitized structure only.

## Evidence rules

Corpus observations are `observed`, never `confirmed`. Requiredness,
defaults, enum completeness, OS availability, and exclusive types require
verification beyond single samples. Candidates carry their evidence status
and confidence so future sessions can weigh them correctly.

## Round-trip verification

```sh
sh tools/shortcut-corpus/scripts/roundtrip.sh tests/calc.cherri
# compile -> decompile -> recompile -> shortcut-corpus compare
```

The comparator ignores volatile metadata and UUIDs while comparing action
order, identifiers, parameters, serialization shape, fixed values, and
variable relationships.

## Upstream synchronization

Forks keep `origin` = our fork, `upstream` = original project:

| Repo | origin | upstream |
| --- | --- | --- |
| Cherri | davidpovarsky/cherri | electrikmilk/cherri |
| Docs | davidpovarsky/cherrilang.org | electrikmilk/cherrilang.org |
| Preview | davidpovarsky/preview-shortcut | electrikmilk/preview-shortcut |
| Editor (shadow) | davidpovarsky/CodeEditorView | mchakravarty/CodeEditorView |

`.github/workflows/upstream-sync.yml` opens a draft sync PR when upstream
Cherri advances; never blind-merge. It is long-term infrastructure pinned to
`main`: scheduled runs execute on the default branch and manual dispatches
check out `main` explicitly, so experimental `agent/*` branches are never a
sync base. For docs/preview, sync manually before substantial changes and
re-pin downstream commits afterwards.
