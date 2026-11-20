# P15 compiler-owned Optional function presence — 2026-11-20

## Result and limits

Clean production source **b94e82f0e42149a042e960f03d75078c7ecd3e09** passed the complete selected root replay: **763 named tests across 95 top-level tests**, no failures or skips, in 83.797 seconds. The complete internal/language/types, analysis and lower packages then passed **53 named tests**, no failures or skips, in 10.078 seconds. Both commands ran with clean source before and after and unchanged HEAD. The original finite budgets remained in place (root Go 5m / outer 330s; packages Go 3m / outer 240s).

This is a bounded compiler/native-structure increment. It does **not** complete T05, L01, FN05 or RP20, does not register or inherit any formal 179 pass, and establishes no Apple execution. No original acceptance contracts, source runtime fixtures, signed-output goldens or native import payloads were rewritten.

## Protocol and behavior

Function omission and logical value presence are independent. Existing has_arg_n chooses the supplied/default branch. An explicitly Optional function parameter additionally carries Text has_value_n: 0 for None, 1 for a present value. A supplied None therefore skips a default while a missing argument evaluates its default once under the existing guard. Present false, zero, empty Text, empty List and empty Dictionary are tagged present; none of these values is inspected for truthiness to infer absence.

Optional function returns use the private Dictionary marker _cherri_optional_result_v1 with Text has_value and a value field only for a present value. The caller decodes the flag and reads the value only on the present branch; the absent branch emits real Nothing. Authored return none and the compiler's existing implicit no-value fallthrough produce the same explicit absent carrier. A bare typed return remains a Void return-type error. Top-level None clearing and non-Optional function returns preserve their prior boundaries.

Authored comparisons to None on tracked parameters and direct Optional call results use the compiler-owned presence flag compared to Text 0, including both operand orders, negation, statement conditions and yielded conditions. Direct result-producing calls are evaluated once. The protocol does not use HasAnyValue/count/native type guessing as a logical presence oracle. The existing outer function-dispatch input guard remains unrelated to semantic Optional presence.

Private function carriers now retain explicit semantic Value:[] for empty arrays and explicit zero Dictionary rows. Opaque native.plist values and Legacy/raw array omission templates are unchanged. The codec descriptor at internal/language/backend/contracts.go is versioned v2.1, advertising Optional argument flags and the tagged result form; this component's computed fingerprint is 16c8733d5903ac8c96e7cd3b91bbd0503134e760702262323a7f354860dfa5df. Any later combined descriptor change must be recomputed from its own source. Consumer pins are owned by the integration pipeline.

## Independent checks

The new root tests compare complete expected native action graphs, control nesting, every semantic wire parameter, producer/consumer edges, UUID uniqueness and action multiplicity. They are static native inspections, not substitute Apple interpreters.

- Nineteen supplied/default scenarios distinguish explicit None, empty Text, false, zero, empty containers, named arguments, None defaults and ordinary defaults. Text-producing default actions are required exactly once inside the omitted branch.
- The explicit empty-List result has a complete value-presence graph and six corruption controls; preceding noise cannot become its output.
- 108 parameter predicate scenarios cover six logical values, six equality/inequality/negated forms and three contexts. Eight direct-call predicates require one evaluation and the returned presence edge.
- Two implicit no-value return routes include branch returns, and a separate real diagnostic control requires E_RETURN_TYPE for a bare typed return.
- Fifty-three new corruption subtests cover flag value/type, supplied/default independence, decoder edges, branch polarity, truthiness substitution, Nothing, stale output, invented values, explicit empty arrays, default placement, joins and output routing. Unchanged native-preserving clones must pass before mutation.
- Existing T01 characterization now strictly requires the repaired explicit empty array field and rejects its historical omission in both affected supplied cases. All original presence flags, default guards and runtime fixtures remain unchanged.
- Existing F03 None yield typing now requires exact OptionalUnknown|Text and MayBeAbsent, preserving its original native clearing and mutation checks. NewOptional(Unknown) intentionally collapses to Unknown and therefore was the wrong constructor for an authored-None oracle.

## Preserved failures and corrections

All original streams and summaries are copied byte-for-byte, with SHA256 hashes in manifest.json. Earlier dirty-source attempts remain labeled as such. The 330-second outer timeout is INCOMPLETE; 18 passing events do not turn it into a passing run. The final clean replay genuinely reran the old tests as well as the new ones.

