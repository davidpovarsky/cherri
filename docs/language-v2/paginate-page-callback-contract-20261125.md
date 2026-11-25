# Paginate immutable page callback: qualified compiler component

Base: 4f1bb436dec91a051eacf40f5987ad57634c2173. Original task: 1220544665232021; shared blocks: 1220544718901241.
The compiler component below is implemented and qualified against original source and graph evidence. Signed Apple execution, full pagination and release acceptance remain separate.

The optional trailing body is parsed by the existing scoped-block parser. Exactly one immutable page parameter has type Map<Text,AnyContent>. Existing ordinary paginate calls, source-order argument evaluation, headers, caps, modes, native Dictionary aggregation and result type stay intact.

A callback executes once after each successfully decoded page and its selected items pass their existing guards; it is outside the per-item loop and before next-marker processing. The precise decoded page is restored as Previous immediately before the body. A later authored producer inside the body owns Previous. The parameter is lexical, shadows only inside that body and has no escape outside it. Nested/same-name callbacks retain distinct native identities.

The body is Void: empty/statement/effect bodies are allowed, with one exact Nothing boundary at normal body completion. A yielded/returned/implicit final value is rejected rather than silently discarded. Native Stop/error still aborts normally; no finally/catch/cancellation guarantee is introduced. The macro itself still returns the ordered selected Dictionary list.

Captured mutable writes have a zero-or-many control-flow join, including cycle/cap/no-page paths. Limits of zero retain type checking and emit no callback/request/helpers. The original fixed-point/source-identity planner and shared lexical storage are reused.

RED selector: TestP15PaginatePageCallback(Graphs|ZeroBudgets|OrdinaryControl|TypedLocalFlow|TypedParameter).
Expected 67 named terminal identities: ordinary controls 5 PASS including their parent, 62 new capability FAIL including parents. There are 62 actual compile product records. The typed-parameter test uses the real parser/analyzer and retains its source in the test file. New negative-contract cases are authored before production but deliberately excluded from causal capability RED; they join GREEN after implementation.

The planned native graph oracle checks original HTTP→Dictionary identity, exact bounded control regions, full item-validation-before-body order, callback/parameter/Previous edges, exact body effects/Void boundaries, nested shadow recovery and captured storage. Further causal mutations and an independent model will qualify these rules after production; no Apple runtime claim follows from this plan.

Still outside this component: general typed item selectors/returned callback items, Link/response status/error recovery/redirects, generic retry, native acceptance and full final release.

## Actual capability RED and implementation checkpoint

The exact published test checkpoint `dd26f4536b84dd4cd4161c3a17b02481396ab09c` ran on Oracle/Linux arm64 with the existing Go 1.24.13 toolchain. The original 67 identities produced the expected 5 PASS / 62 FAIL, zero skips, no timeout/capacity abort and unchanged full physical source. All 62 compile product records were preserved. The causal intake distinguishes 34 V2 compile syntax failures at the unsupported `in` header, 24 Legacy compile failures at the unsupported opening brace, the analyzer-only header failure and their three parent failures. The four ordinary-pagination positive products remained actual native graphs. No fixture was replaced to obtain that RED.

- Original execution: `/mnt/development/projects/evidence/cherri-p10r15-paginate-callback-20261125/oracle-red-01/result.original.json`, SHA256 `03526b7f394dcad36b81a4102f520c8e4f1ff493cdce968106e073429e82558f`.
- Causal intake: `oracle-red-causal-intake-02/result.original.json`, SHA256 `2ede1da66bd8d85fb17470d74c10c3896ef6bf6328d668643cb60e4a685f6a5b` in the same evidence root.
- The first causal classifier attempt remains preserved: it incorrectly expected the diagnostic code in the parser error's plain `String()`; only that classifier expectation was corrected, with no Go rerun.

The implementation checkpoint adds actual lexical parameter identity, shared canonical pagination argument binding and helper emission, and the same validated-page/Void lifecycle in both frontends. It preserves the ordinary no-block builder, source-order header evaluation, original source-owned cell admission, zero-or-many captured writes and a narrow canonical numeric Delay contract. It grants no name-only authority to an arbitrary user action named `wait`.

**Qualification was pending at implementation checkpoint `06be06e0c4314737f9e3cea4a8748be1fa457929`.** The original 67 cases and all preauthored value-rejection sources remain unchanged. The first qualification additionally selects their 16 negative leaves, actual native-graph causal faults, exact source-identity corruption controls and helper/body error propagation boundaries. No Apple runtime or final release claim follows from this source implementation.


## Qualified source and root-cause repair

Production is pinned to `b20b334ec73e89b2860bd2cb4d87abfc8b490bc4`. The final tested component is `fa0d584e792bdf0bfb0de6658ab747261cc341f1`, tree `032fcccf3d71bd68cd0bda889cf9878b61354829`. The latter changes two Go test files and adds the exact qualified Python model; all compiler Go source is identical to b20.

The first GREEN attempt on 06be retained 139 PASS and 16 FAIL. Captured list assignments exposed a general inference problem: an empty yielded List had `Unknown` elements even when the same actual declaration had a later concrete producer. The repair uses equality components keyed by the original binding/RHS/AST identities. Every concrete producer and explicit annotation must agree exactly. Only authored empty List literals in enrolled result paths receive the resulting contextual element type. A component with no concrete evidence stays Unknown; absence, arbitrary unknown producers, conflicting concrete types and unrelated diagnostics are not erased.

