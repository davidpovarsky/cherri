# P15 computed Map key identity and evaluation order — 2026-11-20

## Verified result

Clean repaired source 34459b40b688b2772b1ce596e9eaf821492d0393 passes all 49 selected root test parents: 309 named passes, zero failures/skips, exit 0, 27.047 seconds. The complete internal/language/lower package then passed 15 named tests, zero failures/skips, exit 0, 4.781 seconds. HEAD and working tree stayed clean and unchanged through both runs.

This repair preserves explicit dynamic Text keys in semantic Maps and their source evaluation boundaries. It does not grant new formal acceptance IDs, claim Apple execution, or complete the project. The frozen R3 source remains separate.

## Clean causal baseline

Tests-only source 60aa680f445569718924488fc226905bdd4e0605 actually failed five cases: mutable-reference, action-result, and Unicode f-string keys; an earlier value before a later key's assignment; and an earlier key before its own value's assignment. The two literal controls passed. The clean run exited 1, with zero skips, in 22.953 seconds. Original raw SHA-256: 3cfe9432d7a5c30feb65b6e089f9b6f2a94a546f4cacdf0d4ad2b485446e4b86.

The native Dictionary key contained the literal Go representation of an attachment, such as &{Variable  key []}, or a Go map representation of an f-string. The parser and analyzer accepted those authored keys, but the lowerer stringified the intermediate value. Separately, sibling-write analysis included Map values only, so assignments in keys did not establish a capture boundary.

## Bounded production change

A DictEntry may now carry explicit KeySegments using the existing shared TextSegment/reference types. Literal and scalar key spellings retain their existing representation. Computed reference and f-string keys use the existing shared native text-token encoder, including UTF-16 attachment offsets; authored literal braces never enter Legacy interpolation.

The Map lowerer treats each source key followed by its value as one evaluation sequence. It captures an earlier mutable reference before a later key or value can assign to that binding. Nested Map key segments participate in the existing recursive semantic capture. Each key/value expression is lowered once. Pure, unrelated-binding and shadowed-binding controls keep the original reference without adding a snapshot.

Canonical literal-key ordering remains sorted. When a key is computed, rows preserve source order because a computed key may collide with another key. The semantic value adapter also retains typed Dictionary rows for computed keys nested in a canonical List argument; a Go map cannot represent such keys.

The existing explicit native.plist route is unchanged. The CodecFingerprint function and descriptor bytes are unchanged, verified against the clean baseline: the repair routes V2 keys into the already supported Legacy WFKey/text-token wire contract and shared codec. It introduces no new native serialization representation or action schema/default metadata. The behavior change is pinned by its compiler source SHA; no fingerprint or consumer pin was manually changed for internal IR plumbing.

## Executed independent checks

The new tests check exact key wrapper, reference identity, scalar value, row cardinality and order. A computed action key must identify its one real producer, including that producer's output name. Unicode interpolation requires the independent expected UTF-16 offset 3 after an emoji and colon. Eight corrupted native keys/fields and three mixed-row corruptions must be rejected.

Eight actual compiler routes preserve the explicit dynamic key: let, var, value-block yield, function argument transport, a native List of Dictionary producers, a Dictionary array, a canonical List argument, and a canonical named Dictionary argument. The HTTP-body specimen is compiled only; no request is executed.

The evaluation-order checks cover an earlier value before a later key write, a key before its own value write, an earlier key before a later key write, and computed keys nested in Maps or Dictionary arrays before a later outer write. They independently require the actual original binding, one direct Get Variable snapshot, exact initialization/capture/conditional/write ordering, two outer stores, and exactly one later write-effect producer.

Two computed-key specimens undergo 16 real filesystem import/recompile combinations across XML/binary, hybrid/lossless, and Legacy/V2. Every combination preserves the full native tree and reruns the independent Unicode or mixed-key-order oracle. The broader replay also executes the earlier 24 BRG09 round trips, all prior semantic Map/template/order controls, FN01–FN03 tests, T01 presence/default tests, C01 argument/default/snapshot tests, V01 declaration snapshots, and all ten existing codec-matrix parents.

The root raw SHA-256 is 3d598d78dc5aa6d2383e7e8cdf01ad2042479aefd0337b84826c86cbe9ae921b. The complete lower-package raw SHA-256 is 8a2d0e28a248b26d391e73d75761cd9d5c8a8c85382267a786f43d2a201912ce. Their unchanged finite Go budgets were five and four minutes respectively; neither run timed out. Exact commands, selected parent names and observed counts are retained in the original summaries.

## Retained iteration and evidence integrity

The initial repaired precommit run passed eight named checks in 40.547 seconds. Its dirty source state is explicitly retained and is not substituted for clean proof. The final tests use a dynamic-key-aware native row reader because a valid computed key is not a literal string, while retaining the strict snapshot/effect oracle. Action-result key expectations bind the name to the real producer rather than incorrectly requiring a missing name.

One launch wrapper stopped at its file allowlist before committing or executing Go: str.strip removed the first Git porcelain status-prefix space. Its original notification is retained as NO_TEST_EXECUTION. Correcting the wrapper to preserve leading status columns changed no source/test assertion or budget.

[evidence/p15-computed-map-keys-20261120](evidence/p15-computed-map-keys-20261120) retains all twelve original test output/stderr/summary files and the original saved launch notification. A separate derived JSONL file contains the exact forty emitted native-specimen JSON payloads, extracted across Go Output events; sixteen belong to computed keys and twenty-four to the existing BRG09 replay. All explicitly record runtime_observed: false. The manifest binds artifact sizes, source identities and SHA-256 values. Staged Git blob bytes were checked against every original file.

Read-only peer review found no concrete defect in the repaired key/value suffix-write ordering or conditional typed-row route. A key produced by native.action with lazy UUID resolution remains a separate useful causal control for the next container work. Whole L01/T05/FN05/RP20 scenarios, standalone nested native Lists, authored JSON null, all expression-order cases and Apple runtime behavior remain separate obligations. No immutable contract, native runtime fixture/golden or formal registry was changed by this repair.
