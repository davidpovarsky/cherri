# HTTP 429 and connection-closure native probes

Both runs used immutable compiler source `52284f8ad179997bf35d41dde21a758dfc611059`, the existing signed Simulator harness, and synthetic loopback requests. The audit independently verified all 21 published files in each run, their byte counts and hashes, the exact published file set, the committed fixture source bytes, and signing/compiler provenance. It replayed the runtime oracle extracted from that exact Git source.

| Run | Native observation | Strict result |
| --- | --- | --- |
| [44345197107](https://github.com/davidpovarsky/cherri/actions/runs/44345197107) | HTTP 429 returned the exact synthetic body; one separate continuation GET followed | PASS |
| [44345223487](https://github.com/davidpovarsky/cherri/actions/runs/44345223487) | Three primary GETs were closed without an HTTP response; a fresh correlated ERROR with numeric code -1005 followed; no continuation GET | FAIL_RUNTIME |

## HTTP 429

The exact callback summary was `CHERRI_QA_RETRY_HTTP429_RESULT:P15_HTTP_BODY_OK`. Its fresh nonce, fixture and compiler identities matched the manifest and assertions. The actual callback trace matched the published trace: two inline events bounded by two collector events. The server recorded one completed 429 response and one completed 200 continuation response, then stopped.

This proves body return and continuation for this synthetic 429 response. The server observed the status externally; the Shortcut did not receive a demonstrated same-request status value. The fixture contains no compiler retry or catch operation.

## Connection closure

Import and invocation gates passed. The exact callback was ERROR, code `-1005`, with no semantic result or inline trace. The ordinary runtime and trace gates remain failed. The published trace contains one terminal collector failure record, not a successful BEGIN/END envelope.

The server receipt recorded three primary connection closures and zero continuation requests. The diagnostic intentionally remains `MULTIPLE_REQUESTS_OBSERVED` with `diagnosticObservationComplete=false`. Repeated GETs cannot be assigned to compiler retries: the source contains only one primary GET action, followed by its separate continuation action. This observation does not establish catchability or the origin of each repeated request.

## Boundaries

Both environments reported iPad mini (A17 Pro), iOS 27.0, Xcode 27.1 build 27A9275 and macOS 27.0.1 build 26A434. Editor Inspector remains `NEEDS_REVIEW`; no physical device was used.

The paired results, together with the earlier HTTP 200 and 500 controls, do not authorize a general retry macro. Same-request native status values, a transport failure that can be caught while continuing the Shortcut, and bounded production retry semantics remain unproved. The earlier 404 run remains import-blocked, with no native HTTP 404 observation.

Only bounded published text artifacts were independently rehashed. Raw signed Shortcut bytes and raw server receipts remain quarantined; their reported hashes are recorded as provenance, not as independently inspected raw files. Detailed hashes, source bindings, exact oracle replay results and server events are in [the audit](retry-http-429-transport-native-audit.json).
