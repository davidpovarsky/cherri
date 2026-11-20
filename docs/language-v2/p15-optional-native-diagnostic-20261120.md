# Optional native forwarding diagnostic — 20 November 2026

## Result and scope

Clean source `e84d02744eca729306898775992b545a305e6979` passed **76 named local Go tests**, with no failed or skipped named tests, in **4.625 seconds**. The working tree was clean before and after. This is source, native serialization and test-oracle evidence. **Apple execution remains NOT_RUN and no formal requirement is marked complete.**

The public fixture is `tools/shortcuts-simulator-qa/fixtures/qa-optional-forwarding-diagnostic-v2.cherri`. Its adjacent expected JSON fixes an eleven-field, 233-byte summary followed by a newline and the existing required NDJSON trace envelope. The reserved QA selector is `qa-optional-forwarding-diagnostic`; signer, callback and publication wiring are a separate Apple-agent task.

The fixture observes two nested calls for explicit None, empty Text, false, zero, empty flat List and empty Map. It also observes a populated flat List's count and first/last payload, a populated Map's key count/value/native type, an earlier Optional argument captured before a later argument writes its source, the source after that write, and an omitted default. None predicates read compiler-owned presence; value truthiness is never a presence oracle. The English native type label Dictionary is an explicit diagnostic expectation for the populated map. Actual contradictory outputs must remain preserved.

| Field | Exact expected value |
| --- | --- |
| NONE | NONE |
| EMPTY_TEXT | TEXT[] |
| FALSE | BOOL[false] |
| ZERO | NUMBER[0] |
| EMPTY_LIST | LIST[0] |
| EMPTY_MAP | MAP[0] |
| LIST | LIST[2\|A\|B] |
| MAP | MAP[1\|VALUE\|Dictionary] |
| SNAPSHOT | NONE |
| WRITTEN | TEXT[] |
| DEFAULT | TEXT[DEFAULT] |

The fixture is not evidence for general value joins, nested List transport, File transport, all Optional semantics or any full requirement.

## Independent graph and causal controls

The test authors the entire **399-action native graph** independently. Only generated UUIDs, control-group IDs and the private mutable-presence variable are taken as alpha names. Every operation, semantic parameter, typed Dictionary/Array row, presence predicate, default guard, authored order, reference edge and final consumer is compared. The six function bodies and all sixteen self-calls are included.

The existing trace wrapper is modeled explicitly: Nothing-based debug clear, before/after NDJSON Text templates, exact step counters and predecessor identifiers, CurrentDate UTF-16 ranges, append targets, Combine Text input/separator, collector run identity, summary field offsets, all three anchors and final Stop and Output. There is no opaque wrapper allowlist.

Eighteen target-contract mutations and thirty-three native graph corruptions must fail after unchanged source and clone positive controls pass. They cover source rebinding, None/false/zero/empty-container confusion, order/payload changes, earlier-argument snapshot, default presence, workflow identity, missing rows, source references, trace writes/run IDs/timestamps/counts, constant summary, offset drift and output bypass. Derivative summary hashes are reconciled in contract mutations so they cannot hide a changed semantic oracle.

The shared graph helper only gains an explicitly supplied expected workflow basename; its old default remains unchanged. The actual twenty-one existing complete forwarding graphs were replayed alongside the new fixture.

## Actual attempts

| Attempt | Source state | Actual result |
| --- | --- | --- |
| diagnostic-draft-v1 | Dirty draft on f16dd992 | One test failed in 15.578 s: the new oracle expected omitted action parameters on debugClearLogs' Nothing, while the existing native action correctly has a UUID-only parameter dictionary. This was a test expectation error, not a compiler failure. |
| diagnostic-draft-v2 | Dirty revised test on f16dd992 | 76 named tests passed in 13.969 s. The clear action now requires an actual UUID while comparing its exact empty semantic parameter map. |
| diagnostic-clean | Clean e84d0274 | 76 named tests passed in 4.625 s; zero failures/skips; clean before/after. |

The public source and expected output did not change between those attempts. The original first failed raw log, process receipt, concrete graph and exact test bytes are retained. A script copied the unchanged first graph to a v2-named scratch path after the green second run; that stale copy is not used or retained as fresh v2 evidence. No original evidence was rewritten.

## Reproduction and pins

```sh
GOMAXPROCS=2 go test -count=1 -json -timeout=3m   -run '^TestP15Optional(NativeDiagnostic|ForwardingCompleteNativeGraph)' .
```

The three new top-level tests have prefix `TestP15OptionalNativeDiagnostic`. The complete forwarding component has prefix `TestP15OptionalForwarding`; an integrated release replay must select both families explicitly.

- Fixture SHA-256: `e6f195aadcd2d46f25c6fe1de5165798c18a40f4c1942785b9e39171b1cc2139`.
- Expected JSON SHA-256: `da1335a22389cbe97e72a706192cb6b93ece8eff380080fa0541dcb5cbc50cef`.
- Exact 233-byte summary SHA-256: `86c2f4ae634e41a462d7b273f76513bd667a5f667f726f510885f54310585a1b`.
- Clean raw Go events SHA-256: `3efbbeaf6c44b02869e79a549f074c80f72f405fd7741eef9473079b7265cee7`.
- Clean stderr is empty; SHA-256 `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`.

Original receipts, logs, graph, source blobs and component diff are retained in `evidence/p15-optional-native-diagnostic-20261120/`; `manifest.json` binds every byte. This component changes no compiler production code, codec descriptor, immutable179 contract or native golden.
