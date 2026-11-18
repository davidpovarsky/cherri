# P15 metadata and setup acceptance

This stage implements and verifies M01, M03 and M04 through the existing compiler, canonical backend, native import pipeline, editor service and mobile bridge. It adds named production evidence producers for only those three immutable acceptance IDs. It does not close M02, RP31, other linked gates, Apple import-question UI, Swift UI execution, or release readiness.

## Changes and causal observations

| Requirement | Observed defect | Shared correction |
| --- | --- | --- |
| M01 | A `targets` list overwrote native `WFWorkflowTypes` with OS names, replacing the requested invocation surfaces | OS targets continue to constrain action availability; `from` controls native invocation surfaces |
| M03 | A setup argument bound before a later argument emitted another action, so its question pointed at the wrong action | Defer binding until the call is emitted and resolve the final index from its existing call-node identity |
| M03 | A declaration name matching another question's final wire key rebound the wrong question | Keep declaration identity separate from the native `ParameterKey` |
| M03 | Later dated-debug expansion can insert actions before a bound question | Canonical finalization locates the actual emitted parameter map after all insertions |
| M04 | Aliases, expression uses, function arguments and repeated setup references could compile with no editor error | Shared semantic validation accepts one direct concrete native parameter use and rejects unsupported value flow |
| M04 | Required palette arguments and five construct snippets silently inserted runtime `Ask` | Reuse existing `<#name#>` editor holes; unchanged holes fail compilation and must be explicitly filled |

No native UUID or metadata field is added to carry setup identity. The backend keeps compiler-only declaration/call/parameter associations. Missing, duplicate or removed target identity fails with `E_SETUP_BINDING`; it is not guessed from an expected action plan.

## Supported setup source

```cherri
setup apiKey: Text {
    prompt: "Enter the API key"
    default: "synthetic-default"
}
text(apiKey)
```

A setup declaration has a Text default and binds to exactly one direct, concrete native action parameter. `String` is the existing Text alias. Separate questions are required for separate parameter slots. Aliasing a setup value, embedding it in arithmetic/interpolation/collections, passing it as a function argument, using it in a compiler macro, or binding it to an enum, literal-only or variadic slot is rejected. Other declared setup types fail explicitly rather than receiving a silently coerced text default.

A direct native parameter use inside a user-function body is supported: the function dispatcher emits one body, and repeated function calls do not duplicate its import question. A local variable or function parameter that shadows the setup name remains an ordinary scoped value. An unused declaration reports `W_UNUSED_SETUP` and emits no question pointing accidentally at action zero.

Explicit authored `text(Ask)` remains valid. The palette no longer chooses that runtime behavior on the user's behalf. For example, `show(<#input#>)` must be filled as `show("example")` or deliberately changed to `show(Ask)`.

## Verification scope

Production tests inspect serialized native fields, not only lowering IR:

- Known header mappings cover all existing `from` surface spellings, input/output content classes, quick-action surfaces, no-input behavior, icon values and platform availability.
- Native unknown header fields, nested values and future surface/content-class names survive exact XML/binary import and reconstruction through hybrid/lossless modes in both Legacy and V2.
- Setup tests cover action zero, later argument producers, function expansion, dated-log backfill, declaration/wire-key collisions, shadowing, and direct function-body binding.
- Setup native round-trips retain `ActionIndex`, including zero, and the actual parameter value. Causal mutations of index presence/value, parameter key, target value and question multiplicity fail the oracle.
- Backend controls reject a missing/duplicate emitted action, a removed parameter and attempted question reuse. Late insertion and UUID changes leave the correct parameter binding intact.
- Shared compiler/service tests reject unsupported setup value flow. All 352 current required-input catalog snippets fail while unfilled through the compiler and Go mobile bridge; construct snippets compile only after explicit synthetic values fill their holes.
- Existing C01 argument order, V02 lexical scope, R01 envelope preservation and raw-result identity regressions are retained.

The existing `WFQuestion` serializer already emitted `ActionIndex: 0`; this stage preserves that behavior. Imported exact metadata is not rewritten by authored-setup validation.

Swift core/UI tests retain their exact completion/prefix assertions and now add an unfilled-build rejection followed by explicit filling and successful compilation. Those updated Swift assertions require a fresh Xcode/Simulator run; local Go/mobile bridge proof is not represented as Swift execution.

## Reproduction

```sh
go test -count=1 ./internal/language/analysis ./internal/language/lower ./internal/language/schema ./internal/language/service ./tools/language-schema
go test -count=1 -run '^TestP15M0(1|3|4)|^TestP15AC01ConstructGrammarAudit$|^TestIOS003' .
go test -count=1 -run '^TestMetadataProductionEvidence$' -v ./tools/language-acceptance
go run ./tools/language-schema --verify-generated
```

The generated schema comes from the existing live canonical generator and retains the same 505 actions. No acceptance status is established by this document alone; exact clean-source test and formal-producer receipts accompany the source commit.


## Clean source receipt

Immutable source `7991ea61ffb627fae566c297452e828a912cec49` passed the named M01/M03/M04 production proof with six required-tier records. The shared verifier rejected twelve detached, missing or incomplete evidence mutations. The source tree was clean before and after the run, and canonical schema verification passed.

- [Command and hash receipt](evidence/p15-metadata-7991ea6/receipt.json)
- [Production records and bounded scope](evidence/p15-metadata-7991ea6/production-evidence.json)
- [Complete formal proof log](evidence/p15-metadata-7991ea6/formal.log)
- [Canonical schema verification](evidence/p15-metadata-7991ea6/schema-verify.log)

Before this clean proof, the complete analysis, lowerer, schema, service and schema-generator test packages passed, followed by the focused root metadata/mobile/C01/V02/native-envelope/native-result/R01-R05 regression selection. The clean formal proof does not convert those local executions into Swift UI or Apple import-question UI evidence.
