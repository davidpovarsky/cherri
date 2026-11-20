# E05 atomic versioned analysis snapshot — actual local component

Tested source:46246596dee9032fb41ce9bf35107183fda19bab. Base:1720ccd8f9272cfcc379ad5c5bea0e4720a55aa8.

This component repairs a real shared-mobile service response race. It does not establish E05's required UI, cancellation/publication in SwiftUI, Apple execution, full179 or final consumer identity.

## Actual causal failure

The existing CherriAnalyzeVersioned export updated the shared document, read diagnostics and then read active version through three separately locked Service operations. Behavior-preserving extraction a1b83438 moves that exact response producer into mobile_language_service.go, with the C export retaining exact delegation and response encoding. The portable tests execute the actual production Go response function; the C wrapper is checked structurally, not represented as an executed iOS ABI.

The corrected clean05a45465 replay executes128 actual calls across eight concurrent workers.107 returned a newer response.Version with diagnostics from an older source marker, including Version12 with missingVersion0011. The expected diagnostic identity/range is independently authored from each returned version. Sequential stale input, independent documents, returned-copy isolation, healthy recovery, close/reopen, C delegation and11 mutations of actual response fields pass. This isolates one genuine concurrency failure.

## Repair

Service.AnalyzeVersioned updates the document and obtains diagnostics, schema identity and active version under one existing service mutex. OpenDocument and Analyze delegate to private locked helpers so their public signatures, stale lower-version policy, module/asset refresh and exact diagnostic dedup remain unchanged. The actual mobile response producer calls the new atomic API; no wire schema, compiler mode, semantic type, codec or179 contract is altered.

## Retained test issues

The first a1b83438 attempt executed no tests: the new test imported go/token without an alias, colliding with the root package's existing token declaration. The separate d22ec46a correction fixes only that test import and C delegation guard. Its first executed oracle then wrongly expected lowercase error, while the existing protocol deliberately emits ERROR. That failed original is retained;05a45465 corrects only the expected spelling and leaves the actual107 cross-version mismatches visible. No production output is changed to satisfy either test mistake.

## Actual execution

| Attempt | Source state | Exit | Named PASS | Named FAIL | Named SKIP | Seconds |
| --- | --- | ---: | ---: | ---: | ---: | ---: |
| e05-mobile-baseline-a1b83438 | a1b83438 clean | 1 | 0 | 0 | 0 | 17.735 |
| e05-mobile-baseline-d22ec46a | d22ec46a clean | 1 | 2 | 7 | 0 | 40.594 |
| e05-mobile-baseline-05a45465 | 05a45465 clean | 1 | 19 | 1 | 0 | 58.235 |
| e05-mobile-clean-46246596 | 46246596 clean | 0 | 20 | 0 | 0 | 49.922 |
| e05-packages-clean-46246596 | 46246596 clean | 0 | 235 | 0 | 0 | 23.547 |

The clean repaired root command go test -count=1 -json -timeout=3m -run ^TestP15E05 . completed20 named PASS,0FAIL/0SKIP,49.922s, clean before/after. All128 actual production responses matched their versions, with zero mismatches. Original17,717B raw SHA256 de45acd65d4bfb68f0bdbb27997242d340b10d29fd9c150f2f9112e5e26f7480.

The same immutable source then passed the entire service/analysis/LSP packages:235 named PASS,0FAIL/0SKIP,23.547s, original217,437B raw SHA256 e1091d512e68351d46f20dbccddf744b389894cec0233a783bdeab2bf1e162a6. Both commands used GOMAXPROCS=2 and retained their finite original budgets.

## Evidence scope

All original logs, receipts and submitted scripts are byte-preserved. provenance.json records exact source commits, event counts, immutable Git witnesses and three unchanged acceptance contracts. The ZIP is verified against every original; proof bytes are reread from the separate committed object. These20 production controls are local evidence only; no synthetic process events, inherited requirement IDs, native UI pass or full E05 closure are generated.

Swift publication/cancellation behavior and its real UI execution remain separate follow-ups. The shared versioned service is now coherent, but a UI can still mishandle results unless it independently preserves current request ownership.

## Proof assembly correction

The first seal correctly stopped before any commit because its witness inventory named nonexistent internal/language/schema/registry.go. The actual registry definitions are in schema.go and registry_clone.go. The original failed seal script/output are preserved, and the continuation verifies the same tested source plus the owned partial proof directory before completing the exact Git witnesses. No test, original evidence or source byte was changed by this correction.
