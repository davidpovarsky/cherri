# Legacy condition producers and short-circuit qualification — 2026-11-25

## Qualified source and result

Production commit: `4c681b69cf96cd3075f9efab5c9fcbb45278dd56`
Production tree: `83803ad8dccd2034e9130816c6cee8b727c30fbf`
Branch: `agent/cherri-p10r15-legacy-compound-conditions-20261125`

One actual Oracle Go execution passed **325 named test events, zero failed and zero skipped**, with no timeout, capacity stop, or source change. The execution finished at 2026-11-25T11:07:08Z and took 42.94 seconds. Parent test events are included in the named count. The result contains 208 existing conditional/inline/control regression identities and 117 compound-condition and defense identities.

This is compiler and emitted native-graph evidence. It does not assert that these shortcuts ran on Apple Shortcuts, and it does not close the full 179-requirement acceptance run.

## General implementation

Legacy condition inputs and operands are materialized through the shared typed action pipeline. Earlier mutable input values are captured before later operand producers, and each producer is evaluated once.

Compound conditions now preserve all operands on iOS targets older than 18. Conditions with later effectful or dynamic producers also use guarded canonical control actions on newer targets. A private integer carrier starts inside the authored control scope; AND and OR guard each subsequent predicate using the current carrier value. This preserves left-to-right evaluation and short-circuit behavior for actions, expressions, ranges, loops, nested conditions, and macro-produced effects.

Pure conditions on supported modern targets retain their native multi-condition representation. Existing Legacy grammar restrictions on mixed AND/OR remain explicit. The serializer rejects any unlowered old-target compound with `E_CONDITION_TARGET` before emitting an incomplete condition. No raw-import normalization or alternate native codec was introduced.

## Causal checks and retained failures

The first unchanged causal test cohort contains 56 written-source leaves across Debug/Release and iOS 17/27. Its original RED execution failed all 56 intended leaves, with 18 named independent controls passing.

A separate unchanged cohort added 18 later-right-operand leaves across iOS 17/18/27. Its RED execution failed those 18 intended leaves; 22 other named pure-modern and grammar-boundary events passed.

Both original RED reports remain immutable. The final green execution includes the same 74 written-source leaves, the existing regressions, independent ordinary-single-condition graph references, causal corruptions, and the serializer defense. Comparison permits only validated graph identity and private-variable renaming; literal values, action ordering, control groups, producer edges, and execution placement remain exact.

Earlier partial failures are also retained. In particular, the previous operand run's two Debug snapshot mismatches were resolved in a separate test-only correction that recognizes the unique Current Date timer write/read pair and rejects six malformed alternatives. That correction does not normalize arbitrary names or authored text.

## Original evidence

All paths below are under:

`/mnt/development/projects/evidence/cherri-p10r15-final-completion-20261125/`

| Evidence | Original path | SHA-256 |
|---|---|---|
| First causal RED | `legacy-compound-causal-red-01/result.original.json` | `195f7fb43dcb26308ca0ab465cb768cddb0c84c928eccc46352ab3571bc2779a` |
| Right-operand causal RED | `legacy-compound-rhs-red-01/result.original.json` | `75275f3bc44078bf9f66b9afe50e5b21148279fad5cb47cd309c2839e943d4ef` |
| Production publication | `legacy-compound-production-checkpoint-01/result.original.json` | `e2ae9b4d0b72b6ac96722b508165202bfe67af1116a49ec0279d4b466b8e9919` |
| Comprehensive actual GREEN | `legacy-compound-comprehensive-green-01/result.original.json` | `7b4f5c46c6340efe25aa85e239966536966120215be78437f17b1c6c47dc7b68` |
| Original Go event stream | `legacy-compound-comprehensive-green-01/go-test.original.jsonl` | `9137b0285669c22caa2a76e412f0e519253a61c95c8cc5aaf3941465f2837b04` |
| Original execution owner | `legacy-compound-comprehensive-green-01/owner.original.py` | `7dcb170788c6a1f11b130bed38eb4acaa4d989fcdd013dde00680ef9a96ef52e` |

The green receipt records the exact command, toolchain, source inventories before and after, event identities, original stream hashes, and minimum free development/root/RAM-cache/memory capacity. Historical evidence files in this linked worktree are partially hydrated; the owner explicitly inventories its physical compilation inputs and makes no complete-physical-checkout claim.

## Remaining acceptance work

Merge the qualified change with the other independently qualified branches, refresh the final compiler source reports, run the combined regression suite, and collect fresh signed Apple runtime proof on that source. In particular, native numeric coercion and `require(0)` counterproofs are independent requirements; passing these compiler graph tests does not retire them.
