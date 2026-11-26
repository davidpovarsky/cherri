# Language and macro boundaries

Use the compiler and documentation commits in `compatibility-manifest.json`.
Run `doctor.sh` before using a cached toolchain. A development source change
needs a fresh coherent compiler/docs/Skill tuple before installation; matching
capabilities alone do not authenticate that tuple.

## Typed values

V2 local nested Lists, Maps, Optional values and JSON null retain their declared
shape through supported source-owned storage and consumers. Mutable loop
sequences support branches, repeated and zero-iteration paths, immutable
snapshots, exact-type assignments and append. An explicit annotation or an
agreeing concrete source can provide an empty List's element type. Conflicting
types, unrelated unknown values and unsupported representation boundaries must
still be diagnosed.

`none` is absence. `false`, zero, empty text, empty List/Map and JSON
`null` are distinct values. A private carrier used to preserve those distinctions
is not a general native object, a raw plist null or a serializable public API.
Use compiler-approved accessors; never manually construct or decode its private
marker fields. Preserve native import/export structures instead of substituting
these carriers for arbitrary user Dictionaries.

## Scoped macros

| Macro | Implemented authoring contract | Boundary to retain |
| --- | --- | --- |
| `require` | Without a `type` or `nonEmpty` guard, an actual Bool is tested once; false alerts and stops, and success forwards the same value. Typed presence guards remain separate. Both profiles keep the semantic guard. | Do not infer truthiness from Number, Text, Optional or AnyContent. `require(false)` is a false condition; `require(false, nonEmpty: true)` checks presence. Native failure/dialog/cancel behavior needs actual runtime evidence. |
| `measure` | Shared Legacy/V2 value-or-Void body executes once in source order. Debug adds timers; Release preserves body effects. A Void body has no fabricated result and cannot be consumed as a value. | A native stop can prevent timer completion. This is not try/finally or a cancellation handler. |
| `cache` | A literal key and finite nonnegative TTL select the implemented Text/Number/Bool archive. A hit skips the body; a miss executes once. TTL zero bypasses storage and clocks. Positive TTL preserves authored Previous across its helper sequence. | Requires the version-27 Stored Content actions. Complex values, File/Date/object persistence, storage-error recovery, concurrent atomicity and general serialization are not implemented by the scalar archive. Missing-key, clock and persistence behavior require native qualification. Sensitive values need explicit author marking and opt-in. |
| `paginate` | Bounded GET pagination supports URL, cursor, page and offset markers, request headers, item/page caps, origin/cycle guards and an optional immutable page/Void callback. The callback receives the validated page once, before the next marker; zero budgets execute no callback. | A callback cannot silently return selected items. Typed selectors/returned items, Link headers, HTTP status/recovery/redirect policy, generic cancellation and complete native runtime remain separate obligations. |
| `retry` | The planned general retry contract remains unqualified. | Do not advertise generic error catching or status/cancel recovery by wrapping an arbitrary native action. Use an explicitly supported, tested workflow for the actual operation. |

`debugLog`/`debugLabel` preview modes retain their dedicated native evidence
and object-preservation rules in [debug logging](debug-logging.md).
`debugExport` offers explicit payload/redaction controls and opt-in label/item
Creation Date metadata. Creation Date is item metadata, not the log execution
timestamp; absent, ambiguous and unsupported metadata remain observable.
Generated JSON/envelope structure is not proof of a returned native File, File
byte identity, or successful native date extraction.

## Verification and delivery

1. Use `check --json`, compile unsigned, and inspect the relevant native action
   structure when exact value/identity preservation matters.
2. For an imported edit, retain the original plist and apply the exact allowed
   delta policy through the signing wrapper. Keep all unknown native fields.
3. Record actual signing, import and execution separately. An `AEA1` prefix or a
   valid plist does not prove the final workflow ran successfully.
4. Keep Pure Original on its pinned upstream engine. Pack actions are an
   explicit opt-in route; do not silently enable Pack for a Pure request.
