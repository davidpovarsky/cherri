# P15 explicit None clearing: actual native boundary repair

## Completed scope

Production source f604f2981a3fc45eb08eff1e0182f4eb11dabf95 now emits the existing native Nothing boundary before a None assignment can consume implicit prior input and before return none can emit an implicit prior output. This covers immutable binding, mutable initialization, mutable assignment, top-level return, function return, and a return inside a nested function branch.

The source change is confined to four existing lowering routes: 7 insertions and 10 deletions. It reuses emitClearedVariable and materializeYieldValue rather than inventing plist null or an empty Text value. Native encoding, codec descriptor, schema fingerprint, acceptance contracts and runtime fixtures/goldens remain unchanged.

This is verified local compiler/native-structure evidence. No Apple execution occurred in this component, and the missing-clear native structure is not presented as an observed device leak.

## Actual failures retained

The initial tests-only source 5b8830cbc579ff44382ca47a0c869269a9f25270 exposed all six missing-clear cases. It also contained an incorrect positive-control assumption that Bool false used an integer plist zero. The existing canonical Bool producer emits real/float zero, while an authored Number 0 uses integer zero. The original failed raw stream is retained unchanged.

Test-only source 856e326850b1891204dfa1c7ed6924cbc5e9ef1a corrected only that exact existing Bool wire expectation and made one mutation construction safe when an output action omits its parameter dictionary. No None-boundary assertion was weakened.

The corrected clean baseline exited 1 in 21.578s: all nine false/zero/empty positive cases passed, while the six actual None binding/return cases failed the required native boundary. The mutation suite correctly stopped at its failed real positive specimen rather than claiming negative controls had run.

Corrected RED raw SHA256: 1cdd104c1ba3cbc1b3bd184a1a2e74b8235fbcb03edcdb9b698fffb93d7903a9.

All six original bad native plists were extracted from their exact raw Go Output bytes, then decoded independently with Python plistlib. The serialized Set Variable actions lacked both explicit input and a preceding Nothing. The return actions immediately followed the public MUST_NOT_LEAK marker without a Nothing boundary. The original bytes and derived observations are retained separately.

## Clean repaired replay

At f604f2981a3fc45eb08eff1e0182f4eb11dabf95, all 82 selected root parents ran: 465 named test/subtest passes, zero failures/skips, exit 0, 59.828s. This includes all six repaired cases, all nine present false/zero/empty cases, nine actual-specimen corruption controls, complete existing empty-list clearing regressions, prior optional-presence/raw-null/function/container/codec checks and native roundtrips.

Root raw SHA256: 6deada99d3a8d5ef3526fc2bbefe76af86a95ad48f7f612730de7649e0905b73.

The complete lowering package passed separately: 15 named passes, zero failures/skips, exit 0, 8.719s. Lower raw SHA256: 6e8591372cd503b8fbcea698ff45ae79c98a8a3cbd64d7fd96853de0599a0e7b.

Each clean replay verified the exact same HEAD and clean worktree before and after; finite budgets were unchanged. Exact commands/timestamps/exit states are in the original summary files.

## Separate open optional transport findings

The earlier exploratory probe is also retained, including its original source as a .go.txt evidence file, raw streams, source-state summary, independent decoded observations, and nine exact public native plists. The probe explicitly records a dirty test-only source and makes no acceptance claim.

It showed that echo(none) and echo("") produced identical full function WFItems payloads, and that an ordinary Map entry none was encoded as empty Text. This clearing repair does not fix those value-transport aliases. A List containing none also produced a row without a WFValue; that native shape is not promoted to verified runtime cardinality. Precise transport rejection or a separately validated presence protocol remains required.

Lazy native.action keys without authored UUIDs compiled with resolved action-output identities in direct and nested Dictionaries during that exploratory run. These observations are not yet the independent causal key regression requested for the wider container scope.

## Integrity

manifest.json binds every original or explicitly derived artifact to its byte length and SHA256. Original native bytes are never reserialized. Git attributes preserve exact bytes; only the public synthetic plist artifacts are explicitly staged despite the repository-wide plist ignore rule. The three immutable requirement files are unchanged. No T05, L01, FN05 or RP20 completion, Apple limitation, CI completion or full project closure is inferred from this component.
