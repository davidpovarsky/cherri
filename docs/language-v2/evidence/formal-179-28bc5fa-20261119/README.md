# Completed all-179 local acceptance at K2

Executed source: `28bc5fa29d0c8d3ca4dfa2d96b42ee7374d65b46`. Documentation: `daa58850140bc1d739806633932d01e084463404`.
Manifest-pinned compiler: `7b0100a2a9ce5991ca8ffe82c805278e5aa7754e`. The executed runner/test
source is K2; the compiler pin is a different immutable identity.

The actual `go run ./tools/language-acceptance --phase local` process completed
in 1652.531 seconds, exit 0, without timeout. Source, docs, the three
immutable contracts and compatibility manifest remained unchanged and clean.
All 179 IDs agree across the contracts, results, execution audit and evidence map.

| Result | Count |
| --- | ---: |
| PASSED | 73 |
| FAILED | 0 |
| NOT_RUN | 100 |
| PENDING_EXTERNAL | 5 |
| AI_EVAL_NOT_RUN | 1 |
| Total | 179 |

The runner retained 120 distinct passing evidence records covering 78
requirements; only 73 meet their full criteria. Partial evidence remains
explicitly blocked. `closure_complete` is false.

Fourteen requirements passed since the completed 1cf9ae7 baseline:
AC02, AC03, BRG05, BRG17, E04, F05, IM04, K05, RP05, RP09, RP10, RP30, RP31, RP32.

K05 used the actual configured remote signing service for a public synthetic
two-action workflow after one explicitly allowed edit. The signed input equals
the verified unsigned bytes. The signed output has an AEA1 header and its
recorded digest. This establishes the integration behavior required by K05;
it does not establish Apple import or runtime execution.

Every original output is archived with original and archived SHA256/byte counts.
Only host-home prefixes in textual operational records are sanitized. The
signed artifact, unsigned input, original plist and edited public source retain
their exact bytes. No private corpus inputs or raw private-test logs are included.

This is an intermediate source-bound result. Clipboard, module, diagnostic
harness and later changes require their own final combined-source validation.
