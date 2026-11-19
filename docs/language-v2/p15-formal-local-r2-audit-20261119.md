# P10R15 R2: actual local 179 audit

The complete Linux local acceptance runner at immutable source
`4e56dbcfe895baa8f944b83715ecff0d07bfd24e` reports **87 PASSED, 0 FAILED,
82 NOT_RUN, 9 PENDING_EXTERNAL and 1 AI_EVAL_NOT_RUN**. All 179 requirements
remain present. The process exited 0 because its local execution completed
without a failed requirement; `closure_complete` remains **false**.

Source run: https://github.com/davidpovarsky/cherri/actions/runs/44634809329

## Identity and independent checks

The original workflow artifact `14583751257` is retained byte-for-byte in
[evidence](evidence/p15-formal-local-r2-20261119/), together with the original
report, process receipts, execution audit and GitHub run/artifact metadata.
The complete ZIP SHA-256 is
`a733d4619a4f3163f9a9250a4d1e4521dd0507163505502514432d9a41f453e6`;
the report SHA-256 is
`4c7c4e5102d18143c96633d071489537bde822a4c2561410921558ad2ce85153`.

Independent inspection matched the process/report/log hashes, all 179
unique IDs against the three exact immutable Git contract blobs, the full
requirements map and audit, and all 159 evidence-record implementation SHAs.
Every referenced artifact digest was checked against the untouched ZIP.
S03/E08/RP29 raw process, stderr and benchmark digests were also replayed,
and every recorded source-file digest was compared with its immutable Git
blob at the exact R2 source. No evidence paths or source identities were
rewritten. The original collector reported clean compiler/docs before
execution and unchanged trees afterward.

The actual command was `go run ./tools/language-acceptance --phase local`
with a workflow-owned output path. It ran from 23:07:47 to 23:14:38 UTC on
2026-11-19. The pinned docs commit remains
`205217a256b5f220724363b44b5ae516ffe6a539`; the consumer compiler pin remains
`b97ae752205f1e747b6c98ff7511f3484dd53bc2`. Public-fixture signing was explicitly
opted in for the existing K05 route; the private corpus was not requested.

## Exact change from the earlier e61 report

| Requirement | e61 | R2 |
|---|---|---|
| T08 contextual signature | FAILED | PASSED |
| N01 fractional arithmetic | NOT_RUN | PENDING_EXTERNAL |
| RP29 asset authorization, service and benchmark | NOT_RUN | PASSED |
| BRG32 complete accounting and executable harness controls | NOT_RUN | PASSED |

All other row statuses are unchanged. The new N01 local numeric evidence
cannot supply its Apple tier. The actual RP29 Linux child executed all 11
required tests in 12.323 seconds and retained the real raw benchmark; it
received only security/service/benchmark scope.

## Remaining obligations

The 82 NOT_RUN rows comprise 24 explicitly downgraded smoke probes, 57 rows
without executed dedicated formal evidence in this run, and D03's separate
installed/published/app-consumer gate. Existing implementation or a related
requirement passing does not satisfy these rows. The full per-ID wording,
required/executed tiers, test names and blockers are retained in
`remaining-requirements.json`; `independent-audit-summary.json` lists every
status and the four-row delta.

The nine PENDING_EXTERNAL rows are V01, V03, S01, L02, L04, N01, F01, E07 and
RP50. The first seven require actual source-bound Apple runtime; E07 needs
actual iOS UI evidence; RP50 has harness-unit evidence but still needs its
external evaluation tier. AI01 remains AI_EVAL_NOT_RUN. No model-quality,
physical-device, installed-consumer or final-release completion is inferred.

FN01–FN03 producer development after the R2 freeze is subsequent work and
is not substituted into this report. The actual Apple e61 timeout and
zero-based-loop mismatch likewise retain their own source/run provenance.
The 179 contracts and the frozen R2 source remain unchanged.
