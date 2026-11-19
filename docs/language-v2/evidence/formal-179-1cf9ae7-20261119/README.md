# All-179 intermediate local baseline at 1cf9ae7

This archive preserves the actual completed local acceptance run on source `1cf9ae776a9560217d04288a1ecf4b5277053736` and docs `9fb6256a4549d93414e9b65318d4794ae40fbee7`. It is a historical intermediate baseline, not final S2 acceptance. Preparation also records the manifest-pinned compiler source `b34ae47ba79d15c08e8a8bdec9a761a7a4b722bb`; this must not be confused with the executed runner/test source.

The actual command was `go run ./tools/language-acceptance --phase local --out <EVIDENCE_DIRECTORY>`, with `GOMAXPROCS=1`, `GOFLAGS=-p=1`, a controlled docs directory, and other inherited CHERRI variables removed. It completed in 1,338.859 seconds with process exit 0, no timeout, and unchanged clean source, docs, three contract files and compatibility manifest. Process completion does not imply acceptance closure.

| Requirement result | Count |
| --- | ---: |
| PASSED | 59 |
| FAILED | 0 |
| NOT_RUN | 113 |
| PENDING_EXTERNAL | 6 |
| AI_EVAL_NOT_RUN | 1 |
| Total | 179 |

`closure_complete` remains false. The 179 unique IDs match the pinned 94 acceptance requirements, 52 repair requirements and 33 recovery gates exactly in the result report, execution audit and evidence map.

The producer retained 95 distinct passing local evidence records, all bound to the exact executed source. These cover some assertions for 64 requirement IDs; only 59 meet their pass criteria. E07, F01, L02, L04 and S01 retain partial local evidence while external obligations remain blocked. The separate 40 historical smoke observations are explicitly not formal tier evidence. Local tier names such as native or native-roundtrip do not establish signed Apple execution or device behavior.

All 13 actual input artifacts are retained here, plus this scope note, a verification record and a hash manifest. Reports with no private host paths retain their exact original bytes. User-home prefixes in the runner and a few operational records are replaced with explicit placeholders; the manifest records each original and archived byte count/SHA256 and every replacement count. Hashes embedded in original receipts still refer to original bytes; use the archive manifest when verifying a sanitized copy. A host-side scan against all 38 actual private corpus names found no private specimen identities in the archived material. No private originals or raw private corpus logs are included.

The original runner, launch and progress records are archived for reproducibility, not scheduled or re-executed by this archive operation. Final S2 requires its own complete source-bound gates and external evidence.
