# Optional immutable binding presence: verified compiler increment

Tested source: 69ca2c232ffb6dc567bb4a9c2830c7e114a08c6c.

The compiler now retains the value-presence edge when an immutable binding aliases a tracked Optional parameter or function result. Explicitly Optional immutable None, empty Text, false and zero use their semantic presence rather than a native truthiness test. Leaving a lexical scope restores both the outer value and its presence metadata.

The production change is limited to lowerBinding in internal/language/lower/lower.go. It derives presence after evaluating the RHS, preserves it in the normal alias/materialization postlude and in the existing early None-clear path. No new native carrier format or codec descriptor change is introduced: the existing v2.1 private function protocol remains authoritative.

## Actual clean verification

The final clean source passed 810 named root events across 100 top-level tests in 29.375 seconds. The full types, analysis and lower packages passed 53 named tests in 7.000 seconds. Both commands exited zero with no failure, skip or timeout; Git HEAD and clean status were unchanged before and after.

The exact complete commands, finite budgets, raw Go events, stderr and process receipts are retained under evidence/p15-optional-immutable-bindings-20261120/runs. The root suite re-executes the prior 95 function/container/native/None/codec regressions plus five new binding test parents; it is not a full repository or Apple run.

## Causal controls and limits

The clean tests-only baseline reproduced 31 leaf failures: 18 parameter alias routes, eight bound-call predicates, one immutable None return, and four explicitly Optional static bindings. The repair made all 34 named focused events pass before the broader run.

Two additional lexical-shadow scenarios compare the full independent native graph. Each first validates an unchanged typed clone, then rejects five mutations: wrong inner flag, outer use of the inner flag, outer use of the data value, truthiness instead of presence, and duplicated function execution. The intentional shadow must still emit exactly its existing W_SHADOWING warning; the test requires that warning rather than filtering diagnostics.

The first broader replay passed 797 named events but failed the two new scope scenarios because their initial oracle incorrectly expected no shadow warning. That failed run is retained unchanged. A separate draft upload failed gofmt before any commit or Go test execution; its original tool output is retained as NO_TEST_EXECUTION. Neither attempt is counted as complete success.

Mutable assignments, general dynamic forwarding, Optional type aliases, expression joins, nested container absence and actual Apple value fidelity remain distinct open work. This increment adds no formal179 producer or linked-ID credit and does not complete T05, L01, FN05 or RP20.

## Retained execution history

| Attempt | Source | Go exit | Named passes | Seconds | Raw SHA-256 |
| --- | --- | ---: | ---: | ---: | --- |
| Clean causal baseline | 94867d7a080c | 1 | 0 | 62.031 | fcd86d30140b67b038e6aa54a770cbeceb0d37d996a1455effa99aa814f9e5f2 |
| Dirty focused production repair | 94867d7a080c + dirty | 0 | 34 | 26.890 | 0d0664e407432f8c386ff5600f791de5bdb7743116765318f01aa3a11e530040 |
| Clean broader replay; new warning oracle failed | 1d082a1ce336 | 1 | 797 | 43.187 | f0b597f74923010647d5da97448ad5647e2e662941e9149d1737196523b1e5e0 |
| Clean complete root replay | 69ca2c232ffb | 0 | 810 | 29.375 | 9dd8906dd284978b1d9dae4f1aca581c978e2f97caed3ffde3ce586920a3487c |
| Clean types, analysis, lower packages | 69ca2c232ffb | 0 | 53 | 7.000 | fb3dbb433d2e4d7fab4b49dc7e88bd176da82ca7bcb70bc6f130b4c952a2fd0c |

## Provenance

All copied artifacts and baseline/fixed source blobs are listed with exact byte counts and SHA-256 values in evidence/p15-optional-immutable-bindings-20261120/manifest.json. The manifest SHA-256 is 803810c221968ed1797929f447cb0862d23fa24a4af05f920aed3f7eeed9b793. Source copies come from immutable Git blobs; original run bytes are copied without newline rewriting.

Implementation sequence after a9ecb5ec24cef9435af07981ac11abadfb0a7ea1:
- 94867d7a080c5379fd7dc428a5392b72dbc8462e: clean tests-only causal baseline.
- 1d082a1ce3365f8a2d40e2cd38c1c9ec743ed521: immutable metadata propagation and lexical controls.
- 69ca2c232ffb6dc567bb4a9c2830c7e114a08c6c: exact intentional-shadow warning assertion.
