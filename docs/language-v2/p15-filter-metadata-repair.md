# Canonical filter metadata transport repair

The V2 canonical adapter must preserve a filter specification as compile-time metadata until the existing `buildContentItemFilter` validator and encoder consumes it. A filter specification is selected by the actual canonical parameter key `WFContentItemFilter`.

## Observed regression

Linux CI run `44402799497`, job `135150026633`, failed these unchanged production tests:

- `TestP10_IPadAcceptanceKit_CompileAndStructure` at `ipad_07_content_item_filters`.
- `TestP10R2_A08_DispositionCodePathVerification`.
- `TestP10R_R08_FilterTwoLevelAndOrRoundTrip`.

The public CI log SHA-256 is `fa551dacdb36fbbd7a266d77b2b2fa9ecae05cf71e7895e48048ecab9145cb86`. Local execution on clean source `57a72ed05f97158c85d2ed591801b439041caa41` reproduced all three failures with the same message: the filter builder received `[]main.WFDictionaryFieldValueItem` instead of a predicate array or map. The three original test files remained byte-for-byte unchanged.

The general V2 `ValDict` argument conversion correctly produces typed Dictionary field items for runtime Dictionary parameters. Applying that conversion before the filter metadata builder changed its input representation. Predicate arrays already used the semantic parameter converter and remained successful controls.

## Bounded production change

`convertCallArgForCanonicalParameter` examines the canonical definition's actual wire key after argument position resolution. For an explicitly supplied semantic Dictionary or List in `WFContentItemFilter`, it calls the established `ConvertSemanticValueToParam` converter. The existing filter builder then validates and encodes the specification.

All other parameters retain `convertCallArgToActionArg`. Omitted arguments retain their existing path. The repair introduces no action registry, predicate serializer, parser, runtime collection producer or inference from ordinary dictionary key names. It does not alter `actions_std.go`, the Legacy parser, raw native emission or the typed runtime Dictionary codec.

## Verification scope

`p15_filter_metadata_test.go` exercises actual compilation and native output:

- Compound Dictionary, predicate-array, shorthand-map, empty-map and empty-array inputs.
- Exact predicate order, compound mode, numeric zero, false Boolean values, literal braces and backslashes.
- The filter's input UUID and absence of extra runtime List/Dictionary producers for metadata.
- Explicit serialized predicate tables with unknown nested fields and ordinary reserved-key spellings, including an actual Legacy compilation. The exact expected numeric kinds are frontend-specific: V2 integers and the existing Legacy JSON real values. A retained pre-fix CLI independently verifies the Legacy representation.
- Eight malformed cases reaching the existing specific validation failures.
- Eight deliberate corruptions rejected by the independent native table oracle.
- XML/binary, hybrid/lossless, Legacy/V2 exact native import round trips.

The three original Linux-failing tests run unchanged. Existing malformed-filter and ordinary Dictionary/List literal tests supply compatibility controls. The immutable receipt records actual outcomes, compiler/source identity and preserved RED evidence.

This repair proves compiler input transport and serialized filter preservation. It does not execute queries against Files, Reminders, Calendar or user data, and it does not claim Apple runtime filter results or promote a formal acceptance requirement.
