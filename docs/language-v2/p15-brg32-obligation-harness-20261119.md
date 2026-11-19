# P15 BRG32 installed obligation harness evidence — 2026-11-19

## Exact scope and source

Implementation: `5f4408cb74c63ab99d8ce1f144dc29640169514e`. This component registers BRG32's immutable unit, integration and repository tiers. It does not rewrite any of the 179 contracted obligations. The latest complete integrated report remains the separate e61 snapshot.

The producer executes `TestRealContractsLoadAndVerifyDigests`, `TestLocalReportIncludesAll179WithoutInheritedClosure`, `TestP15RP03FreshCloneContractIntake`, `TestP15BRG32RequirementMapMatchesAllContracts`, `TestP15BRG32InstalledAIHarness`, and `TestAIEvaluationHarnessCannotChangeRequiredTiers`. The fresh-clone case actually clones and builds the CLI. The mapping case reconciles every ID, title and required tier across the94 original,52 repair and33 backend obligations and validates the complete serialized report.

The installation case obtains exactly four tracked harness files via Git, copies them outside the repository, hashes their bytes and executes all fifteen Python controls from that directory. It verifies unchanged files afterward. The controls cover malformed input, execution authorization, identity/usage, subprocess failures, timeout termination, secret-stderr handling, output replacement and validator binding. Fake adapters exercise control flow; no model quality is measured.

The shared RP50 helper retains its exact fifteen-test selection and two-minute subprocess budget. RP50 still requires external evaluation, and AI01 remains unexecuted without a real eligible endpoint, adapter and validator.

## Actual execution

| Attempt | Result | Scope |
|---|---|---|
| First precommit | Failed | New assertion compared Go nil and empty maps after JSON omitempty. Corrected to complete serialized JSON value equality; no contract/product weakening. |
| Second precommit | Failed before executing tests | Existing Go cache object archives were temporarily absent at link/vet time. No pass credit. |
| Third precommit | Passed | Same bounded command after a read-only check found the archives restored. |
| Clean implementation replay | Exit0,119.688s;16 named pass events,no failure/skip | Actual BRG32 producer plus eight corruption controls; RP50 producer plus six controls. |
| Direct installed capture | Exit0,17.984s;two parents and all15 Python controls passed | Complete actual discovery/execution receipts and installed-file hashes. |

Both clean executions recorded the exact implementation SHA and a clean tree afterward. Clean production JSONL SHA-256: `aa2b1093dc1a4cd6d81507442a2cd67eb1465c6333fba91240db8096e3c198af`. Installed JSONL SHA-256: `a8b6a958f2777f1e87ef2f022732962d1ca808307668028df507f7ee06d15aff`.

Go's JSON stream splits long output across events. The installed summary joins the complete output before parsing both JSON markers; unmodified raw events remain authoritative.

## Strict controls and limits

Eight BRG32 mutations reject partial command selection, wrong producer, missing installation scope, invented AI quality, missing Python discovery, borrowed AI01 identity, missing repository tier and foreign SHA. Six RP50 controls separately reject invented evaluation, borrowed AI01, wrong command, missing tests, quality claims and stale SHA.

No external model, device, Apple runtime, deployment, final CI or full179 closure is claimed. All seven public raw logs/summaries, including unsuccessful attempts, are under [evidence/p15-brg32-20261119](evidence/p15-brg32-20261119). `retained-files.json` records exact byte counts/digests; `.gitattributes` preserves raw JSON bytes.
