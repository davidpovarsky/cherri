# Make Archive format wire codec

The public `makeArchive` signature and its `archiveFormat` enum stay unchanged.
Literal formats use the native IDs retained in
[the macOS 27 toolkit metadata](apple-runtime/macos-27/toolkit-registry.json)
for `is.workflow.actions.makezip` / `WFArchiveFormat`. The metadata distinguishes
each ID from its displayed title.

| Public source spelling | Native ID |
| --- | --- |
| `.zip` | `zip` |
| `.tar.gz` | `tar.gz` |
| `.tar.bz2` | `tar.bz2` |
| `.tar.xz` | `tar.xz` |
| `.tar` | `tar` |
| `.gz` | `gz` |
| `.cpio` | `cpio` |
| `.iso` | `iso` |

## Compatibility and source ownership

The bounded codec in [archive_format_codec.go](../archive_format_codec.go)
runs in the shared action serializer for both Legacy and v2. It converts only
known literal values on this action's existing enum parameter. Omission, a
skipped optional argument before `name`, runtime references, the file input and
the archive name keep their existing encoders. The public default remains
`.zip`; an omitted format remains absent from the native parameters. Explicit
`.zip` writes `zip`.

The decompiler maps the eight canonical native IDs back to the public dotted
spellings. Historical dotted native values and unknown IDs use lossless
`rawAction` fallback rather than changing their native value when recompiled.
Explicit `rawAction` / `native.action` parameters remain unchanged.
The metadata's additional `cpgz` format is not added to the public enum.

A converted format is not a direct source-to-wire preview edit origin.
`WFArchiveFormat` stays read-only in that direct-edit path; an ordinary
`WFZIPName` literal retains its existing editable origin. Editing source and
compiling again still uses the normal enum validation.

The callable name, signature, parameter keys, enum domain, default, action
count, and generated schema are unchanged, so catalog or public consumer
schema regeneration is not required by this correction. CLI and mobile
compilers share the corrected backend. No UI implementation or new public
codec/schema is introduced.

## Verification boundary

[The focused Go tests](../archive_format_codec_test.go) cover six families:

1. The eight mappings in both language modes, with expected IDs read
   independently from retained Apple metadata.
2. Omitted format/name, a name after an omitted format, and explicit default.
3. Rejection of bare native IDs, `.cpgz`, and unknown public enum values.
4. Runtime variable/output references and explicit raw/native parameter values.
5. Exact semantic native action graph roundtrips before any provenance overlay,
   including raw preservation of historical dotted and unknown IDs.
6. The preview origin boundary and the unaffected archive-name edit.

[The Cherri fixture](../tests/archive_format.cherri) records the eight public
source spellings for compile-only use.

```sh
GOMAXPROCS=2 CGO_ENABLED=0 go test -p=2 -count=1 -timeout=5m \
  -run '^TestP10R15ArchiveFormat' -json .
```

This is source and wire-metadata compatibility evidence. It does not establish
Apple execution, File output, full macro acceptance, signed import, physical
iPad behavior, or the formal 179-case gate. The separate native experiment
failed with both dotted and bare `zip`; this correction neither explains nor
claims to repair that runtime failure.

## Recorded component verification

The 2026-11-27 run used Go 1.27.4 on Linux/arm64, shared caches, and the
command above. The base was `992729d9fbd56bf811e7905916c07f8944e2d1f7`.

- RED: the eight wire mismatches failed in both modes (16 subtests; exit 1).
- GREEN: six families and all 52 subtests passed (58 named PASS; exit 0).
- One earlier verification launch was interrupted by a 20-second outer
  execution timeout with no JSON test events. It is retained as an
  infrastructure interruption, not a test result.

Source snapshots were unchanged during the completed RED and GREEN runs.
The original receipts are retained under `archive-format-evidence` in the
authorized Oracle workspace:

| Receipt | SHA-256 |
| --- | --- |
| `red-01/receipt.original.json` | `fb9aff70db0c62393d0da4af7539d160ba0b779b85156b6cf9bae16fa6d8e610` |
| `green-02/receipt.original.json` | `fd12c51e6a52602a60a17d7f202242f4fe139ce110ef94b576261f44706783fc` |
