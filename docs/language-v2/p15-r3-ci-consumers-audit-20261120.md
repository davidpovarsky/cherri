# R3 frozen CI, publication and consumer evidence — 2026-11-20

## Frozen identities and scope

- Implementation / Skill K: 8e66254d56ea3e4ac70f6cda6ef9d31217e0937c.
- Compiler consumer pin S: cdc8be2c000698dddbe73185f69d8fd94273e1d6.
- Documentation D5: 2bc5f6992cd252f287530b039ee393b1b50ee39e.
- Original staging remains d7052f4568db26dba169bdefc72623e48c04968a.

This audit branch records results without editing the frozen compiler or docs. The earlier state-race proof is in p15-state-race-r3-20261120.md. Subsequent source repairs require new integration evidence; none retroactively change these results.

## Actual full179 result

Run [44651827685](https://github.com/davidpovarsky/cherri/actions/runs/44651827685) ended with failure. Independent artifact review verified all 179 unique immutable IDs, the report/audit status correspondence, 196 implementation-bound evidence records, all referenced uploaded bytes, all three unchanged contract digests and clean source/docs before and after execution. The original ZIP contains 53 files.

| Result | Requirements |
| --- | ---: |
| PASSED | 95 |
| FAILED | 2 |
| NOT_RUN | 66 |
| PENDING_EXTERNAL | 15 |
| AI_EVAL_NOT_RUN | 1 |
| Total | 179 |

The two failures are RP02 and BRG04. Both selected TestMeta_UnitAndNativeCannotSatisfyIOSInFinal, whose synthetic local FN01 records still used the retired generic runFunctions producer. The strict validator rejected those records. RP02 retains the full producer diagnostic; BRG04 retains the named missing/failed test and a truncated output tail. The original failures are preserved. A separate test-only repair and component replay do not alter this run's count.

Artifact 14592460171 original ZIP SHA256: 0191d1c5c1d6006b54aeccb1f376988bd1bacee69203919784982e88a57eb462.
Original report SHA256: 2daf9799385008ceff4e2a7d2a627f5869abe0853cd65b47caed69a9029a5b60.
Original runner log SHA256: 02dacfc84ff5a2ee2e9ae99576336baca67176aed28930edee3941d5ecc1e2b7.
Closure is false.

## Documentation CI and real publication

[Docs CI 44651777275](https://github.com/davidpovarsky/cherrilang.org/actions/runs/44651777275) passed at D5: 39 actual examples (36 runnable, three expected diagnostics), 524 reference-only blocks, 12 provenance/cache controls, 505 native actions and 15 macros, 520 rendered signatures, 87 local links, 50 search entries, zero historical entries in default search and six actual site corruption controls. The generated action reference exactly matches compiler S.

Feature-branch Pages attempt 44652290559 built successfully but deployment was rejected by the existing github-pages environment branch rule. That rule was preserved. After reviewed two-file changes and successful CI, an ordinary fast-forward advanced docs main from D4 to D5. [Main Pages run 44652711917](https://github.com/davidpovarsky/cherrilang.org/actions/runs/44652711917) built and deployed successfully. Main Docs CI 44652711963 also completed successfully.

The publication evidence is the original 1,531-byte artifact ZIP, SHA256 2fce2e6f2883fcf16d9c7ee09c87540f600409df241a2c3f4d0bf121023910a2. A successful built-site receipt alone is never treated as publication.

## Installed and public D03 producer

Original consumer run 44652992750 stopped during publication preparation with HTTPError, before installation/collection/intake. No specific HTTP status or cause is inferred from that receipt. A separate host metadata probe later returned HTTP 200; it does not explain the runner failure.

One bounded retry, [44653538935](https://github.com/davidpovarsky/cherri/actions/runs/44653538935), kept the same source and input ZIP and passed. Original artifact 14592630805 SHA256: 7ebb963e30f22e9886aa8f4693493299bababf7cece837ffb93092316de9d2d1.

Independent review verified all 34 original references without modifying their paths/digests; seven source Git blobs; eight installed search cases including invalid regex, missing current docs and restoration; seven actual HTTP 200 public responses; live action/search bytes equal to the deployed build; and the original Go intake's actual passing test. Its docs record is accepted.

The receipt explicitly states d03_complete=false, app_integration_executed=false and closure_complete=false. Its one-row requirement report remains FAILED because the separate local app-integration tier is missing. The successful docs producer therefore does not close D03.

## OpenMinis and macro local scopes

[OpenMinis 44651834990](https://github.com/davidpovarsky/cherri/actions/runs/44651834990) passed at K. All eight original GitHub ZIP digests match. The source package has 20 entries; the ARM64 runtime package has 23. Nineteen packaged source files match immutable Git objects. The packaged ARM64 compiler has embedded VCS revision S and vcs.modified=false; the separate current-build binaries identify K.

The actual TestSkillPackageProductionEvidence passed. Its complete chunked JSON receipt was reconstructed from unchanged Go output and equals the published K01 evidence file. Each architecture's wrapper observations contain eight actual CLI operations and ten explicitly mock installation scenarios.

| Local scope | Actual evidence |
| --- | --- |
| Package source ZIP | SHA256 21173bb6320657dd19f6f16aa13027048e033a316e41b167648bdb98aba4eff5 |
| Package runtime ZIP | SHA256 4051a92130b36691aa9f74273c19dffd4f65e14943341d2ed0fbff1e1c9a2509 |
| Compatibility | 21 checks passed |
| Signing protocol | 34 loopback controls; no Apple cryptographic validation |
| Macro contract | 19 compile checks, eight causal mutations, 19 verifier controls |
| Paginate | 32 compile/roundtrip cases, 144 model cases, 44 CLI negatives, seven causal faults |
| Debug export | 16 compile/roundtrip cases, 80 model cases, 16 CLI negatives, seven causal faults, 12 compiled native probes |
| Continuation / storage | 26 source Git blobs verified; 67 probes compiled, two expected diagnostics, three storage roundtrips |

The macro ledger keeps all 11 children open. Native execution, real external installation, remote-builder deployment and final release remain unproved. Source/model results are not Apple runtime evidence.

## Original-byte preservation

The evidence folder retains original API metadata, logs, archives and receipts, with an exact file manifest. Five large OpenMinis binaries/archives remain in the retained host evidence directory and original GitHub artifact, with their size and SHA256 recorded; their reproducible binary contents are not duplicated in Git. All other stage files are copied unchanged.

Original receipts/logs/Markdown may contain CRLF or trailing whitespace. Exact files that trigger whitespace lint are separately listed only after their staged Git blobs are checked against the original bytes. Authored prose and source changes must still pass normal whitespace lint.
