# P15 BRG09 semantic container and native template recovery proof — 2026-11-20

## Verified result and scope

The actual BRG09 producer at clean source 1f5fb81abe6ce2c3018e62f1d72e71a8dcfc815d executed its complete three-test selection and emitted two verified local records: native-structure and native-roundtrip. The selected immutable BRG09 row passed; the complete execution audit still contains 179 rows and grants no PASS to an unexecuted requirement. This is one local gate, not overall project closure or Apple runtime proof.

The clean final intake replay passed 16 named tests, with zero failures/skips, exit 0 and 62.844 seconds elapsed. It includes the real production child execution, an explicit positive tier-hierarchy control, twelve evidence-corruption controls, explicit producer inventory, and missing-producer rejection. Both HEAD and the working tree remained unchanged and clean. Raw stream SHA-256: d4b895b4917f2d25884fbceaac7d14282ca4eff1a717cb59f24e6ada36ef7f1b.

## Independent native expectations and actual round trips

p15_brg09_container_test.go independently authors the expected native shape for semantic Dictionary values and a native List of Dictionary producers. It checks nested numeric and Boolean values, explicit empty lists/maps/text, the exact large integer 9007199254740993, ordered typed-array entries, real reference producer/consumer edges, action order and cardinality, and ordinary user keys named WFSerializationType, WFColorRepresentationType, and Value. A literal {origin} remains literal text rather than gaining an invented reference.

A separately authored native.plist specimen uses similarly named fields and existing native wrappers. It must remain opaque through the native-template route. Nineteen mutations of actual compiled trees reject lost wrappers/rows/empties, flattening, changed scalar type or numeric precision, reordered or duplicated contents, redirected references, and template coercion. These are independent expected shapes and mutations of real compiler output, not an alternative runtime.

The real filesystem round-trip test covers three specimens across XML/binary input, hybrid/lossless import, and Legacy/V2 recompilation: 24 combinations. Each checks exact full native trees and reruns the independent semantic/template oracle. The original clean root replay at b059a05c1db6fb3855c3576899c431bb1deb8804 passed 49 named tests, zero failures/skips, exit 0, in 64.406 seconds. All three required parents were observed. Its raw SHA-256 is 363eb07ca9165c54848208addf416e6909140a9f9569c785b04ee149a918af96; 24 emitted specimen payloads bind actual source/input/output bytes and explicitly record runtime_observed: false.

The final producer reran the same complete three-test selection at 1f5fb81abe6ce2c3018e62f1d72e71a8dcfc815d. That commit changes only the meta test described below. The archived standalone native log retains its original b059a05c identity; it is not relabeled as a later source.

## Preserved failed attempt and tier-policy correction

The first clean intake replay at b059a05c1db6fb3855c3576899c431bb1deb8804 failed the meta-test case missing_structure. It produced 13 named passes, one failing child plus its parent/package failure, zero skips, exit 1, in 24.891 seconds. Raw SHA-256: 491c7688d72e4c5bd4b9b93686004e44ec8fae61e758824e56e05d0c4c37e793.

That negative test expected removing only the separate structure record to invalidate the gate. The existing global evidence hierarchy already permits native-roundtrip to satisfy native-structure; the surviving round-trip record still bound the actual complete three-test execution, including the independent structure oracle. The production execution and the original global hierarchy were correct. Test-only commit 1f5fb81abe6ce2c3018e62f1d72e71a8dcfc815d adds an explicit positive control for that complete-execution hierarchy and makes the negative control remove the actual required structure test from the surviving command and required-test assertion. No global tier policy, minimum evidence tier, requirement contract or production compiler behavior was changed.

The other negative controls reject omitted round trips, mismatched tier producers, a partial test selection, absent assertions/scope, a stale source SHA, invented runtime evidence, inherited or extra requirement IDs, a failed result, and a nonzero exit code.

## Reproducible command and limits

The three required root tests are:

- TestBackendRecovery09SemanticContainersAndNativeTemplates
- TestBackendRecovery09NativeRoundTrip
- TestBackendRecovery09OracleRejectsCorruption

The clean standalone command used go test -count=1 -json -timeout=3m -run with an anchored alternation containing exactly those names and package ".". The final intake replay used the same flags with a five-minute Go budget and exactly TestContainerRecoveryProductionEvidence, TestSupplementalProductionScenarioInventory, and TestSupplementalAcceptanceMissingProducerFailsClosed in ./tools/language-acceptance. The original finite outer budgets were 210 and 330 seconds respectively; neither clean run timed out.

A preceding precommit structure/oracle run is also retained: source c707e985b7044ae412f8b170eb9cf0c855f64afb with the new test untracked, 24 named passes, zero failures/skips, exit 0, 203.360 seconds. Its first Go event arrived late during build/startup; it completed within its existing bound. It is labeled precommit and is not substituted for the clean executions.

## Evidence integrity and remaining boundaries

[evidence/p15-brg09-container-recovery-20261120](evidence/p15-brg09-container-recovery-20261120) retains all twelve original raw streams, stderr files and summaries byte for byte, including the failed replay. Its manifest records each artifact size, SHA-256 and actual source. The two additional files are explicitly labeled exact JSON-payload extractions from the original Go Output events; those payloads may span multiple Go JSON events. Original streams remain authoritative. Staged Git blob bytes were compared with every original artifact.

Implementation commit: b059a05c1db6fb3855c3576899c431bb1deb8804. Test-only hierarchy correction: 1f5fb81abe6ce2c3018e62f1d72e71a8dcfc815d. The registrar adds only BRG09 and preserves the immutable 179-item contracts. The receipt scope explicitly excludes Apple execution, authored JSON null, and standalone native List<List> support. Other semantic/type/function requirements receive no inherited credit. Computed Map keys and their evaluation boundaries are a separate unverified hypothesis, outside this gate's authored specimens.
