# P15 semantic container entry ordering repair — 2026-11-20

## Observed result

Actual clean source `b186ab6d1b0cb1b9d013bc5be3c53c72b665d65a` passes the four newly reproduced sibling-write cases, seven corruption controls, and three cases that must preserve their original references. The broader replay passed all 26 selected root test parents: 192 named passes, zero failures/skips, exit 0, 8.860 seconds. The entire `internal/language/lower` package then passed 15 named tests, zero failures/skips, exit 0, 7.078 seconds. Both working tree and HEAD remained clean and unchanged through both runs.

This is compiler source/native-structure proof. No Apple runtime, standalone nested native List, optional/JSON-null support, whole FN05/L01/BRG09 acceptance, or 179-item closure is inferred.

## Clean failing baseline

Read-only peer review of the preceding function Map identity fix identified a separate shared lowering gap. A Map's first value could retain a live mutable Variable reference while a later entry emitted an assignment to the same variable. The whole Dictionary was materialized after that assignment, so the earlier entry did not have an independent read boundary.

Tests-only commit `ee1c10b1cf8f0af4b379c9bb76d07703d3be10f8` requires the early field's native edge to point to one real Get Variable producer before the later conditional begins. It reproduces the defect for a direct Map, a function Map argument, a nested Map, and a Dictionary array field. The exact clean replay failed all four cases, with actual exit 1, zero skips, 47.890 seconds. Its raw SHA-256 is `21bf153dfa2053f63565b1e10834e9dbf1bcc31f7364874cef1d974e48b2c30b`.

The test observes serialized producer/consumer edges and action order; it does not run an alternative interpreter or claim to observe Apple's runtime value.

## Bounded production change

The new `semanticSiblingWrites` helper precomputes later source-visible assignments to current mutable bindings. `lowerSemanticValue` uses that result at each authored Map or inline semantic List entry boundary and applies the existing typed reference-capture routine. The earlier referenced value is consumed before the later sibling emits its write. Nested Maps and Dictionary arrays retain their semantic kinds and their explicit empty collections.

The preexisting outer-call-argument capture boundary remains intact. The explicit raw/native.plist route is unchanged. Values that have no later write to their referenced binding retain their previous native representation. Controls separately exercise a pure later value, a later assignment to another variable, and an assignment to a shadowed local variable.

## Causal controls and broader replay

Seven independent changes to actual compiled native actions must be rejected: restore the live Variable reference, remove the capture producer, move it after the write, move it into the conditional, change its source binding, duplicate its UUID producer, and duplicate the later value effect. The positive oracle requires exactly two stores to the outer binding (initialization and later write), exactly one later effect, one direct snapshot producer, and the strict initialization → snapshot → conditional → assignment order.

The clean broader replay also executes all earlier Map identity/template/unsupported-null controls, FN01–FN03 typing/native graphs, T01 presence/default guards, C01 source-order/default-effects/argument-snapshot cases, and V01 declaration snapshots. All 26 selected parents were observed in the actual Go JSON stream. Raw root replay SHA-256: `f1cfe6bfdcb3eae8f0113a2f791373b5cb1dc9d115ef60e5e9661f2f10129058`. Raw complete lowering-package SHA-256: `b6ab45e47c9c34cf960a74a0e6b4c292cede242c75604c2a0d289c2bfe01da9c`.

## Evidence and remaining scope

[evidence/p15-container-entry-order-20261120](evidence/p15-container-entry-order-20261120) retains all 15 original raw streams, stderr files and summaries byte for byte, including the clean failure and both precommit runs. The manifest binds each artifact by size and SHA-256; staged Git blob bytes were verified against each original.

No immutable requirement contract, runtime fixture, runtime golden or formal producer registry changed. This repair concerns sibling entries of semantic Maps and Dictionary arrays. It does not certify all expression-order cases, all native List content behavior or formal native runtime gates. The broader semantic-container/native-template gate will receive its own complete executed structure and round-trip evidence.
