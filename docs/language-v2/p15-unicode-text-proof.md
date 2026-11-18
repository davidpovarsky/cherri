# S01 Unicode text: local production proof

The immutable S01 contract requires unit, native round-trip and iOS evidence for multiple variable tokens after Hebrew, emoji and combining marks. The local producer now executes named tests against the actual compiler and import pipeline.

## Exact local scope

Two independently specified strings cover Hebrew niqqud, decomposed Latin e plus acute, precomposed accented text, a woman-technologist ZWJ emoji, a supplementary-plane globe, repeated references to the same original producer, a mutable native variable, and a literal U+FFFC that is not an attachment.

| Synthetic case | Required attachment ranges in UTF-16 code units |
| --- | --- |
| Hebrew, combining marks, ZWJ emoji and repeated producer | `{15, 1}`, `{19, 1}`, `{22, 1}`, `{26, 1}` |
| Literal U+FFFC followed by an emoji and variable tokens | `{3, 1}`, `{4, 1}`, `{7, 1}`, `{11, 1}` |

The test oracle reads serialized native objects directly. It checks exact text bytes, attachment cardinality, each numeric range, original producer UUID and output name, the mutable variable's identity, and the Set Variable input that initialized that mutable value. It does not normalize text or reuse the compiler's attachment encoder.

The two strings are compiled through six paths each: action argument, immutable binding, mutable binding, reassignment, List field and Dictionary field. Ten mutations cover byte/rune offsets, off-by-one offsets, normalization, removed niqqud, removed ZWJ, missing attachments, swapped producers, a different variable, and an incorrect range length.

Six independently checked native workflow trees then pass through actual filesystem import and recompilation: XML and binary plists, hybrid and lossless import, and both Legacy and V2 frontends. All 48 routes must preserve the entire native tree and pass the same token oracle again. This is serialized-native preservation; it does not run Shortcuts.

Two additional isolated Go overlays deliberately replace the ordinary lowerer's and canonical shared text codec's UTF-16 calculations with rune counts. Each selected test execution must fail at the numeric UTF-16 range assertion. These overlays are untracked causal probes; they do not change the committed encoder. An initial probe of the lowerer's secondary IR-only path did not affect canonical output and is retained as an unsuccessful probe, not counted as causal rejection evidence.

## Formal evidence boundary

`TestP15S01ExactUTF16TokensAndLiteralBytes` supplies the unit tier. `TestP15S01UnicodeNativeRoundTrip` supplies the native-roundtrip tier. The recorded command also executes `TestP15S01UnicodeOracleRejectsCorruption`, and explicit discovery checks reject a command that did not execute the named tests.

The formal producer preserves the original S01 contract and records only the two executed local tiers. The Runner remains PENDING_EXTERNAL; the local manifest report remains unclosed with its existing NOT_RUN external-tier status; the 179-requirement audit records BLOCKED_EXTERNAL for S01. No other acceptance, repair or recovery gate inherits this local result.

The verifier rejects a historical generic smoke producer, cross-tier producer substitution, and a local Go test relabeled as iOS runtime. Final validation still fails without current-source iOS evidence. A future native result must satisfy the existing runtime assertion and provenance contract; successful compilation, native round trips and this local receipt cannot supply that result.
