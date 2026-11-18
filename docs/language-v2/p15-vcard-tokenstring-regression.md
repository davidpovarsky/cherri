# P15 makeVCard literal-template regression

The Linux differential gate at source 800270786c5d02c7fd1f9a36ed1ac6d44d64ec3c rejected the migrated canonical menus fixture with E_LITERAL_REQUIRED. The original Legacy fixture contains inline strings with runtime values, and the existing migration correctly expresses them as explicit V2 interpolated strings.

The canonical makeVCard title and subtitle parameters already declare both Literal and tokenString. Its existing shared serializer emits WFTextTokenString, retains ActionOutput references, and shifts title/subtitle attachment ranges using UTF-16. The literal-purity validation introduced for T02/C02 rejected this supported template form before that serializer could run.

The correction accepts an explicit FString expression only when a compiler-declared literal parameter also declares tokenString. Bare bindings, action calls, dynamic concatenation, and confirmed literal-only enum parameters retain their existing diagnostics. No native serializer, migration rule, generated schema, original fixture, or parity normalization changed.

## Production verification

The new tests exercise the existing Legacy serializer, actual Legacy migration, authored V2 calls, named-argument reordering, exact producer UUID references, non-BMP/combining-mark text, UTF-16 ranges, and five causal native-graph corruptions per positive case. Removing tokenString from a copied real registry action must restore E_LITERAL_REQUIRED. The original menus fixture is compiled with both actual CLI paths and compared through the unchanged differential comparator, while its bytes are checked for preservation.

## Immutable execution receipt

Implementation commit: 495db211bcf9dd02459ad1e3fb4305c8b3fd580f, based on e9da6c1438589527da8535d3e068444adc366a98. The [evidence bundle](evidence/p15-menus-495db21/evidence-audit.json) binds raw execution logs and reports by SHA-256.

- The causal RED run reproduced the original E_LITERAL_REQUIRED for the migrated menus CLI path and both authored/migrated token-string cases; the Legacy wire control and six rejection controls already passed.
- The clean implementation passed eight focused production families, with 91 named test results, including all existing C02/T02 producers. Full analysis and migration packages passed 43 named results.
- The unchanged original menus fixture is PROVEN_EQUIVALENT through the existing normalized comparator. Its original bytes and the canonical manifest, generated registry, and differential runner were preserved.
- All 17 representative action parity cases passed.
- The full public differential gate remains FAILED: 45 of 47 fixture paths passed, with two V2 compilation failures in date-comparisons.cherri and nil.cherri. There were zero migration failures and zero emitted parity differences.
- An actual fresh-compiler replay on exact parent e9da6c1 reproduced both remaining type failures and the original menus failure. That parent run passed 44 of 47 fixture paths. The new token-string acceptance therefore repairs menus without causing or concealing the two remaining type failures.

The remaining date failure reports a comparison operand with AnyContent instead of Number/Date. The remaining nil failure reports assignment of Unknown to the File-inferred mutable variable media. They require separate typed migration/analysis investigation; this change does not weaken those checks or alter their fixtures.

Compile and native-structure verification do not establish Apple runtime behavior for this interactive fixture. No signing, Apple execution, private-corpus run, complete-release claim, or additional formal-179 closure is made by this receipt.
