# Explicit nil native diagnostic

This fixture observes native cardinality after explicit V2 Optional absence. It does not change the nil emitter or establish that an omitted Set Variable input clears a previous native result.

## Exact contract

The separately selectable `qa-nil-clear` compiles the reviewed `qa-nil-clear-v2.cherri` source in V2 debug mode. The exact expected callback is:

```text
CHERRI_QA_NIL_CLEAR:binding=0:assignment=0:control=0:present=1
```

The four fields are independent Count action outputs. A nonempty Text action immediately precedes each nil declaration, nil assignment, and explicit-clear control. The control uses the existing native Nothing followed by Set Variable without WFInput. A present Optional Text supplies the positive cardinality-one control. Raw `native.ref("Variable", ...)` is used only to observe the native value without claiming that ordinary Optional consumption has passed the analyzer's required narrowing.

The production graph oracle requires the four distinct variables, the reassigned variable's earlier nonempty initialization, actual Count producers, each preceding seed, the explicit control's Nothing, the positive control's exact producer, and every UTF-16 summary attachment. Eleven graph corruptions must be rejected. The shared callback oracle also rejects each altered count, stale or duplicate events, wrong shortcut identity, missing trace, ERROR/CANCELLED, self-test events, and a release-profile exception.

## Evidence boundary

On compiler `d61b59b9d1589a06b75fa7b545370b925337f8c7`, an explicit nil binding and assignment both emit Set Variable without WFInput and without a preceding Nothing. The explicit-clear control emits Nothing followed by Set Variable without WFInput. This is an observed wire distinction, not proof of the native behavior of either nil path.

The independently observed historical empty-list split established cardinality zero for the explicit-clear control, but that observation does not certify the nil forms. Original collection, nil, File, and short-circuit fixtures and their strict expected outputs are unchanged.

A fresh signed run on the final integrated compiler must provide the exact callback, correlated before/after trace, nonce, source/compiler hashes, and preserved publication manifest before this diagnostic can establish native absence. Any wrong count remains FAIL_RUNTIME; it is not an accepted diagnostic success. The compiler nil implementation and the overall Optional/native acceptance remain separately assessed.
