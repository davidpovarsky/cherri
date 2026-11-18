# L02/L04 immutable local proof

Source: `6f3db1184173c8872d015d18757e2ae207f79a8a`

Compiler SHA-256: `1d4e6b3f2475ae63873cc3de78e4fe1a13f3c5749781a03d4e9879cc5f11ce01`

The normal Git checkout was clean before and after verification. The built compiler embeds this exact revision and `vcs.modified=false`. No signed Apple runtime was executed for this source.

## Actual outcomes

- Four index tests passed, comprising 18 boundary cases, 18 intentional graph corruptions and 32 exact native round trips (72 test/subtest PASS events).
- Existing root controls passed (62 events), as did seven lowerer controls and 15 formal/accounting test events.
- Ten actual CLI compilations retained source and unsigned-output hashes. All seven known-integer/static graphs were unchanged after normalization of generated UUID identities only. The three dynamic graphs gained exactly the integral guard; their native lookups are nested inside all three valid branches.
- The final test was run again with only `lower.go` replaced through a Go overlay by its immutable pre-fix bytes from `27daf7994ca20479e9dcee5645b255408b66b2fe`. It failed with the expected missing third guard. This is the causal RED, not a failed repaired implementation.
- Both L02 and L04 remain `PENDING_EXTERNAL`. Their complete iOS tier, RP21 and release closure are not claimed.

`cases/` contains the ten exact synthetic source inputs and the corresponding actual native action graphs. `native-specimens.json` records each XML/binary, hybrid/lossless, Legacy/V2 round trip. `formal-receipt.json` contains only records produced by executed tests; the source was clean when those records were generated.

## Retained failure history and collector correction

The initial new regression failed on the old production graph. Its original receipt and log are preserved. The preexisting compatibility test then reported its old expectation of two dynamic If starts versus the repaired three. It was strengthened to compare the added predicate with an independently authored `input == 0`, while retaining both original bounds comparisons.

Exact-source execution job `job_7ad9819e1b32` completed all four test commands with exit zero, but its outer evidence collector attempted to decode a long Go JSON log message before concatenating split `Output` events. The collector raised `JSONDecodeError: Unterminated string`. No compiler or test source was changed in response.

Continuation job `job_55fabec0c754` concatenated the retained output fragments, independently checked complete package PASS events, source SHA and source cleanliness, then executed only the previously unrun causal overlay and ten CLI comparisons. The final receipt states which logs came from the first job. The collector failure was not reclassified as a native or compiler failure.

These files contain synthetic source and bounded compiler evidence only. The historical baseline compiler hash and contract hashes are in `receipt.json`. They do not establish behavior on a Simulator or device.

Published JSON uses LF newlines with unchanged data values. The manifest retains original captured file hashes where normalization changes bytes; CLI graph rows separately retain raw and published JSON hashes. Exact source inputs and test logs are unchanged. The public VCS display removes its filename path and trailing display whitespace only.
