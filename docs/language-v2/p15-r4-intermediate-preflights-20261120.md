# Intermediate R4 verification and retained failures — 20 November 2026

## Scope

These are exact-source intermediate checks. All three original combined preflights remain FAILED. Focused later passes do not replace a failed suite, close the final179 gate, inherit another requirement ID, or provide Apple runtime credit. The frozen R3 report remains95 PASS,2 FAIL,66 NOT_RUN,15 PENDING_EXTERNAL and1 AI_EVAL_NOT_RUN.

## Original combined executions

| Source and selection | Actual completed evidence | Unresolved outcome |
| --- | --- | --- |
| 891ff4c2c2520cbde7d0352c61ceeb1eba0daa9b | Fresh compiler;47/47 Legacy fixtures and17/17 action cases;420 affected root events;75 lower/analysis events;528 completed acceptance events | The20-minute acceptance package deadline expired inside SupplementalAcceptanceProductionEvidence. The partial parent is not credited. |
| 0dc408530082059ec06e08506fc05eed2fb210d2, root selection | Fresh compiler;47/47 Legacy fixtures and17/17 action cases;558 root events | TestP15SchemaOptionalDeclarationPropagation failed restoring generated schema with an empty original diagnostic. F03 failed its authored-None type oracle. |
| Same0dc source, remaining selection |76 lower/analysis/types events;260 RP11,280 RP16 and135 U01 outer events | SupplementalVerificationProductionEvidence failed the RP03 fresh-clone child's required completion. The preserved child tail is TempDir cleanup; no body PASS is inferred. Generated-doc checks after this gate were not run. |

The891 acceptance stream is667,173 bytes, SHA256 abc6f2e62e6c78913ba5b04e93fb2d0122e386e0e05cf7633e195cab005078ac. The0dc root stream is536,125 bytes, SHA256 b2f82b1d39061251c2e4ba729ff421a8aca3024ba236e1585e5f05c3519dd2c1. The0dc remaining acceptance stream is833,523 bytes, SHA256 eabe09916acc5a5043f40020b83ad64f08ecd54bba113e87708150507204549f.

## Independently audited completed children at0dc

A separate auditor checked every completed child process, exact required/discovered/pass event sets, command, time window, source binding, and original artifact hash. It verified92 distinct source blobs covering the three producers and all three immutable requirement contracts.

| Requirement | Actual root events | Bound source files | Evidence records |
| --- | ---: | ---: | --- |
| RP11 |95 |45 | unit and native-differential |
| RP16 |114 |45 | unit and native-differential |
| U01 |14 |50 | repository |

These five records are valid completed component observations at0dc. The overall invocation still failed on its separate supplemental parent. Later U01 original-object hardening has its own19-event clean proof and is not retroactively attributed to this14-event execution.

## Focused follow-ups

The narrow F03 oracle fix preserves authored Optional(Unknown) instead of collapsing it with the generic Unknown helper, while retaining every native clear/value mutation assertion. Its clean replay at6ec590940e0179f9739ebcf4198696d2662cd100 passed the original test in19.640 seconds. Original stream SHA256:505263d39e70f831ba5284bfe5e26102fd2daade466264abf14db3ad5d0301f2.

The schema helper atf10bf2cfce9751aa8f4ca740bce0c1304c1996cd now preserves exact command and context-deadline diagnostics on error. The original2-minute command deadline is unchanged. Its focused clean test passed in151.813 seconds; original stream SHA256:94c492c737e252b1ef68048248115aef4317b33893483099eba6cb4f2f8f6f35. This does not establish the cause of the earlier empty-diagnostic exit1.

## Reproducible record

[Original archive](evidence/p15-r4-intermediate-preflights-20261120/original-evidence.zip) and [index and independent audit](evidence/p15-r4-intermediate-preflights-20261120/index.json) preserve original process/log bytes, completed child receipts, the auditor, both focused replays, and component commit maps. The two reproducible compiler binaries remain in their original external evidence directories and are listed by size/hash. Every archived entry and staged Git blob was byte-verified.
