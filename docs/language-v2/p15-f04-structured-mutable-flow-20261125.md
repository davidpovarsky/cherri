# F04: mutable loop-result values across structured flow

Date: 2026-11-25. Base: `1e02755bbfaf470b4dba1802aa410f29ab4d1188`.
Owner: built-in subagent `/root/docs_remote_consumers` (Noam development/QA lane).
Branch: `agent/cherri-p10r15-general-language-completion-20261125`.

## Implementation

The local cell admission analysis now tracks a set of possible source producers for each mutable declaration. Values are keyed by the actual declaration AST, while lexical names resolve that identity. An immutable alias retains its source-point snapshot. Writing an outer variable and then declaring a local variable with the same name no longer loses the outer write.

If/Else and Menu join their continuing source paths. Repeat and For calculate a finite fixed point over entry and backedge origins, retaining the zero-iteration entry. The For iterable is consumed before the body analysis, matching its evaluation position. Repeated body snapshots are recomputed from the origins that can reach that position; dependencies that need several loop iterations are included.

The native lowering still uses the actual variable token and existing private cell representation. Origin sets never replace a runtime value. A distinct backend name for a shadowed declaration now preserves the original AST for cell admission and Optional demand, instead of asking a same-text cloned declaration to inherit permissions.

This implementation admits explicit exact-typed mutable aliases initialized or wholly assigned from immutable source-owned cell sequences inside lexical blocks, If/Else, Menu, Repeat and For, including immutable producers declared within a loop. It adds no native action, wire key or alternate catalog.

## Actual focused qualification

The first causal baseline ran the new tests against unchanged K implementation. All 16 planner cases and the initial 13 production native-graph cases failed. Production compilation failed with `E_TARGET_VALUE_SHAPE` for List/Optional loop-result transport; these were not build or parser failures.

The first implementation qualification passed **95 named test events**: **87 leaf tests and 8 parents**, across three packages. Package pass events are separate. There were no failures, skips or timeout. The selected obligations were:

| Layer | Actual leaf tests | Scope |
| --- | ---: | --- |
| Production native graph | 16 | Every emitted operation, control boundary, value edge and relevant private identity |
| Corrupted native graph | 6 | Stale RHS, moved snapshot, wrong loop count, wrong alias variable, wrong backedge target, stale final read |
| Production semantic rejection | 8 | Widened targets and changed element types inside structured flow |
| Schema flow | 40 | Existing origins/rejections, new joins/fixed points, and seven preserved formerly rejected sources |
| Independent lowering | 17 | Sixteen existing enrollment/type/representation mutations plus ordinary-assignment control |

The native graphs cover If/Else, Menu, Repeat, For, a producer declared inside a loop, zero iterations, immutable snapshots, two backedge dependencies, writes before scalar or mutable shadowing, present false, and typed absence. The oracle constructs expected operations and edges independently. It learns only required producer UUIDs, control groups and private alpha names from their required positions.

Seven original schema source cases intentionally change their expected behavior: `if_write`, `else_write`, `menu_write`, `repeat_write`, `for_write`, `block_write`, and `nested_mutable_binding`. Their authored sources and AST identity assertions are retained as positive obligations. The first GREEN used a separate positive parent; the follow-up will restore their original full test IDs with explicit expected-admission values before the original 330 inventory is rerun. The former rejection semantics are not described as passing unchanged.

## Originals and attribution

Evidence root:

`/mnt/development/projects/evidence/cherri-p10r15-general-language-completion-20261125/`

| Original receipt | SHA-256 |
| --- | --- |
| `red-control-flow-01/result.original.json` | `ec72762a6fd13b9b545fb2a31a3975d44268a1ec9865e82530a9084af9ae4e79` |
| `green-control-flow-01/result.original.json` | `fe3ef0bca28bdc38d7fdda09892e370727199546498fd663825ead2d114db9ad` |

The RED and GREEN owners ran for 40.82 and 43.10 seconds under their original 180-second command bounds. GREEN admitted only above 1 GiB free on both filesystems. Original capacity samples recorded minima of 1,494,466,560 development bytes and 1,456,340,992 root bytes. Source files were hashed before and after each actual process, with original JSONL, stderr and source snapshots retained. [Machine-readable attribution](evidence/p15-f04-structured-mutable-flow-20261125/verification.json) binds those precommit bytes; no later commit is retroactively stamped onto an earlier run.

The root agent and CI/release agent independently reviewed the source. Their review found no blocking issue in the finite monotone fixed point, zero-iteration path, source identities, snapshots or lexical write propagation. Review is separate from execution.

## Boundaries and next qualification

This is V2 source analysis and actual native plist structure, not Apple execution. The Legacy parser, import/export wire preservation, Pure/Pack boundary, source contracts and catalog remain unchanged, but their affected regressions must run on the new component. General F04, T05/L01 runtime transport and full 179 acceptance remain open.

Direct mutable producers, inferred mutable cell types, mutable RHS identifiers, uninitialized cell variables, compound mutation, ordinary-list/cell representation mixing, arbitrary function/import interfaces and unsupported dynamic payload shapes are still rejected. The next implementation assessment will address the remaining safe source-owned cases after the affected regression.

The original 330 identities, affected Optional/lexical tests, broader integration, original source-bound coverage production and new Apple execution remain separate next gates. Frozen S627c, K1e027, D16 and the installed K18 component have not been changed by this isolated branch.

## Follow-up: the original 330 identities passed

The seven positive flow cases now retain their original full IDs under `TestLoopResultMutableAliasPlanKeepsClosedBoundaries`, with an explicit expected-admission boolean. All eighteen original authored sources remain: seven positive flow obligations and eleven unchanged rejection obligations. The first GREEN's separate parent and exact source snapshots remain historical originals.

The original two selectors and their original 180-second Go / 240-second outer limits were rerun. All **260 root and 70 internal named package/test identities matched the hash-verified original JSONL exactly**, with no failures or skips. The two actual processes took 12.02 and 5.01 seconds. The source was published checkpoint `74d15da729e833d64e4df08305cf3c4da877ecff` plus only the recorded schema-test identity restoration; every tracked Go source was hashed before and after, unchanged. This is current implementation verification with the seven declared expectation changes, not a claim that their former rejection semantics passed unchanged.

Original: `/mnt/development/projects/evidence/cherri-p10r15-general-language-completion-20261125/regression-original330-01/result.original.json`, SHA-256 `a1c0b6e836d35e964467d6e21ee89ed26fd466358c93b377143f767cf058516e`. The previously listed broader affected regressions and additional semantic implementation remain next work.
