# F04 declaration storage and cell append — 25 November 2026

Status: focused GREEN passed. The original330 successor regression and final integration are still pending for this source.

A mutable cell-sequence declaration now owns one storage representation for its
whole lifetime. The planner collects every initialized and assigned producer
origin by the actual declaration identity. Those storage origins are separate
from consumer demand, so an overwritten or never-read version no longer
requires an invented Count/index/for use. The existing explicit-input and
escape checks remain in force.

An authored `+=` accepts the same exact source-owned cell sequence type and
uses the existing native AppendVariable action. Its reaching origins join the
target state after evaluating the RHS with the RHS origins, matching the actual
lowering order when the RHS itself writes the target. The source stamp retains
the authored operator. Changing an assignment to append, or append to assignment,
after planning still rejects before any native RHS/store is emitted.

## Actual evidence

- Before production edits, 46 new source cases failed with `E_TARGET_VALUE_SHAPE`; one existing control passed. There were no build/parser failures, timeouts or skips.
- After the two schema production edits, 214 named tests across three packages passed with zero failures or skips.
- All 47 original new graph-source cases remain byte-identical to RED: SHA256 `c14cdafd1f2ae614739e8d9514d3ba86ddb154ac86cd3b898f5a79bf76afd61c`.
- Coverage includes typed/inferred declarations, completely unread mutable chains, overwritten versions, branch/zero/repeated iterations, immutable and mutable RHS append, self-append, direct producer append, lexical shadowing, Optional false/zero/empty text, and RHS target writes.
- Seven independent graph corruptions are rejected. Four additional operator/source identity corruptions reject before lowering; a healthy append emits exactly one native AppendVariable with the original target and source.
- The earlier 16 independent lower controls and 25 phase2 lower controls remain unchanged and pass.

All original schema test sources and IDs remain. Four more original cases now
require positive enrollment: `compound_assignment`,
`unread_overwritten_initializer`, `unread_assigned_version`, and
`other_origin_demand_is_insufficient`. There are now 16 positive schema cases
and two still-closed representation cases.

The original graph oracle requires Nothing when clearing each private
accumulator before its producer, and checks explicit main output. It does not
claim that Nothing is emitted after every loop iteration. Private native Repeat
Results remain unconsumed. The completely unread source fixtures explicitly
return 7; every preceding producer and store is still checked in the graph.

See [verification.json](evidence/p15-f04-declaration-storage-20261125/verification.json)
for original receipts and input binding. These are actual compiler/native graph
checks, not execution of Apple's Shortcuts runtime or formal179 acceptance.

## Remaining scope

Ordinary/empty/uninitialized list transitions need the shared container
representation being implemented in the T05/L01 lane. This change does not add
a parallel codec or change cell markers, payload wire layout, function/import
interfaces, or all nested/dynamic payloads. Final CI, original330, broader
integrated regression and native acceptance remain separate gates. Frozen
S/K/D and the installed K18 release are unchanged.
