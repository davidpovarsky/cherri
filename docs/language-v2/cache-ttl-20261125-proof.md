# Scalar cache/TTL and authored Previous — 25 November 2026

## Qualified source and scope

The compiler implementation qualified here is **319160836f8f0b69dfd8629e7afcc861172a7dbc**, tree **b0e9ffb142c4b6aed017669551999bf7c5c905c6**, on `agent/cherri-p10r15-cache-ttl-20261125`.

This component implements the existing planned `cache` scoped macro in Legacy and V2 for present **Text, Number and Bool** values, using native iOS/iPadOS 27 Stored Content actions. It preserves the authored body and lexical bindings. A cache hit avoids body execution; a miss executes the body once. TTL zero follows the body directly without storage, clocks or input-snapshot helpers. Positive TTL uses a complete, versioned seven-field scalar archive, a post-body creation clock, inclusive expiry, and explicit namespace/type/TTL validation. Noncanonical Base64 or a changed UTF-8 round trip cannot silently earn a Text cache hit.

The final source change captures native implicit Previous before all cache helpers and restores that private slot only inside the miss branch immediately before the authored body. An authored action inside the body can still provide its own subsequent Previous. Nested invocations use distinct slots. Capture and restore emission failures prevent body emission and preserve the original error.

This is bounded compiler-source and native-graph qualification. It does not grant Apple runtime, physical-device or full P10R15 acceptance.

## Actual verification

| Producer | Actual outcome | Original receipt SHA-256 |
|---|---|---|
| Corrected Legacy Previous RED, unchanged production | 19 named tests: 12 positive controls passed; six intended semantic failures plus their parent failed | `e5e0f724c9116590d81d5b9d483c2cddf3663c5951fcefa1e033f7e2c7bb30f1` |
| Final two-package compiler qualification | 227/227 named tests passed in 47.23 seconds; no skip, timeout, capacity abort or source change | `d11599ca0d9d050f98c8743514bee5882747b91bc58e3e47ff91248522e1defb` |
| Independent model on the new emitted graphs | 44 actual graphs; 1,496 scenarios and 30 rejected causal mutations | `4996838886dbc9818e08c7b098d751bd32d2c6e4cd23bf7e1cc79650a0a53a95` |
| Independent source review of the exact backend delta | No blocker in the reviewed delta; no Go or Apple execution credited by review | `0c2face2ed8dad28c887ee24fdbb2e2e427b0da8f740d0fff48b5481d4976af6` |
| Receipt, event, source-pair and model intake | Passed original-stream and input binding checks | `2202656602c8df2844f7726c6caad066815ceb77469b2daed4a9ef6c6c2508ac` |

The single final Go run includes:

- 40 paired Previous scenarios: Date input, workflow-entry input, implicit debug logging, body-owned Previous and nested cache; both dialects, Debug/Release and TTL zero/positive.
- 16 baseline graph corruption controls and 24 actual snapshot graph corruption controls.
- Backend snapshot-boundary and error-propagation controls.
- All 120 previously qualified cache test identities, plus nine captured-write/shadowing identities.

The intake checks all 80 baseline/cache source pairs against their original RED artifacts. The only fixture setup correction was the existing calendar include required by Legacy Date calls. It also binds all 44 scalar/captured-write source records to the original fixtures and checks every model input and mutant digest. The independent interpreter changed only Set Variable's omitted-input rule: it consumes the preceding value when `WFInput` is absent, without coercing that value to Text.

The earlier explicit-input qualification remains preserved separately: 556 joined Go identities and its original model evidence are historical component results, not a claim that 556 tests were rerun on the final Previous change.

## Preserved failures and setup attempts

The original cache RED, the initial invalid native conditional wire graphs, the original Previous RED including Legacy calendar setup errors, and all failed/blocked attempts remain intact. The first corrected Legacy replay stopped before Go at the capacity admission gate; the next was terminated by the original live capacity floor after two positive controls. The subsequent unchanged-source replay produced the complete intended RED.

The first final GREEN launcher had a Python owner quoting error before Go. Its submitted source and the original Sentinel result are preserved separately. The corrected owner was syntax-checked before the single successful final Go run. These setup failures are not compiler results and were not relabeled as successful tests.

## Remaining obligations

The tagged/native Previous observer must be integrated with this component and separately prove that source-owned private List/Map/Optional carriers cannot cross into an unsupported native consumer through the new snapshot slot. The cache change creates no conversion permission.

Real Apple proof is still required for missing stored keys, scalar persistence, clock/TTL behavior, complete input-content sequences, cross-run/device scope and physical-device behavior. The model states its native-action assumptions; it is not a simulator.

Complex values, native File/Date/object persistence, general serialization/reconstruction, atomicity/concurrency, cancellation and catchable storage errors are outside the implemented scalar archive. Unsupported source shapes remain explicit. Malformed native JSON or native I/O can still terminate the shortcut; this implementation does not pretend to catch such errors. Sensitive-value caching requires the explicit existing opt-in and does not claim automatic secret detection.

The existing Legacy compact-assignment parser limitation is recorded independently. The fixture's established whitespace does not claim to repair that parser.

## Original evidence locations

All component originals are under `/mnt/development/projects/evidence/cherri-p10r15-cache-ttl-20261125`.

The final Go receipt is `previous-green-02/result.original.json`; the model owner is `previous-model-qualification-01/result.original.json`; and the full binding intake is `previous-qualification-intake-01/result.original.json`. The adjacent JSON proof preserves exact source and receipt bindings. No old original was overwritten.

Original Asana task: [cache/TTL 1220544665232085](https://app.asana.com/0/1220544665232085).
