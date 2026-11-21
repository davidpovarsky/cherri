# Actual startup lifecycle observation — 20 November 2026

## Outcome and source

The one authorized diagnostic run [44741698137](https://github.com/davidpovarsky/cherri/actions/runs/44741698137), attempt 1, executed exact source `954cd45f49742a6b824047852d8735a24738b9c7`. Job `136306683965` ran from 13:41:10 to 14:06:27 UTC. The Simulator app build and bounded collectors succeeded. The actual Simulator test command exited 65, and **all three original selected UI tests failed**. The workflow conclusion remains FAILURE. There was no repeat dispatch.

This was explicitly `STARTUP_OBSERVATION_ONLY`, with the original 28000-millisecond readiness decision budget, 3000-millisecond paste budget, one document-creation policy and unchanged feature assertions. Both published acceptance record lists are empty. It is not a full-suite or formal requirement result, and it cannot be inherited by another source commit.

## Authenticated originals

The original GitHub metadata, job log and ZIP bytes were retained before parsing. Neither downloaded artifact was rewritten.

| Artifact | GitHub artifact ID | Original ZIP SHA256 |
| --- | --- | --- |
| startup-lifecycle-observation | 14633864372 | 5203ea896127d3c36d670d5c02784c34066f1a694e0df613617e4da614520759 |
| ios-build-evidence | 14633754554 | 2e6e105469257a65b94e90cb9b287e14ace0de88cae20f1d96e3d4ddc8fab45f |

Intake matched the authenticated artifact ID/name/run/source/digest/size metadata, checked ten original extracted files, and compared all eleven declared source files with actual Git objects using `--no-replace-objects`. The exact source's `validate_app_events` function then replayed each original app event file and reproduced the published event rows and health observations. Twelve actual in-memory corruptions of those native rows—foreign nonce, different process, changed sequence and a false main-thread heartbeat for each test—were rejected. These are intake controls, not additional Apple executions.

The retained auditor is identified by SHA256 `caf3678b2c5c42877551c7f86d1967aa36c2582fae4a84484054cedd3f93e0c3`. The accompanying JSON proof records its actual report and original file digests. Raw XCTest descriptions and container paths were not published.

## Observed test boundaries

| Original test | Actual result | Readiness trace elapsed | In-flight observation at trace | App rows / background heartbeats |
| --- | --- | ---: | ---: | ---: |
| BRG27 editor workflow | Failed | 28290 ms | 1 | 26 / 24 |
| D03 documentation search | Failed | 28280 ms | 1 | 21 / 19 |
| E01 completion insertion | Failed | 34305 ms | 0 | 23 / 21 |

BRG27 observed a completed document-control wait at 3996 ms and returned from its single creation tap at 8194 ms. D03 returned from its tap at 5054 ms. Both then failed while the typed known-controls accessibility observation was in flight; their source locations point to the creation-button/editor query group. The original private compact descriptions were each 174 bytes. The published fixed-signature classifier did not identify their text, and there was no associated NSError in either issue. Their exact exception messages therefore remain unavailable in the public artifact; no exception cause is inferred from the source line alone.

E01 returned from its creation tap at 4211 ms. Its editor observations remained false. One known-controls query took 9429 ms, followed by a creation-control hittability observation taking 10610 ms. The completed readiness wait was false, and a later final snapshot ended at 34305 ms. The original startup assertion failed; late state was not accepted as readiness. The timing is from the instrumented execution, not an uninstrumented performance baseline.

All three teardown receipts retain `syntheticSourceResetVerified=false` and `editorExists=false`. There are no documentation UI state records, input receipts or native compiler runtime events from these feature bodies. Failure at setup does not establish a defect in documentation search, completion insertion or compilation.

## What the app-side observations establish

Across the three fresh nonce-owned app invocations, the collector read 70 actual event rows. Each invocation recorded `app-initialized`, an opened observer and continuing background heartbeats. The heartbeat spans were 46001, 36001 and 40001 ms; the largest observed gaps were 2231, 2029 and 2421 ms. The files contain no document-initialization/read/write, workspace or editor-container event.

That is an observed event inventory, not proof that every corresponding lifecycle callback was absent. Background writer health does not establish MainActor responsiveness, and the observer explicitly records `absenceProvesMissingLifecycle=false`. The source's document hooks and the completed creation taps make the document-creation transition a useful next diagnostic boundary. The current evidence does not distinguish an uncompleted document-browser action from a blocked main thread or another framework transition.

A read-only check found the documented app type consistent across the source and generated-project declaration: the FileDocument uses `org.cherrilang.cherri.file`, the project exports that type as plain text with the `.cherri` extension, and the document browser/open-in-place flags are enabled. No configuration repair is claimed from that inspection.

## Privacy and limits

The original XCTest descriptions were deliberately quarantined on the original runner. The public capture contains byte counts, hashes, fixed diagnostic signatures and bounded source metadata. The ordinary published log omits the private issue/session lines. Those hashes authenticate the observations but do not recover the original description text; the private originals are not part of the downloaded artifacts.

The actual iOS SDK successfully built this component. Its three runtime tests remain failed. No original deadline, expected result, source binding or feature assertion was changed during intake. A further native run must be justified by a reviewed source change and must preserve this failure evidence.
