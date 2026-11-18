# P15 literal text and collection diagnostics

## Scope

S02 now has dedicated production evidence for exact ordinary and raw string
bytes through the V2 compiler and serialized native plist. L03 has dedicated
production evidence for duplicate literal keys and invalid index types.

L01 remains **UNVERIFIED**. The new tests establish nested Dictionary field
shape, cardinality, order and explicit empty values. They do not establish a
valid representation or runtime hierarchy for standalone native List<List>.

No linked repair/regression gate, Apple runtime requirement, physical-device
result or release closure inherits these local results.

## Causal defects and corrections

The initial tests ran against integrated base
`be960c1157b0da958915b8ddb359b8adf579d2fb` before the production correction.

1. Ordinary JSON, JavaScript, CSS and native-looking braced text inside
   Dictionary values was passed to the Legacy inline-reference recognizer.
   Eleven initial cases failed: ten value routes and one literal key acquired
   native variable attachments. The original text bytes were therefore changed.
2. Explicit empty arrays inside authored Dictionary fields omitted the native
   `Value` field through `WFArrayValue`'s historical omission tag. All four
   nested Dictionary routes failed the independent explicit-shape reader.
3. The 12 duplicate-key/invalid-index cases and five positive controls already
   passed. No analyzer policy change was required for L03.

The correction keeps authored semantic identity until the existing canonical
codec. A V2 ordinary string is a literal, while explicit f-strings retain their
reference segments. Typed Dictionary keys and text fields use the existing
native text structure without running the Legacy interpolation recognizer.
Runtime Dictionary literals in bindings, assignments, returns and yields reuse
the same typed Dictionary producer already used for Dictionary values in Lists.

The existing `WFArrayValue` gains an internal authored-empty flag. Its native
representation remains `WFArrayParameterState`. With this flag, the canonical
plist encoder retains an explicit empty `Value` array; without it, the historical
Legacy/raw omission shape is unchanged. Exact imported native payloads do not
enter the authored typed path.

## Executed test surfaces

| Requirement | Production assertions | Result scope |
| --- | --- | --- |
| S02 | Six literal families across ten expression/materialization routes, plus a literal Dictionary key | Exact UTF-8 bytes and absence of inferred attachments |
| S02 | Explicit f-string original producer UUID and native token range; Legacy interpolation control | Genuine references remain references |
| S02 | Removed backslash, removed braces, changed Unicode and fabricated-reference mutations | The independent literal reader rejects each corruption |
| S02 | XML and binary import/recompile, hybrid and lossless, V2 and Legacy | Exact native literal/container tree and opaque payload preservation |
| L03 | Duplicate ordinary/raw/nested keys; Text/Bool/List/Map indices; known dynamic Text index | Deterministic compiler rejection and matching editor-service diagnostic |
| L03 | Numeric/static/dynamic List index, Text/static/dynamic Map key, repeated key in separate Maps | Valid controls compile to actual plist |
| L01 partial | Four nested Dictionary routes with false, zero, empty text/list/map and ordered nested elements | Explicit typed Dictionary field preservation |
| L01 partial | Missing empty array, dropped map, removed/reordered elements, changed false and flattened nesting | Each mutation fails the independent shape reader |

The compatibility selection also executes existing call ordering/argument/enum,
lexical binding, native preservation, numeric/Dictionary List, codec, native
reference, require/debug/profile and editor insertion tests. The complete
lowerer test package is included. The clean source receipt records the exact
commands, hashes and results.

## Remaining standalone nested List gap

The checked-in Apple metadata describes List `WFItems` as
`WFContentArrayParameter` and permits content references. It supplies no verified
nested-array row type or promise of retaining a nested List hierarchy.
Dictionary metadata separately permits array fields; that contract cannot be
applied to a standalone List row.

The previous direct literal `[[], [1, 2], [[3], []]]` compiled to raw arrays in
`WFItemType=0` text rows. The compiler now fails closed for direct nested List
literals on this native List path with `E_NESTED_NATIVE_LIST`. Dictionary array
fields and exact raw/import payloads remain available. This diagnostic is a
bounded invalid-wire guard, not proof that every form of nested collection is
supported or that Apple can never represent one.

The acceptance runner executes the partial Dictionary tests as observations
and deliberately produces no L01 passing tier records. Its separate scope
test rejects promotion of these observations to full L01 acceptance.

The signed empty List count issue is distinct: an existing native diagnostic
compares an empty List with the cleared-variable mechanism. Its runtime
evidence and any production repair are recorded separately.

## Boundaries

- Ordinary V2 literal text never acquires a variable reference from its braces.
- Explicit f-string and native-reference identities remain on their established
  canonical encoding paths.
- Legacy interpolation and historical raw omission remain unchanged.
- Optional values, omission and JSON null are separate T05 work; these tests do
  not certify their wire contract.
- Fresh integrated corpus/consumer/platform gates remain required after
  integration. Local serialized-native evidence is not a Simulator execution.
