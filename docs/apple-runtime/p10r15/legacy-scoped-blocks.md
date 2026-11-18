# Legacy scoped measure bodies

This stage extends the existing `measure` compiler macro to Legacy. Both frontends use the same backend ordering policy: start the timer, emit the body once, capture its typed result, stop the timer, and forward the original value. Release removes the existing timer instrumentation and retains the body, its side effects and its result.

This is a bounded value-block implementation. The shared-block and measure Asana tasks remain open. Source compilation and exact native-action roundtrip do not establish Apple runtime timing, cancellation or early-error behavior.

## Syntax and value flow

```cherri
const outer = text("outside")
const measured = measure("example") {
    text("captured before shadow: {outer}")
    const outer = text("inside")
    const payload = {"message": outer, "flag": false}
    yield payload
}
getVariable(measured)
text("outside again: {outer}")
```

A Legacy body ends with either a value-producing call or a final `yield` for a literal or reference. The label is one non-empty literal. The opening brace follows the call on the same line. Nested measurements use distinct timer keys even when their labels match.

The body is parsed once by the existing Legacy parser into existing token, action and value structures. It is not serialized to source or reparsed by V2. Each statement retains the lexical environment in effect before its declaration. Local native variable names are hygienic, while assignments to a captured mutable binding preserve that binding.

A scoped constant can shadow an outer binding. A second declaration in the same lexical scope is rejected. If/Else, Repeat, For and Menu statements retain the existing native control structures; branch, loop and menu-case bindings are restored when that scope ends.

Existing native attachments carry values. A mutable variable or system value is read before timer-stop work so the result cannot accidentally become the elapsed duration. Action-output references keep their original producer. Final forwarding uses the existing native Get Variable action. File, Dictionary and List references are not converted to Text.

Nested action arguments inside these bodies, and an explicit measure-valued argument outside a body, materialize through existing constant tokens before the consuming action. This prevents a nested action from disappearing into an empty native parameter and preserves exactly one producer/effect. Ordinary Legacy source outside this new path retains its previous parser behavior.

## Boundaries

- General parameterized blocks, escaping closures, optional-block overloads and user-defined block macros remain unimplemented.
- Empty or Void bodies, nil final values, nonfinal yields and yields inside a nested control region are rejected explicitly.
- A Legacy control-result declaration inside this body, such as `const value = if ...`, is not yet supported. Existing control statements can update a local/captured mutable value and a final yield can return that value. V2 keeps its existing typed control-expression path.
- Raw native actions and workflow metadata/question/raw-declaration directives inside the parsed body are rejected because their absolute/global identities cannot promise lexical scope.
- Primitive construction retains each frontend's existing contracts. This stage does not introduce new Bool/List wire encodings or promise arbitrary native object serialization.
- Runtime errors and shortcut exit retain native behavior. No catch, finally, retry or cancellation-cause mechanism is claimed.
- These graph checks do not repair or overturn the recorded failed label, export or paginate native probes.

## Validation

Production Go tests cover Debug/Release typed result identity, Unicode Text, nested body effects, nested argument materialization, mutable and CurrentDate snapshots, structured property references, captured mutation, same-label timer isolation, If/Else/Repeat/For/Menu scope restoration, trailing comments, invalid forms and compiler-state restoration after rejection. Every positive Legacy fixture checks that native producer UUIDs are unique and every ActionOutput reference points to an earlier emitted producer.

The existing macro-contract producer now compiles Legacy measure in both profiles, performs exact lossless native-action roundtrip, and retains all previous causal checks. Additional graph mutations exercise a cyclic Legacy result alias and an inserted Text coercion. Identity traversal rejects missing, duplicate, forward, cyclic and transformed result edges.

The first draft exposed a canonical-codec type-assertion compile error. A stronger File-input check also showed why checking only the outer File UUID was insufficient: the nested Text call had disappeared and WFInput was empty. That producer-reference check now requires the actual Text UUID feeding the File action. The original failure logs remain local and are summarized in the source-bound receipt.

Native signed Debug/Release body/result acceptance, elapsed-time measurement and physical-device evidence remain outstanding.
