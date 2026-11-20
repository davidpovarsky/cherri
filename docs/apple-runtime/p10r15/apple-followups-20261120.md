# Source-bound follow-ups on 2026-11-20

## R2 N03 observation

[Native run 44634905206](https://github.com/davidpovarsky/cherri/actions/runs/44634905206) imported the original N03 diagnostic from source `4e56dbcfe895baa8f944b83715ecff0d07bfd24e`, then returned a fresh correlated ERROR callback with code 516. The original publication policy redacted the 149-character native error message. No comparison fields or trace payload were produced; case-sensitivity behavior remains unobserved. Strict intake verified 21 published hashes and preserved FAIL_RUNTIME_PRESERVED.

Original artifact 14587340771 is 272971 bytes with SHA256 `248eab1406c6ee77062b4a5930c26661c9a632c9177a708ccad97696dd01097b`; it is retained unchanged together with original run metadata. The original source/expected values are unchanged. See `r2-n03-error-20261120-audit.json`.

## Three isolated File-carrier experiments

Commit `b29d071e44a9a8be5f3c22e7b09d5fe3965f8716` adds three fixed public fixtures: Get File of Type(public.data) from Text; the same from decoded JSON Dictionary; and Get Variable with explicit File coercion. Each source compiles to 77 native actions and observes 8 independent type/count/byte/name fields with a binary File witness. All encoded observations are bounded to 512 characters. Each local graph guard rejects 16 causal corruptions, including replacement data, incorrect conversion, constant results and broken zero clear.

Clean source replay executed the real compiler plus Python graph guard in 13.48 seconds and all 8 existing/new dispatch-oracle control methods in 64.84 seconds. The first draft failed the existing reset-source assertion; the fixtures were corrected to include the original zero-output clear, and the assertion was retained. No production export or original acceptance fixture changed.

[Bundle 44644166680](https://github.com/davidpovarsky/cherri/actions/runs/44644166680) was dispatched after remote exact-SHA verification at 00:13:56 UTC. At this receipt stage all three jobs were queued: native behavior is NOT_RUN, and no formal R3 evidence or passing acceptance record is claimed. The three jobs run sequentially, each with the existing 45-minute bound; one failure does not cancel the others. See `file-carrier-controls-b29d071e-local.json`.

## D03 workflow validation repair

The first push exposed [failed workflow-file run 44644165529](https://github.com/davidpovarsky/cherri/actions/runs/44644165529), with zero jobs. The workflow incorrectly referenced `runner.temp` in job-level env. GitHub's [context availability table](https://docs.github.com/en/actions/reference/workflows-and-actions/contexts#context-availability) permits runner context in executing steps, not job-level env.

Checksum-verified actionlint 1.7.12 independently rejected the original at line 30 with that exact expression error. It accepted the repaired workflow with exit 0. Commit `3f20362487ba39ed6570af4131bb6b751c5d9973` initializes the run-owned directory from RUNNER_TEMP and actual run/attempt IDs in a step via GITHUB_ENV, and prevents publication with an uninitialized root. The original public ZIP gate, immutable pins, collector, same-workspace Go intake, outside-source install and separate missing app-integration requirement are unchanged.

This is validation of the repaired workflow definition. Its actual installed/live consumer execution remains NOT_RUN until an authorized exact-source dispatch completes. The original failed run is retained. See `d03-workflow-context-20261120.json`.
