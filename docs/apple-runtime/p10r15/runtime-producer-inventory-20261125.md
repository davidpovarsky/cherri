# Remaining Apple runtime producers: source audit, 25 November 2026

Audit base: a25a65784c22e53bd1f110e1fc7baaaab13daac6. This is a source inventory, not an execution result.

The current Go acceptance producers explicitly keep the full scenarios below pending their own iOS runtime proofs. The historical generated requirements-evidence-map.json still contains broad PoC aliases; current tools/language-acceptance/evidence_map.go bypasses those aliases for these complete scenarios. The six verified historical PoC records supply only their reviewed ten requirement facets, on their historical source.

Existing signed probes are useful diagnostic components. A registered nearby probe does not establish the complete requirement in this table. Missing acquisition/independent observation/intake is distinguished from an implementation failure: absence of an exact producer does not prove the language feature unsupported.

| ID | Current complete local producer | Registered nearby signed probes | Missing actual full-scenario proof |
| --- | --- | --- | --- |
| F02 | tools/language-acceptance/loop_execution.go | qa-collect-counts-v2; qa-collect-counts-legacy | Complete named repeat and nested for owner/index/outer-scope observations. |
| FN04 | tools/language-acceptance/function_default_execution.go | No exact registered caller-input probe | Direct/nested system.input and ShortcutInput under actual caller input, including no-functions entry. |
| M02 | tools/language-acceptance/main_output_execution.go | qa-nil-clear; runtime PoC functions facet | Actual main empty/Void/explicit/bare/None return, zero-iteration and child-continuation boundaries; native and migrated entry modes. |
| T01 | tools/language-acceptance/function_default_execution.go | qa-typed-defaults; qa-optional-forwarding-diagnostic | All positional/named empty defaults plus nested/reserved-key Map, Optional Map omitted/None/present-empty and default effect ordering. |
| RP12 | tools/language-acceptance/capture_order_execution.go | No complete registered named-effect probe | Reverse/mixed/positional/alias/native argument forms with exactly-once written-order effects. |
| RP14 | tools/language-acceptance/capture_order_execution.go | qa-binding-capture | Saved Number/Bool/Text/List/Map/Clipboard values and native types after mutation; existing probe covers only Number and Clipboard. |
| RP15 | tools/language-acceptance/combined_arithmetic_execution.go | runtime PoC numbers facet; qa-number-coercion-control | Complete action-derived compound updates, returned Number 11 and each RHS effect observed exactly once. |
| RP17 | tools/language-acceptance/combined_arithmetic_execution.go | qa-number-comparison-diag; qa-n03-text-equality-diagnostic | Full mixed arithmetic/unary/parenthesized and comparison outputs/types from the joined source. |
| RP18 | tools/language-acceptance/short_circuit_execution.go | qa-short-circuit; qa-short-circuit-local | All eight dynamic AND/OR cases and four mixed trees with all lexical helpers, observed counts/order and Bool outputs. |
| RP19 | tools/language-acceptance/combined_unicode_execution.go | qa-rtl-hebrew; qa-export-unicode-diagnostic | Full literal 42/false, adjacent/action tokens and exact Hebrew/non-BMP/combining text in the joined scenario. |
| RP21 | tools/language-acceptance/combined_index_arithmetic_execution.go | runtime PoC indexing facet | Static/dynamic empty/first/last/negative/fractional/out-of-range/uint64-max behavior and no invalid native lookup. |
| RP22 | tools/language-acceptance/loop_execution.go | qa-collect-counts-v2; qa-collect-counts-legacy | Full named-repeat/tuple-for same-name shadows and restored outer item/index observations. |
| RP24 | tools/language-acceptance/function_return_execution.go | No exact registered joined probe; source tests/language-v2/v2-rp24-return-recursion.cherri exists | Actual add(2,b:3)*2 = 10, factorial(3) = 6 and continuation = 7 from the exact complete source. |
| RP25 | tools/language-acceptance/function_combined_default_execution.go | qa-typed-defaults; qa-optional-forwarding-diagnostic | Thirteen joined default/transport variants, structured result types and Optional argument/value-presence observations. |
| BRG08 | tools/language-acceptance/combined_unicode_execution.go | qa-rtl-hebrew; qa-export-unicode-diagnostic | Joined repeated refs/raw braces/inline transformed values and exact rendered text/ranges. |
| BRG10 | tools/language-acceptance/capture_order_execution.go | qa-binding-capture | One joined program combines mutable/Clipboard captures with independently observed out-of-schema argument effects. |
| BRG11 | tools/language-acceptance/parameter_forms_execution.go | qa-typed-defaults | Full joined valid literal/reference/numeric-text/enum/variadic parameter routes and typed default presence behavior. |
| BRG12 | tools/language-acceptance/loop_execution.go | qa-collect-counts-v2; qa-collect-counts-legacy | Mixed nested loops both directions, siblings, shadows, A:1/A:2/B:1/B:2 and restored outer item/index. |
| BRG13 | tools/language-acceptance/short_circuit_execution.go | qa-short-circuit; qa-native-boundary-diagnostic; qa-n03-text-equality-diagnostic | Joined counted dynamic logical cases, typed inputs and exact dynamic same/different-case Text and Number comparisons. |
| BRG14 | tools/language-acceptance/combined_value_block_execution.go | qa-f04-yield-list; qa-f04-yield-empty-list; qa-nil-clear | Joined value if/menu/loops, actual cardinality, early/bare returns and main Void no-leak across frames; general nested List/Optional remains a compiler gap. |
| BRG16 | tools/language-acceptance/combined_index_arithmetic_execution.go | runtime PoC indexing/numbers facets; qa-number-coercion-control | Joined action-derived division/modulus, ordered compound updates and guarded dynamic indexing with actual Number/Text/None outcomes. |

## Execution order

First restore the shared native acquisition path without changing its source, timing, privacy or value assertions. The S627 short-circuit run 45529529353 failed its actual cancellation/descendant control before orchestration. The S627 nested-list control 45529532308 passed Python controls, compiled and signed its exact control, then rejected project-prerequisite observation before native execution. No nested candidate follows a failed control.

After the control reaches native execution, existing qa-functional-legacy, qa-typed-defaults and qa-optional-forwarding-diagnostic can produce new source-bound diagnostic observations. The RP24 source is already a standalone complete source and is a useful next candidate for a specifically reviewed native acquisition and verifier. Such new evidence must preserve exact program/input/output/type identity and be registered against the full corresponding requirement; the old functions trace cannot supply it.

Final release evidence still needs one fresh final-candidate Runtime PoC. R22 additionally requires actual Darwin and simctl iOS27 runtime inventory, scripts/ios27_runtime_poc.sh exit zero and the exact IOS27_SHORTCUTS_RUNTIME_POC=PASS marker. Physical iPad acceptance remains a separate boundary.

## Sources

The immutable original requirements are tests/language-v2/acceptance-contract.json, repair-regression-plan.json and backend-recovery-gates.json. The table was checked against the scenario definitions and local-tier declarations in each named Go producer, current evidence_map.go and the explicit selections in .github/workflows/cherri-ios27-shortcuts-simulator-qa.yml.