The first new predicate baseline confirmed 116 incorrect predicate scenarios and unconditional bare fallthrough. Its branch-fallthrough specimen additionally had an oracle omission: the existing Text parameter comparison legitimately carries WFStringContentItem coercion. That omission was corrected explicitly, the old failing stream was retained, and a new missing-coercion negative now rejects removing the legitimate cast. The later clean 724-pass attempt failed because the old T01 empty-array characterization expected an omitted Value field; the final stricter characterization requires Value:[] and causally rejects omission. Neither failed attempt is presented as a full pass.

| Attempt | Source | Named passes | Exit | Classification |
| --- | --- | ---: | ---: | --- |
| tagged-optional-baseline-clean | 22863b89 | 0 | 1 | clean RED; new parameter-presence contract absent |
| tagged-optional-result-baseline-clean | 82de3670 | 2 | 1 | clean RED; new result-presence contract absent |
| tagged-optional-root-precommit | 82de3670 | 604 | 1 | dirty precommit failure; supplied empty Array omitted its explicit Value field |
| f03-optional-type-integration | 9e44c3c1 | 1 | 0 | dirty focused PASS only; root separately replays integrated F03 |
| tagged-optional-root-precommit-v2 | 89aeae2c | 18 | -1 | INCOMPLETE; original 330-second outer budget reached after 18 passes |
| tagged-optional-focused-precommit | 89aeae2c | 145 | 0 | dirty focused PASS; not clean-source proof |
| tagged-optional-predicate-baseline-clean | 0aba3359 | 1 | 1 | clean RED: 116 real predicate scenarios and unconditional fallthrough; branch-fallthrough also had a later corrected Text-coercion oracle omission |
| tagged-optional-predicate-precommit | 0aba3359 | 272 | 1 | dirty precommit failure: 272 passes; branch-fallthrough oracle omitted existing Text coercion |
| tagged-optional-root-clean | 6f70bbfc | 724 | 1 | clean failed replay: 724 passes; old T01 characterization required historical omitted empty Array Value |
| tagged-optional-root-clean-v2 | b94e82f0 | 763 | 0 | clean complete PASS: 763 named / 95 top-level tests |
| tagged-optional-packages-clean-v2 | b94e82f0 | 53 | 0 | clean complete PASS: all 53 types/analysis/lower tests |

## Outstanding semantic work

Presence propagation through aliases, mutable bindings/assignments, value-block joins, general forwarding and Optional type aliases is a subsequent component. Existing precise rejection controls remain for unsupported function forwarding and ordinary Dictionary/List Optional transport. This report does not claim complete arbitrary Optional expressions or native container cardinality, nor any intrinsic Apple impossibility. An independently executed signed public fixture is still required to establish native runtime behavior.

## Source and receipts

Root raw SHA256: a4b4dfa4df60cf125aec41febe98ade3d256b85ac4623ed057469d5559bfca15.

Package raw SHA256: 25bae8ed6626cfa36e8b718ac2a72082bf4dbce37116a20e01533314a2d016ad.

Original commands, times, source identities, process receipts where captured, failed/incomplete attempts and selected immutable Git source blobs are retained under evidence/p15-function-optional-presence-20261120/. The evidence directory's .gitattributes disables newline conversion so retained bytes stay exact.

Commit sequence after 46c0763a:

- 22863b892b952a2a9b693a07e6e474d2d3f1e634 test: distinguish supplied None from value and default presence
- 82de3670e415a02d4a015e4a34ad6499abaacb37 test: require tagged optional result and guarded default carriers
- 9e44c3c182956ee55db3d430eb618c9ced530a8d test(language): retain authored None in branch result type oracle
- 89aeae2caf754087bef377234feba8f3946298e4 test(language): require explicit empty optional values and guarded defaults
- 57746c8eda2d82ab282ce6e359bb8ce8dbfc7399 fix(language): tag optional function arguments and results independently
- 0aba3359461112d7924b63656c3f9e27c582d642 test(language): require tagged None predicates and fallthrough results
- 5004ec753bf21e254038848f4c2019528fa05509 test(language): retain declared Text coercion in optional return oracle
- 6f70bbfc35d1372834f5f6fea395b8ea2e793f47 fix(language): consume tagged presence in None predicates and fallthrough
- b94e82f0e42149a042e960f03d75078c7ecd3e09 test(language): require explicit empty array fields in function envelopes
