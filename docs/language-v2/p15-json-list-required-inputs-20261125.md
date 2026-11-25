# T05/L01 required authored inputs and representation investigation

This checkpoint adds fifteen causal capability regressions at compiler source
`82b8201dc3b38fd20a9ae8eafc5e50e540e20403`. No production compiler or
immutable acceptance contract changes in this checkpoint. Execution is pending
until the bounded clean-source owner records the result.

The cases cover first-class JsonNull, Optional<JsonValue>, function forwarding,
a Map null field, Optional List/Map elements, None versus present empty Text,
literal and dynamic nested Lists, checked index, iteration, mutable snapshots,
structured function inputs, and a dynamic List loop payload.

These tests check the first necessary boundary: the real compiler must accept
the authored input and emit a real plist without a fabricated null value. They
are not full semantic or native-runtime proof, are not registered as complete
acceptance producers, and cannot independently close T05, L01, RP20, or BRG15.
A passing implementation also requires independent complete graph and causal
corruption checks, source-bound layout provenance, and the required actual
Apple observations.

## Existing evidence and boundaries

The original contracts are unchanged:

- T05 distinguishes supplied presence, Optional absence, and JSON null.
- L01 preserves nested shape, cardinality, and order; silent flattening or
  omission is forbidden.
- RP20 covers false, zero, empty Text/List/Map and Optional/JSON-null contexts;
  it explicitly forbids fabricating plist null.
- BRG15 requires shared function transport, nested/recursive calls, defaults,
  caller continuation/input, and complete error propagation, including iOS.

The historical native observation at source
`03f17a785f31f7bdc8c7607bef1ac9a16c28d3b4`, run `44713853346`, is described in
`docs/apple-runtime/p10r15/nested-list-native-observation.md`: nested matrices
were observed through a Dictionary JSON body; the experimental item-type-5 key
was absent, not null. The original source is retained. The earlier WorkflowKit
serialization observation `44693452074` alone did not prove that candidate.

The current compiler therefore correctly rejects raw native/plist null instead
of silently changing its meaning. That rejection must remain for unimplemented
raw boundaries. A general solution needs explicit compiler-owned typed value
transport and JSON serialization boundaries; an arbitrary Dictionary marker
must never be treated as compiler provenance. The existing Dictionary array
and ordinary present-value controls remain independent positive controls.

No unchanged Apple probe, paid provider request, release, or acceptance run is
started by this checkpoint.
