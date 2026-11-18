# L02/L04: guarded dynamic list indexes

The V2 production lowerer now checks that a dynamic numeric index is an integer before it reaches the existing bounds checks and native Get Item from List action. This is a compiler and native serialization repair. Apple runtime values and no-item behavior remain pending a fresh signed run on the repaired source.

## Demonstrated defect and bounded repair

A compiler built from clean source `51e94503d8a36feb3504118c8d1653f8ecb2cf54` emitted this path for `var index = number(0.5)` followed by `items[index]`:

1. Add one to the original index.
2. Count the original list and subtract the one-based index.
3. Check that the one-based index is at least one and the difference is at least zero.
4. Pass the one-based result to Get Item from List.

For a three-item list, the fractional value 1.5 satisfies both existing bounds checks. The native graph therefore permitted a fractional lookup. The literal expression `items[0.5]` already emitted Nothing without a lookup.

The repair reuses the canonical Math action and numeric If encoding. It computes `index % 1` from the already-lowered index value and checks that the remainder equals zero. The complete preexisting conversion, lower bound, upper bound, and lookup are in the true branch. The false branch emits Nothing. The caller consumes the new outer End If result, so it receives the selected item or the no-item branch result.

Known `int64` and `uint64` values retain their existing path without an extra remainder action. Literal fractional handling, Dictionary indexing, native escape actions, imported raw data, numeric comparison coercion and registry definitions are unchanged by this patch.

The native action identifiers and parameter names come from existing compiler paths and the checked-in Apple metadata at `docs/apple-runtime/macos-27/builtin-actions.json`. That metadata lists the scientific Math operation Modulus and describes native list indexes as one-based. Metadata is not a substitute for executing the repaired shortcut on Apple platforms.

## Local production evidence

`p15_indexing_acceptance_test.go` reads the actual serialized production output. It follows producer UUIDs, exact collection values/cardinality, arithmetic inputs, conditional group nesting, Nothing branches and the final consumer reference. It does not run a replacement compiler or claim to execute Apple actions.

- `TestP15L04DynamicFractionalIndexGuard` is the causal regression. It fails on the original production graph because the lookup has only the two bounds guard ancestors.
- `TestP15L02IndexBoundaryWire` covers 18 cases: first/last, negative/out-of-range/empty, the largest unsigned integer without lossy conversion, dynamic integral/fractional/negative values, an inline action index, an empty Text item and literal fractions. Dynamic Number producers appear once.
- `TestP15L04IndexOracleRejectsCorruption` rejects 18 mutations affecting integrality, bounds, original inputs, control ancestry, fallback, result identity and collection values/cardinality/order.
- `TestP15L02L04IndexNativeRoundTrip` performs 32 actual filesystem imports and compilations: four graphs, XML and binary, hybrid and lossless, Legacy and V2. Each entire native tree must match exactly, then the guarded graph is checked again.
- The existing predicate compatibility test compares all three dynamic guard predicates with independently authored `== 0`, `>= 1` and `>= 0` conditions. Its previous two-guard expectation is preserved in the historical failure log and updated to require the new third guard.

The formal producer is `TestIndexProductionEvidence` in `tools/language-acceptance/index_execution_test.go`. It executes named production tests, binds each accepted tier to its actual producer, rejects cross-tier substitution and rejects a Go test relabeled as iOS evidence.

| Requirement | Local evidence | Remaining requirement |
| --- | --- | --- |
| L02 | Unit graph/value invariants and exact native round trips | Actual first/last/no-item values on iOS |
| L04 | Exact guarded native graph and import round trips | Actual absence of an unsafe lookup on iOS |
| RP21 | No promotion from these two cases | Its complete independent unit/native/iOS contract |

Both L02 and L04 remain `PENDING_EXTERNAL` in the local runner and `BLOCKED_EXTERNAL` in the execution audit. The separate manifest validator retains its existing `NOT_RUN` status for an incomplete external tier. Neither status is a release PASS.

## Fresh native validation still required

The repaired source must be frozen and built before a new signed indexing run. That run must retain fresh source/compiler/nonce/callback bindings and strict result checks, including dynamic fractional and negative indexes, first/last, an empty list and out-of-range values. A successful compile, an exact round trip, a locally evaluated formula or an unrelated previously passing Bool/List fixture cannot supply this runtime proof.

L01 standalone nested native List hierarchy and the separate nil-clearing diagnostic remain outside this repair.
