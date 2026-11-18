# Native action result identity at the canonical consumption boundary

A real CLI audit found that some native.action expressions returned a generated ActionOutput UUID even though the emitted action had no UUID or retained a different explicit UUID. Twenty-one frozen-binary specimens exposed the difference between an unused raw action and a result that was actually consumed. The first focused Go oracle reproduced 13 failures across 23 source cases. This was a serialized producer/reference defect; no native-device cause is inferred.

## Corrected behavior

The existing canonical session now associates each native action expression with a compiler-private result handle. Normal binding, alias, reference, collection and formatted-text lowering carry that handle. An actual canonical argument or ordinary native parameter consumer resolves it to the emitted producer's identity.

For ordinary native parameters, a missing generated output UUID is added only when a consumer needs it. For exact native.plist parameters, the compiler retains the supplied bytes and requires a usable explicit UUID before consuming the result. An unused let binding or unused alias remains accepted without introducing a UUID or another action. A var initializer already emits a Set Variable consumer, so a missing raw output identity is diagnosed even if that mutable variable is not read later.

| Condition at actual consumption | Result |
| --- | --- |
| Exact parameters contain a non-empty string UUID | Reference targets that exact identity, retaining its case |
| Exact parameters have no usable UUID | NATIVE_OUTPUT_ID_MISSING |
| UUID argument conflicts with the identity inside exact parameters | NATIVE_OUTPUT_IDENTITY_CONFLICT |
| Consumed identity names more than one final native producer | NATIVE_OUTPUT_AMBIGUOUS |
| A private handle reaches an opaque exact payload without normal resolution | NATIVE_OUTPUT_REFERENCE_UNRESOLVED |

Exact raw parameters are also protected from automatic CustomOutputName renaming, including an explicit native.ref alias. Parameter map identity remains stable across the existing debug date-backfill insertion. Finalization checks that consumed producer identities still exist, are unique, and have not changed after consumption.

This is not a global validity rejection for imported or opaque native graphs. Unused raw duplicates and explicitly supplied opaque dangling references remain preserved, including the existing SOURCE_INVALID control. An exact workflow header cannot exempt a newly consumed, unresolved compiler result. No raw UUID is rewritten merely to make that invalid source look valid.

## Release profile boundary

The result-resolution hook respects the existing Release removal of diagnostic macros. Argument side effects still lower, but a removed debugTrace consumer does not require a result UUID that is never emitted. The original new-hook draft reproduced this boundary failure; the focused Debug/Release control is now green.

The exemption applies only to the existing removable diagnostic macro set. A real output consumer remains checked in Release, and require remains checked. No build-profile implementation or debug standard-library emitter was replaced.

## Verification

The final focused suite passes 32 real source cases, four Debug/Release cases, typed-value preservation, and three finalization corruption controls. It covers direct use, unused and used aliases, mutable initialization, inline native calls, explicit identity, lower-case identity, formatted text, dictionaries, lists, late duplicate producers, raw renaming protection, date backfill, and opaque invalid-wire controls.

The argument traversal changes only compiler-private references. Its preservation control verifies maximum uint64, signed integers, zero, false, binary bytes, dates, empty maps/lists, and ordinary UUID-looking dictionary fields without changing their Go representation. It does not serialize and parse values through JSON or add a second emitter.

The executed compatibility selection includes existing native escape/verification, P10R8 fidelity, debug, build-profile, and new runtime-diagnostic graph tests. The entire internal/language/lower package also passes. The exact commands, exit codes, timings, source digests, and causal RED evidence hashes are in raw-native-output-identity-validation.json.

Only three hook lines change in lower.go's native-action branch; the function/block scope work is separate. The existing canonical backend owns the result association and checks. The quarantined IR-only migration emitter is unchanged, and this report makes no new guarantee for that non-production path.

The committed CLI binary and combined integration/private-corpus gates are recorded separately. Native execution of the new compiler result paths is not claimed by these structural tests. Existing original runtime fixtures and strict expected outputs remain unchanged.
