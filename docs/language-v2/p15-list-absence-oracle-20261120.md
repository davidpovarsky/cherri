# List absence diagnostic alignment

## Verified change
Source b880f27697cb09b94d73e8e68bf2804c4f5c6c1a changes only the expected diagnostic for the optional_absent case in TestP15L01InferredNestedNativeListFailsClosed. It requires both E_TARGET_VALUE_SHAPE and the exact List-element absence message. The compiler and mobile routes must still return no native or signed bytes. All fourteen other rejection/positive cases and their native cardinality checks remain intact.

The combined R4 run at 059d6bf4 exposed the stale oracle: the earlier semantic absence guard correctly prevented reaching the later nested-List shape guard. The retained integration raw SHA reported by the integration owner is 76cfd833e9bd226258ce201a1f5f0531bf3aaecfd9d7eef2e4b7064302ed7e6a; that complete run is not represented here as successful.

## Clean replay
The exact committed source above ran go test -count=1 -json -timeout=3m -run '^TestP15L01InferredNestedNativeListFailsClosed$' . on the Windows host. Result: **16 named passes, zero failures and skips**, exit 0, 25.171 seconds. Git was clean before and after.

Original raw SHA-256: aa904363c36ddfcd12c93a6a62b10d58804ea9a9ef9556bbbcda61c73aa5e596. The evidence directory preserves raw output, stderr, process and result receipts, and the exact source blob under its manifest.

## Limits
This is a test-oracle alignment, with no production or immutable acceptance-contract change. It does not establish support or intrinsic unencodability for nested native Lists, Optional container transport, Apple runtime behavior, or completion of L01/T05/FN05/RP20.
