# P15 backend state: actual producers and strict intake

## Verified result

BRG06 and RP51 now execute independent production scenarios under their unchanged contracts. The actual available Windows security and benchmark portions passed. Both rows correctly remain `NOT_RUN`, because this machine did not execute the required Linux CGO race configuration. Strict manifest intake rejects missing local tiers; no linked requirement or full 179 closure is inferred.

| Source | Verified execution | Result |
|---|---|---|
| `c4f1aad56ba991cc81dfc1fee25d5fc2231a5020` | Actual security/benchmark producers and causal intake controls | 96 named passes, zero failures/skips, exit 0, 187.453s; clean unchanged source |
| `1355a217661bed0953c9d4e1a915c8874ca7fe34` | Acquisition rejection plus all focused intake/inventory controls | 84 named passes, zero failures/skips, exit 0, 11.579s; clean unchanged source |

First raw SHA256: `4e8da41648d4f90c2e398d2979b971e8374ca30312f0ced37b4bacc3b85b408c`. Second raw SHA256: `c39b7728813f8b4d8cf7e6e9480efa69ea90c47fa850ff42b132536d3e4b4d2a`. The second replay did not repeat the first revision's benchmark/security subprocesses; it verified the bounded acquisition change. Fresh integrated Linux execution remains required.

## Exact independent scenarios

| ID | Unchanged required tiers | Producer |
|---|---|---|
| BRG06 | unit, integration, security | Complete actual Linux race bundle for both state tiers; independently executed analysis CLI side-effect scenario for security |
| RP51 | benchmark, race | Independently executed six-fixture, twenty-phase, 400-sample service/cache benchmark; complete actual Linux race bundle |

The race selection requires four top-level tests and both error/cancellation children. Raw witnesses require 64 mixed concurrent cases, 32 cold import cases, error/cancellation recovery and panic containment. The exact original runner's immutable Git-blob evaluator independently checks raw events; a completion marker alone is insufficient. The state source inventory is compared with 21 exact immutable Git blobs, without substituting normalized worktree bytes.

The actual security process ran `TestP15CLIAnalysisHasNoExternalEffects` with filesystem comparison, working shortcuts/open observers and a loopback signing endpoint, including positive observer controls. It passed in 55.609s; raw SHA256 `b3e9e655c69cfb7841d4a33afa85f969a4a7f87aaaf64ad25b75af14c513abb4`. No successful external signing is claimed.

The actual RP51 benchmark independently ran `TestP15E08EditorPerformanceFixtures` in 115.506s. Its process requirement and source are RP51/current, without relabeling an old E08 record. Raw SHA256 `78a02896def2690f642aba27e44251ed8280fcb90cc6d9a24868f41d6216c646`; benchmark SHA256 `c453a2a12b440e73f0c69b1529affac1c4818a55ce8a8570a55182ecaeb06980`. Raw samples, clock ticks, p50/p95, cache events, input digests and environment are retained.

## Causal rejection controls

The positive race intake control is the unmodified original Linux artifact from run 44634819471 / source `4e56dbcfe895baa8f944b83715ecff0d07bfd24e`, separately acquired/audited in the R2 race report. It is historical intake verification only. Its original ZIP/source/run identity is never credited to current production.

Controls reconcile all outer/inner hashes while changing every source-file digest, raw test/witness completeness, instrumentation, failure/skip status, environment, timing, selected producer, source identity, scope or tier. Invalid or partial evidence is rejected. Original 179 contract bytes and final closure rules are unchanged.

Peer review exposed a byte-fidelity defect in the initial draft: Go string/JSON conversion could sanitize invalid UTF-8 before Python's strict decoder saw it. A real archived Output byte was corrupted and every content hash reconciled. The rejection regression failed before repair because the corruption was accepted: raw baseline SHA256 `8d04d7404c16c5efb1dc73255f693e2639fc17ee8a50a58630705c4a8094ee21`. Lossless base64 now transports stdout/stderr unchanged; JSON/security inputs also reject invalid UTF-8. The exact negative control now passes by rejecting corruption.

The production adapter rejects `CHERRI_STATE_RACE_EVIDENCE_DIR`, including an authentic historical bundle and a bundle with source/run fields rebound. An arbitrary directory cannot establish independently observed current execution. Only a fresh child process in configured, clean Linux CI supplies production race evidence; original output and failure receipts are preserved.

The supplemental meta-suite executes its explicit selected producer families and counts their contract-selected rows. Its full 179 audit, unselected-row non-promotion and missing-producer controls remain intact. The separate state suite actually executes the complete available state producer selections and rejects partial evidence. The production full 179 runner still executes all 179 obligations.

## Observed local service/cache measurements

Windows amd64, Go 1.25.6, Intel Core i7-9750H, 12 logical CPUs, GOMAXPROCS=2, 16,870,088,704 memory bytes. QueryPerformanceCounter frequency 10,000,000Hz. Cold means a fresh Cherri service and asset cache; OS filesystem cache was not evicted. These are actual local service timings, without native UI latency or speedup claims.

| Fixture | Phase | Samples | p50 (ms) | p95 (ms) |
|---|---|---:|---:|---:|
| small_source | cold_service_cache | 20 | 11.502 | 15.304 |
| small_source | warm_analyze | 20 | 0.001 | 0.002 |
| small_source | unrelated_edit | 20 | 10.738 | 14.711 |
| medium_source | cold_service_cache | 20 | 18.757 | 55.444 |
| medium_source | warm_analyze | 20 | 0.031 | 0.034 |
| medium_source | unrelated_edit | 20 | 21.645 | 46.858 |
| large_source | cold_service_cache | 20 | 96.319 | 126.722 |
| large_source | warm_analyze | 20 | 0.230 | 0.351 |
| large_source | unrelated_edit | 20 | 106.198 | 321.562 |
| inline_text_4mib | cold_service_cache | 20 | 315.129 | 1087.780 |
| inline_text_4mib | warm_analyze | 20 | 0.016 | 0.036 |
| inline_text_4mib | unrelated_edit | 20 | 220.622 | 303.771 |
| local_text_8mib | cold_service_cache | 20 | 120.664 | 494.529 |
| local_text_8mib | warm_analyze | 20 | 0.442 | 0.606 |
| local_text_8mib | unrelated_edit | 20 | 13.421 | 28.878 |
| local_text_8mib | asset_edit | 20 | 142.490 | 202.549 |
| local_binary_8mib | cold_service_cache | 20 | 149.755 | 870.077 |
| local_binary_8mib | warm_analyze | 20 | 0.381 | 0.777 |
| local_binary_8mib | unrelated_edit | 20 | 11.389 | 14.961 |
| local_binary_8mib | asset_edit | 20 | 67.557 | 77.563 |

## Preserved proof and next gate

`evidence/p15-state-acceptance-20261119/` retains 23 byte-identical original files plus the exact production receipt reconstructed from consecutive Go Output chunks. The failed UTF-8 baseline is retained. `sha256-manifest.json` lists original paths, byte counts/digests and the three immutable-contract Git-blob hashes. Original artifact references and the frozen R2 full 179 report are not rewritten.

The next gate must execute the combined current source on Linux with CGO enabled, preserving fresh race, security and benchmark receipts. Windows partial success does not complete BRG06 or RP51. This component claims no Apple runtime, concurrent signed compilation, network signing success or AI quality.
