# Self-contained F01 runtime differential

The R1 missing-target F01 fixture returned an actual error callback. Its redacted
message alone could not distinguish an executed RHS from native prevalidation of
a missing Run Shortcut target. The original fixture and its expected result stay
unchanged. This separate diagnostic removes that external ambiguity while
retaining an observable test of RHS evaluation.

Use the explicit workflow sample `qa-short-circuit-local`. It selects
`qa-short-circuit-local-v2` in Debug and requires the exact callback result:

```text
CHERRI_QA_F01_LOCAL_RESULT:P15_F01_LOCAL_OK:calls=1
```

The body uses native Number actions to derive its false/true operands. Both RHS
expressions increment a local counter and yield a Bool. `false && RHS` and
`true || RHS` must leave the counter at zero and produce their correct Boolean
values. The identical RHS then executes directly as a positive control and must
increment exactly once. Explicit failure branches inspect the two skipped
results, the intermediate count, the direct result, and the final count. The
callback includes the actual counter value; it is not a constant success string.

The source reuses the independently checked standalone fixture from core commit
`6fd2a895351c603a64358a286dc1decc30ff09da`. The wrapper adds only the existing
trace initialization/export and synthetic anchor comments. A production-plist
test checks the standalone native graph and compares the complete experiment
body with the wrapped graph. Only random UUID identities and the intentional
name of the final Text binding are mapped for that comparison. Parameters,
actions, conditions, values, and order remain exact. Changed increments and a
constant replacement for the observed result are rejected.

The target remains subject to the ordinary fresh callback/source/nonce and
NDJSON trace contract. Counts zero/two, a failure verdict, missing trace, replay,
wrong identity, and a forged Release classification cannot earn runtime success.
The trace-absence exception is still limited to `qa-measure-release-v2`.
No Swift privacy prefix change is needed because the existing
`qa-short-circuit` prefix already covers this synthetic fixture.

Local validation completed 46 passing Go test events, 49 Python tests, all four
workflow shell-step syntax checks, YAML parsing, and the actual fixture-compilation
phase through the real CLI. All 13 fixtures compile. The new fixture emits 91
native actions and contains no Run Shortcut action. The full counts, source
hashes, emitted action identifiers, and log hashes are recorded in
[local-f01-harness-validation.json](local-f01-harness-validation.json).

Signed Simulator execution remains **NOT_RUN** for this stage. A future pass of
this local experiment would isolate local short-circuit behavior; it would not
retroactively pass the separate missing-target fixture. Their runtime outcomes
must be reported separately.
