# Cherri P10R5 — Complete Negative Mutation Matrix (`41` Tests)

> **Verification Standard**: Every audit finding and corrective mechanism in P10R5 must be proven by at least one authentic negative mutation test that intentionally injects corruption, verifies that the detection mechanism catches it, and confirms fail-closed behavior.

- **Total Negative Mutation Tests**: `41` (`41` verified)
- **Part A6 (Native Specimen Parity)**: `9` tests
- **Part B6 (Parameter Contract Matrix)**: `9` tests
- **Part C5 (Ledger & Crosswalk Consistency)**: `7` tests
- **Part D4 (Canonical Structural Comparator)**: `8` tests
- **Part E8 (Consumer Drift Detection)**: `8` tests
- **Overall Status**: `ALL_PASS` (100% fail-closed verification)

## Complete 41-Mutation Evidence Matrix

| # | Mutation ID | Target Area | Mutated Property | Expected Detection Mechanism | Actual Result | Status |
| :---: | :--- | :--- | :--- | :--- | :--- | :---: |
| 1 | `A6_1` | Part A — Native Specimen Parity | WFWorkflowActionParameters.WFInput replaced by wrong string value while action identifier matches | EvaluateNativeSpecimen performs deep canonical comparison and returns NATIVE_SPECIMEN_PARITY_FAIL | `NATIVE_SPECIMEN_PARITY_FAIL` | `PASS` |
| 2 | `A6_2` | Part A — Native Specimen Parity | Native parameter key 'WFFilename' replaced by 'WFTitle' | EvaluateNativeSpecimen detects key divergence and returns NATIVE_SPECIMEN_PARITY_FAIL | `NATIVE_SPECIMEN_PARITY_FAIL` | `PASS` |
| 3 | `A6_3` | Part A — Native Specimen Parity | Parameter value 'Ascending' mutated to 'Descending' | EvaluateNativeSpecimen detects value inequality and returns NATIVE_SPECIMEN_PARITY_FAIL | `NATIVE_SPECIMEN_PARITY_FAIL` | `PASS` |
| 4 | `A6_4` | Part A — Native Specimen Parity | Numeric parameter 123 mutated to string '123' | EvaluateNativeSpecimen type check fails; returns NATIVE_SPECIMEN_PARITY_FAIL | `NATIVE_SPECIMEN_PARITY_FAIL` | `PASS` |
| 5 | `A6_5` | Part A — Native Specimen Parity | Nested dictionary key mutated inside WFDictionaryFieldValueItems | Canonical recursive dictionary comparison catches inner key mismatch | `NATIVE_SPECIMEN_PARITY_FAIL` | `PASS` |
| 6 | `A6_6` | Part A — Native Specimen Parity | AppIntentDescriptor.BundleIdentifier mutated to 'com.apple.corrupted.bundle' | AppIntent descriptor field comparison catches bundle identifier mismatch | `NATIVE_SPECIMEN_PARITY_FAIL` | `PASS` |
| 7 | `A6_7` | Part A — Native Specimen Parity | OutputUUID link in WFInput changed to foreign UUID '11111111-2222-3333-4444-555555555555' | Variable dependency graph and UUID bijection check detects invalid reference | `NATIVE_SPECIMEN_PARITY_FAIL` | `PASS` |
| 8 | `A6_8` | Part A — Native Specimen Parity | Specimen metadata OSFamily set to 'visionOS' while platformAvailability is ['macOS'] | EvaluateNativeSpecimen returns SPECIMEN_PLATFORM_MISMATCH | `SPECIMEN_PLATFORM_MISMATCH` | `PASS` |
| 9 | `A6_9` | Part A — Native Specimen Parity | Missing specimen file, bad SHA-256 hash, and authoredManuallyInAppleShortcuts=false self-oracle | EvaluateNativeSpecimen returns SPECIMEN_MISSING and SPECIMEN_INVALID | `SPECIMEN_MISSING and SPECIMEN_INVALID` | `PASS` |
| 10 | `B6_1` | Part B — Parameter Contract Verification | Optional parameter allowsSnooze wire key corrupted to CORRUPTED_OPTIONAL_WIRE_KEY | verifyCatalogActionParameterContract OptionalParams check fails closed | `FAIL (OptionalParams FAIL, ContractPassed=false)` | `PASS` |
| 11 | `B6_2` | Part B — Parameter Contract Verification | Optional parameter marked as required in catalog metadata so omitting it compiles | verifyCatalogActionParameterContract MissingRequiredRejected check detects omission was accepted | `FAIL (MissingRequiredRejected FAIL, ContractPassed=false)` | `PASS` |
| 12 | `B6_3` | Part B — Parameter Contract Verification | Required parameter operation wire key mutated to WFStatisticsOperation_WRONG_WIRE_KEY | verifyCatalogActionParameterContract RequiredParams check fails closed on missing expected key | `FAIL (RequiredParams FAIL, ContractPassed=false)` | `PASS` |
| 13 | `B6_4` | Part B — Parameter Contract Verification | turnOnAlarm (state=1) vs turnOffAlarm (state=0) static parameter difference compared | CompareShortcutPlistsCanonical catches static default parameter divergence | `FAIL (plists are not equal, cmp.Equal == false)` | `PASS` |
| 14 | `B6_5` | Part B — Parameter Contract Verification | Unconstrained string parameter marked as enum in metadata so invalid value compiles | EnumValidation check fails closed when invalid value is accepted by compiler | `FAIL (EnumValidation FAIL, ContractPassed=false)` | `PASS` |
| 15 | `B6_6` | Part B — Parameter Contract Verification | Dynamic variable attachment replaced by plain literal string in plist | plistContainsDynamicReference returns false; DynamicReference check fails | `FAIL (DynamicReference lost detected)` | `PASS` |
| 16 | `B6_7` | Part B — Parameter Contract Verification | Literal-only flag set on parameter that actually accepts dynamic variable @dynVar | verifyCatalogActionParameterContract DynamicReference check fails closed | `FAIL (DynamicReference FAIL, ContractPassed=false)` | `PASS` |
| 17 | `B6_8` | Part B — Parameter Contract Verification | Universal action marked as macOnly in metadata so #define mac false still compiles | PlatformGate check fails closed when unenforced gate compiles on invalid target | `FAIL (PlatformGate FAIL, ContractPassed=false)` | `PASS` |
| 18 | `B6_9` | Part B — Parameter Contract Verification | Parameter contract result initialized with NOT_TESTED dimension | evaluateContractPassed strictly requires all dimensions to be PASS or N/A; never sets ContractPassed=true | `FAIL (ContractPassed=false, FailureReason reports optionalParams is NOT_TESTED)` | `PASS` |
| 19 | `C5_1` | Part C — Ledger & Crosswalk Single Truth | Crosswalk appleCoveredVerifiedIdentifiers tampered to 390 (while ledger has 0 COVERED_VERIFIED) | verify_ledger_consistency.py detects count mismatch between crosswalk and ledger | `FAIL (consistency check fails closed with descriptive error)` | `PASS` |
| 20 | `C5_2` | Part C — Ledger & Crosswalk Single Truth | p10-coverage-report.md table tampered to claim COVERED_VERIFIED = 389 | verify_ledger_consistency.py detects markdown table mismatch against authoritative ledger | `FAIL (consistency check fails closed on markdown report mismatch)` | `PASS` |
| 21 | `C5_3` | Part C — Ledger & Crosswalk Single Truth | Denominators tampered to double count 86 DEFERRED_NO_EVIDENCE records (2482 total / 1613 eligible) | verify_ledger_consistency.py detects denominator arithmetic violation | `FAIL (crosswalk appleRuntimeDiscoveredTotal=2482 != 2396; appleEligibleUserFacingTotal=1613 != 1527)` | `PASS` |
| 22 | `C5_4` | Part C — Ledger & Crosswalk Single Truth | Reconciliation formula cherriToAppleMappingEquation tampered with stale hardcoded '381 unique ShortcutIdentifiers' | verify_ledger_consistency.py verifies dynamically computed unique identifiers count (390) | `FAIL (formula contains stale hardcoded 381)` | `PASS` |
| 23 | `C5_5` | Part C — Ledger & Crosswalk Single Truth | Matrix entry inflated to NATIVE_SPECIMEN_VERIFIED while nativeSpecimenParityStatus is NEEDS_EVIDENCE | verify_ledger_consistency.py and schema validator reject inflated evidence tier | `FAIL (anti-inflation validator fails closed)` | `PASS` |
| 24 | `C5_6` | Part C — Ledger & Crosswalk Single Truth | apple-discovered-not-supported.md tampered to use pre-remediation '100 MISSING_HIGH_PRIORITY' | verify_ledger_consistency.py verifies authoritative 97 NEW_NATIVE_ACTION count | `FAIL (missing authoritative NEW_NATIVE_ACTION count)` | `PASS` |
| 25 | `C5_7` | Part C — Ledger & Crosswalk Single Truth | catalog-supported-by-compiler.md parameter dimension summary tampered from 201 to 505 eligible | verify_ledger_consistency.py compares parameter dimension summaries between matrix and report | `FAIL (dimension row mismatch caught)` | `PASS` |
| 26 | `D4_1` | Part D — Canonical Structural Comparator Modes | WFWorkflowTypes header mutated from ['ActionExtension', 'QuickActions'] to ['ActionExtension'] | CompareShortcutDocsFullWorkflowSemantic catches top-level header divergence | `FAIL (res.Equal == false in FullWorkflowSemantic mode)` | `PASS` |
| 27 | `D4_2` | Part D — Canonical Structural Comparator Modes | WFWorkflowMinimumClientVersion mutated from 900 to 1100 | CompareShortcutDocsFullWorkflowSemantic compares client version without stripping | `FAIL (res.Equal == false in FullWorkflowSemantic mode)` | `PASS` |
| 28 | `D4_3` | Part D — Canonical Structural Comparator Modes | WFWorkflowInputContentItemClasses mutated by appending 'WFImageContentItem' | CompareShortcutDocsFullWorkflowSemantic detects input content item classes divergence | `FAIL (res.Equal == false in FullWorkflowSemantic mode)` | `PASS` |
| 29 | `D4_4` | Part D — Canonical Structural Comparator Modes | WFWorkflowOutputContentItemClasses mutated by stripping 'WFStringContentItem' | CompareShortcutDocsFullWorkflowSemantic detects output classes divergence | `FAIL (res.Equal == false in FullWorkflowSemantic mode)` | `PASS` |
| 30 | `D4_5` | Part D — Canonical Structural Comparator Modes | WFWorkflowNoInputBehavior mutated from ShowError to GetClipboard | CompareShortcutDocsFullWorkflowSemantic detects NoInputBehavior divergence | `FAIL (res.Equal == false in FullWorkflowSemantic mode)` | `PASS` |
| 31 | `D4_6` | Part D — Canonical Structural Comparator Modes | WFWorkflowImportQuestions header deleted entirely from second document | CompareShortcutDocsFullWorkflowSemantic detects missing semantic header key | `FAIL (res.Equal == false in FullWorkflowSemantic mode)` | `PASS` |
| 32 | `D4_7` | Part D — Canonical Structural Comparator Modes | OutputUUID reference mutated inside action parameters | CompareShortcutDocsFullWorkflowSemantic detects variable reference divergence | `FAIL (res.Equal == false in FullWorkflowSemantic mode)` | `PASS` |
| 33 | `D4_8` | Part D — Canonical Structural Comparator Modes | GroupingIdentifier on conditional action mutated to non-matching UUID | CompareShortcutDocsFullWorkflowSemantic detects control flow grouping divergence | `FAIL (res.Equal == false in FullWorkflowSemantic mode)` | `PASS` |
| 34 | `E8_1` | Part E — Consumer Synchronization & Promotion Pipeline | Skill compatibility manifest schemaFingerprint corrupted to all-zeros | verify_consumer_compatibility.py detects schemaFingerprint divergence against compiler | `FAIL in Skill consumer` | `PASS` |
| 35 | `E8_2` | Part E — Consumer Synchronization & Promotion Pipeline | Skill compatibility manifest codecFingerprint corrupted to all-ones | verify_consumer_compatibility.py detects codecFingerprint divergence against compiler | `FAIL in Skill consumer` | `PASS` |
| 36 | `E8_3` | Part E — Consumer Synchronization & Promotion Pipeline | Documentation actions-reference.md schema fingerprint corrupted | verify_consumer_compatibility.py detects documentation schema fingerprint drift | `FAIL in Documentation consumer` | `PASS` |
| 37 | `E8_4` | Part E — Consumer Synchronization & Promotion Pipeline | Skill manifest compilerCommit set to foreign SHA 'deadbeefdeadbeef...' | verify_consumer_compatibility.py verifies git merge-base ancestry against compiler HEAD | `FAIL in Skill consumer` | `PASS` |
| 38 | `E8_5` | Part E — Consumer Synchronization & Promotion Pipeline | mobile_compiler.go tampered to use localHandMaintainedRegistry() instead of schema.DefaultRegistry() | verify_consumer_compatibility.py enforces SSoT registry consumption in bridge | `FAIL in iOS Bridge consumer` | `PASS` |
| 39 | `E8_6` | Part E — Consumer Synchronization & Promotion Pipeline | CherriCompiler.swift tampered with a hardcoded 64-hex fingerprint string literal | verify_consumer_compatibility.py enforces dynamic fingerprint decoding from bridge | `FAIL in iOS UI consumer` | `PASS` |
| 40 | `E8_7` | Part E — Consumer Synchronization & Promotion Pipeline | package-lock.json resolved preview-shortcut commit tampered to mismatch package.json pin | verify_consumer_compatibility.py enforces exact lockfile pin concordance | `FAIL in Preview consumer` | `PASS` |
| 41 | `E8_8` | Part E — Consumer Synchronization & Promotion Pipeline | Compiler capabilities actionCount tampered from 505 to 504 | verify_consumer_compatibility.py verifies compiler actionCount == 505 | `FAIL in Compiler consumer` | `PASS` |

## Execution Commands by Section

### Part A: Native Specimen Parity (9 mutations)
```bash
go test -run TestP10_NativeSpecimenParity_ValidatorAndNegativeMutations -v .
```

### Part B: Parameter Contract Matrix (9 mutations)
```bash
go test -run TestP10_ParameterContract_NegativeMutations -v .
```

### Part C: Ledger & Crosswalk Consistency (7 mutations)
```bash
python tools/p10r3/verify_ledger_consistency.py --mutate-check
```

### Part D: Canonical Structural Comparator Modes (8 mutations)
```bash
go test -run TestP10_StructuralComparator_NegativeMutations -v .
```

### Part E: Consumer Drift Detection (8 mutations)
```bash
python tools/p10r3/verify_consumer_compatibility.py --mutate-check
```
