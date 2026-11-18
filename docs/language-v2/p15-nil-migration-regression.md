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

## Clean immutable receipt

Implementation source: 4e83628b5778c48fc1cb6a449eb35100bde320c2, based on 76c65fbf73f24bdc6d28465559948829f40dbe05. The [evidence audit](evidence/p15-nil-4e83628/evidence-audit.json) binds all raw execution files and retained synthetic native outputs.

- Six new nil-focused families passed, including 29 named results in their focused run. On the clean commit, the complete selected nil, menus, C02, T02, and T04 production command passed 151 named results.
- The full analysis, migration, and types packages passed 44 named results.
- The actual migrate CLI produced the explicit File? declaration; the actual Legacy and V2 compiler commands completed. The retained source and both unsigned native artifacts are hash-bound to that clean compiler.
- The canonical nil fixture is PROVEN_EQUIVALENT through existing normalization. All 47 original fixture byte hashes remained unchanged, and the source tree stayed clean before and after.
- All 17 representative action parity cases passed. The full differential gate remains FAILED on this exact base: 46 of 47 fixture paths passed, with only the already known date-comparisons.cherri type failure remaining. There were zero migration failures and zero emitted parity differences.

The clean gate did not include the independently developed Date repair. Its combined-source rerun is separate. No broad type relation, native reset guarantee, private-corpus result, or complete release closure is inferred.
