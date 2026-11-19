# T06 control-result diagnostic oracle

Test-only implementation: `3132ce8946929f251b26e8474eec92f38d55f036`. Production code is unchanged.

The clean baseline `359135c1ff06ee79f3d630696f7d7bc676d5a629` reproduced the three failures reported by the full
suite: `missing_branch_yield`, `overwritten_yield`, and `nested_missing_yield`.
The result was 7 passing named cases and 4 failures including the parent, with
zero skips. The compiler had already rejected all three cases without native
bytes; the stale assertion expected `E_RETURN_TYPE` to be the first compiler
error even though the explicit-yield validator correctly reports
`E_YIELD_CONTEXT` first.

The repaired oracle requires exactly two errors for those three cases:
`E_YIELD_CONTEXT` at the precise invalid branch span, followed by
`E_RETURN_TYPE` at the precise `return value` span. Other invalid control cases
still require exactly one return-type error. The shared language service must
produce the same complete diagnostic set. The test rejects missing or changed
codes, shifted spans, weakened severity, duplicates, and changed primary order.

Every invalid case still requires a failed compiler call and zero native bytes.
The three explicit-yield violations also execute the real CLI and must produce
no Shortcut artifact. A healthy CLI control must create a nonempty artifact, so
an unavailable or always-failing compiler cannot satisfy the rejection test.
Existing valid branch joins and heterogeneous union results remain covered.

The exact clean implementation commit passed the repaired T06 family together
with all existing `TestP15F03*` tests: **59 named tests/subtests passed, 0 failed,
0 skipped**, in **8.71 seconds**. Source cleanliness was verified before
and after execution. This is compiler/test evidence, not an Apple runtime claim.

See [execution-receipt.json](execution-receipt.json) for the exact commands,
source SHAs, discovered test names, counts, and hashes;
[clean-baseline.jsonl](clean-baseline.jsonl) and
[clean-source-focus.jsonl](clean-source-focus.jsonl) are public synthetic logs.
