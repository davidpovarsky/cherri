# Native preservation production acceptance

The seven bounded contracts V04, C05, and R01-R05 pass on clean source `afc52fdccf9f073ad1a0f9b2d78a5da674094b40`. The actual formal wrapper ran in 32.337 seconds; its Go package reported 25.960 seconds. The source tree was clean before and after execution.

The complete source-bound result is in [receipt.json](evidence/p15-native-preservation-afc52fd/receipt.json), with the actual [verification log](evidence/p15-native-preservation-afc52fd/verification.log) and [causal evidence](evidence/p15-native-preservation-afc52fd/causal-evidence.json).

| Contract | Executed tiers | Actual production coverage |
|---|---|---|
| V04 | unit, native-structure | List/Dictionary typed payloads, large integer preservation, original action-output bindings, alias/reference integrity, and five causal corruptions |
| C05 | native-roundtrip | Two future wire enum values and a known-member control through real hybrid/lossless imports and both frontends |
| R01 | native-roundtrip | Unknown action, outer action fields, nested/root data, arrays, binary, dates, exact UUID spelling; all eight format/fidelity/frontend routes |
| R02 | native-roundtrip | Known native identifiers with unmodeled variants and parameter shapes, with readable known controls |
| R03 | native-roundtrip | Coercion-only, property-only, ordered chained aggrandizements and unknown property fields |
| R04 | native-roundtrip | Omitted versus explicit false/zero/empty text/list/map, and absent versus empty action parameters |
| R05 | unit, native-roundtrip | Existing raw output exports, missing/ambiguous authored identity rejection, atomic opaque regions, outward End If references, whole-workflow fallback and preserved preexisting invalid raw input |

## Causal correction

The original R01 compiler lost `P15UnknownActionField` beside an action's identifier and parameters. Removing only that field made all eight routes pass; retaining it made all eight fail. Exact-envelope support now carries that source data through the existing shared record, canonical emitter and serializer. It also preserves the field before any import overlay audit, so metadata restoration cannot hide the loss.

The schema was regenerated from the canonical catalog, with the same 505 entries. Existing raw calls retain their behavior. The new optional Legacy envelope argument and V2 named envelope are documented in [native-action-envelope.md](native-action-envelope.md). Extra arguments, conflicting source forms and compiler-private references fail explicitly.

A separate first formal run correctly rejected V04's unnormalized evidence-tier label even though its production test passed. The producer now uses the existing contract tier parser. The immutable contracts and verifier were not weakened or rewritten.

## Evidence scope

Each producer invokes named production Go tests and verifies their actual completion. The formal self-test rejects wrong producers, missing records, wrong contract identities, and missing required tiers; 23 bounded evidence mutations are covered. Every individual audit confirms that unrelated requirement rows do not inherit PASS.

These receipts prove local source analysis, compilation, import and serialized native-format behavior. They do not claim Apple runtime execution, unknown-action availability, physical-device proof, linked repair/regression-gate closure, or overall release readiness. The private corpus was not rerun by this agent; the integrated compiler/schema boundary requires the root-owned exact corpus refresh.
