# P15 typed function defaults and caller input

## Verified component

The bounded implementation is `6b049e6959a82ca8fae9bef7d961d132c57cdabd`, based on `774662ef064e0eb1fdb673659370288796a3bc7d`.
Actual clean replay passed **1,490 named root events** in **40.906 seconds** and **147 named events from six complete packages** in **8.562 seconds**.
Both commands exited zero, with no failed/skipped named events or timeout, and source remained clean at the exact implementation before and after.
These counts include parent tests and subtests; they are not counts of requirements.

The production change is one new Map-default helper and two narrow function-default call sites. The component also contains independent complete native graphs, semantic diagnostics and causal graph controls for typed defaults and preserved caller input.
It does not execute Apple, close formal T01/FN04 or another requirement, update the 179-row contracts, change a codec descriptor, or claim general Optional-join support.
The separate Dictionary lookup repair and its later root integration retain their own source/proof; this component is based on the handed-off function-boundary branch.

## Original failures and test corrections

[All original evidence](evidence/p15-function-defaults-caller-input-20261120/) is preserved byte for byte.
The first two baselines were run and retained by root before this worktree was handed off. The later tests-only and repaired runs use the same owned branch.

| Attempt | Source | Observed result and meaning |
| --- | --- | --- |
| Initial new fixtures | `595a86baf5a04499f8e35af892c9d0246c5ccc85` | 128 pass / 17 fail events, exit 1, 22.238 s. Missing whitespace joined the generic closing bracket and assignment into a maximal `>=` token. These new-fixture syntax errors are not claimed as production failures. |
| Syntax-corrected baseline | `4a2dfabf34c61c3af44f1cb1937a843166a3bea8` | 173 pass / 8 fail events, exit 1, 26.984 s. Two real Map default failures and one new empty-List oracle mismatch occurred in both positive and corruption-suite setup. FN04 complete caller-input checks passed. |
| Precise empty-List oracle plus Optional Map cases | `e830670044173d1854481a6a9c623b2aa17b1f7d` | 223 pass / 26 fail events, exit 1, 19.093 s. Empty-List graph and its three new corruption controls pass. Two ordinary Map cases and nine Optional Map source cases still fail; each corruption suite correctly refuses to start from a bad positive graph. |
| Repaired exploratory selection | Dirty `e8306700` plus exact retained source ZIP | 1,102 pass events, exit 0, 46.391 s. Actual precommit observation, not clean-source proof. |
| Additional effect/scope control | Dirty `e8306700` with retained test source | 19 pass events, exit 0, 24.718 s. Two ordinary/Optional complete graphs plus 16 corruptions verify the same nested calls and previous parameter reference. |
| Clean implementation | `6b049e6959a82ca8fae9bef7d961d132c57cdabd` | 1,490 root + 147 complete-package pass events; both exit 0. Authoritative local proof for this component. |

The empty-list default was already emitted through the existing Nothing → private Set Variable without input → Get Variable bridge.
The new test had incorrectly required an empty native List action. Existing `empty_list.go` documents the prior native differential reason for the clearing route, and `p15_empty_list_test.go` independently checks its exact source/value boundary.
No empty-list production code changed. The corrected new oracle requires the complete clear sequence, exact consumed private variable and producer identity, branch position and final consumer.
New mutations reject missing Nothing, inherited stale input and detached variable reads. Existing presence/default/return mutations remain active and target the actual last default producer rather than assuming a one-action fallback.

## Actual Map defects and repair

The clean original decoded graphs show:

- An ordinary Map default containing WFSerializationType and Value was emitted as a Text action.
- Nested empty List/Dictionary defaults omitted their explicit native empty fields.
- The corresponding Optional carrier reinterpreted ordinary reserved keys as a Text field.
- In an Optional Map default, literal `"{ordinary}"` became a native variable attachment instead of literal text.

Optional cases run each authored Map with an omitted argument, explicit None and an explicitly supplied empty Map. The bad fallback graph is retained even when a particular supplied argument would skip that branch; these are source/control cases, not separate runtime observations.

`lowerFunctionDefaultValue` preserves the original Map AST before serialization.
For an ordinary parameter it uses existing `lowerRuntimeLiteral`, producing the typed Dictionary as the branch result.
For an Optional parameter it uses existing `lowerSemanticValue`, preserving the typed Map inline within the existing private carrier.
It evaluates the Map once, restores the expression-context flag on every return, and stays inside the existing omitted-argument branch.
All non-Map defaults keep `lowerCallValue`. Empty Lists, None, raw native values and existing argument transport keep their prior paths.
No serializer, raw-template codec, catalog, public protocol or codec fingerprint changed.

The two production hooks remain separate from root's literal U+FFFC carrier work.
The helper reuses the common semantic Map path for key/value ordering and sibling capture. It adds no independent encoding.

