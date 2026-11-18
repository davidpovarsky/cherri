# P10R15 release evidence refresh

## Observed R1 failure

Validation candidate `d00bc0b9e9a4d8c460623de3d883680b94289e04`, Build & Test run [44223692029](https://github.com/davidpovarsky/cherri/actions/runs/44223692029), completed its Linux full Go suite, matrix, discovery, decompile, package, parity and oracle checks successfully. The subsequent release ledger gate failed because four recorded input dependency fingerprints referred to older compiler inputs. Nix passed; macOS had no executed steps when inspected. This is a recorded failed Linux job, not all-platform success.

## Correction and execution

Implementation `92f834e1878e02d6357021e97c491c55437c038b` expands the existing dependency fingerprint to cover all root Go files, internal compiler modules, module dependencies, registry generator and release evidence producer code. Addition, mutation and deletion of new macro/lowerer inputs now invalidate an old receipt; line-ending normalization remains deterministic and generated receipts remain outside their own dependency graph. The runner uses its active Python interpreter rather than an unrelated executable or Windows Store alias.

Actual runs on this implementation:

| Producer/check | Exit | Seconds | Scope |
|---|---:|---:|---|
| Existing action-contract generator --refresh | 0 | 35.45 | Executes all 505 compiler/decompiler entries and binds fresh source hash |
| Existing catalog/pack ledger generator | 0 | 23.53 | Rebuilds historical crosswalk, matrix and baseline from execution receipts |
| Existing release runner --run-all | 0 | 137.45 | 55 PASS, 1 BLOCKED_PLATFORM, 0 FAIL across 56 gates |
| Source ancestry/dependency fingerprint verifier | 0 | 0.62 | All four artifact sources are ancestors with matching inputs |
| Ledger consistency verifier | 0 | 0.22 | Existing cross-ledger invariants |

All 28 adversarial gates passed. Of 28 acceptance gates, 27 passed; R22 remains BLOCKED_PLATFORM on Windows and explicitly preserves the separate BLOCKED_DEVICE physical iPad requirement. The resulting verdict is **PARTIAL_WITH_EXTERNAL_BLOCKERS**, never READY.

These files were regenerated from actual executions. Hash fields were not edited to relabel old tests. Later compiler changes require another source-bound refresh and exact-candidate CI run. Private corpus was not involved in the public 505/gate producers. This stage does not certify planned macros, all native parameter variants, signed execution or physical-device behavior.