A final analysis pass reuses the same authored AST and resets diagnostic, symbol, expression and registry state. Dedicated tests cover branches, two backedges, aliases, shadowing, annotations, stable repeated analysis and no leakage into another program. The lowerer retains its exact source and type guards. This is not a placeholder null or an assumed type.

Two old negative cases needed only the original actual diagnostic spelling (`E_UNKNOWN_NAME` and Legacy `Cannot redefine constant`). Their authored sources, and every other original RED source, are unchanged. The independent intake rehashes all 80 products and verifies all 62 RED inputs against the later execution.

## Actual compiler qualification

These are **three separate Go processes**, not one combined suite or full acceptance.

| Scope | Actual result | Source | Original receipt SHA256 |
| --- | --- | --- | --- |
| Callback, typed flow, source identity, graph faults and empty-source inference | 183 PASS, 0 FAIL/SKIP; 80 original product records | b20b334e | `1034977eece11ead3d9de661c972c85f56d5d2f7b53181776dae4e66d367cc61` |
| Complete affected analysis package | 43 source-declared parents and 22 children, all 65 PASS | b20b334e | `52999492e9002d89a158f6773a30b9531979e117c84513710639d4e572ab4d2b` |
| Original F04 identities and full existing parent closure, plus separate Delay graph faults | 330 original identities in unchanged 346-event closure, plus 13 new causal events: 359 PASS | fa0d584e | `2274caf572bec24e5cab3413c22d112f2b0fb125e97a34eb8b2dbd167d266490` |

The evidence root is `/mnt/development/projects/evidence/cherri-p10r15-paginate-callback-20261125`. The respective original receipts are `oracle-green-02/result.original.json`, `affected-analysis-01/result.original.json` and `affected-original330-03/result.original.json`. Each owner retained its original JSONL, stderr, full physical source manifests, environment, resource minima and selection. No timeout, skip, capacity abort or source drift occurred in these three passing processes.

The first affected F04 attempt did not execute Go because its admission floor failed. The next actual run retained 344 PASS and two failing events: the historical `unselected_action_after_cells` leaf and its parent. That exact source is still:

```cherri
let values=repeat 2 {yield []}
wait(0)
return count(values)
```

Canonical numeric Delay is an intentional source extension needed for scoped Void bodies. The original source and test ID now require the complete cell/list/count graph, with Delay exactly after collection materialization and before Count. Its descriptor identity and explicit Number input remain checked, and existing omitted-input/native-export/noncanonical-descriptor rejections remain intact. Twelve new graph corruptions form a separate family; no original330 inventory entry was deleted or replaced. The inventory file remains SHA256 `e13092e2a74d0a41d050d3191e7d1439156c2e27b11785e45b053529c3e2674c`.

## Declared lifecycle model

The checked-in `tools/p10r15/verify_paginate_callback_lifecycle.py` is byte-identical to the source that interpreted 44 actual compiler-emitted graphs. It checks 440 zero/one/many/empty/cycle/page-cap/item-cap/failed-page cases and rejects 11 causal graph mutations. Checks include exact request/callback/effect order, the actual page object identity, captured writes on failure paths, nested lexical callbacks, entry Previous, body-owned Previous, runtime headers and Release erasure.

This is an explicit bounded model: no Apple action, network request or compiler ran during the evaluation. The 404-with-valid-body case deliberately records that response status remains unverified. Delay, native content cardinality and Nothing have declared model semantics. The earlier two model attempts remain failed originals: the first assumed parameter maps were always present on Nothing/Exit; the second assumed canonical empty HTTP Dictionary always carried an Items array. Only those exact native wire shapes were admitted, with new corrupt-shape controls; no compiler graph was rewritten.

- Model receipt: `lifecycle-model-03/result.original.json`, SHA256 `1d60d993192b9c2621e084c647d6ff2b5b6e0fd3a51a6e986c1188b77418a45b`.
- Model report: `lifecycle-model-03/model.original.json`, SHA256 `9a3488f92b9d12c955cf737e17d8333ad3a101c582c15379961f284c209213db`.
- Model source: SHA256 `887d6c644157b3f69fc4dafc15b34df44670056bda78ee8c0c3644b0bd58c6cf`.
- Independent joined intake: `qualification-final-intake-01/result.original.json`, SHA256 `d3fb62a6d4346fe7c58d75524747052742afe21116b8e71fb9fea0cdb6492ee9`.

The joined intake re-parses original Go streams, checks every selected identity and package result, verifies all original330 identities and unchanged 346-event closure, rehashes every compiler product and model dependency, and checks all physical tracked files against Git. It grants component source/graph credit only.

## Remaining original obligations

The original task stays open. The supported page parameter is immutable and implicitly typed `Map<Text,AnyContent>`; this component does not implement general typed item-selector callbacks or returned-item accumulation. Link handling, same-request HTTP status/headers, redirects, retry, transport error/cancellation behavior, native execution and final consumer/release pins still require their own implementation or original observations. Original obligations remain recorded in `macro-original-completion-obligations-20261125.json`.
