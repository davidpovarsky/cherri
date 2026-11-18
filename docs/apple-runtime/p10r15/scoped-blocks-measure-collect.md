# Scoped value blocks, measure, and existing Repeat Results

Date: 2026-11-18. This is a bounded implementation stage for Asana tasks
[typed blocks](https://app.asana.com/0/1219603572114432/1220544718901241),
[measure](https://app.asana.com/0/1219603572114432/1220544665232149), and
[collect](https://app.asana.com/0/1219603572114432/1220544651247337).
These roadmap tasks remain open pending the acceptance items below.

## Implemented source contract

V2 accepts a trailing zero-argument lexical value block:

```cherri
let result = measure("Fetch") {
    let payload = run("SyntheticTarget")
    payload
}
getVariable(result)
```

The body is inline compiler syntax, not a closure or another Run Shortcut call.
It executes once. Its final expression or final `yield` supplies the result.
Bindings inside it have lexical scope. Local mutable variables receive unique
native storage names, including locals in nested branches and loops. Assigning
an outer mutable variable still changes that outer variable intentionally.
Hygienic lowering clones the AST and retains source spans and analyzed types;
string values, Dictionary keys, and the original source AST are not rewritten.

The parser uses one generic trailing-block field on CallExpr. Capability
metadata for `measure` lives in the existing compiler macro catalog. Other
calls reject blocks unless they have a supported lowering contract. The native
Apple catalog and its fingerprint are not extended with a fictional action.

The brace must follow the closing parenthesis on the same line. In an `if`,
`for`, or `repeat` operand, parenthesize a block call so the enclosing control's
body remains unambiguous:

```cherri
if (measure("condition") { true }) {
    text("selected")
}
```

Formatting preserves that boundary. Ordinary `if check() { ... }`,
`for item in source() { ... }`, and `repeat count() { ... }` keep their existing
parse. Existing user-defined functions named `measure` continue to work when
called without a trailing block.

## Native measurement and value flow

Lowering invokes the existing canonical `debugStartTimer` and `debugStopTimer`
contracts. They use CurrentDate directly with the existing context-specific
native date codec. There is no added Get Current Date action or alternate plist
serializer. Each source measurement has its own timer key, so two nested
measurements with the same label cannot overwrite one another's start time.

The body's action result retains its producer UUID and typed reference. Timer
elapsed Numbers never replace it. Mutable and system values are frozen at the
body boundary; in particular, returning CurrentDate does not reread it after
timing work. A native Get Variable forwards the saved body result after the
timer, restoring implicit data flow for Repeat Results and following actions.
This is object forwarding, without Text/Number coercion or body reexecution.

In Debug, elapsed seconds append to the existing execution-local debug log.
The label scopes the timer key; this stage does not add a labeled structured
performance-event schema. Timing precision is that of the existing Apple
actions, with no millisecond claim. An abort, native error, Cancel, or an
explicit enclosing Shortcut return can prevent the stop/log actions from
running. No try/finally behavior is promised.

Release removes the timer macro expansions through the existing canonical
build-profile policy. Body effects and typed result forwarding remain. The
forwarding action is retained to make the value boundary explicit. A measured
body that explicitly consumes a removable diagnostic result remains subject
to the existing `RELEASE_DIAGNOSTIC_VALUE` rejection.

## Collection decision: retain the existing expression

The collect task explicitly requires checking the current loop before adding
an alias. Existing V2 syntax already provides the desired native contract:

```cherri
let results = for item in items {
    yield {"name": item, "flag": false}
}
getVariable(results)
```

Compilation emits one Repeat Each start/end pair with a shared grouping ID.
The consumer references the end action's Repeat Results UUID. No Add to
Variable accumulator is introduced. Causal tests compare N=0, 1, and 100 and
require a constant number of native actions, one compiled loop body, and the
same direct result edge. An explicit `result += item` fixture demonstrates the
additional accumulator action that the result expression avoids. Dictionary
and List body cases retain native producers.

Therefore this stage adds no redundant `collect(...)` syntax. The fixture
`v2-collect-existing-repeat-results.cherri` is reusable for a future signed
order/value run. Compile-time graph size is not an Apple memory/latency
benchmark. Empty-list behavior, null omission, and nested Repeat Results
flattening still need native evidence; this stage does not promise a new
nested-list shape beyond the existing loop behavior.

## Explicit boundaries

- General block parameters, escaping closures, user-defined block macros, and
  optional block overloads are not implemented.
- `measure` currently requires a non-empty literal label and a materializable
  final value. Empty/void bodies and standalone none/null values fail with
  diagnostics rather than silently producing elapsed time.
- A nonterminal top-level `yield` is rejected because it is not an early
  block return. Final control expressions require explicit final yields on
  every result path. Their branch types are joined instead of trusting only
  the first branch.
- Raw native calls and global Copy/Paste constructs inside the scoped body
  are rejected when their absolute variable names or global scope cannot be
  reconciled with lexical block guarantees. Ordinary native imports outside
  these new blocks remain on their existing path.
- Legacy receives `E_V2_SCOPED_BLOCK`. Its supported alternative is explicit
  debugStartTimer/debugStopTimer calls around a saved result. This release
  does not pretend the V2 trailing-block syntax is Legacy syntax.

## Evidence and remaining acceptance

Actual focused Go tests passed in the root, syntax, analysis, and lowering
packages. The full syntax, analysis, lowering, schema, and language-service
suites also passed. Tests inspect independently decoded canonical plists for
single execution, original producer identity, Dictionary preservation,
nested timer keys, native variable shadowing, Release stripping, and direct
Repeat Results edges. Parser/formatter tests cover Unicode and braces inside
strings, nested blocks, and control-body disambiguation. Negative tests assert
specific diagnostic codes and source spans.

The runtime fixture waits one second inside nested measurements and checks
the returned Dictionary/List plus an outer timer's elapsed interval of 1..8
seconds. That tolerance is an explicit future acceptance condition, not a
recorded runtime result. No new Apple run or timing pass is claimed here.

Outstanding: signed Debug and Release body/result checks; live timing and
abort behavior; generalized block parameters; native nested collection shape;
latest exact import/private-corpus gates and full cross-platform CI on the
integrated candidate. Native decompilation preserves the emitted actions;
recovering `measure` source sugar is not claimed.

Rollback is a revert of this feature commit. The isolated feature extends the
existing frontend/backend path and changes no signer, device, production
service, or private input.
