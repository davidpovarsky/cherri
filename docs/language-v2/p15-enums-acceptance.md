# C04: contextual and qualified enum resolution

## Contract and reproduced failures

The immutable C04 contract in tests/language-v2/acceptance-contract.json requires contextual and qualified enum members, correct type/member suggestions and exact wire values, with both unit and service evidence. Its previous runner accepted one raw string and a nonempty completion list; that did not exercise enum syntax.

Baseline job job_20e5553ca3dc on source bbc60bc0c0fef4c8e56f831615486c5ec0f3543f reproduced the defects through the production parser, analyzer and compiler:

- Contextual native and custom members such as .Ocean passed analysis but failed lowering as an unknown empty enum name.
- A lowercase-qualified native enum was mistaken for a runtime property and rejected by its literal-only parameter.
- An explicitly empty source wire value became the member name. The parser and formatter also discarded significant quotes and empty assignments.
- Unknown members, absent contextual types and mismatched source domains had no enum diagnostic.
- Completion used every parameter's enum and, after a dot, every catalog enum. Its labels and inserted dots disagreed with the editor's actual identifier-prefix replacement behavior.

The first baseline invocation stopped at an incorrectly quoted test oracle. That oracle was corrected against the checked-in single-quoted canonical declaration before the causal baseline above; the preliminary failure is not implementation evidence.

## Corrected production behavior

A shared enum catalog combines canonical registry domains with the current program's declarations. It keeps each member name and exact wire string separately. No sanitized native member names or replacement wire values are invented.

The analyzer resolves native and source members, including lowercase qualifiers, in the expected action parameter, custom-action parameter, function parameter/default/return, annotated binding and assignment contexts. Typed final values of if/menu/scoped blocks receive that context. Lexical value bindings retain precedence over same-named enum qualifiers. Unbound shorthand, nonexistent members and directly mismatched domains fail with E_ENUM_MEMBER; duplicate source members fail with E_DUPLICATE_NAME.

Resolved enum values remain semantic Text, as in the existing language model. They are stored by expression identity without rewriting the source AST. The lowering adapter transfers this metadata and scoped-block renaming preserves it. Map membership distinguishes an empty resolved value from a missing resolution. The existing imported-declaration qualified path remains available and respects lexical bindings. Native actions continue through the shared serializer.

Custom-action parameter schemas now retain their enum domain and existing wire binding, so default and named argument checks use the actual declaration. A resolved qualified literal is checked as a literal; arbitrary property accesses do not gain literal status.

The service tokenizes the document prefix to identify the active call argument across lines, comments, nested calls and quoted punctuation. It proposes only the relevant domain or an explicitly qualified domain. Labels are member names, which the editor can filter against the typed identifier prefix. Insertions add a dot only when the user has not already supplied one. Nonidentifier member spellings are offered as exact quoted wire literals in a bare argument context and are not emitted as invalid dot syntax.

## Executed production proof

p15_enum_acceptance_test.go contains these producers:

| Producer | Actual assertions |
| --- | --- |
| TestP15C04EnumResolutionProduction | 17 exact native-wire cases, including canonical native identifiers/keys independently anchored in actions/a11y.cherri; contextual and qualified source/native values; omission/default, empty, quoted and Unicode wires; source format/recompile identity; 20 invalid member/domain/declaration cases with exact diagnostics and zero emitted bytes; wrong-wire and wrong-diagnostic controls |
| TestP15C04EnumServiceAndCLI | 15 real completion replacements compiled to independently expected native values; active parameter, lowercase qualifier, empty prefix, existing dot, multiline, nested call, comment, quoted comma and Unicode contexts; unrelated-domain rejection; invalid-completion mutation rejection; all invalid cases through service edits and real CLI JSON/exit/range checks, followed by corrected document analysis |
| TestP15C04EnumContextsAndShadowing | Binding/assignment/optional/default/function return/if/scoped-measure contexts; inferred Text and exact resolved values; format/recompile; quoted source member declarations; four required/defaulted uppercase/lowercase/native-name lexical dictionary shadow controls with original-producer/key wire and mutation checks |
| TestP15C04EnumFunctionService | Four contextual or qualified user-function replacements, including the public underscore alias; actual native function-envelope Text item and exact empty wire; separate explicit empty and nonempty argument controls |

Unicode fixtures use Go escapes for the intended Greek, Hebrew and non-BMP code points, avoiding shell encoding substitutions. The CLI range oracle independently counts UTF-16 code units.

The complete four-producer batch passed in job_fbea85c64c74. Earlier expanded attempts found invalid test inputs (an optional value passed to a required Text parameter, a missing required measure label and a map passed as a record); those fixtures were corrected to obey the existing contracts. No type restriction was weakened.

## Formal accounting and limits

The C04 runner executes all four named producers and records both required tiers with the actual command and implementation SHA. The service tier names the real service/CLI producer. Its evidence test rejects missing service evidence, unrelated producers, absent records and a changed requirement ID. The audit keeps all 179 requirement rows; no other requirement inherits this result.

