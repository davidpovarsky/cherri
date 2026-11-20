# Retained R4 preflights and isolated RP03 replay — 20 November 2026

## Exact source and scope

The retained combined source is bd5d82e82828ca1cd0c27981705969c245e85306, clean before and after every invocation. These component results do not close the full 179 requirements, inherit other requirement IDs, establish Apple runtime at R4, or validate the final consumer/deployment tuple. The frozen R3 report remains 95 PASS, 2 FAIL, 66 NOT_RUN, 15 PENDING_EXTERNAL and 1 AI_EVAL_NOT_RUN.

## Actual retained checks

| Selection | Named PASS | Named FAIL / SKIP | Original elapsed time | Original stdout SHA256 |
| --- | ---: | --- | ---: | --- |
| Optional and List root | 414 | 0 / 0 | 81.125 s | d55b7f37854bdc6d9e833b235616a14010777b300322b4d9680827e79ca57805 |
| Affected lower, analysis and types packages | 76 | 0 / 0 | 12.750 s | d34f0b0a0c893a1619f2fe3238ba71c9ee2d050085de255e9c3d999878bda6c4 |
| Five strict production producer suites | 1,189 | 0 / 0 | 114.031 s | 7dce60e6cfde892571413793ae633b1a5cf9389177083aa2dfbe80be6c144626 |

Every child was explicitly retained under its producer evidence-output directory. An independent audit checked required/discovered/passed test membership, parent completion, actual command and time window, clean exact-source bindings, all raw/stderr/process artifact references, and 136 distinct source blobs read with git --no-replace-objects cat-file --batch. All three immutable contracts retain their original SHA256 values.

| Producer | Actual child named PASS | Source files bound | Accepted component records |
| --- | ---: | ---: | --- |
| RP11 | 95 | 45 | unit; native-differential |
| RP16 | 114 | 45 | unit; native-differential |
| U01 | 19 | 50 | repository |
| RP33 | 59 | 64 | unit; native-differential |
| BRG19 | 53 | 68 | unit; native-differential |

There are nine records total. None is a synthetic replacement for another ID or for a runtime tier. The independent audit SHA256 is 1e3fe360dddf741e2553b38b655385dc41e59f05ffa6cc387e487f1292d5bd93.

## Earlier combined source remains failed

At clean 059d6bf45ebc27bf58a695c1f5cc016b511b069b, the root selection produced 1,117 PASS and two failure events: TestP15L01InferredNestedNativeListFailsClosed/optional_absent and its parent. The compiler correctly emitted zero bytes with E_TARGET_VALUE_SHAPE; the older test expected the nested-list diagnostic. The later narrow oracle correction retains the precise rejection and zero-output checks.

The same source passed 205 named package tests and 1,189 named strict producer tests. Their original outer logs are preserved. Their child files were not explicitly retained, so this report does not use that older execution as retained child-artifact evidence. The fresh bd5 invocation above resolves the retention gap without changing those older records.

## Isolated RP03 completion

The exact original test TestP15RP03FreshCloneContractIntake was replayed at bd5 with the original three-minute Go deadline. It completed in 50.172 seconds with its parent and both repository_default and unrelated_explicit cases passing, zero failure/skip, and empty stderr. Original stdout is 3,406 bytes, SHA256 6d86200f64cf954068ce33194b872ecdbf80b49cdbfd987be8344294371c49a1.

The earlier combined supplemental execution at 0dc remains failed with its original missing-completion/cleanup tail. This isolated pass does not establish the cause of that earlier failure and does not replace the full supplemental parent.

## Reproduction and retained evidence

[Original combined archive](evidence/p15-r4-retained-preflights-20261120/original-evidence.zip) preserves 34 original entries, 5,839,837 uncompressed bytes, compressed to 373,153 bytes. Archive SHA256: a203e7623bcecc3e636e0a2c4413f5a2ff73ec007e4b756dfece4a72f5652fe8.

The [index and independent audits](evidence/p15-r4-retained-preflights-20261120/index.json) include original process metadata, exact hashes, integration maps and the auditor. The separate [RP03 process](evidence/p15-r4-retained-preflights-20261120/rp03/process-original.json) and original stdout/stderr preserve the later follow-up. Every archived entry and staged Git blob was byte-verified.
