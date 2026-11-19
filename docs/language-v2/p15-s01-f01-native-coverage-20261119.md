# P10R15 S01/F01 native coverage fixture — 2026-11-19

## Verified local stage

Commit `c4151d3ade25ff1260395f4e65c84aa7b80eceba` adds a public synthetic fixture and an independent source/native-graph oracle for the missing S01 and F01 runtime scenarios.

The clean source replay passed **26 named tests / three top-level tests**, with **exit 0**, **zero failures or skips**, in **24.219 seconds**. The source SHA and clean-tree state agree before and after the run. It compiled the real fixture into **271 native actions**.

**Apple execution of this new fixture remains NOT_RUN in this receipt.** This stage proves that the reviewed source, expected bytes and emitted native structure agree. A signed Apple run must independently produce the exact clipboard bytes before the iOS tiers of S01 or F01 receive credit.

## Source and expected output

- Source: [CherriS01F01CoveragePOC.cherri](../../tests/runtime_poc/CherriS01F01CoveragePOC.cherri)
- Contract: [CherriS01F01CoveragePOC.expected.json](../../tests/runtime_poc/CherriS01F01CoveragePOC.expected.json)
- Oracle: [p15_runtime_s01_f01_coverage_test.go](../../p15_runtime_s01_f01_coverage_test.go)

| Identity | SHA256 |
| --- | --- |
| Source file | `4a13f90af2bdc326e24f9dbf785f76a1ae41043e176b5a3188ab794694a19aa3` |
| Expected JSON file | `8baa3279a770ca20e3daa6036f416360a308d6f40eb1dd0874a38afeb7d0b10c` |
| Expected complete clipboard, 313 UTF-8 bytes | `f2e79540f2c95547ac9c83d59f25b63ce4167153d81731a5e00b746f041037f9` |

The expected clipboard has exactly four lines and no final newline: the public sentinel `CHERRI_S01_F01_COVERAGE_OK`, then `S01_OUTPUT`, `F01_TRACE` and `F01_RESULTS`. The JSON contains their exact values. It is an expectation contract and deliberately cannot claim execution.

All inputs are synthetic constants. The native action allowlist is Text, Number, If, Set Variable, Comment and the final Set Clipboard. The source has no external shortcut target. The original runtime fixtures are unchanged. The Git ignore change permits exactly this new named public source file.

## S01 coverage

The two strings place actual Hebrew marks, astral characters, a ZWJ emoji sequence and combining marks before dynamic attachments. They cover adjacent tokens, a repeated producer, a mutable producer, a Hebrew identifier and literal U+FFFC.

The raw oracle checks original literal bytes, exact producer identities and independently specified UTF-16 ranges:

- First string: **15, 19, 22, 26**.
- Second string: **3, 4, 7, 11**; literal U+FFFC at zero is not an attachment.

The final output must consume both actual interpolated values. Replacing the observed Unicode value with a hardcoded expected string fails the oracle.

## F01 coverage

The Boolean seeds are comparisons of actual native Number outputs. Every effect appends its distinct marker to a mutable trace and then yields its Boolean result. The oracle matches complete logical trees, native branch identities, producer order, update edges, Boolean yields and the final observations.

| Case | Result | Executed effect markers |
| --- | --- | --- |
| and_skip | F | none |
| or_skip | T | none |
| false_or_true | T | OR_TRUE |
| false_or_false | F | OR_FALSE |
| nested_or | T | NEST_OR |
| nested_and | F | NEST_AND |
| outer_and | F | none |
| outer_or | T | none |
| mixed_run | T | MIX_LEFT, MIX_RIGHT |

The exact final trace is `START|OR_TRUE|OR_FALSE|NEST_OR|NEST_AND|MIX_LEFT|MIX_RIGHT`. The nested cases exercise both orders of AND/OR composition and skipped whole subtrees. The false-OR cases require observable RHS execution for both true and false results.

This structural oracle does not emulate Apple execution. The independent expected clipboard still requires an actual native observer.

## Integrity controls

Eight contract mutations reject source drift, changed expected text or trace, a terminal newline, extra requirement membership, self-claimed execution and changed hashes.

Fifteen native graph mutations reject wrong Unicode offsets, normalization, swapped sources, hardcoded text, an eager effect, wrong or fractional Boolean predicates, a missing native wrapper, a dropped false-OR effect, a literal swap of the nested AND branches, crosswired stores, hidden trace, hardcoded results, an unrelated clipboard source and an external shortcut dependency.

The contract's memberships are exactly **S01 and F01**. Related RP/BRG identifiers are not inferred from these results.

## Evidence and integration

[Clean run receipt](evidence/p15-s01-f01-native-coverage-20261119/clean-run.json) and [original Go JSON events](evidence/p15-s01-f01-native-coverage-20261119/clean-run.jsonl) bind this local result to the committed source.

The Apple observer and acceptance exporter are separate integration work. They must bind the reviewed source and expectation hashes, the signed artifact, a fresh run and attempt, and the complete observed clipboard bytes. This receipt provides no iOS pass, physical-device result, final 179-requirement acceptance or release closure.
