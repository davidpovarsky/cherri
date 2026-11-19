# F04 fail-closed native List boundary

Verified compiler source: `3d08eab1c2770a3ad2836fd695907a22aa2ef794` (implementation series `46d3853`, test-fixture correction `b031493`, and terminated-path refinement `3d08eab`).

## Result and limits

The compiler now rejects statically established List-valued final results of value-producing `for` and `repeat` expressions with `E_TARGET_VALUE_SHAPE` before native emission. This prevents a nested List cardinality promise that the native platform did not satisfy. Optional and union results that may be Lists are covered. Guaranteed-return paths, overwritten early List yields, scalar results, records containing a List field, statement loops, List-valued branches outside a value loop, and explicit opaque native references retain their distinct behavior.

Unknown/AnyContent and the untyped native-last-action fallback remain unverified native result boundaries. This change does not manufacture one preserved semantic element per iteration for them. It does not wrap Lists in Dictionaries, change the expected native result, or complete F04.

The historical signed source `ff04ee8887c701824e5161f3fff1431ff204bb65` remains bound to [run 44431486363](https://github.com/davidpovarsky/cherri/actions/runs/44431486363). Its exact target callback observed `pairs=4:scalars=2:shape=COUNT_MISMATCH` instead of two nested Lists. The original runtime failure is separately preserved under `docs/apple-runtime/p10r15/native-followup-20261119/f04-list-ff04ee8-44431486363`.

## Actual causal verification

| Gate | Observed result |
| --- | --- |
| Original production baseline from e5f941f, with the same final public tests | 13 rejected-boundary controls lacked the required rejection; 15 named failure events including parents, 14 named passes, no skips |
| First guard's guaranteed-return regression | One unreachable-yield leaf failed plus its parent, reproduced by restoring the b031493 guard |
| Exact clean 3d08eab focused F04/F03/storage/empty-collect tests | 93 named PASS events; zero failures or skips |
| Complete analysis and lower package tests | 52 named PASS events; zero failures or skips |
| Exact clean compiler build | PASS; embedded VCS revision 3d08eab and vcs.modified=false |
| Related Python dispatch, callback, storage and continuation checks | 25 tests PASS; no skips |
| Actual Phase 0 CLI fixture attempts | 62 attempted: 60 native artifacts emitted, 2 exact diagnostic rejections with no native artifacts |
| Selected fixture guards | Both unsupported F04 targets stop at exit 3 before signing; storage and Unicode export targets pass |
| Causal rejection, release-body and publication controls | 4 + 3 + 5 damaged cases rejected |

The compiler SHA256 is `d619018494464ea928b607bded13a4f5b79d3f762d506faa9d76fede6a473332`. Full commands, timestamps, binary build provenance and log digests are in `verified/receipt.json` and `verified/consumer-receipt.json`. The source tree was clean before and after the final runs.

The Phase 0 manifest separates actual compiled fixtures from `fixtureRejections`. Rejection rows carry BLOCKED_PLATFORM and NOT_RUN, never a native execution PASS. A continuation audit refuses a rejected target even if a synthetic control attempts to add a compiled row and callback. Unrelated rejected F04 rows do not block an independently valid executed target. Formal F04 acceptance registration remains incomplete.

This archive contains structural/compiler, diagnostic and publication-policy verification. Signing and Apple execution were not performed by these local producers.

## Retained development failures

The first 46d3853 gate had two invalid positive fixtures written as list([...]) instead of the existing variadic list(..., ...). Their E_ARGUMENT_TYPE failures occurred both before and after the guard; the exact logs remain under `retained-development/46d3853`. They were corrected in b031493 without a production change.

The b031493 retry never executed a test: both link steps failed with “no space left on device.” Those infrastructure failures and their zero executed-test counts remain under `retained-development/b031493`. A subsequent clean run after owner-coordinated cleanup produced the verified results above.

## Retention and transfer

All raw Linux evidence was retained. An attempted opaque ZIP response was rejected by automatic approval review. The completed transfer used the reviewed 22-member inventory and a fixed upload destination on the owner's already connected Windows host; no base64 payload or upload URL was printed. The received ZIP was exactly 52,543 bytes with SHA256 `050bbbb6c8674c6497c1cfca05e6dda653c15b932362ca3362740b47c61a69dd`.

The cleanup receipts record only explicitly authorized reproducible Cherri intermediate trees. The inactive candidate receipts and synthetic outputs were copied before deletion, and critical live entrypoint/configuration hashes remained identical. No original corpus, evidence files, shared cache, or live builder tree was deleted.

`archive-input-manifest.json` binds the original 21 data files. `archive-manifest.json` additionally binds this explanation and the exact-byte Git attributes. Each archived file is checked against its committed Git blob after the archive commit.
