# Native number kinds through Legacy rawAction

Source: `3a5e333199c2c89d7f5400b002d69f071c1ce915`.

## Causal result

The exact same public test definitions were executed against two production variants on Linux ARM64 with Go 1.24.13, GOMAXPROCS=2, and -p=1. The baseline restores only parser.go, decompile.go, and raw_actions.go from `d6872ba6f17f3cc592a587430541b803078a212e`. It reproduces both strict filter roundtrip defects and the new numeric controls: 11 named PASS events, 7 named FAIL events (including parent cases), exit 1. The clean source candidate produces 34 named PASS events, no failures or skips, exit 0. The working tree is clean after restoration and validation.

The complete command, timestamps, source identity and exact log digests are recorded in receipt.json. The unchanged comparator checks native plist integer and real kinds separately.

## Repair and scope

Legacy rawAction dictionaries now reuse the strict native JSON number decoder, preserving integer lexical tokens and their 64-bit precision. Decompilation preserves real lexical tokens even when their value is mathematically integral, such as 1.0 or negative zero. Ordinary Legacy dictionaries retain their prior parser behavior. References continue through the symbolic reference conversion rather than becoming an opaque plist.

The fixture tests native integer values, the full unsigned 64-bit limit, a value above binary64's exact integer range, signed integers, integral and fractional reals, negative zero, nested filter operator/prefix values, Boolean and numeric Text controls, and a producer-output reference. Both XML and binary input plists are decompiled and recompiled with strict whole-workflow comparison.

This is a compiler structural roundtrip proof. The synthetic unknown action is never presented as an Apple executable action or a native Simulator result. The independent F04 runtime flattening failure remains a separate unresolved contract.
