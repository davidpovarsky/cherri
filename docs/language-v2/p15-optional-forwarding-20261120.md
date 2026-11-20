# Compiler-owned Optional forwarding

## Scope and result

Clean compiler source **7571baca9e6e86ac5c92205b93a51630906ba993** preserves independent value presence when an Optional is passed through a user-defined function argument. Supplied None, supplied empty values and an omitted argument have distinct representations. The affected existing root selection passed 1,061 named tests (124 top-level) in 23.610 seconds; full types, analysis, lower and service packages passed 75 named tests (70 top-level) in 6.438 seconds. The modules package built and has no test files. Both runs exited zero with no named failure/skip and clean Git state before and after.

This is compiler/analysis/native-structure evidence, not an Apple execution. The immutable 179 requirement files and original native runtime fixtures/expected outputs were not changed. No complete requirement is credited by this component.

## Causal baselines retained

Tests-only 83a613bc8a086900b6be6072d05505f05fc0993c reproduced all 15 initial forwarding failures: compiler and editor rejected immutable/mutable/inferred None, aliases, typed function results, parameter forwarding, named calls and a later-argument mutation. The real result includes false, zero and empty Text/List/Map cases. It exited 1 in 16.468 seconds, with no pass/skip, raw SHA-256 68f93e414ee3fb41deb31ed7ab2d0f79625e7a62b4435539fbe3eeaea6ab22f3.

The first opaque-output control at ccf635426fc98885a8cbe0002a67399ec36785b6 used Text?. Existing type checks correctly rejected AnyContent before emission. That failed test expectation is preserved and is not counted as a native leakage finding. Corrected tests-only 85e2dc7111fec7e93dcb41af714ffef84453a48f used the compatible AnyContent? destination. All five argument/result/binding/assignment/default routes accepted native Nothing and emitted bytes without compiler-owned presence. This genuine red run exited 1 in 13.875 seconds; raw SHA-256 b5e900f5d2b655bbe5e91cbc74ee129658617fc499e776990cc4a3b75cb307dd.

The three dirty implementation attempts remain original artifacts. The first left inferred None rejected because Unknown? alone is insufficient type evidence. The second exposed an independent zero-parameter positional-mirror oracle assumption and an extra diagnostic after an already-invalid compound assignment. The final dirty attempt passed 555 named tests. The final clean runs below, rather than any dirty run, establish the repaired source.

## Protocol and source-order behavior

The private function envelope keeps its existing magic, caller input, function identity and has_arg_n flags. has_arg_n selects a supplied argument versus an omitted/defaulted argument. For an Optional parameter, a supplied arg_n now references a Dictionary with the marker _cherri_optional_argument_v1, a Text has_value flag (0 absent, 1 present), and a value field only on the present branch. A None carrier is still a present Dictionary; it does not require a plist-null value or an empty-Text alias. The existing _cherri_optional_result_v1 result protocol is unchanged.

The callee first selects the supplied carrier or evaluates its default under has_arg_n == 0. It then reads has_value and emits either the actual value read or Nothing. A default remains unevaluated when explicit None is supplied. Existing Boolean argument representation and non-Optional function payloads are retained; this component does not establish native content-class fidelity beyond its actual structural evidence.

Parameter identities are bound without executing expressions. The primary argument and following written arguments execute once in written order. When a later written argument mutates an earlier Optional binding, lowering captures the independent presence flag immediately before the corresponding data snapshot. All Optional carrier construction is deferred until every authored argument has evaluated, so a pure metadata Dictionary cannot become the implicit input of a later authored native action. Complete graphs and causal corruption controls cover those edges.

Analysis records whether a value has compiler-owned presence and separately records a path-local known-None fact. These are not native truthiness observations. Present semantic types exclude Unknown, AnyContent, Void and JSON null as evidence. Optional function parameters/results, None and tracked bindings supply the required metadata; opaque native content is rejected precisely instead of receiving a fabricated present flag. Known-None facts permit an inferred None alias to satisfy an Optional destination without widening Unknown assignability. Assignments update the facts, branch merges retain only common facts, and writes in a loop invalidate a prior known-None fact before analyzing a later iteration.

The codec descriptor is deliberately refreshed to v2.2 for the changed private Optional argument representation. Its exact bytes and component fingerprint are retained in manifest.json. Integration must preserve other independently approved descriptor clauses and recompute the combined tuple; this component fingerprint is not a final deployment tuple.

## Verification and oracle integrity

All 21 forwarding sources are exercised through compiler/editor routes and independently authored complete native graphs, including the four exact function sources previously required to fail by the absence guard. Those four are now positive controls; the remaining container/key rejection cases and zero-byte checks remain active. Module controls use the defining module's Optional alias despite a conflicting caller alias and require exactly the expected definition/call dispatch occurrences.

The full graph checks operations, serialization kinds, source order, control groups, producer identities, all argument/default/result edges and the final consumer. Seventy-two causal corruptions reject wrong carrier markers/flags/types, empty-Text substitutions, argument omission, missing carrier edges, value-as-presence, truthiness, missing Nothing, wrong joins, missing call input and duplicated calls. Additional order controls reject a data reference substituted for the presence snapshot, a metadata carrier moved before an implicit-input action, and a default effect moved out of its supplied/default guard. Stale-None facts after assignment, branch writes and later loop iterations remain exact type failures with zero native bytes. Reused analysis cannot inherit an earlier source's presence facts.

Existing Optional wire oracles were updated to the new explicit carrier protocol; they do not accept both old and new representations. The non-Optional zero-parameter positional mirror still omits empty rows, as before. That mirror is not the representation of a supplied empty List: supplied empty List/Map values retain their explicit typed empty fields and their causal tests.

| Clean run | Named passes | Elapsed | Raw SHA-256 |
| --- | ---: | ---: | --- |
| Affected root selection, including old function/default/native/raw/List/container/F03 cases and new forwarding cases | 1,061 | 23.610 s | cd9348ad458650dc46b8de964b25fc377d1a2418c8d2fb81b1aef08485485309 |
| Full types/analysis/lower/service packages; modules build | 75 | 6.438 s | 212deabc7f81fa8181c6c7d8c813cc7ffc95cbeb304e5aa588e80a43fce94851 |

The manifest binds all original raw streams, stderr, process/results, two intermediate source snapshots, three tests-only source versions, the final 19 changed source files and the exact implementation patch. Original failure receipts were not rewritten. Staged Git blob bytes were verified against the manifest hashes.

## Remaining obligations

A dedicated source-bound signed diagnostic is still required to observe the actual Apple value/content behavior. Optional value-producing If/Menu/loop joins remain unimplemented when they do not carry independent presence. None inside ordinary List/Map values and the wider T05/L01/FN05/RP20 obligations remain open. No intrinsic native unencodability, complete container fidelity, final deployment acceptance or inherited native result is asserted here.
