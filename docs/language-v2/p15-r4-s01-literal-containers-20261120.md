# S01 literal container and Release integration — 2026-11-20

## Verified source and result

Implementation `917843ff18d67fe05c3bc89c97f92f1ae986b313` was merged with the complete R4 root `8ba17fcccade143d21adf21a28d8ad12fcfaf73a` at `2906245c34926e69384ec2c3d7923b0595421500`. The merge preserves the Dictionary-presence and ordinary/Optional function-default fixes, the complete packaging component, the exact T01/FN04 registrar, and the v2.2 function envelope descriptor.

A fresh clean-source run passed **2,077 named root tests**, with no failed or skipped events, in 202.109 seconds including a cold build. Original raw SHA-256: `d3626817f0e4cf9c0017127c6d5c6a7c01d2c306e84013188e692a02e8701383`. The selected root suite covers S01, typed defaults and functions, Optional transport, argument order, empty containers, indexing, Nil, build profiles and Release. It is a bounded component replay, not the whole repository CI.

The six syntax/analysis/lower/service/types/backend packages passed **173 named tests** on the same clean source in 7.156 seconds. Original raw SHA-256: `bb66103ec19131b947236bc5ea1577a44fa4699a0f37295f4c65484af2959d63`.

## Native observation and actual defect

The authentic Intel WorkflowKit observations from runs 44682741096 and 44693452074 show that an unbound literal U+FFFC in native text-template fields can be consumed during native serialization. Dictionary values and keys exhibit the loss. The newer observer passes original native objects directly between hops and retains typed false/zero/null/deep controls, avoiding the older JavaScript observation bridge's limitations.

The older observer's later converted numeric/deep snapshots are not promoted to typed fidelity. The newer JSON observer does not execute an HTTP request or establish item-type-5 null semantics. Neither observer executes this current compiler revision.

On the merged pre-fix compiler, the original Unicode tests retained five failing Map leaves. The first new complete-graph fixture used an unknown public dictionary API; that fixture error is retained. After correcting only the new fixture to the existing getDictionary API, ten complete-graph cases genuinely failed before the implementation.

## Implementation

The lowerer prepares explicitly typed semantic containers after their authored expressions have executed. Literal U+FFFC text in Dictionary/array fields and mixed text segments uses the existing ordinary Text producer and shared reference/text encoder. Computed keys use their actual segment references; literal keys retain their original source key and canonical literal-key sorting. Raw native envelopes and arbitrary raw maps remain opaque.

Private function and Optional envelopes apply the same preparation after supplied/default/return evaluation, preserving their existing empty-value and presence representation. A prepared argument is reused for its named slot and positional mirror.

The prior S01 input work is included in this integration: pure literal carriers are deferred past authored interpolation effects, native enclosing-input identities are preserved, and checked native declaration outputs remain required. This is a shared compiler/codec change; no action catalog fork or handwritten parallel registry is introduced.

Release diagnostics query the existing canonical removable-diagnostic policy before argument materialization. Authored expressions and necessary snapshots still execute; scalar reference helpers and literal carriers needed only by a removed consumer are omitted. Canonical validation and removed-output checks still run. Debug and semantic require consumers keep their needed carriers. No emitted action list is pruned after construction.

## Independent graph controls

Sixteen complete source graphs cover immutable/mutable Maps, literal/computed keys, nested arrays, List/Dictionary values, later implicit-input actions, registered calls, ordinary function arguments and Optional scalar/Map arguments and results, plus plain and exact raw-native controls. Every expected native action, parameter, reference edge, producer identity and UTF-16 attachment range is inspected. The 168 deliberate mutations reject altered literal bytes, producer kind/UUID/name, consumer UUID/name, missing/wrong ranges, unbound placeholders, reordered helpers, extra actions and duplicate identities.

The first Release draft incorrectly passed a Map to debugLog's Bool option and is retained as a fixture API error, never a product failure. Valid debugTrace, label, trace-event and semantic require controls then exposed real unnecessary helper operations. All seven removal/effect-order cases and both consumed-carrier cases pass after the repair. The complete targeted S01/profile/Release run passed 413 events before the clean integrated replay.

## Retained failed attempts and disk recovery

The first integrated root run had 2,048 passes and three failures. Each failed while linking the real CLI with an explicit no-space-on-disk error; no semantic assertion failed in those three tests. Its original raw hash is `929196cfee260dfb15cde6ca97488db255a75ff122f30ecc3f01fd1270571485`.

All local builds were paused. Transparent NTFS compression preserved the checked evidence bytes. The standard Go build cache was then cleared after checking for active build processes; source, downloaded module dependencies and original evidence were retained. Available space increased to 13.1 GB. The identical clean source and unchanged test command then passed all 2,077 events; the formerly blocked CLI tests also executed their nested cases. The original failed run remains unchanged.

The archive also retains earlier boundary failures, source patches, precommit source ZIPs, receipts and native originals. Files under container-precommit-v1-graphs and retained-pre-v2-dist-graphs are retained earlier failure snapshots; they are **not** claimed as fresh successful v1/v2 graphs. The original CRLF-sensitive merge check failure is retained alongside the scoped new-source check and byte-preserved proof documents.

## Provenance and remaining gates

The manifest hashes every archive member, all sixteen changed integration source files and unchanged requirement/oracle/action-provenance boundaries. All archive members were reopened and compared byte-for-byte. Historical source identities, native artifacts and failing logs were not rebound to this source.

This report grants no new formal requirement completion, no current-source iOS runtime credit, and no full-repository CI, final179, coherent installed consumer tuple or deployment acceptance. Those gates must execute separately against their actual final source.
