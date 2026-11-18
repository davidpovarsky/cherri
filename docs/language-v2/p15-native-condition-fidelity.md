# Native condition fidelity and Boolean values (P10R15)

The F01 short-circuit repair initially made imported compound conditions less editable. The immutable R1 candidate `d00bc0b9e9a4d8c460623de3d883680b94289e04` retained exact final shortcut bytes through native fallback, but the private corpus audit dropped below its unchanged natural-source floor: 22,936 verified natural actions versus a required minimum of 23,340.

## Causal finding and recovery

The preflight deciding whether a logical expression can use one native `WFConditions` table allowed identifiers and literals but rejected reference-only member/coercion expressions, dictionary-key aggrandizements, exact `native.ref` metadata, and text-token interpolation. Lowering these conditions through Boolean expressions added actions. The importer correctly rejected that expanded natural candidate because its action count no longer matched the input; the fallback preserved bytes but lost natural editable source.

The repair accepts only recursively checked native reference/token forms. Arbitrary action calls, calls hidden under members or indexing, calls in `native.ref` arguments, and producer calls inside interpolation remain excluded. Dynamic and list indexing still use lazy expression lowering. Encoded `native.plist` metadata is accepted only as a literal.

Nine synthetic production imports cover coercions, properties, dictionary keys, output-name overrides, exact aggrandizements, text-token RHS values, and native presence forms. Ten nested producer cases separately prove that an RHS Run Shortcut action appears inside the correct AND/OR branch. The first seven native-operand cases failed before the correction and passed afterward.

On exact fix commit `1cd80624eb5caa2800421e0f6d0550dda3e65a20`, the unchanged `TestP13_CorpusFidelityAndSourceSizeAudit` passed on all 38 private inputs:

| Metric | Result |
| --- | ---: |
| Total actions | 27,652 |
| Verified natural actions | 23,353 (84.45%) |
| Exact native fallbacks | 4,298 |
| Source-invalid actions | 1 |
| Stage-3 demoted actions | 0 |
| Test duration | 111.43 seconds |

That test also checks exact recursive Legacy and V2 output equality for every input. No threshold was relaxed. Private source content and specimen names were kept outside published evidence; only aggregate results are reported here. This receipt belongs to the listed commit and does not replace the final integrated candidate audit.

## Negated predicates and predicate values

A second causal test found that modern/header-bearing compound conditions could lower `!(value beginsWith "x")` to Has No Value on a computed Boolean. False is still a present native value, so that was incorrect. Predicate helpers such as `isTrue`, `isFalse`, `isToday`, and `isBetween` were recognized by analysis and statement lowering but failed when used as values or under negation.

Negated conditions without a direct native inverse now use the Boolean expression path inside the selected lazy branch. Predicate helpers used as values reuse the existing native statement-condition emitter with explicit true and false result yields. This gives the result a real Boolean output and evaluates its argument only once. Existing direct imported native predicates keep their original wire shape.

The regression suite covers both comparison modes, newly authored predicates appended to a genuine imported workflow, typed Boolean bindings, and helper arguments containing Run Shortcut producers. The signed F01 fixture includes negated helpers, a negated string-prefix check, and a range predicate value; its expected marker remains `P15_F01_OK`. Apple execution proof remains separate from local compile/structure evidence.

## Provenance limit

Existing import metadata identifies symbols rather than the authorship of each condition. The compiler preserves native presence semantics for an original imported symbol and its native reference aggrandizements. A newly authored check on that same imported reference can express Boolean value intent explicitly with `== true` or `isTrue(...)`. New Boolean bindings and computed Boolean expressions use value semantics. This ambiguity is documented; header provenance is never a blanket exemption for eager RHS producers.
