# Original completed-wait iOS replay: 45 passed, 11 failed

The full Intel iOS workflow [44707326745](https://github.com/davidpovarsky/cherri/actions/runs/44707326745), attempt 1, ran source a8d4af3d2c708bec4cd3294e8a34dc12feb0ec38. Its test step failed with Xcode exit 65. Collection and upload succeeded. The original ZIP, six published members, API metadata and job log were retained and their bytes were rechecked before this report.

## Observed result and unchanged acceptance

All 56 named tests remain present: **45 passed and 11 failed**. Compared with the preceding b6329e22537793dd453401c7ae85a45235c5f048 replay, the sole changed named result is testGeneratedCloserCompletesAndOvertypesWithRealKeyboard, which now passed. Its actual completed readiness wait finished at 26,329 ms after one owned document creation. The other 11 UI tests remain failed. The source still declares all 13 editor test methods; this is not a reduced suite.

The real CodeView paste test passed its original 3-second completed-wait and exact-text assertions: the native text observation was at 2,622 ms, predicate match at 2,814 ms and completed wait at 2,939 ms. The later stock UITextView diagnostic also completed, but it ran in an already warm process and is not a cold-paste comparison. All three preview lifecycle tests and the two documentation unit/integration tests passed. They do not establish the documentation UI tier.

The startup decision budget is still 28 seconds. A final diagnostic snapshot after a failed wait is never accepted as readiness. The reset helper and all test bodies from that point onward are byte-identical to the prior reviewed source; the proof records their shared SHA-256.

## Remaining observed failure boundary

Eight tests failed the final interactive-editor assertion. D03 and E03 failed at control.elementType in the filtered startup observer, with the fixed SNAPSHOT_QUERY_FAILURE category. The exact source classifier assigns this category only when the immediate XCTest error prefix is Failed to get matching snapshot or Failed to get matching snapshots. E02 failed at the query's array evaluation and remains UNCLASSIFIED.

The publication intentionally excludes arbitrary AX error payloads. The original GitHub job log contains the sanitized collector summary, so this evidence does not establish why those matching snapshots became unavailable. A stale handle during a document transition is a hypothesis, not a proven application defect.

The retained timing traces also show long known-control observations: 17,244 ms for BRG27, 18,154 ms for completion, and 19,359 ms for palette selection. The generated-closer positive control used a real new document and passed within the unchanged budget. These observations motivate removing the redundant per-element type read through exact typed queries; they do not authorize a larger budget, a fabricated ready state or skipping feature bodies.

## Evidence and scope

The original artifact 14618142018 is 78,030 bytes with SHA-256 198e807e87ab05d1bca5884cf9ceb5921044eb473e9f9cbcd4429754fbeddac4. The companion JSON records the six exact member hashes, all 56 named outcomes, 11 failure categories, all 33 startup receipts, all nine completed timing traces, the original paste observations and the separate stock probe.

ios-test-evidence.json still contains zero records. D03 UI is NOT_VERIFIED, and there are no documentation UI states or native Shortcut runtime events. Neither a failed run nor its individual passing tests grants final-candidate or formal 179-requirement acceptance. The later query correction, if tested, must retain this original failure and run on its own exact source.
