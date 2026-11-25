# Paginate immutable page callback: source qualification plan

Base: 4f1bb436dec91a051eacf40f5987ad57634c2173. Original task: 1220544665232021; shared blocks: 1220544718901241.
This is a proposed implementation contract, not evidence of support.

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

**Qualification is pending at this source checkpoint.** The original 67 cases and all preauthored value-rejection sources remain unchanged. The first qualification additionally selects their 16 negative leaves, actual native-graph causal faults, exact source-identity corruption controls and helper/body error propagation boundaries. No Apple runtime or final release claim follows from this source implementation.
