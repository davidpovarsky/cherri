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