## Executed behavioral and structural coverage

Nine typed default scenarios cover false, zero, empty Text, supplied empty List/Map, empty List/Map fallbacks, reserved Map keys and nested empty Map/List values.
Every complete graph contains explicit positional, named and omitted calls, separate has_arg decisions, exact default branch, payload type/value, return edge and caller continuation.
Five wrong typed defaults retain exact diagnostic code/span/explanation and zero emitted output.
The graph suite retains its 12 corruption families per default, plus three dedicated empty-clear controls.

Twelve Optional Map scenarios cover four Maps (empty, reserved keys, nested empty/scalar values and literal braces) across omitted, None and explicit-empty modes.
Each independent graph checks supplied-argument presence separately from value presence, exact typed default payload, decode/return carriers and final caller consumer.
Nine corruption families per scenario reject absent/default flag substitution, type/value/null corruption, truthiness guards, wrong slot/join, wrong supplied flag and unrelated return.

Two complete default-effect graphs, one ordinary and one Optional, include a previous parameter reference plus two authored text calls.
Both calls appear exactly once, in source order, only inside the omitted-value branch.
Sixteen corruptions reject reordered, duplicated, eager or lost calls; lost previous-parameter reference; changed supplied/omitted decisions; and always returning the default.

FN04 has complete direct and nested function graphs for both system.input and ShortcutInput, plus a no-function control.
Every authored read and nested payload must use the preserved caller reference. Only the guarded function dispatcher reads the transport ExtensionInput.
Twenty-six causal mutations reject snapshot loss, unguarded/wrong envelopes, truthiness marker checks, restoring arguments or the transport envelope, substituted body/caller/nested references, wrong self-call input and detached consumers.
No production FN04 change was needed in this component.

The final selection also executes existing FN01–FN03, function/Optional transport, C01 evaluation order/default effects, empty-list, inferred List guard, computed-key, semantic-container, explicit-native, None-yield, V02 and original Nil/Legacy checks.
Full syntax, analysis, lower, service, types and backend package suites run afterward.
Parity independently reviewed the three production files read-only and found no concrete value/effect/order defect; that review is not counted as execution or native proof.

## Exact clean commands

```text
go test -count=1 -json -timeout=5m -run ^TestP15(T01|FN0[1-5]|Function|Optional|C01|EmptyList|L01|Computed|SemanticContainer|ExplicitNativeContainer|F03NoneYield|V02|Nil) .
go test -count=1 -json -timeout=3m ./internal/language/syntax ./internal/language/analysis ./internal/language/lower ./internal/language/service ./internal/language/types ./internal/language/backend
```

The regular expression is a single recorded command argument.
Environment: Go 1.25.6, Windows/amd64, CGO_ENABLED=0, GOMAXPROCS=2.
Receipts retain exact commands, source hashes, timestamps, finite limits, exit/timeout status, complete named-event lists and output/graph hashes.

| Evidence | SHA-256 |
| --- | --- |
| Original syntax-corrected root baseline | `5c222b28462293c9f8411fde223fed5bccb0613015c050b0a6b1ba797a416ede` |
| Precise new baseline | `90113e3bbc08f07f77af9a518775a6d6556e114cd7a8c4d2806268cb3c05e7de` |
| Clean root stream | `634e79e38a913e6d201b2031268593ab11060d7d36710f0ce366444345b91524` |
| Clean package stream | `b23e3c0819ccef7ee390d9f43c4ca3d29790aa33cddddc32400f35fc85eea06f` |

The manifest hashes the original root and component receipts/graphs, exact exploratory source, clean proof, complete component patch and immutable source binding.
Source binding includes the seven changed files and unchanged shared empty/default/argument routes, original fixtures, contracts and codec descriptor.
The evidence metadata explicitly leaves formal_requirements_proved empty and native_runtime_executed false.

## Integration sequence and remaining work

After `774662ef064e0eb1fdb673659370288796a3bc7d`:

1. `595a86baf5a04499f8e35af892c9d0246c5ccc85` — root's new T01/FN04 graph and typing tests.
2. `4a2dfabf34c61c3af44f1cb1937a843166a3bea8` — generic/default token separation in new fixtures.
3. `e830670044173d1854481a6a9c623b2aa17b1f7d` — exact existing empty-list route and additional Optional Map baselines.
4. `6b049e6959a82ca8fae9bef7d961d132c57cdabd` — narrow Map-default repair and effect/scope checks.
5. The documentation-only proof commit containing this report.

Formal producers for T01 and FN04 are a subsequent task. Their immutable native requirements cannot be met by these local structural observations alone.
General Optional control joins, arbitrary action-result presence and native container/cardinality/U+FFFC boundaries remain separately tracked.
