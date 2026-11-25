# F04 general mutable storage candidate — 25 November 2026

Status: implementation checkpoint; the new implementation has **not yet passed GREEN qualification**.

This checkpoint follows the published structured-flow implementation at
`ee25605af06be180e2a268d49da31a446b3d7690`. It extends the same source-owned local
cell representation to inferred mutable declarations, mutable RHS reads, self
assignment, cyclic swaps, and directly assigned Repeat/For producers. Source
stamps retain the authored declaration, annotation (including an authored
absence), mutable flag, expression and producer-body identities. Lowering enters
the frame of the actual assignment expression; it does not manufacture a
declaration or use a reaching-origin set as a runtime value.

## Actual RED evidence

Before production changes, the new root test file ran on `ee25605`:

- 27 new source cases failed with `E_TARGET_VALUE_SHAPE`.
- The existing explicitly typed immutable-source alias control passed.
- No compilation failure, timeout or skipped test occurred.
- Runtime was 23.61 seconds with the original Go 180-second / outer 240-second bounds.
- All tracked Go inputs and the new test were unchanged during the run.
- The exact test source remains SHA256
  `7fbd3dd5dc303845fc09b287b75725fcfb9687b39b0ce84e444f40d61d4d1559`.

Original receipt:
`/mnt/development/projects/evidence/cherri-p10r15-general-language-completion-20261125/red-general-mutable-01/result.original.json`
SHA256 `79e38a600f5df64dfc6c7ebd712b784a7deb7d89c2d08a630ea4053a92255049`.

The new native graph oracle checks actual compiler serialization, whole-list
stores, mutable reads at their authored positions, immutable snapshots, direct
producer accumulator clearing, private cell presence and payloads, and control
order. It is not evidence of execution by Apple's Shortcuts runtime.

## Test preservation and intended new qualification

All 18 original schema source cases and full test identities remain present.
Five additional sources now have positive admission expectations:
`inferred_mutable_alias`, `direct_mutable_producer`, `mutable_rhs_initializer`,
`mutable_rhs_assignment`, and `nonidentifier_rhs` (its exact source is a direct
Repeat assignment). The two mutable-RHS sources contain two mutable declarations;
their expected declaration counts are now explicitly two. This is a feature
expectation change, not an unchanged pass of the historical rejection behavior.

All 16 original independent lower corruption controls remain unchanged. New
controls require inference and mutable-source admissions to continue rejecting
altered source identity, producer bodies, expression types and transport metadata
before any native store is emitted.

GREEN, the current original 330 identities, affected language packages and
affected root regression still need execution on this candidate. The storage
capacity admission remains in force; an exhausted shared filesystem is not a
passing test result.

## Remaining scope

This candidate does not yet admit ordinary-list/cell representation joins,
uninitialized cell variables, overwritten cell versions with no selected
consumer, compound cell mutation, arbitrary function/import/action interfaces,
or all nested/dynamic payload forms. Legacy regression, native runtime evidence,
full 179 acceptance and final release must be assessed separately on the final
integrated source. Frozen compiler S, implementation K, documentation D and the
installed K18 release are unchanged.
