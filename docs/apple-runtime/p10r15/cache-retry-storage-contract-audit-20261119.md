# Cache, retry and storage contract audit — 2026-11-19

Audit source: `a432646e28c419d91995927a5eb3d34db1337466`. The scoped follow-up repairs the loopback HTTP probe and its readiness consumer. It does not implement a compiler macro or change an Apple runtime oracle.

## Task boundaries

| Task | Required behavior | Established evidence | Remaining work |
| --- | --- | --- | --- |
| [cache() / TTL](https://app.asana.com/0/0/1220544665232085) | Reuse Stored Content actions, typed value envelope, expiry, namespace/isolation, lazy miss body, corruption handling and physical persistence | The task is explicitly planned. Current storage signatures accept Text; two native runs of the same installed shortcut preserved Text | `cache()` implementation, safe miss/delete semantics, TTL and clock behavior, a truthful type support matrix, cross-shortcut isolation and physical-device cases |
| [retry() / bounded backoff](https://app.asana.com/0/0/1220544651247273) | Catch an observable failure from the same HTTP request, retry only reviewed conditions, cap attempts/wait, preserve the final native value and avoid duplicate side effects | Native 500 and 429 probes returned response bodies and continued. A transport probe terminated with an Apple runtime error | A supported same-request status/catchability boundary, retry implementation, attempts/backoff/cancellation/idempotency controls and physical-device cases |
| [Stored Content / Lumen integration](https://app.asana.com/0/0/1219814215189434) | Persist the application's state, preserve supported value types, handle missing/deleted values and validate complete device flows | Exact cross-run Text readback on one Simulator shortcut | Restart, missing key, deletion readback, global/cross-shortcut isolation, typed archives, physical iPad and complete Lumen/provider flows |

These task scopes were read directly from their existing Asana records. Full task notes, personal paths and unrelated project data are not copied into this audit.

## What the source actually exposes

[`actions/storage.cherri`](../../../actions/storage.cherri) declares `storeValue`, `getStoredValue` and `deleteStoredValue` with an iOS 27 version gate. The authored store signature accepts a Text value and maps it to `WFInput`; the native input UI uses `WFTextInputParameter`. Generic native content metadata does not establish Dictionary, List, Date or File archive preservation. The existing `qa-storage-delete-v2` fixture asks whether deleting a known value permits a subsequent read with zero items. Its native result remains pending; a first cache miss cannot safely assume that a missing-key read returns an empty value.

[`actions/web.cherri`](../../../actions/web.cherri) uses the native download-URL action for HTTP bodies. `getURLHeaders` issues a separate request, so it cannot establish the response status of the original request. No proven status-as-value or recoverable transport-error contract was found in the reviewed source/fixtures. This is a scoped implementation finding, not a claim about every possible Apple API.

## Native receipts retained

- [Storage run 44428362458](https://github.com/davidpovarsky/cherri/actions/runs/44428362458), source `d4305179ca471e4b6ec8fceb87339955249dc2d5`: distinct correlated WRITE and READ runs returned `TYPE_OK:VALUE_OK`. The bounded conclusion remains `PASS_NATIVE_CROSS_RUN_TEXT_ONLY`. See [storage publication audit](storage-cross-run-publication.md).
- [HTTP 500 run 44344880086](https://github.com/davidpovarsky/cherri/actions/runs/44344880086), source `52284f8ad179997bf35d41dde21a758dfc611059`: one 500 response body and one continuation request were observed. [HTTP 404 run 44345179210](https://github.com/davidpovarsky/cherri/actions/runs/44345179210) remained blocked before the target invocation. See [500/404 audit](retry-http-500-404-native-audit.md).
- [HTTP 429 run 44345197107](https://github.com/davidpovarsky/cherri/actions/runs/44345197107), the same source, returned the body and continued. [Transport run 44345223487](https://github.com/davidpovarsky/cherri/actions/runs/44345223487) actually failed with Apple error `-1005`; three closed GET connections and no continuation were observed. Those connections do not prove a compiler retry macro. See [429/transport audit](retry-http-429-transport-native-audit.md).

The historical transport failure and the separate F04 nested/empty List cardinality failures remain failures. Scalar success, response-body continuation and expected compiler rejection cannot replace those acceptance results.

## Real local defects and repair scope

The unchanged probe accepted receipts after seven non-GET methods had reached its loopback listener: PUT, PATCH, DELETE, OPTIONS, TRACE, CONNECT and an unknown BREW method. Python's default HTTP 501 handler bypassed the receipt counter. A valid GET/continuation sequence after each request still passed receipt validation. The repaired parsed-request gate rejects every non-GET method through the existing counted path, stores no raw path/header/body, and makes `validate_receipt` reject the run with `UNREVIEWED_REQUEST_OBSERVED`. POST and HEAD remain positive controls for rejection accounting.

The unchanged readiness helper also returned the old port/nonce after a real service had STOPPED or EXPIRED and closed its socket. The actual `input` process returned exit 0 for a stopped service. The repair retains the historical ready file, reconciles its identity and port against the startup-state and server receipt, and requires both records to describe SERVING with no stopped timestamp. Known terminal states produce `SERVER_NOT_READY`; identity/shape mismatches remain bounded contract failures. This validates recorded service state. It does not promise abrupt-process-death detection or eliminate a race after the readiness check.

Causal tests send real requests over `127.0.0.1`, stop/expire owned finite servers, and invoke the actual Python `input` command. Separate mutations swap receipt identity, port, JSON shape and terminal state, then restore the original files and prove the live route still works. Synthetic compiler/callback records used by existing classifier controls remain explicitly synthetic. None of these local tests grants Apple execution, cache TTL, typed archive or retry acceptance.
