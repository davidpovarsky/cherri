# Four native pilots at 85b6

All four runs used exact implementation 85b6c352d1e0f9f272cfc6f33c10d0becd29eee6. Their reviewed source hashes match the corresponding Git objects, each callback has its exact fixture/name and fresh run nonce, and all 80 declared published-file hashes were independently verified. Quarantined files were absent from the downloaded publication sets.

| Run | Probe | Native actions | Actual outcome |
| --- | --- | ---: | --- |
| [44248683227](https://github.com/davidpovarsky/cherri/actions/runs/44248683227) | F01 counter |18| Strict PASS, exact calls=1 |
| [44248694890](https://github.com/davidpovarsky/cherri/actions/runs/44248694890) | F01 predicate |38| Strict PASS, P15_F01_PREDICATE_OK |
| [44248707476](https://github.com/davidpovarsky/cherri/actions/runs/44248707476) | File carrier |47| Correlated SUCCESS with FILE_BYTES_FAIL; required semantic result FAIL |
| [44248722472](https://github.com/davidpovarsky/cherri/actions/runs/44248722472) | Unicode JSON export |68| Correlated ERROR, code 0, redacted message length 82 |

## F01 causal scope

The counter graph's Number/Math/store path produced the exact expected calls=1. The predicate graph's computed Bool results, stored variables and ordinary guards produced its exact success marker. Each has two actual inline events and two collector records with before/after and begin/end ordering, and both existing runtime/trace gates pass.

These prove the two particular native graphs independently. The original combined local-F01 failure is still open. The already-reviewed conditional-counter control and pure lazy-logic probes are the next split: one combines conditional result flow with Math; the other exercises AND/OR composition without Math. No output-name or conditional encoding patch is justified merely by the two successes.

## File carrier result

The File probe ran to its final callback and reported P15_LABEL_FILE_BYTES_FAIL. Its raw callback contains two inline and two collector records, but the strict oracle rejects the wrong semantic result and correctly keeps both required gates at FAIL_RUNTIME.

The source checks baseline bytes, restored type, restored bytes and original-after bytes using one overwriteable verdict. The final value can mask an earlier baseline/type failure, and there is no independent byte/type control immediately after getVariable(original). Consequently this evidence does not identify Dictionary set/get as the sole cause, certify type preservation, or establish a generic typed label carrier.

The next bounded diagnostic must retain independent baseline, transport, type, restored-byte and source-invariance flags. The historical source and failed oracle remain unchanged.

## Export result

The Unicode JSON export produced a correlated ERROR with numeric code 0. The public callback contains no result or inline trace. Its redacted message length is 82. This identifies an actual native failure, but does not identify which action failed or prove that Unicode itself is unsupported. Export construction, artifact conversion/parsing and payload decoding need separate native observations before an emitter is changed.

## Inspector and publication scope

Publication passed in all four runs, with 20 text files each and no quarantined data present. The AX comparison remains NEEDS_REVIEW. Observed card counts are page observations with unresolved identity: counter 40 for 18 expected native actions; predicate 66 for 38; label 85 for 47; export 121 for 68. They are not counts of distinct native actions and do not establish one-to-one action-ID/UUID binding.

Per-action typed coverage remains UNKNOWN_AT_TRACE_EVENT. Physical-device testing remains BLOCKED_DEVICE. The adjacent JSON preserves exact source/plist hashes, nonces, results, trace observations, gates, Inspector scope and all publication hashes.
