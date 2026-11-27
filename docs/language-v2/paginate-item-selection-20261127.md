# Typed paginate item selection — 27 November 2026

This component implements literal field selection and typed native result lists for the original [paginate task 1220544665232021](https://app.asana.com/1/1219597879016953/project/1219603572114432/task/1220544665232021). It extends the existing shared paginator and immutable page callback. It does not complete that task's independent native, Link/status, recovery, general selector-body or release obligations.

## Source contract

Two optional arguments follow the original eight positions:

| Option | Accepted source | Meaning |
| --- | --- | --- |
| `itemKey` | Omitted/`nil`, or a literal matching `[A-Za-z_][A-Za-z0-9_-]{0,63}` | Omission returns each item itself. A present key requires each item to be a Dictionary and selects that exact, present field. Dotted paths and arbitrary selector callbacks are not this option. |
| `itemType` | Literal `"dictionary"`, `"text"`, `"number"`, or `"bool"` | Selects the required native category and V2 list element type. The default remains Dictionary. |

The shared schema owns the finite kind list, defaults, key validation and result type. The analyzer resolves the same literal options as the native emitter. Explicit optional `nil` holes are omitted consistently with the canonical binder. Invalid or dynamic selectors do not become omitted defaults.

```cherri
let titles: List<Text> = paginate(
    "https://example.invalid/pages/one.json",
    itemKey: "title",
    itemType: "text"
)
for title in titles {
    let checked: Text = title
    getVariable(checked)
}
```

Without `itemKey`, `itemType:"number"` selects numbers directly from the page's items. Legacy supports the same two added positional options after `queryKey`. Both frontends retain the optional `{ page in ... }` callback. The callback still receives the original immutable page Dictionary after selected items have passed validation; its body remains Void.

## Value and control guarantees in the emitted graph

The result accumulator remains the original native List. A selected item flows through a singleton Count and a native type comparison, then directly into Add to Variable using the same original reference. The implementation inserts no JSON serialization, Text conversion or private value cell.

A field selector first validates the source Dictionary. It enumerates keys, compares SHA-256 digests of exact key bytes, requires presence, and only then performs the field lookup. Missing/wrong-case keys stop with the fixed diagnostic `PAGINATE_ITEM_KEY_REQUIRED`. Wrong selected categories stop with `PAGINATE_ITEM_<KIND>_REQUIRED`; no partial result is returned by that failure branch.

Number and Boolean selection additionally compare an authored native Boolean witness with the native Number witness before traversal begins. If the runtime cannot distinguish the categories, the graph stops before any GET with `PAGINATE_NUMBER_TYPE_UNAVAILABLE` or `PAGINATE_BOOL_TYPE_UNAVAILABLE`. A Number zero/one is not accepted as Bool merely because its numeric value resembles a Boolean. The same protection prevents false/true being accepted as Number when native categories alias. The selected values themselves are never converted to establish their category.

Either zero cap returns the original empty List without witness helpers, GET or callback. The existing default Dictionary graph is exactly equal to the pre-implementation normalized graph in both dialects. Existing URL/cursor/page/offset traversal, caps, cycle detection, headers and callback control remain shared.

## Shape matrix and native boundaries

| Requested shape | Compiler result | Actual graph/model qualification | Apple native determination still required |
| --- | --- | --- | --- |
| Whole Dictionary | `List<Map<Text, AnyContent>>` | Existing default graph unchanged; ordered original objects, including nested native fields, preserved by the model. | Native Dictionary type/cardinality and nested field behavior. |
| Selected Dictionary field containing a Dictionary | `List<Map<Text, AnyContent>>` | Required exact field; original selected Dictionary reference appended. | Native field lookup and Dictionary preservation. |
| Whole or selected nonempty Text | `List<Text>` | Unicode, combining marks, quotes, escapes, duplicates and case distinctions retain exact values/order. Actual downstream typed loops consume the same references. | Native Text cardinality/category and append behavior. |
| Whole or selected Number | `List<Number>` | Zero, negative and fractional values remain Number; Text and Bool are rejected under distinct native-category assumptions. Number/Bool alias simulation stops before GET. | Whether the real platform distinguishes Boolean and Number. No unconditional availability is claimed. |
| Whole or selected Bool | `List<Bool>` | False and true remain Bool; numeric 0/1 and Text false are rejected under distinct-category assumptions. Alias simulation stops before GET. | Whether the real platform distinguishes Boolean and Number. No unconditional availability is claimed. |
| Empty Text | `List<Text>` source is accepted | The explicit shared model counts empty Text as one item and preserves it. The emitted singleton guard rejects a zero-item lookup. | Actual Content Graph treatment of empty Text, including possible omission before the item loop. This is not a proved lossless native case. |
| Empty/singleton/multiple nested List selected as a scalar | Scalar source contract only | Named model cases reject the category/cardinality mismatch. | Whether native lookup/Get Type flattens or aliases singleton collections; model rejection is not native evidence. |
| Selected JSON null or absent field | No Optional/null result contract | Missing field has a presence guard. Named model null cases fail the selected-value guard. | Native null/absence and item-enumeration behavior can differ from the declared model. |
| Standalone List, Optional, JsonNull, mixed AnyContent, Date or File item type | Rejected literal option | `PAGINATE_ITEM_TYPE`; no public selection codec is fabricated. Ordinary scalar/Dictionary Lists use the existing native representation. Nested List/absence/null source cells remain private. | A lossless public codec and its original native acceptance would be separate work. |

The exact fixtures are defined by `cases(kind, key)` in [the executable selection verifier](../../tools/p10r15/verify_paginate_selection.py). In particular, `empty-text-model-assumption`, `singleton-list-not-a-scalar`, `null-rejected`, `number-one-is-not-bool`, `bool-false-is-not-number` and `boolean-number-alias-before-get` retain the distinctions above. Native runtime acceptance is **not run** by this component.

## Causal and final qualification

Original evidence is under `/mnt/development/projects/agent-workspaces/cherri-macro-completion-20261127/paginate-evidence`. The [compact qualification](evidence/p15-paginate-selection-20261127/qualification.json) records exact source hashes, original artifact hashes, command results and joined identities. Compressed streams are byte-preserving copies; the final model report is copied without modification.

These are distinct processes, not one combined suite:

| Attempt | Actual result and scope |
| --- | --- |
| `red-01` | Original capability tests against base `a7631a04`: 13 named PASS and 48 named FAIL. The package adds one separate FAIL event. Frozen original Dictionary graphs were captured here. |
| `green-01` | Build failed before tests because the new analyzer used the nonexistent name `TokenNil`. The spelling was corrected to the existing `TokenNone`; the failed stream is retained. |
| `green-02` | 399 root paginate identities and 100 shared paginate identities passed in separate Go commands. This precedes the symmetric Number-alias correction. |
| `model-01` | First actual CLI/model run: 56 compiled graphs, 440 model checks and 6 causal controls passed. It predates the Number-alias correction and is not relabeled as final. |
| `model-alias-red` | The added Number/Bool alias scenario ran against the prior binary and failed: Number mode reached a request instead of the required pre-request capability stop. |
| `green-03` | On the final production code, 409 named root identities passed and 17 new callback-composition identities failed. Their authored sources compiled; the new oracle incorrectly expected the callback before item selection. Production correctly retained the established callback-after-validation lifecycle. |
| `green-04` invocation | Runner construction had a Python syntax error before any Go command. It is a preserved invocation failure, not a test run. |
| `green-05` | Only the incorrect callback-order assertion was changed. The same 17 original callback identities passed; actual compiler build and final model also exited zero. Production Go source stayed byte-identical to `green-03`. |

The final root join is **409 prior PASS plus 17 focused replay PASS, covering 426 distinct named identities**. The replay identity set exactly equals the preceding 17 failures. No skip, test removal, changed authored source or production lifecycle change is used to obtain this join.

The final [model receipt](evidence/p15-paginate-selection-20261127/final-model.original.json) contains **56 actual CLI graphs, 56 exact native-action import/recompile roundtrips, 448 model checks and 7 rejected causal corruptions with passing restored controls**. Eight actual V2 downstream typed-loop graphs consume every selected original object. Zero-cap graphs perform no model request, including under the unfavorable Boolean/Number alias simulation. `nativeExecuted` is exactly zero.

The source and shared model are unchanged during each recorded execution. The final qualification also binds the supplied compiler's hash to the recorded successful build and checks the final verifier hash. The model reuses the existing loopback server and graph interpreter; it observes source identity without replacing compiler output or converting selected values.

## Reproduction and original remaining obligations

Build the compiler from the checkout, then run the focused source tests with `P15_SELECTION_BASELINE` pointing to the preserved frozen baseline directory. The semantic producer accepts an already built compiler so a caller can reuse the exact qualified build:

```sh
python3 tools/p10r15/verify_paginate_selection.py \
  --compiler /absolute/path/to/cherri \
  --output-dir /absolute/path/to/selection-products \
  --report /absolute/path/to/selection-report.json
```

The existing [original obligation inventory](macro-original-completion-obligations-20261125.md) remains authoritative. General item selector bodies and nested/optional/null public results, native type/cardinality and dictionary/list preservation, same-request Link/status/headers, redirects, malformed body and transport behavior, catch/retry, cancellation, final signed Apple/device execution and final metadata/consumer/release pins remain outside this component's qualification. The original Asana task stays open.
