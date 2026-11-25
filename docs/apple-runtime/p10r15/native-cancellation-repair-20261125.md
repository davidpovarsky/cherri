# Native acquisition cancellation and rejection diagnosis, 25 November 2026

Base: a25a65784c22e53bd1f110e1fc7baaaab13daac6.

## Observed problem and repair

The S627 F01 run 45529529353 failed the actual mid-child SIGTERM/descendant assertion before native orchestration. A separate actual-process control reproduced a process ownership gap: SIGTERM delivered after Popen starts its child but before the handle returns leaves the descendant live. The ordinary post-spawn cancellation control passed. This proves the gap; the original failure alone does not identify its precise scheduling point.

Both bounded process runners now defer callable cancellation handlers only across assigning the spawned process handle. Handlers are restored before replaying cancellation inside the owning cleanup scope. The child does not inherit a blocked signal mask. Existing process-group kill, parent reap, cancellation receipts, deadlines and rejection semantics remain.

The original cancellation test remains and now separates descendant readiness from a write explicitly released only after cancellation. It checks actual descendant termination and absence of the forbidden write. The former fixed two-second pre-cancellation timer could fire before a delayed test driver sent the signal. A second real SIGTERM control exercises the Popen ownership gap, and the capture runner has its own actual cancellation/reaping control.

## Validation

Evidence directory: /mnt/development/projects/evidence/cherri-p10r15-apple-completion-20261125.

- Actual two-control red: ordinary cancellation passed; spawn-boundary cancellation left a live descendant and failed. process-cancellation-red.original.log SHA256 eef2c6a10b06b2502e853baaa06eb4b0b3e3c42749e6a17a33f4ef395abedbef.
- Both controls passed after repair. process-cancellation-green.original.log SHA256 447565786380b4b9c9514a33efb1be74754ed1bc8374bfeda3f389f1baf0267a.
- Full Python discovery executed 476 tests. Its first attempt exposed the new diagnostic helper exceeding the existing 64KiB observer source limit and the exact test-inventory count still describing 31 tests. Those failures are retained in native-qa-python-after-cancellation-fix.original.log SHA256 8f05b6ff76540ca93501292721b26907dec80b0f8010515c304ad825831efc1d.
- The helper was placed in the already source-bound recovery module, preserving the 64KiB limit. The inventory now counts all original 31 plus two added observer controls. All 87 affected prerequisite, observation and terminal-receipt tests then passed in 18.085 seconds. native-qa-affected-controls-after-source-boundary-fix.original.log SHA256 f42d37047560535d66d844c07e334f7c13c3ac023ab5e938c8441c25b514dded.
- The remaining 389 tests passed in the retained full attempt; the complete native workflow must still verify the final source.

## Nested-control diagnosis remains open

Run 45529532308 passed all original 473 Python controls, compiled and signed its exact nested-list control, then rejected Phase3 project prerequisite observation before native execution. Its actual environment matches the required Xcode27.1/macOS27.0.1 identity. The observer previously reported only a generic constant when strict packet sealing rejected.

The observer now publishes only the failing source line, fixed phase and fixed error category, never exception text, paths, credentials or captured service streams. Privacy canary controls verify that restriction. Exit status, source/file guards, original retention, encryption, budgets and all native acceptance requirements are unchanged.

On the existing Scaleway Mac, exact a25 source binding and complete checkout guard passed. The original prepare/capture/source-after/strict-private-validation/archive/encryption components also passed. These are component observations under the Mac's actual Xcode27.0 identity, which differs from the CI observer's expected Xcode27.1; they are not a forged CI context or iOS runtime pass. A new source-bound CI control is required to locate any remaining CI-only rejection.

No native output, nested/Optional transport, physical-device acceptance or complete requirement pass is granted by this repair.
