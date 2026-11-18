# Native HTTP error and continuation probes

## Scope and status

These causal diagnostics support [Asana retry task 1220544651247273](https://app.asana.com/0/0/1220544651247273). They do not implement retry, add an HTTP registry, modify signing, or claim native exception handling. Local HTTP client tests and compiled plists are not Apple runtime evidence.

The task requires proving native error capture and same-request status access before implementing retries. Checked-in macOS 27 metadata describes downloadurl as returning content (public.data, “Contents of URL”), with eight request parameters. It exposes no response-status or error-as-value contract. ShowHeaders edits request headers. The separate url.getheaders action makes another request and therefore does not prove status from the original GET.

Existing runworkflow metadata establishes a child result, not a catchable error value. An outer x-error/x-cancel callback observes a terminated Shortcut; it does not provide an error value inside the running block. The existing Go HTTP echo tests use a Go client against reconstructed request data. The native specimen manifest contains zero specimens.

Inspected inputs:

| Input | SHA-256 |
| --- | --- |
| docs/apple-runtime/macos-27/builtin-actions.json | 8fb303591888bcb01fe5935b1571d590add428c9bd044f0ab52dcdf99490571a |
| docs/apple-runtime/macos-27/parameter-encodings.json | 9b153807299ae503ed2f9f665ab17ffbe650a4f6defec9f81eb0deac5a9e7969 |
| actions/web.cherri | 96c52949f4f3b6549c9df607c08961b6a710b0af7697d72e6dda08a4f33fb711 |
| p10_http_json_test.go | bfc0304ca82b1be3e7b0c81bedb26837a828272ae7ba716a0b9ed03367456c09 |
| testdata/native-specimens/manifest.json | 154f1885c221c35730da4dd7f79ddb9f499b5c3c93ef9e1ad972b3a2130f4925 |

The broad retry macro remains **BLOCKED_NATIVE_ERROR_CONTRACT**. This states a missing prerequisite, not impossibility.

## Finite native experiment

The existing signed Simulator QA workflow has five explicit selections:

| Selection | Primary response | Exact successful output |
| --- | --- | --- |
| qa-retry-control | HTTP 200 | CHERRI_QA_RETRY_CONTROL_RESULT:P15_HTTP_BODY_OK |
| qa-retry-http500 | HTTP 500 | CHERRI_QA_RETRY_HTTP500_RESULT:P15_HTTP_BODY_OK |
| qa-retry-http404 | HTTP 404 | CHERRI_QA_RETRY_HTTP404_RESULT:P15_HTTP_BODY_OK |
| qa-retry-http429 | HTTP 429 | CHERRI_QA_RETRY_HTTP429_RESULT:P15_HTTP_BODY_OK |
| qa-retry-transport | Connection closes before any HTTP response | CHERRI_QA_RETRY_TRANSPORT_RESULT:P15_HTTP_BODY_OK only if a genuine result can satisfy the normal oracle |

Fixture IDs append -v2. All four HTTP status responses have exactly the same plaintext body and content type. Status is observed at the controlled server; a body field is never substituted for status.

The emitted graph makes one primary GET, then a separate /continued GET, then reads the original primary producer into the semantic result. The second request observes that execution reached the next action. It does not supply the primary body, status, or an error substitute. No retry loop, child Shortcut target, catch emulation or new macro is emitted.

**Run the 200 control first.** Only after its exact result, normal trace and both endpoint receipts pass may the remaining four experiments run. Cancellation and the full 500→500→200 retry acceptance sequence remain open.

### Static source and fresh endpoint

Reviewed source bytes never change during compilation. URLs contain the literal loopback host and fixed route, with ShortcutInput as their only token attachment. The server binds an OS-assigned loopback port, then supplies port/freshRunNonce through the existing text input launch mechanism.

The input helper validates the exact allowlisted fixture, compiler commit, fresh bounded nonce, loopback host and integer port. A previous ready file is rejected. No external URL, environment-selected source path or template rewrite is accepted. The graph validator independently checks both GETs, token ranges, input identity, action kinds and primary-to-summary UUID edge.

Input-free fallback launches are disabled only for these five input-dependent fixtures; a missing endpoint would execute a different experiment.

### Lifetime and cleanup

The helper uses the existing temporary loopback-server orchestration pattern. It runs at most 1,800 seconds, records at most 16 requests and bounds connection handling. Wrong paths and methods are rejected. A stop file and EXIT cleanup close it after native execution. Expired, reused, unreviewed-request or incomplete-write receipts cannot count as verified.

Receipts contain only reviewed route classes, response status, fixed-body hash, sequence, timestamps and write-completion flags. They contain no arbitrary URL, headers, request body, raw error message or response text. Multiple GETs remain visible: Apple transport or harness repeats cannot be claimed as compiler retries.

## Interpretation and unchanged gates

The bounded diagnostic is separate from the existing runtime and trace gates:

| Observation | Meaning |
| --- | --- |
| CONTROL_VERIFIED | Fresh successful 200 callback, exact result, normal trace, one primary and one continuation receipt. |
| HTTP_RESPONSE_CONTINUED | A non-200 response returned the expected primary body and reached the observer with strict callback/trace proof. It does not expose status as a Shortcut value. |
| TERMINATED_WITHOUT_CONTINUATION_RECEIPT | Correlated ERROR after the primary request without a continuation receipt. This alone does not identify the failing native action. |
| CANCELLED_WITHOUT_CONTINUATION_RECEIPT | Corresponding observation with actual CANCELLED callback; catchability is not established. |
| CONTINUATION_OBSERVED_OUTPUT_UNVERIFIED | Observer reached, but exact normal result or trace not verified. |
| Other incomplete/rejected states | Preserve observations without acceptance claims. |

ERROR/CANCELLED remain ordinary FAIL_RUNTIME/trace failures. require_runtime=true still fails even when a separate diagnostic observation is complete. The only Release trace exception remains qa-measure-release-v2; all retry probes require debug-profile NDJSON.

Every diagnostic retains nativeCatchability=NOT_PROVEN, sameRequestStatusValue=NOT_PROVEN and retryImplemented=false. Successful continuation of non-200 content can inform later work but does not itself satisfy the requested retry API.

## Publication and local evidence

Only runtime/http-probe-diagnostic.json is newly publishable. Ready files, raw receipts, stop files and server logs stay quarantined. Its validator rejects extra public fields, arbitrary text, unknown IDs, bad digests, changed source hashes, stale nonces and inconsistent event counts.

Causal tests exercise real loopback 200/500/404/429 responses, connection close without status, finite lifetime/request count, socket cleanup, wrong endpoint bindings, malformed receipts, stale/duplicate callbacks, changed source hashes, producer-edge mutations, unexpected control flow and publication quarantine. They also execute the actual workflow selection and callback oracle. A negative elapsed-time mutation initially passed; validation now requires nonnegative monotone time.

All five sources were compiled by the actual CLI at 85b6c352d1e0f9f272cfc6f33c10d0becd29eee6, yielding 16 actions each. testdata/retry-control-compiled-graph.json explicitly records compiler output, not native runtime proof. Signed CI recompiles at its exact dispatched commit and records source and unsigned plist hashes.

The full local QA suite passed 98 tests with zero failures, errors or skips; shell syntax and all 20 embedded Python blocks also passed. See retry-native-probes-validation.json for local evidence. Signed run IDs and results must be recorded separately after execution.
