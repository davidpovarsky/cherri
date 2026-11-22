# RP24 return, continuation and recursion

RP24 now executes its complete joined local scenario on source
`558ca8acfacdca4e856cee4242b3b4afdf8e4ce0`. Both required local tiers passed;
the requirement remains **PENDING_EXTERNAL** because its immutable contract also
requires actual iOS execution. The source is based on FN05 proof
`408780be93e6a1c5f410fb2397ae281b086a330a`, whose compiler base is integrated
R6 `d2f86d2998bb15d9271a58279a51a400eb13d3d5`.

## Complete scenario

The retained public fixture calls `add(2, b: 3)`, multiplies its returned value
by two, calls terminating `fact(3)`, continues with the recursive result plus one,
and returns an interpolation of the three actual result references. Its intended
Apple output is `10|6|7`; that value is an expectation, not a runtime observation.

The semantic producer checks each operation and call type. It separately rejects
immutable, mutable and later main-scope captures with exact diagnostic spans and
codes, including the consequent return-type diagnostic, and verifies that invalid
sources emit no native artifact. Wrong argument and return types are also rejected.

The native producer independently consumes every action in the serialized graph:
the dual-marker dispatcher, argument slots and casts, add result and return,
factorial base condition and return, decrement, recursive call and cast, product
and return, both main calls, both continuations and the final consumer. Generated
UUID names are the only values learned from the actual graph. Native scalar
kinds, values, authored names, action order, group ancestry and producer/consumer
relationships remain explicit expectations.

Thirty-three native corruption controls cover missing and extra actions, invalid
and duplicate identities, native integer control codes, argument selection and
presence, operators and thresholds, recursion target and decrement, return edges,
caller casts, continuation values and a reconciled authored-name substitution.
Replacing the final reference with the expected literal `10|6|7` is rejected.

## Actual clean execution

One clean replay ran on 22 November 2026, 13:59:38–13:59:42 UTC:

| Scope | Observed result |
| --- | --- |
| Outer producer, inventory, intake and affected full179 negative control | 14 PASS, 0 FAIL, 0 SKIP; 3.415015911 seconds |
| Semantic test and exact invalid-source controls | 6 PASS |
| Complete native graph and corruption controls | 34 PASS |
| Whole nested production process | 40 PASS, 0 FAIL, 0 SKIP; 2.324448980 seconds |
| Generated native original | 55 actions, 55,881 bytes |
| Requirement result | PENDING_EXTERNAL; exactly unit and native-structure records |

The outer command was:

```text
go test -count=1 -json -timeout=4m -run '^Test(FunctionReturnProducerInventory|FunctionReturnActualProductionEvidence|Full179ContractRequirements_RejectsUnexecutedScenarioClosure)$' ./tools/language-acceptance
```

The producer runs only the two RP24 root tests with a three-minute Go timeout and
a four-minute process bound. It records all 40 required run/pass identities,
72 immutable Git source witnesses, command, timing, exit status and original
stdout/stderr. The worktree was clean before and after execution.

The intake controls reject evidence borrowed from FN01 or FN05, a fabricated
runtime tier, a partial command, a missing capture or recursion control, and a
changed fixture hash even when the receipt's artifact hash is reconciled. The
existing full179 anti-retagging test retains its rejection and now additionally
expects the distinct RP24 producer's command/event/scope rejection. No contract,
required tier, production compiler behavior or historical result was changed.

Independent inspection reread all 72 Git blobs, reconciled all outer and nested
events, and decoded the original native payload from ordered Go output fragments.
The fixture SHA-256 is
`ff0ab9a41e1cffeea4860ce185a176e8430d8b9dee4520216994191c7ab6cc04`;
the native SHA-256 is
`61e7ea7166010dbe7e60fd272c57af7687ebef6472ae8195cd04c5dbd883caed`.

## Evidence and remaining scope

Original process receipts, streams, the native payload, independent intake and
development attempts are retained in
`evidence/p15-rp24-return-recursion-20261122/originals.zip`.
`manifest.json` records each member's size and SHA-256.

The first development attempt failed to build because the new test redeclared a
Go error variable. Its unchanged output and source snapshot are retained. The
declaration was corrected before the source commit. The corrected development
run, producer preflight and clean source run remain distinct evidence.

- Clean outer stdout SHA-256:
  `b1ba7902982189489e56be04540c0dfe01a3f9fafcaa2cb9e4b85ac2e405bdfd`.
- Clean inner stdout SHA-256:
  `e2046a9035bbae249ea5345baa32f467628d49d18b4c0c11ff197c94634a4635`.

The focused full179 audit retains every immutable requirement and grants no
requirement PASS. Manifest validation has zero passed, zero failed and one
skipped requirement; its generic missing-external representation is `NOT_RUN`,
while the producer result is `PENDING_EXTERNAL`. Final validation rejects the
missing Apple tier. The historical short function trace is no longer mapped as
runtime evidence for this distinct complete scenario.

This change does not close RP25, RP26, BRG11, BRG15 or project acceptance. No
already completed component suite was repeated. The historical full179 K1 result
remains 106 PASSED, 4 FAILED, 45 NOT_RUN, 23 PENDING_EXTERNAL and
1 AI_EVAL_NOT_RUN until a new combined-source run replaces it.
