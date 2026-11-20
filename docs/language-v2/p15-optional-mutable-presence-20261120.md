# Mutable Optional value presence: verified compiler increment

Tested source: f922222006a5450246004de88e99387c22314544.

Mutable Optional values now carry an independent compiler-owned Text presence flag. Every supported plain assignment writes that flag and the actual value; an immutable alias snapshots both. None, empty Text, false, zero and forwarded empty List/Map parameter values retain separate presence semantics. This is source and serialized native graph proof, not an Apple runtime observation.

## Changes and semantic boundary

The lowerer adds one private native variable per tracked mutable binding. The authored RHS is evaluated first. The flag is written before the final ordinary value write, preserving the native Previous input of following authored actions. Assigning None retains the existing Nothing plus clear sequence. A mutable copy receives its own flag, and immutable aliases freeze a mutable flag before the corresponding data snapshot.

Compound assignment reads both operands. The analyzer now rejects an absent or possibly absent operand using its existing path-local read type. Known-present initializers and the checked branch of a comparison against none remain valid. The lowerer marks their resulting value present; no native truthiness observation supplies that fact.

The production delta is confined to three files: six analyzer lines, fifty Optional helper lines, and twenty-one lowerer lines (including replaced comments/conditions). It uses existing native Text, Set Variable and Get Variable forms and the existing v2.1 function envelope. No further envelope field, codec descriptor, original179 contract or existing signed fixture changes are made.

## Actual clean verification

The final source passed 851 named root events across 107 top-level tests in 22.625 seconds. The full types, analysis and lower packages passed 53 named tests in 5.453 seconds. Both commands exited zero with no fail, skip or timeout; the exact HEAD and clean Git state were unchanged before and after. Original finite budgets remain five minutes for the root Go command and three minutes for the package Go command.

This selected root run re-executes the previous complete 810-event regression selection plus the new mutable component. It is not a full repository, CI or device run. Exact complete argv, process receipts, raw Go events and stderr are retained below.

## Causal evidence

A clean tests-only baseline reproduced fourteen value, transition and snapshot failures. A second clean tests-only baseline found four source programs with absent or Optional compound operands accepted by analysis with zero diagnostics, plus three missing mutable/narrowed-presence graphs.

The repaired positive scenarios cover six parameter value classes, both directions between None and empty Text/false/zero, immutable value and predicate snapshots before a later write, a known-present compound update, and actual native control structure for a checked compound update with None or zero input. Separate controls require distinct flags for two mutable copies and preserve the exact predecessors of three authored implicit-input actions.

Ten snapshot mutations reject live-flag substitution, data-as-presence, invented presence, a stale write and a missing snapshot. A reconciled private-name collision must still reject shared flag identity. Three order mutations move metadata before the RHS, after the final value write, or remove the explicit Nothing clear. Each corruption first validates an unchanged typed native clone.

## Retained execution history

| Attempt | Source | Go exit | Named passes | Seconds | Raw SHA-256 |
| --- | --- | ---: | ---: | ---: | --- |
| Clean mutable value/snapshot causal baseline | 7253ac48c8dd | 1 | 0 | 18.500 | 53a877254faa9e50e88644a7c2820653de4cd4628cb6efbd3ed47042f198c44c |
| Clean compound operand/narrowing causal baseline | 1a5d0db59959 | 1 | 0 | 32.781 | bcc949e4e8ec88a58884046285c3d858403c699b6d97bf6526aca1f36171e112 |
| Dirty focused repair | 1a5d0db59959 + dirty | 0 | 36 | 36.031 | fc5e0973551787281c849c6238f430d609d33aa08ec0026592c9b673ef4aadca |
| Dirty copy/implicit-input controls | 1a5d0db59959 + dirty | 0 | 41 | 17.516 | ff9c7569b33372a604fb931fd2041864104088afbe9918c904755cd81b3ea48a |
| Clean complete root replay | f922222006a5 | 0 | 851 | 22.625 | d99e926f31ff68b97086067b385f8aed9a21c28cc404214b4d94d14e2801e678 |
| Clean full types, analysis, lower packages | f922222006a5 | 0 | 53 | 5.453 | 933aebcf7fa7c040d7e9612ef1bc65b116b25f45f104f68cc7e7a084afc5d926 |

## Remaining scope

General Optional argument forwarding, Optional type aliases, value-expression joins, nested container absence and actual Apple payload fidelity remain separate open requirements. The four unsupported compound programs are invalid absent-operand reads; their rejection is not an intrinsic native container limitation. This component gives no automatic T05/L01/FN05/RP20 or linked acceptance credit and adds no formal179 producer.

## Provenance

All original run bytes and immutable source blobs are retained under evidence/p15-optional-mutable-presence-20261120. Every copied file is listed with its exact byte count and SHA-256 in manifest.json; that manifest's SHA-256 is 55a82e7e123f1792281a7bed6388ceb84f80d186a477209ae0aa28437f3563d9. Source copies come from immutable Git blobs, and original artifacts are not normalized or relabeled.

Cherry-pick sequence after fa2ba0ddd4141036ded29327693730501de24296:
- 7253ac48c8dd5c857b6d83cfc6040a32d56f71d1: tests-only mutable value/snapshot baseline.
- 1a5d0db59959a25bddf22c1532cb119264fe1cc7: tests-only absent compound and valid narrowing baseline.
- f922222006a5450246004de88e99387c22314544: paired mutable presence, immutable snapshot and checked compound implementation.
