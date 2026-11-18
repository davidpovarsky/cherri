# Storage timeout publication repair

The native run [44367783153](https://github.com/davidpovarsky/cherri/actions/runs/44367783153), on source `42e7d1cb94542557ada88201178bf9edfb4fa994`, reported `crossRunTextVerified=false` and `dispatchResult=CALLBACK_TIMEOUT`. Its later publication gate rejected `runtime/storage-cross-run-diagnostic.json` as `INVALID_BOUNDED_STORAGE_DIAGNOSTIC`. GitHub published no artifacts. The quarantined diagnostic's logged digest is `159f542d200aaedd6aa3de7c27057d7c775862261f2e03456449efae688a5271` (1,718 bytes); its content was not recovered. The locally retained failed-job log is 121,193 bytes with SHA-256 `d6588b8f3bd3ff92d45597016f1ecf02b0b23abbe45046577456971f33ada1b8`.

## Reproduced reporting defect

The actual orchestrator binds the two-phase diagnostic before its final output reporting. After a reader timeout, the reader's native trace is absent. The generic final reporter then created a synthetic collector event at that same trace path. Rebuilding the diagnostic during publication therefore produced a different trace digest, so the unchanged equality gate rejected the truthful failure report.

A new regression test runs the real final-reporting Python block, the existing cross-run validator, and the actual publication gate. On parent `76c65fbf73f24bdc6d28465559948829f40dbe05`, with only the new test, it fails with `CROSS_RUN_DIAGNOSTIC_NOT_BOUND_TO_ACTUAL_PHASE_EVIDENCE`. The raw baseline log SHA-256 is `92823b4c578305eeaa56da64effa224a50cb350c700fe37ceb930c18b28ef666`.

## Repair and scope

Final reporting now preserves the absence of the storage reader trace. The existing writer trace, phase diagnostic, callback oracle, publication allowlist, raw-plan quarantine, and equality check are unchanged. A timeout remains `FAIL_RUNTIME`, with no reader trace, no verified runtime, and no cross-run success. The new test also proves that changing the bounded report to claim success is still rejected.

All 140 Python QA/inspector/publication tests passed against the working source in 133.781 seconds, raw log SHA-256 `694085173afba683cff923c84c04fa61ed045ae4e5970539be5295fc06927fcc`. A source-bound clean receipt follows separately.

This repairs reporting and artifact availability. It does not establish native storage persistence, typed archives, cache, TTL, deletion, cross-shortcut isolation, or physical-device behavior. The original native timeout and failed workflow remain failures; a new immutable-source native run is required.

## Clean source receipt

Exact source436331ebb164ac362c00d216d41caf8770d19fc9 passed all46 storage, callback, and publication tests in64.406s with a clean tree before and after. Raw log SHA-256:886f476b89071d6e8123d418e240a9f9296f132025393e53e709c027fbae0d7d. Source-bound receipts and explicitly normalized log derivatives are in evidence/storage-publication-436331e/. Original raw bytes are retained unchanged locally.
