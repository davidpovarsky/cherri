# P15 Legacy nil-clear migration

The canonical nil fixture assigns getFile(nil) to a mutable Legacy variable, uses it as media, and then explicitly clears it with nil. The previous migration produced an unannotated V2 variable inferred as File followed by a nil assignment. The stricter T04 assignment evidence correctly rejects that assignment to non-optional File.

The repair expresses the source's existing clear contract as an Optional annotation at its own generated declaration. The same fixture now contains var media: File? = getFile(nil), while the clear statement and native emission remain unchanged.

## Evidence and scope

The migrator records exact generated Legacy binding positions. A structural traversal resolves explicit nil/none assignments through lexical scopes, including function parameters and loop variable masks. Authored V2 declarations are not candidates. Comments, strings, unrelated assignments, and unused copy bodies do not establish a clear contract for a runtime binding.

Type knowledge comes from the existing analyzer and canonical registry. Because analysis expands copy/paste in place, it runs on a separate tree made by the existing parser; formatting uses the original tree and retains copy/paste source form. An inferred known type is translated structurally to its matching annotation and wrapped in Optional. Existing Legacy annotations retain their declared inner type. Opaque or unrepresentable inferred types remain unchanged and retain their compiler diagnostics.

The analyzer's Unknown/AnyContent assignment rules, original 47 fixtures, migration formatting policy, native serializer, and parity normalization are unchanged. An Optional File declaration still rejects an opaque value as proof of File. Fresh authored shadows, loop variables, and function parameters cannot widen an unrelated same-named Legacy binding.

## Verification boundary

Production controls exercise File, Number, Text, false, zero, header offsets, typed Legacy declarations, branch/loop/function/copy/scoped-block bodies, known List/Map values, masked names, actual migrate/compile CLI paths, and unchanged canonical nil fixture parity. Causal wire faults add an input to the clear, remove it, or retarget it.

This is source typing and native-structure evidence. Existing V2 nil assignment emits Set Variable with omitted WFInput; Legacy also emits a preceding Nothing action. The existing normalized comparator accepts that established difference. This migration change makes no Apple runtime clearing claim and does not change that emitter. A signed runtime check is a separate obligation.

A clean immutable execution receipt will follow the implementation commit.
