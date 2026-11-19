# Exact S2 private regression evidence

This archive records the five actual private-corpus regression tests on compiler source `7b0100a2a9ce5991ca8ffe82c805278e5aa7754e`. It proves this corpus scope only. It is not a declaration of complete P10R15 acceptance, a final S3 release, or native Apple runtime success.

## Results

All five required tests were discovered and passed separately with zero failures, skips, missing tests, timeouts, or NOT_RUN markers. Each ran against the same clean ordinary Git clone using Go 1.24.13 on Linux ARM64, GOMAXPROCS=2, and serialized package execution (-p=1). Test caching was disabled with -count=1. The unchanged runner enabled CHERRI_P10R10_CORPUS=1, selected the existing host-private corpus through CHERRI_P10R10_CORPUS_DIR, and disabled generated reports with CHERRI_P10R13_WRITE_REPORTS=0.

| Actual test | Seconds | Result |
| --- | ---: | --- |
| TestP10R10CorpusMeasure | 178.46 | PASS |
| TestP11_WIRE_01_Fresh38x2NativeActionWire | 216.05 | PASS |
| TestP12_BASE_02_LifecycleStagesAndReconcileCorpus | 320.04 | PASS |
| TestP12_PAR_12_CorpusComplexParamsAndZeroStage3Gate | 216.27 | PASS |
| TestP13_CorpusFidelityAndSourceSizeAudit | 162.79 | PASS |

The measured corpus contains 27,652 actions: 23,353 NATURAL_VERIFIED (84.45%), 4,298 EXACT_NATIVE fallbacks, and one existing invalid action. The complex-parameter audit reports 137 third-party actions. Fresh and sequential lifecycle runs both retain 23,353 natural actions. The zero-Stage3 gates pass with zero demotions; the required natural floor remains 23,340. The four-action deficit observed on the earlier b34 candidate is absent on this exact source.

## Input and source integrity

All 38 original plist files (62,980,131 bytes) and all 76 input files including JSON companions (132,843,610 bytes) remain unchanged. Both content-set hashes and the private identity-to-content mapping were checked before and after the suite; source identity and clean Git state were checked before and after each test. Content-set digests hash the newline-joined, sorted individual file SHA256 digests.

- Original content-set SHA256: `d1aaa34a0f89ec250dd223320382f2c5e2d34a00267a394b32853f46b0e49e33`.
- Entire input content-set SHA256: `87aecae33a4e295228d400a364418b96f8234cb50e49df3ad5ccbbfe9931446c`.
- Actual unchanged runner SHA256: `7d139bcf4d11ef7d5ae982ade824a722a2b5cfcee6b9132148940e48ea43d277` (10,703 bytes).

The public receipt contains exact commands, source-test file digests, actual named Go test-event counts, per-test timing, and the SHA256/size of each original raw log. Independent post-run verification re-read those logs, checked their hashes and actual test events, re-hashed the inputs, and confirmed the source remained clean. The 7,281,919 bytes of raw private logs and the identity map remain on the authorized host; they are deliberately not included in Git. A host-side scan found zero private input filenames in the public receipt.

## Scope and limitations

These measurements apply to S2 only. Known module and other separately tracked acceptance repairs continue on later source. Exact-native fallback remains a truthful separate category; neither fallbacks nor the existing invalid action are counted as natural success. This archive changes no thresholds, acceptance contracts, original corpus files, or compiler source.
