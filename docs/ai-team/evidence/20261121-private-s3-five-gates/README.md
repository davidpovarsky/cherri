# S3: five actual private-corpus gates

Compiler source: 9c8daf721aa241127df8d1e3f91bbadf90110e73. This is component evidence on that exact source only.

All five original gates passed on the same 38 private inputs (62,980,131 bytes; 27,652 actions). The independent intake reparsed every original Go JSONL, checked the single parent/package terminal, exit code, timeout, duplicates/orphans/skips, unchanged input hashes, exact source identity and clean source before/after.

| Gate | Actual process seconds | Package seconds | Result |
| --- | ---: | ---: | --- |
| TestP12_PAR_12_CorpusComplexParamsAndZeroStage3Gate | 428.937 | 350.251 | PASS |
| TestP13_CorpusFidelityAndSourceSizeAudit | 272.906 | 259.909 | PASS |
| TestP10R10CorpusMeasure | 477.391 | 226.535 | PASS |
| TestP11_WIRE_01_Fresh38x2NativeActionWire | 292.907 | 286.573 | PASS |
| TestP12_BASE_02_LifecycleStagesAndReconcileCorpus | 404.546 | 388.829 | PASS |

Each gate kept its original Go 15-minute timeout and 930-second process ceiling. They ran sequentially; this S3 batch has no gate retries. Earlier c469 timeouts and S2 failures remain in their original evidence records.

original-public-receipts.zip contains the six byte-exact public metadata originals: five process receipts and the independent review. review.json is a normalized review plus the ZIP/member digest index. The raw private input manifest, input filenames, source files and original private stdout/stderr are retained only on the authorized host; their hashes are recorded here.

The original review SHA-256 is cf9df9e48ced135325549e6187006a686d145ce22a6e4ff28a6b93d4507106f6. Private input manifest SHA-256 is 766ac82ef7d92f50a894e50bc84d3fed9cc2d375f81ffe2771dbe8a5a6de5f39.

This packet does not grant evidence to a later compiler source, including the subsequent Legacy math operand fix. It does not close the 179 requirements, Apple, installed consumer/pin validation, Pure mode or release.
