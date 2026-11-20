# P10R15 complete loop ownership: F02, RP22 and BRG12

## Verified local outcome

Component commit 640dd22d6beb8de804199712b56236f1ce8e5c79 passed 85 named tests on clean source: the new 82 analyzer/native graph observations and controls plus the three original unchanged-runtime-fixture, Legacy migration and raw RepeatIndex tests. The actual command exited 0 after 145.625 seconds, with no failed/skipped tests and clean source before and after. Raw Go JSON SHA-256: f26a6b6bcc5a40d3bc91116c327c22e9444f6cca7307be5ac34c431af1f341c4.

Registrar commit 269fd88ce7a202d63e3de388fe59cec35c6851c3 then passed a clean actual inventory/parser/production run: 319 named outer test/control passes, exit 0, 39.312 seconds, no fail/skip/timeout, clean before and after. Raw outer Go JSON SHA-256: 6a010ff5826897bcf362790107d17a9f78255d6a62e20af5697c841217d058f6. The 319 count includes intake controls and their parents; it is not a count of requirements.

The production test launched a separate actual Go child process for each immutable row:

| Requirement | Required named child-process events including parents | Process seconds | Raw child Go JSON SHA-256 |
| --- | ---: | ---: | --- |
| F02 | 26 | 8.843 | db5d5c9a252a3abdee4194aae849b99f5d3d7d841f066bee91b1b1844b02f2b3 |
| RP22 | 28 | 4.037 | 5f2eedf968d3b2e299cf87189d9354fef29bdc537624c249a4bf8104172f1d83 |
| BRG12 | 28 | 5.690 | 1ac8f953410057b0c32a6e67ee9be5f55172c69912db2d78072d1fe75a9a708c |

Each child executed its own full authored source. Six passing local evidence records were retained, one unit and one native-structure record for each row. The local manifest reports zero completed requirements, zero failed requirements and three pending external requirements. Final-phase validation correctly rejects the missing ios-runtime tiers. The full179 audit retains all179 rows and grants no evidence to unexecuted related requirements.

## Complete native graph scenarios

| Row | Actual source and observed local graph obligations |
| --- | --- |
| F02 | Nested tuple for loops over A/B and native Number1/2, distinct item/index owners, exact outer consumers after the inner loop, and a sibling named repeat with an independent zero-based index. |
| RP22 | Outer index/item variables, same-name tuple bindings, a same-name named repeat, a further nested same-name tuple loop, then exact restoration of middle, outer and global consumers. Analyzer references preserve the expected Text/Number types at each authored source span. |
| BRG12 | Outer A/B and inner1/2; mixed for/repeat and repeat/for nesting; sibling C/D loop; nested X/Y loop; same-name scope controls; outer item/index reads after inner loops; final global restoration. |

The new root tests use the real analyzer and CompileSourceToPlist path, then inspect the complete serialized plist action sequence. The independent oracle requires each iterator's exact native values, order and cardinality; matched begin/end group identities; distinct producer UUIDs; scoped Repeat Item and Repeat Index reads; native Math subtracting exactly real1.0; captures linked to their own producer; and every final consumer's exact reference and token position. This is structural proof of the emitted workflow, not an Apple runtime simulation.

The native graph is accepted after consistent UUID renaming and, where applicable, consistent scoped-variable renaming. It is rejected after 18 causal corruptions for F02 and19 each for RP22/BRG12: one-based bypass, wrong native depth, wrong item owner, wrong offset/direction/type, dangling or duplicate producers, reused/mismatched loop groups, changed iterator order/cardinality, changed repeat count, wrong consumer owner, lost outer use, escaped capture, missing/extra action and shadow leakage. Each mutation must actually change the original graph before rejection counts.

Unit scenarios require exact scoped AST type observations and exact out-of-scope diagnostic spans/codes. Tuple index, tuple item and named-repeat index leaks must fail before producing any native bytes; the valid full source must still compile afterward.

The original CherriRuntimePOC source, its zero-based golden, Legacy fixtures and native RepeatIndex behavior are unchanged. No compiler production behavior was modified by these two commits.

## Strict local evidence producer

loop_execution.go launches the selected two root tests independently for each row. It requires the exact clean implementation SHA before and after execution and hashes the relevant source as immutable Git blobs. It retains original stdout, stderr and process receipts outside the source tree; there is no arbitrary imported-evidence acquisition route.

The validator checks the exact requirement, tier, command, producer IDs, complete named child inventory and artifact set. A successful parent cannot hide skipped or omitted children. Raw JSON must be valid UTF-8 with unique keys, the selected package and exact tests, one package start/completion, unique run-before-pass order and all required child passes. Event times must be valid and ordered; elapsed values must be present for passes, finite, nonnegative and bounded; the events must lie within the observed process interval. Receipt timing, exit/timeout state, clean source, immutable path/digest inventory and original raw byte counts/hashes are checked independently.

The actual receipt controls reconcile outer hashes before attempting corruptions. They reject borrowed IDs/scenarios, fake iOS, substituted producers/commands, missing tiers/artifacts/children, duplicates, stale/dirty source, process failures, missing discovery, forged source hashes, invalid UTF-8, invalid/reordered event timing, skipped/foreign/orphan events and output appended after package completion. Every immutable source digest is independently corrupted in a control.

The old seven-action F02 smoke probe has been replaced by this real producer. The evidence map and validator explicitly prevent the old simple nested-loop trace from closing these complete mixed/shadow/sibling scenarios. The existing formal Apple exporter already excludes these loop rows and did not need modification.

## Original evidence and qualification

[original-receipts.zip](evidence/p15-loop-ownership-acceptance-20261120/original-receipts.zip) contains the original public exploration/component logs, failed draft records, clean proof, all three child process artifacts and the complete local manifest. [manifest.json](evidence/p15-loop-ownership-acceptance-20261120/manifest.json) records every original member byte length and SHA-256. [local-summary.json](evidence/p15-loop-ownership-acceptance-20261120/local-summary.json) exposes the independent process identities and local result without requiring the archive.

Two draft failures are retained honestly. The first graph oracle expected an integer Math operand although the already-verified canonical lowering emits plist real1.0; only that draft expected representation was corrected. A later upload escape error put literal newlines inside Go quoted literals and stopped gofmt before any test started. It is labeled NO_TEST_EXECUTION, not a semantic compiler failure. Dirty intermediate passes are not transferred to the clean proof.

This stage proves the local unit and native-structure portions of F02/RP22/BRG12 at the exact recorded sources. It does not prove actual iteration output on Apple, integrated-source CI, or full179 completion. Matching complete ios-runtime scenarios remain required. The original requirements and goldens have not been weakened.
