# Native empty List and If-result controls at 4e7956d

All three runs used immutable compiler
`4e7956d2519307861fe703a11fbe401fc9faca05`. The audit independently verified
20 published file hashes and byte counts per run, the exact publication path
set, the committed fixture bytes, signing/compiler provenance, a fresh
fixture-bound callback nonce, and replay of that commit's strict runtime oracle.

| Run | Fixture | Expected native summary | Actual native summary | Strict result |
| --- | --- | --- | --- | --- |
| [44346443743](https://github.com/davidpovarsky/cherri/actions/runs/44346443743) | Empty List split | `direct=0:repeat=0:cleared=0:emptyText=1` | `direct=1:repeat=1:cleared=0:emptyText=1` | FAIL_RUNTIME |
| [44346470843](https://github.com/davidpovarsky/cherri/actions/runs/44346470843) | If-expression bound with `let` | `P15_F01_IFRESULT_OK` | Exact expected marker | PASS |
| [44346497175](https://github.com/davidpovarsky/cherri/actions/runs/44346497175) | Identical If-expression bound with `var` | `P15_F01_IFRESULT_OK` | Exact expected marker | PASS |

## Empty List finding

A single correlated synthetic invocation returned four independently computed
counts. Direct native `List[]` counted one, and Repeat results from that same
input counted one. The existing cleared-variable mechanism produced zero
Repeat results. A one-element List containing empty Text counted one.

This isolates the direct empty-List representation as a concrete difference
from the zero-item control within this fixture. It does not establish the type
or content of the unexpected item. The original strict expectation remains
`0/0/0/1`; it was not changed to accept the failure.

The callback was a fresh native SUCCESS and carried two inline trace records
plus collector BEGIN and END. Its semantic output was wrong. The strict oracle
therefore retained FAIL_RUNTIME for both runtime and trace gates. The published
strict trace contains one terminal failure record; the callback artifact
retains the actual four records. The audit distinguishes these two counts.

The next production repair should reuse the already observed cleared-variable
mechanism and receive its own exact-source native pilot. This receipt does not
claim that such a repair has run.

## If-result finding

The paired specimens differ only in the source `let`/`var` binding of the
ordinary If-expression result. The former directly references the named End If
output; the latter stores the unnamed End If output in a mutable variable.

Both reached the exact expected output with two inline records and two
collector records. In both cases the persisted trace exactly equals the actual
callback trace, and the frozen strict oracle replays PASS.

This falsifies the narrow hypothesis that naming/directly consuming an End If
output by itself causes the earlier local short-circuit failure. It does not
prove arbitrary nested logical composition, and it does not close the original
unchanged F01 runtime fixture.

## Provenance and limits

The detailed JSON audit records source, unsigned-artifact and signed-artifact
hashes, native action counts (29 / 32 / 33), exact published files, workflow
paths, fresh nonces, gates and oracle replay results.

All three ran on iPad mini (A17 Pro), iOS 27.0 Simulator, with Xcode 27.1 build
27A9275 and macOS 27.0.1 build 26A434. Editor Inspector remains NEEDS_REVIEW.
Physical-device execution and release closure remain unproved.

Raw signed/unsigned Shortcut files stay quarantined. Their hashes are reported
provenance; the independent rehash covers the published bounded text artifacts.
No screenshot, private path or raw OCR is added to this receipt.
