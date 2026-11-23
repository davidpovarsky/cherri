# CI01 observation reference — causal metadata repair

## Original defect

The original R6 and K8 local179 audit files reference acceptance-local-observations.json#CI01 even though their observation files contain no CI01 row. CI01 is NOT_RUN / UNVERIFIED in both original reports. The independent [K8 intake](p15-local179-k8-intake-20261123.md) preserves that finding and the original reports unchanged.

The cause is in tools/language-acceptance/execution_audit.go: the incomplete-scope map caused the writer to create an observation link and an “Observed smoke” scope without checking the actual observation inventory. Meanwhile runNativeEncodingRequirement deliberately records CI01 as unexecuted; it neither runs a smoke nor creates an accepted evidence record.

## Narrow change

Source 1f619b6ef640e0a71d89558564eddb9ea6bdfbe3 indexes the actual retained observations before writing audit rows. A row receives an observation link only when the corresponding observation exists. Otherwise its scope explicitly says that no local smoke observation was recorded. CI01 keeps its NOT_RUN result, UNVERIFIED audit status, negative-ci requirement, original blocker, zero accepted tiers, and zero accepted records.

The only changed source files are execution_audit.go and execution_truth_test.go under tools/language-acceptance. No contract, threshold, native-encoding producer, immutable historical report, or full179 result changes.

## Actual causal confirmation

The same selected test, TestExecutionAuditOnlyReferencesRecordedSmoke, was executed once before and once after the repair using Go 1.24.13 on Linux/ARM64, with Go 120s / driver 180s budgets.

| Phase | Exact scope | Actual outcome |
|---|---|---|
| Red | Original K8-identical writer at e66db33f; only the new causal test was uncommitted | Exit 1 in 11.377s. The retained stream explicitly records CI01's nonexistent reference and fabricated observation scope. |
| Green | Clean fix source 1f619b6e; identical test file SHA256 | Exit 0 in 6.444s. Both subcases, their parent, and the package passed; zero skip, incomplete, orphan or timeout outcomes. |

The control invokes the real runNativeEncodingRequirement. It retains a different B01 smoke observation to reject a global “some observation exists” shortcut, then checks CI01 both without and with an actual retained smoke. It checks that every emitted observation link resolves, that real B01/CI01 smoke links survive, that all 179 audit IDs remain present, and that no formal evidence or closure is minted.

These are metadata regression controls. The real macOS REQUIRED encoding gate remains unexecuted for CI01 in the original reports. There was no full179 replay, private corpus replay, Apple execution, or release acceptance.

## Retained proof

- [Independent causal review](evidence/p15-ci01-observation-reference-20261123/review.json).
- [Original red process receipt](evidence/p15-ci01-observation-reference-20261123/red-receipt.original.json).
- [Original green process receipt](evidence/p15-ci01-observation-reference-20261123/green-receipt.original.json).

The complete original process streams remain in the durable Oracle evidence directory. The receipt digests bind both streams and the unchanged test file. Packaging used JSON parsing, exact receipt copies, source-file/hash checks and Git whitespace checks; no additional test was run.
