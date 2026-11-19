# P10R15 generated stdfunc acceptance proof — 2026-11-19

## Verified scope

The preserved `runJS` and `chooseFromVCard` implementations now have a complete production test selection for **IM03, RP28 and BRG20**. The clean replay used source commit `2e8647ff8b21b163d35b258b697f723b11b28aa5`, based on the integrated candidate `b2961bbd51e45dd2a1b21d9fd2fb9f3390ed5f74`.

| Requirement | Actually executed tiers | Result |
| --- | --- | --- |
| IM03 | unit, native-structure | PASS, two verified records |
| RP28 | generation, native-structure | PASS, two verified records |
| BRG20 | unit, service, native-structure | PASS, three verified records |

Here, native evidence means the serialized Shortcuts action graph. These tests do not execute Apple Shortcuts. The immutable requirements, source library and tracked generated snapshot are unchanged. The reports retain `closure_complete: false`; no other requirement inherits a pass.

The implementation is `e74fcb9e618b3b7852c07a00febac71b7574fada`. The shared contract-derived inventory fix is applied as `69777f9452869e9549c1b9616b6d178ab113deeb` (equivalent to the integration team's `d55f58a85c35c7ca065b4be5d64176e50aa61946`). Commit `2e8647ff8b21b163d35b258b697f723b11b28aa5` explicitly adds RP28 and BRG20 to that independent inventory.

## What the tests establish

The real `tools/language-stdlib` executable generates a temporary snapshot from the original `stdfunc.cherri`. Its output is byte-identical to the tracked generated Go source. An independent Go syntax-tree read extracts the generated string constant and compares it with the original bytes and the compiled module provider. A repeated generation is deterministic.

The generator's negative controls also run the real executable. Check mode rejects a stale snapshot without replacing it. A temporary, behavior-bearing filename change changes both generated content and its source hash. Empty and invalid UTF-8 input fail while preserving the existing destination. These temporary files do not modify the preserved library.

The raw native oracle checks both import aliases `std` and `Std`. It binds the dispatcher to the actual Shortcut input dictionary and checks both generated function bodies, argument slots, call payloads, supplied-argument markers, dependencies, return identities and consuming output. For `runJS`, this includes the preserved replacement operation, HTML data URI, rich-text coercion and URL decode mode. For `chooseFromVCard`, this includes the original filename, contact coercion, prompt input and chosen-item return edge.

Fourteen distinct graph corruptions are rejected for each alias: replacement, HTML carrier, decode mode, rich-text coercion, filename, contact coercion, prompt edge, parameter slot, call target, call value, return edge, missing operation, duplicate UUID and an unmodeled parameter. The public source and expected behavior are not rewritten to obtain equality.

A separate differential invokes the real preserved legacy compiler and current V2 compiler. It compares their complete emitted action graphs, at the same API boundary: the embedded legacy API intentionally omits default workflow headers. Both required legacy action includes are independently removed and must produce actual dependency failures. Whole-workflow fixture parity remains a separate integration gate.

BRG20 independently executes its complete 18-test selection. It covers local and nested module namespaces, aliases and canonical identity, actual service dependencies, duplicate names, refresh/export behavior, uppercase aliases and export rejection, cycles and duplicate aliases, rejected runtime initialization, safe legacy-include migration, the original stdfunc source, separate service-package execution, generation, raw native behavior and the legacy differential. It does not reuse an IM03 result as a substitute for execution.

## Acceptance integrity

The IM03 producer now records its native tier only after the complete five-test selection succeeds and is discovered. RP28 and BRG20 have their own complete selections and exact commands. The tests validate record identities, source SHA, producer names, command selection, required assertions and immutable tiers.

Negative receipt controls reject a missing tier, unrelated producer, truncated selection, omitted required assertion, stale source SHA, invented iOS tier and inherited requirement ID. IM03 additionally retains its unit-only and fake-native rejection controls and requires the separate service-package test. Audit checks preserve all 179 rows and prevent unrelated rows from receiving these scoped passes.

## Clean replay

The targeted clean run completed with the Go test package's terminal **PASS** event: **31 named test passes, five top-level passes, zero failures and zero skips**, with package elapsed time **117.733 seconds**. Git source and clean-tree checks agree before and after the run.

The wrapper initially failed after the tests while extracting a long JSON receipt from a single Go `Output` event. Go had split that output into chunks. The receipts were recovered by concatenating each specific test's complete output stream and parsing the resulting original JSON line. The semantic tests were not rerun for this postprocessing issue. The raw log and wrapper disclosure are retained; the wrapper's exit 1 is not described as a successful wrapper exit.

Evidence is in [the scoped evidence directory](evidence/p15-stdfunc-native-20261119/):

- [Focused command, source and terminal package result](evidence/p15-stdfunc-native-20261119/focused-run.json)
- [Unmodified Go JSON event log](evidence/p15-stdfunc-native-20261119/focused-run.jsonl)
- [IM03 source-bound records](evidence/p15-stdfunc-native-20261119/im03.json)
- [RP28 and BRG20 source-bound records](evidence/p15-stdfunc-native-20261119/rp28-brg20.json)
- [Cancelled broader replay disclosure](evidence/p15-stdfunc-native-20261119/broad-run-cancelled.json)

## Broader gate remains separate

An earlier clean full tools-package replay was cancelled to stop duplicate broad runs competing with the integration gate. It retained 13 named passes and an existing binding-route build failure: `TestP15BindingDefaultsPublicRoutes` reported `build current production CLI: exit status 1` after 91.58 seconds. The shared helper already captures combined output and has a 90-second context; no extra build diagnostic was emitted. That run was stopped before this stdfunc verification and is explicitly **CANCELLED_INCOMPLETE**, not a package pass.

The integration team must replay the combined full tools gate on its final source. This report proves the three scoped requirements on the stated clean source and does not claim final 179-requirement acceptance, Apple runtime success, release or deployment closure.
