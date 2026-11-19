# Direct reference preservation and independent supplemental evidence

Production and scenario registration: `194f7ae440922fd1b32105b590f073f4dfd0f0d8`.
Final test control: `dd2b72a5f4184861e421a205bcb3d7f395ea77e1`.

## Production defect and correction

Explicit `native.ref` transformations retained only known typed fields during native emission. The independent specimen's `P15Extra: false` field disappeared for action-output, variable and system references. The same loss occurred in typed action calls and inline text attachments.

The existing shared transformation representation now also carries the exact explicit native dictionary. Both native and JSON serialization preserve that dictionary, including unknown fields and explicit false, zero and empty values. Generated transformations without an explicit native dictionary retain their established typed serialization. The strict comparator is unchanged.

The specimen checks exact action/variable producers and ordered coercion, dictionary-key and property transformations. It uses a synthetic consumer solely for native structure and import preservation; it makes no Apple runtime claim.

## Executed evidence

| Run | Discovered named cases | Outcome |
| --- | --- | --- |
| Causal baseline | 52 | 41 passed; 11 failure events, comprising nine direct/typed/inline leaf failures and two parents; zero skips |
| Final clean codec/reference gate | 76 | 76 passed; zero failures/skips; 3.036 seconds |
| Final clean supplemental producer gate | 14 | 14 passed; zero failures/skips; 6.797 seconds |

The baseline uses the new independent regression tests at 194f7ae with the six production edits restored to K4ff and the new serialization helper absent. Every failing leaf identifies the missing `P15Extra` field. Original native import and strict mutation controls remained green. The driver restored the exact candidate afterward.

The final gates ran on an ordinary clone at dd2b72a, clean before and after both commands. The codec gate includes 16 XML/binary × hybrid/lossless × Legacy/v2 import/recompile observations, 24 strict reference mutation cases, all six shared typed/inline paths, and existing codec/ordered-aggrandizement regression controls.

The producer gate independently executes eight existing named-list production tests for RP13 and all three new reference tests for BRG07. It validates the resulting four source-bound tier records, rejects incomplete commands, missing assertions, wrong producers, stale source, invented runtime tiers and removal of the strongest native tier. It also verifies that the complete 179-row audit does not award unexecuted requirements a pass.

The focused two-requirement report contains RP13 and BRG07 only: 2 passed, 0 failed, 0 skipped, `closure_complete: false`. It is not a full 179-requirement run, Apple runtime proof, or release closure.

## Preserved initial test correction

The initial 194f7ae codec gate already passed 76/76. Its producer gate had one failing negative leaf because that new control removed only BRG07's weaker structure record while leaving the complete roundtrip record. The established validator correctly allows native-roundtrip to satisfy native-structure. Commit dd2b72a changes the control to remove the strongest required native tier. No validator relation, immutable contract, expected native field or strict comparison was relaxed. Both initial logs are retained.

## Reproduction and integrity

Each receipt records exact command, source file hashes, discovery, final outcomes, source status and raw-log digest. `source-bound-producer-receipt.json` is extracted from the complete concatenated Go test output; long test2json output chunks are joined before decoding.

The archived driver retains its original host paths. For reproduction, run its baseline from 194f7ae (whose parent is K4ff), then its clean mode from dd2b72a. Adapt host paths only. Do not use dd2b72a's parent as the baseline production source.

The original 94 acceptance, 52 repair and 33 recovery contracts are unchanged.
