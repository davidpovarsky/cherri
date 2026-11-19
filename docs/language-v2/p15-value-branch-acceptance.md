# P15 F03: complete normal-path value branches

## Contract and repaired defects

F03 requires typed values from If/Menu expressions and coverage of every normal path. The analyzer now requires each normal branch of an explicitly yielded expression to end with a yield or a value-producing control expression. It rejects Void yields, missing final yields, and yields overwritten by later non-result actions. A return terminates the invocation and no longer contributes AnyContent to the normal branch result type.

The all-implicit native-last-action behavior remains untyped. A direct `yield none` now emits native Nothing at the result boundary so preceding output cannot become its value. Existing bound nil/None Set Variable encoding is preserved: a separate signed native fixture has already verified that omission of WFInput clears that binding. This repair does not rewrite that established encoding.

## Production evidence

The named tests in `p15_value_flow_test.go` and `p15_value_flow_native_test.go` execute the production analyzer, compiler, serialized native graph, actual CLI, and shared service. They cover branch union types, nested controls, terminating paths, exact result UUIDs, typed literal fields, absent output, source-bound diagnostics, and deliberate corrupted-wire and diagnostic controls.

The causal baseline is unchanged production `d2f38aea45792e23670a772b640f040d035b27a8` with the new regression tests. It rejected a valid terminating path as AnyContent|Text, missed five invalid-yield diagnostic cases, and inherited the preceding output for a direct None yield. Its raw log SHA256 is `033faba5be12696fe422669821c555c5c7b6d94af7960b9bc7efdf8ad8f7b117`. The baseline test source is retained because two earlier test oracles assumed a different native shape. Return is an existing single-token WFTextTokenString; bound None follows the original omitted-input variable assignment. No production encoding was changed to satisfy those inaccurate assumptions.

The reviewed source working copy passed all 51 named F03/F04 test events, exit 0, in 38.08 seconds. Raw log SHA256: `0b4b30b39997ea7be92d6b9a5e95d40374f20c6d0a5ee12f254a19e59a262829`. A clean source-bound formal producer receipt must follow the implementation commit. The current integrated 179-case aggregate is not revised by this note.

## Limits

The registered F03 producer establishes its unit and serialized native structure tiers only. It does not establish Apple execution or final release acceptance.

F04 remains UNVERIFIED. The two synthetic `qa-f04-yield-*-v2.cherri` probes inspect List-valued and empty-List-valued Repeat Results with scalar controls. Compilation, loop-end UUIDs and count consumers do not prove one semantic item per iteration or absence of native flattening. Their exact signed native results are required before promoting F04.

## Clean committed verification

Source `27547c55b330dac9085d710597ce9420aab5ab8c` passed the combined F03/T07 formal producer gate and F04 non-promotion test (three passing gate tests, zero failures). All underlying named production tests and causal manifest controls executed. The tree was clean before and after the run. Raw log SHA-256: `443af74fbf9a1c5794adb654597c917cb667198687275e34aeabcb321aa42d72`. Complete source-bound receipts and causal baseline artifacts are in [the evidence directory](evidence/p15-f03-t07-20261119/README.md).

## F04 native failure and current compiler boundary

The original List-yield probe on source `ff04ee8887c701824e5161f3fff1431ff204bb65` reached a correlated target SUCCESS callback in [run 44431486363](https://github.com/davidpovarsky/cherri/actions/runs/44431486363). Its semantic result was `pairs=4:scalars=2:shape=COUNT_MISMATCH`, where two nested List values were required. The exact oracle remains failed; the archive under `docs/apple-runtime/p10r15/native-followup-20261119/f04-list-ff04ee8-44431486363` preserves the original callback, source/signing identities, publication manifest and independent replay. This is not F04 completion.

The current compiler rejects a statically known List-valued final result of a value-producing `for` or `repeat` with `E_TARGET_VALUE_SHAPE` before native emission. The check includes aliased Lists, optional/union List branches, nested value loops and empty Lists. Scalar and record results, statement loops, and List-valued `if`/menu expressions outside that loop boundary retain their existing behavior. None materialization and the proven omitted-input variable-clearing encoding are unchanged.

Unknown or AnyContent results and the existing untyped native-last-action fallback remain unverified native semantics; this guard does not promise one preserved semantic item per iteration for those paths. No Dictionary wrapper is substituted under a List<List> type.

Phase 0 still executes compilation and structured CLI analysis for both preserved F04 sources, requires the exact diagnostic and absence of native artifacts, and records them as BLOCKED_PLATFORM / NOT_RUN. They are not counted as compiled fixtures or Apple passes. Selecting either unsupported probe stops before signing/Simulator use. Historical runs stay bound to their original source; storage, export, scalar collection and other supported fixtures continue through normal compilation.

Canonical native metadata exposes Repeat inputs but no established nested-value preservation option. Apple's [Repeat data-flow guide](https://support.apple.com/en-ie/guide/shortcuts/apdc11deb2c1/ios) describes gathered loop outputs; it does not establish a lossless nested List representation. A future lowering requires independent signed native proof of nested and empty cardinality before this diagnostic can be relaxed.
