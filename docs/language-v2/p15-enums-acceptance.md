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
| TestP15C04EnumResolutionProduction | 15 exact native-wire cases, including canonical native identifiers/keys independently anchored in actions/a11y.cherri; contextual and qualified source/native values; omission/default, empty, quoted and Unicode wires; source format/recompile identity; 17 invalid member/domain/declaration cases with exact diagnostics and zero emitted bytes; wrong-wire and wrong-diagnostic controls |
| TestP15C04EnumServiceAndCLI | 13 real completion replacements compiled to independently expected native values; active parameter, lowercase qualifier, empty prefix, existing dot, multiline, nested call, comment, quoted comma and Unicode contexts; unrelated-domain rejection; invalid-completion mutation rejection; all invalid cases through service edits and real CLI JSON/exit/range checks, followed by corrected document analysis |
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
