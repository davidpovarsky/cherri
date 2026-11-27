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
| `cache` | A literal key and finite nonnegative TTL select the Text/Number/Bool/Date archive. A hit skips the body; a miss executes once. Date uses an epoch-offset archive with type, cardinality and reconstruction guards. TTL zero bypasses storage, clocks and archive helpers; positive TTL preserves authored Previous. | Requires version-27 Stored Content actions. Dictionary/List/Optional/JsonNull/File and arbitrary-object persistence, recoverable native parser/storage errors and atomic concurrency guarantees remain unsupported. Date precision, reconstruction, fresh missing-key, expiry and cross-run persistence require Apple qualification. Invalid numeric Date archives can abort before guards. Sensitive values need explicit author marking and opt-in. |
| `paginate` | Bounded GET supports URL/cursor/page/offset markers, request headers, item/page caps, origin/cycle guards and an immutable page/Void callback. Literal `itemKey` selects an exact required Dictionary field; literal `itemType` selects dictionary/text/number/bool items with a corresponding typed List result. Selected values retain their native references after singleton/category guards. The callback runs after item validation and before next-marker processing. Zero budgets execute no request or callback. | The page callback cannot return selected items. Number/Bool modes stop before GET when their native type witnesses are indistinguishable. General selector callbacks, standalone nested Lists, Optional/null and absent/empty native-value preservation remain outside this contract. Link/response status/headers, redirect/recovery policy, generic cancellation and complete native runtime acceptance remain separate obligations. |
| `retry` | The planned general retry contract remains unqualified. | Do not advertise generic error catching or status/cancel recovery by wrapping an arbitrary native action. Use an explicitly supported, tested workflow for the actual operation. |

`debugLog`/`debugLabel` preview modes retain their dedicated native evidence
and object-preservation rules in [debug logging](debug-logging.md).

### Local debug export

`debugExport` preserves the original native log. With `redactSensitive:true`
and `redactKeys` omitted or empty, all payloads remain redacted. A nonempty
literal `redactKeys` explicitly enables selective Dictionary diagnostics:
protected fields are checked before their values are read, other Text field
values remain redacted, and non-Dictionary top-level payloads remain redacted.
With `redactSensitive:false`, Text, native Number display text, ISO 8601 Date
presentations and bounded Dictionary field diagnostics are enabled; nonempty
`redactKeys` still filters protected fields.

Dictionary output is explicitly nonlossless diagnostic data. Unsupported native
or nested values, multiple-item fields and absence/null/empty ambiguity have
explicit statuses. Source Bool, List and null distinctions are not reconstructed.
Field names and unprotected values in explicit unredacted mode may contain
sensitive content; selective filtering does not detect every secret.

Labels and item Creation Date metadata remain separately opt-in. Labels stay
redacted with `redactSensitive:true`. Creation Date is item metadata, not the log
execution timestamp; absent, ambiguous and unsupported metadata remain observable.
No upload, share, save-to-provider or sync action is implicit. Generated JSON,
a filename or a declared File return type does not prove a returned native File,
exact File bytes or successful native date extraction.

## Verification and delivery

1. Use `check --json`, compile unsigned, and inspect the relevant native action
   structure when exact value/identity preservation matters.
2. For an imported edit, retain the original plist and apply the exact allowed
   delta policy through the signing wrapper. Keep all unknown native fields.
3. Record actual signing, import and execution separately. An `AEA1` prefix or a
   valid plist does not prove the final workflow ran successfully.
4. Keep Pure Original on its pinned upstream engine. Pack actions are an
   explicit opt-in route; do not silently enable Pack for a Pure request.
