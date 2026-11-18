# P15 makeVCard literal-template regression

The Linux differential gate at source 800270786c5d02c7fd1f9a36ed1ac6d44d64ec3c rejected the migrated canonical menus fixture with E_LITERAL_REQUIRED. The original Legacy fixture contains inline strings with runtime values, and the existing migration correctly expresses them as explicit V2 interpolated strings.

The canonical makeVCard title and subtitle parameters already declare both Literal and tokenString. Its existing shared serializer emits WFTextTokenString, retains ActionOutput references, and shifts title/subtitle attachment ranges using UTF-16. The literal-purity validation introduced for T02/C02 rejected this supported template form before that serializer could run.

The correction accepts an explicit FString expression only when a compiler-declared literal parameter also declares tokenString. Bare bindings, action calls, dynamic concatenation, and confirmed literal-only enum parameters retain their existing diagnostics. No native serializer, migration rule, generated schema, original fixture, or parity normalization changed.

## Production verification

The new tests exercise the existing Legacy serializer, actual Legacy migration, authored V2 calls, named-argument reordering, exact producer UUID references, non-BMP/combining-mark text, UTF-16 ranges, and five causal native-graph corruptions per positive case. Removing tokenString from a copied real registry action must restore E_LITERAL_REQUIRED. The original menus fixture is compiled with both actual CLI paths and compared through the unchanged differential comparator, while its bytes are checked for preservation.

The clean-source execution receipt will be recorded separately. Compile and native-structure verification do not establish Apple runtime behavior for this interactive fixture.
