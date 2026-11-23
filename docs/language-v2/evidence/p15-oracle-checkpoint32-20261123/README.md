# Checkpoint 32: verification repairs and scoped qualification

This checkpoint records the original K476 acceptance run and the subsequent verification repairs at implementation **08dab62c1dd838a4a45f4d6bfcea2ad5e0817402**. Its evidence cutoff is **2026-11-23 17:07:36 UTC**, before publication.

## Results and their scope

| Evidence | Recorded result |
|---|---|
| Original K476 full179 | 111 PASSED / 4 FAILED / 28 NOT_RUN / 36 PENDING_EXTERNAL; closure false |
| Precommit RP52 source-boundary checks | 32 named passes on the reviewed dirty candidate |
| Precommit E05 portable Swift probe | 26 controls passed; UI and real compiler execution false |
| Detail-consistency regression | The causal RED failed at the intended assertion; focused GREEN passed six parents and four children |
| Clean qualification at 08dab62c | Three parents and 239 children passed: 242 named events, with no failures or skips |
| Nested RP52 production checks | Nine parents and 89 children passed; one local requirement passed, closure false |
| Editor component results | E01, E02, E03, E05 and E06 remain PENDING_EXTERNAL for ios-ui |

The 98 nested RP52 events are part of the work exercised by the outer qualification. They are not added to the 242 outer events. Four fresh Swift receipts cover 26 publication controls and 17 position controls in each of two parent paths. They establish their recorded local behavior; Apple runtime and UI verification remain required.

The retained precommit E05 probe was also checked against six identical committed source blobs without recompilation or reexecution. Its original dirty-source attribution and all seven original artifacts remain intact. The initial missing-temporary-directory failure and the causal RED are preserved.

## Evidence files

- [Original full179 result](full179.public-projection.json)
- [Harness repair and preserved attempts](harness.public-projection.json)
- [Report/audit contradiction and detail repair](end01.public-projection.json)
- [Clean implementation](implementation.public-projection.json)
- [Clean qualification and independent review](cleanqualification.public-projection.json)
- [Remaining work by requirement](next-work.public-projection.json)
- [Original receipt bindings](original-bindings.json)
- [Source bindings](source-bindings.original.json)
- [Public file manifest](MANIFEST.json)

These files contain bounded metadata and references to retained originals. Raw logs, archives and private input are not included. The original K476 report, its 36 report/audit detail contradictions and its unexecuted END01 remain unchanged. The continuation preserves future producer detail while retaining the canonical equality check.

The proof commit identifies this documentation checkpoint after it is committed; it cannot embed its own SHA. Accordingly, proof and publication fields are null at the stated cutoff. Actual publication is recorded separately. No new full179 execution, actual END delivery, native result, final K, compiler S or release is claimed.
