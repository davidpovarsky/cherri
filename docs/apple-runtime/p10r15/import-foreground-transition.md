# Shortcuts foreground gate after import consent

## Observed failure

S3 run 44459414583, typed-defaults job 135342271508, built its XCTest runner successfully and then exited 65. The exact failure was an unavailable com.apple.shortcuts application during the library cell-count snapshot, at line 561 of the S3 UI harness. The only callback came from the receiver self-test. The compiled shortcut did not execute, so its typed-default and empty-container semantics remain unverified.

The original harness activates Shortcuts and waits for foreground state at startup. It then handles import consent and follow-up alerts, records the later state as a Boolean, and queries library descendants without using that Boolean as a guard. The app's actual exit cause is not established by this observation.

The original sanitized artifact, failure excerpt, source identity, signing receipt, and blocked runtime oracle are preserved in native-followup-20261119/s3-8b3fcf7-typed-default-import-block. That outcome remains BLOCKED_IMPORT.

## Narrow change

The existing alert-dismissal helper first stops querying each target whose actual state is not running or unknown. It still permits the existing background SpringBoard alert route. The shared QALibraryForegroundGate then checks the actual foreground state after the consent transition and final alert handling. If needed, it issues one activation and waits at most ten seconds for foreground state; it then checks the actual state again. A successful activation call by itself is insufficient.

Library snapshots require both the successful transition and a fresh foreground-state read. Failure stops the test before the descendant query, writes an existing editor coverage record with BLOCKED_SIMULATOR_APP, and records an XCTest failure. The fixed receipt contains only six Booleans. It records whether activation and waiting were needed, whether the wait succeeded, the observed before/after state, and whether foreground was subsequently lost before a query.

The existing source-bound import/name/editor identity and runtime callback assertions remain required. No successful import, callback, or typed-default result is inferred from this helper. It performs no retry loop and is not a general solution for arbitrary app crashes or races during a later AX snapshot.

## Validation boundary

The production shared Swift helper is directly compiled and executed by the portable Foundation gate. Six state-transition cases exercise 48 assertions, including unavailable app, failed wait, stale successful wait, and loss between state reads. A separate process compiled with the query guard deliberately removed must fail the unavailable-app assertion.

These controlled lifecycle closures validate the helper's branching and receipt serialization. They are not Apple runtime evidence. The existing Foundation CI step executes the new gate on macOS, and the source-bound Simulator workflow must still compile the XCTest adapter and successfully execute the unchanged typed-default fixture before any runtime claim.
