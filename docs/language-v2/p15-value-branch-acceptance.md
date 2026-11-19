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
