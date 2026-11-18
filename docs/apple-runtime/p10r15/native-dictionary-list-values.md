# Native dictionary values inside V2 lists

The signed R1 Bool/List run `44223779379` executed at compiler commit
`d00bc0b9e9a4d8c460623de3d883680b94289e04` and returned an error callback.
Its error description was redacted and no numeric error code was retained.
This change repairs a separately reproduced serialization defect; that callback
alone does not establish that this was the only native runtime failure.

## Reproduction and correction

Rebuilding the exact R1 fixture reproduces 30 native actions. Its source SHA-256
is `8337913a2b3b888bdfbba1abbfc28ddc8ce63b87a7383531a3ac03cab0b49e00`,
which matches the published R1 compiler manifest. The List action embeds raw
maps beneath `WFItemType: 0` and `WFValue`, then the shortcut attempts to use
those items as dictionaries.

Apple's checked-in metadata declares the List parameter as
`WFContentArrayParameter` and documents including a variable's contents. The
existing source `let first = {...}; let items = [first]` already emits a native
Dictionary producer and an exact ActionOutput attachment inside the List row.
The repaired literal path reuses that established behavior. It does not invent a
Dictionary field representation inside the List parameter or encode data as JSON
text. Explicitly serialized native/import rows and existing token strings retain
their existing handling.

Both immutable bindings and mutable-list materialization now create the native
Dictionary producers before the List action. Nested fields still pass through
the shared Dictionary codec. Item order and exact producer UUID links survive.
The original Bool/List fixture now emits 32 actions: its existing graph plus the
two required Dictionary producers.

## Evidence

The production-plist tests in `p15_dictionary_list_test.go` compare literal-list
output with separately authored Dictionary bindings. Immutable, mutable, and
reassigned lists all fail the oracle before the change and pass afterward.
The oracle rejects the original raw-map representation, a dangling producer,
swapped item order, and a text-coercion attachment.

The focused regression run contains 41 passing test events, including subtests.
It covers existing codecs, lossless Legacy/V2 import parity, native List editing,
measure value preservation, and the singleton-list require guard. The entire
lowerer package also passes. An actual CLI build and compilation of the unchanged
R1 fixture verifies two Dictionary producers, exact references, an explicitly
present native `false`, and the nested `WFArrayParameterState` values `a`, `b` in
order. Hashes, commands, and observed statuses are recorded in
[native-dictionary-list-validation.json](native-dictionary-list-validation.json).

These are local compiler and serialized-graph results. The corrected signed
shortcut has not been run on Apple hardware or Simulator in this stage; runtime
acceptance remains open. The existing fixture's final result checks first/last
names. Its nested false/array values have structural proof here, not an additional
independent native readback claim.

Rollback is a revert of this commit. The change touches only the existing V2 List
lowering path, focused tests, and this evidence. It does not change the native
import normalizer, shared codecs, signing service, debug tracing, or iOS UI.

## Authored dictionaries with native-looking keys

A follow-up causal test found that ordinary fields named `WFItemType`,
`WFSerializationType`, `Value`, and `WFValue` could be mistaken for an already
serialized List row or token. Eight immutable/mutable cases and one nested
Dictionary case failed before the follow-up. Real F-string/attachment rows and
exact imported native fallback already passed, and remain unchanged.

A source `MapExpr` inside a native List now retains its identity through the
existing `backend.SemanticValue` representation. The canonical adapter uses the
shared Dictionary, array, text, and reference encoders for those typed values.
It does not infer data identity from user field names. Dictionary array fields
remain inline data, retaining their existing nested serialization. Native/import
payloads use their existing path, including `EXACT_NATIVE_FALLBACK` where required.

The passing focused run includes the new causal cases, existing codec/import
parity, native List editing, measure/require, and the shared-codec architecture
gate. The full lowerer suite and actual CLI checks pass; exact counts, source and
log hashes are in [the reserved-name validation record](native-dictionary-list-reserved-validation.json).
The unchanged R1 Bool/List source still emits 32 actions. Signed runtime and the
combined private corpus gate remain separate pending verification.