This proves the C04 source and shared-service contract when the complete named command passes on the recorded source. Enums retain the language's Text representation; this is not nominal runtime validation of arbitrary dynamic strings. It does not certify Apple execution, full editor UI behavior, all module-loading obligations, the separate C05 unknown imported-wire round-trip contract, or release completion. No native metadata or canonical catalog fingerprint is changed.

A subsequent review reproduced two additional missing-analysis cases in job_7e52e2df411f: a bare source enum type used as a value and an invalid literal in a contextual branch tail. Both now use the same stable enum diagnostic before native emission. The original type, literal and cardinality contracts remain separate.

The stricter shadow control also exposed missing static types on the two existing function-parameter BindingRef paths. Those paths now retain the declared type, so a shadowed uppercase Map parameter emits its original Dictionary-key reference instead of a generic content-property reference. This is static lowering metadata, not a new runtime cast or dispatcher.

Final expanded source/CLI/service controls and existing C02/custom-enum compatibility passed in job_b893424efafd (10.370 seconds package execution). Full analysis, syntax, lower and service packages passed in job_f84cda66b0c9 before the final two analyzer guards and parameter-type retention; the immutable receipt reruns the complete packages and formal producers on the committed source.

## Immutable source receipt

Exact clean source 6d4db11adaa5a8a7d1fd44b5f68c4792c26127bc passed the complete analysis/syntax/lower/service packages (70 top-level tests) and all five C04/C02/T02/T06/T08 evidence producers in job_b713286ec98c. Both required C04 tiers are verified; the source was clean before and after. See [receipt and hashes](evidence/p15-enums-6d4db11/receipt.json), [the C04 record](evidence/p15-enums-6d4db11/verification.json), [raw package output](evidence/p15-enums-6d4db11/packages.jsonl), [raw producer output](evidence/p15-enums-6d4db11/verification.jsonl), and [the full bounded verification](evidence/p15-enums-6d4db11/verification-all.json). The three retained RED logs reproduce the original enum defects, contextual validation gaps, and typed shadow reference defect. This receipt does not certify unexecuted native/UI tiers or release completion.

## Review follow-up: native domains and scoped completion

The native parameter keeps its actual ParameterSchema.EnumValues after source-name resolution. Two valid source aliases of backgroundSound map to the canonical Ocean wire; contextual, qualified and raw invalid native wires are rejected through compilation, service and CLI. A source enum does not replace the native action\'s allowed-value gate.

Independent review found that the service declaration collector omitted the structured body of a scoped call. Job_ba23ab1d5ab5 reproduced an enum completion leaking through a measure-local value shadow. The collector now walks the existing body with its lexical span. A negative inside the block and a positive after its end check both scope boundaries. No macro lowering, native serializer or canonical catalog changes are needed for this follow-up.

The review follow-up passed all four production enum tests and selected shared service regression tests in job_1d1fe7ed3628. The new alias controls confirm the native allowed-value check was already retained; the only production change is the three-line structured-block declaration walk.

The exact clean follow-up source dd3c26668ae405005c03808729422bc2f7840057 passed the C04 producer in job_8964e28cbc6b (19.468 seconds), including all four real production tests and both required tiers. See [follow-up receipt](evidence/p15-enums-dd3c266/receipt.json), [exact record](evidence/p15-enums-dd3c266/verification.json), [raw output](evidence/p15-enums-dd3c266/verification.jsonl), and [scoped-shadow causal failure](evidence/p15-enums-dd3c266/baseline-red.log). Earlier receipts remain bound to their original source SHAs.


## Supplied registry enum domains: full-suite regression follow-up

The complete integration run exposed a missing registry input: an action can
provide its enum name and allowed wire values on ParameterSchema without
also registering a global enum in Registry.AllEnums. The existing P10
automatic action propagation test supplied exactly that contract. C04's new
catalog incorrectly treated the absent global entry as an empty domain and
rejected the valid "Accurate" value.

The shared catalog now includes explicit enum values carried by the supplied
registry's action parameters. Registry enum members and parameter members are
combined without changing the registry. Each call still validates the resolved
wire against its own parameter's allowed values; a broader action cannot widen
another action that shares the enum name. Source enum declarations retain their
existing override behavior and the independent native allowed-wire check.

TestP15C04ProvidedRegistryEnumDomain exercises a separate registry with no global
enum declaration: exact literal, contextual, qualified and empty wire values;
six unknown-member or narrower-domain rejections; and actual shared service
completion followed by re-analysis. It also checks that neither the supplied
registry nor the default registry is mutated. The original
TestP10_AutomaticActionPropagation_Scenarios1To4 retains its Legacy/V2 actual
native-payload comparison. The new test is included in the C04 production
evidence producer; no other formal ID is promoted by this repair.

Causal reproduction: job_eb5195d44eae failed both the original P10 scenario
and the new registry-isolation test before the catalog correction. The earlier
attempt used an incorrect test helper method name and failed to build; that
attempt is not counted as a behavioral reproduction. Exact committed-source
verification is retained separately from the development run.
