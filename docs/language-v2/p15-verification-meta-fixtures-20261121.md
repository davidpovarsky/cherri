# P15 strict E01 meta-fixture repair — 21 November 2026

## Result and boundary

Source `d6a5ecf384b37ded7521f8c435b5afebd4ce6144` passed the six affected acceptance-validator test families at a clean Git checkout: **104 PASS, 0 FAIL, 0 SKIP**, exit 0, in **171.75 seconds**. The exact run used `-json -count=1 -timeout=6m`; its original stdout SHA-256 is `70a895e1547db061b9b76bbd7eef4503d32aade3bccee3b0914d2c59bd34be5b`.

This repairs stale positive test fixtures. It does not claim a new full-179 result or an Apple execution. The corrected E01 service record has real production evidence; its required UI tier remains unproved. The generic FN01, CI-run and native-encoding inputs in these meta-tests are explicitly synthetic inputs used to exercise the real validator, not production acceptance evidence.

## Original failures remain original failures

[Local acceptance run 44938764062](https://github.com/davidpovarsky/cherri/actions/runs/44938764062) at K1 `d90b69a92e83b5034728c22e062a3dd96153a2a8` reported failures for RP01, RP02 and BRG04 because their positive validator controls still supplied the obsolete E01 `runEditor` service record. The synthetic XCTest line in the same fixture could no longer establish UI evidence under the strict editor intake. The two affected encoding-intake control families also failed in [Build & Test run 44938761076](https://github.com/davidpovarsky/cherri/actions/runs/44938761076), Linux job `136951106499`.

The original Local179 artifact and Linux log remain preserved by the CI intake. Their hashes are recorded in [the verification summary](evidence/p15-verification-meta-fixtures-20261121/summary.json). This later repair does not relabel those runs. K05 and other independent failures are outside this change.

## What changed

Only four test files changed. No validator, compiler, immutable contract, CI budget or permission changed.

The new shared positive fixture executes E01's fixed service producer once per owning test, validates its actual raw evidence, and binds all fixture records to that source. It retains the immutable E01 contract and removes the fabricated UI record. Each mutation receives an independent JSON copy.

The local baseline must have all other selected requirements passing, the service tier verified, exactly one external gap, and no validation error. The final baseline must fail for exactly the missing E01 UI tier. Any detached service receipt or additional error fails the positive control.

All twelve end-to-end aggregator attacks now require their own specific rejection. In particular, the weak-tier attack retags the actual service receipt as UI and requires the strict editor rejection. A pre-existing missing-UI error cannot cause these attacks to pass. The encoding controls retain their own positive CI01/RP47 baseline before applying the existing mutations.

## Exact replay

The selected top-level tests were:

- `TestEndToEndAggregator_All12AdversarialAttacks`
- `TestP15VerificationInvalidRecordsNeverPass`
- `TestP15VerificationMalformedProducerFailsClosed`
- `TestP15RP02ParserOnlyAndForeignSHA`
- `TestRequiredEncodingIntakeRejectsIncompleteOrGenericEvidence`
- `TestRequiredEncodingPhaseArtifactReconciliation`

The outer run began at `2026-11-21T19:50:58.220348Z` and ended at `2026-11-21T19:53:49.970953Z`. SentinelX continued the same process as `job_f1c1913e4ed9`; it was not restarted.

Each of the six owning tests executed a fresh E01 service run with **73 fixed passing events**, exit 0, no timeout, and a clean source before and after. The 107 recorded source files were independently reconciled to immutable Git blobs at the tested SHA during evidence sealing. These repeated runs are not additional distinct requirement coverage. The original process receipts, raw logs, actual E01 records and compile-only preflight are retained byte for byte.

[Original evidence archive](evidence/p15-verification-meta-fixtures-20261121/original-evidence.zip): **30 files, 91,412 bytes**, SHA-256 `17217a5979d169513489462ac03ec9b2048c649a8075f328d7c1621d1dcaa3c3`. The [summary](evidence/p15-verification-meta-fixtures-20261121/summary.json) contains the complete file inventory, nested run hashes and unchanged 94/52/33 contract hashes.

A bounded independent source review found no blocker in the four-file change or the causal fixture boundaries. That review did not rerun tests or grant acceptance. Integration and a fresh full-179 run at the eventual combined source remain necessary.
