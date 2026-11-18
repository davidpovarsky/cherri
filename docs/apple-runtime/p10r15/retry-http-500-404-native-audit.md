# HTTP 500 and HTTP 404 causal native audit

Both runs used immutable source `52284f8ad179997bf35d41dde21a758dfc611059`. Every published file was independently verified against its byte count and SHA-256 manifest: **21/21 for each run**, with no unexpected published path. The selected fixture source was independently compared with its exact committed Git blob. Full hashes and observations are in [retry-http-500-404-native-audit.json](retry-http-500-404-native-audit.json).

| Run | Observed outcome | Scope |
|---|---|---|
| [44344880086](https://github.com/davidpovarsky/cherri/actions/runs/44344880086) | Strict runtime and trace PASS | HTTP 500 returned its exact synthetic body and execution reached the separate continuation request |
| [44345179210](https://github.com/davidpovarsky/cherri/actions/runs/44345179210) | Workflow failure; native target NOT_RUN, BLOCKED_IMPORT | Signed artifact prepared, but import did not complete; no native HTTP 404 observation |

## HTTP 500: observed continuation

The 16-action signed target contains two existing `downloadURL` actions. The first calls the run-scoped loopback `/http500` route; the second calls `/continued`. The first response's value is included in the final output. The compiled fixture introduces no retry or native error-catching operation.

The bounded server evidence records exactly one primary GET with status **500**, followed by exactly one continuation GET with status **200**. Both response writes completed. The primary body hash matches the literal synthetic body `P15_HTTP_BODY_OK`. The server subsequently stopped.

One fresh, correlated Shortcuts callback returned SUCCESS with the exact required output:

`CHERRI_QA_RETRY_HTTP500_RESULT:P15_HTTP_BODY_OK`

Its two inline events and two collector events match the published NDJSON trace in order, including collector begin/end. Source SHA, unsigned plist SHA, compiler SHA, fixture identity, callback nonce and signing provenance agree. The Receiver self-test is a separate callback and is not used as target execution proof.

This establishes body return and continuation for this observed HTTP 500 response on the iPad mini (A17 Pro), iOS 27.0 Simulator, Xcode 27.1 (27A9275), macOS 27.0.1 (26A434).

## HTTP 404: import blocked

The same compiler produced a 16-action fixture and HubSign AEA1 provenance. The coordinated import acknowledgement and its retry timed out; the final gates remain BLOCKED_IMPORT. The runtime assertions explicitly report that the compiled Shortcut was not executed.

The only callback is the Receiver self-test. There is no correlated target callback or valid target trace. The diagnostic remains `PROBE_EVIDENCE_REJECTED`, so this run supplies no finding about HTTP 404 response handling, continuation, transport failure, or catchability. The strict failed result is retained.

## Remaining contract

The observed status 500 belongs to the independently controlled synthetic server. It is not a status value exposed by the HTTP action to the Shortcut. Same-request status access and transport-error catchability remain **NOT_PROVEN**. A production `retry` macro remains unimplemented pending those required native contracts.

HTTP 429, connection-close, the collection empty-input split, and the If-result let/var split remain separate immutable runs under their existing watcher. Their queue state supplies no native result. Editor Inspector is still NEEDS_REVIEW, and physical-device proof is absent.

Only the published synthetic text artifacts were audited and committed. Raw signed Shortcut bytes, raw server receipts, screenshots and unbounded logs remain outside this evidence report.
