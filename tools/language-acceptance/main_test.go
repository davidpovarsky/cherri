package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func sampleTestContract() *ContractFile {
	return &ContractFile{
		CaseCount: 3,
		Cases: []CaseSpec{
			{ID: "C01", Group: "calls", Requirement: "Req 1", MinimumTestLevel: "unit"},
			{ID: "C02", Group: "calls", Requirement: "Req 2", MinimumTestLevel: "native"},
			{ID: "C03", Group: "calls", Requirement: "Req 3", MinimumTestLevel: "iOS"},
		},
	}
}

func sampleContractsSet() *ContractsSet {
	return &ContractsSet{
		Acceptance: sampleTestContract(),
		Repair: &RepairContractFile{
			CaseCount: 1,
			Cases: []RepairCaseSpec{
				{ID: "RP01", Title: "Repair 1", RequiredTestLevels: []string{"unit"}},
			},
		},
		Gates: &GatesContractFile{
			GateCount: 1,
			Gates: []RecoveryGate{
				{ID: "BRG01", Title: "Gate 1", Mandatory: true, MinimumEvidenceTiers: []string{"repository"}},
			},
		},
	}
}

// 1. One actual assertion returns false -> verifier rejects
func TestMeta_AssertionReturnsFalse(t *testing.T) {
	contract := sampleTestContract()
	results := map[string]CaseResult{
		"C01": {ID: "C01", ExecutedLevels: []string{"unit"}, Passed: true, Status: "PASSED"},
		"C02": {ID: "C02", ExecutedLevels: []string{"native"}, Passed: false, Status: "FAILED", Detail: "assertion returned false"},
		"C03": {ID: "C03", ExecutedLevels: []string{"ios-runtime"}, Passed: true, Status: "PASSED"},
	}
	report, err := ValidateAndAggregate(contract, results, "test_fp")
	if err == nil {
		t.Fatalf("expected validation error when a case fails, got nil")
	}
	if report.FailedCases != 1 {
		t.Fatalf("expected failed cases == 1 when assertion fails, got %d", report.FailedCases)
	}
	if report.PassedCases != 2 {
		t.Fatalf("expected passed cases == 2, got %d", report.PassedCases)
	}
}

// 2. Result says success but includes a false boolean or vice versa -> verifier rejects
func TestMeta_ContradictoryPassedStatus(t *testing.T) {
	contract := sampleTestContract()
	results := map[string]CaseResult{
		"C01": {ID: "C01", ExecutedLevels: []string{"unit"}, Passed: false, Status: "PASSED", Detail: "contradictory"},
		"C02": {ID: "C02", ExecutedLevels: []string{"native"}, Passed: true, Status: "PASSED"},
		"C03": {ID: "C03", ExecutedLevels: []string{"ios-runtime"}, Passed: true, Status: "PASSED"},
	}
	report, err := ValidateAndAggregate(contract, results, "test_fp")
	if err == nil {
		t.Fatalf("expected error on contradictory passed=false and status=PASSED, got nil")
	}
	if report.FailedCases != 1 {
		t.Fatalf("expected 1 failed case for contradictory status, got %d", report.FailedCases)
	}
}

// 3. One required ID is absent -> verifier rejects
func TestMeta_MissingRequiredID(t *testing.T) {
	contract := sampleTestContract()
	results := map[string]CaseResult{
		"C01": {ID: "C01", ExecutedLevels: []string{"unit"}, Passed: true, Status: "PASSED"},
		"C02": {ID: "C02", ExecutedLevels: []string{"native"}, Passed: true, Status: "PASSED"},
		// C03 is missing
	}
	report, err := ValidateAndAggregate(contract, results, "test_fp")
	if err == nil {
		t.Fatalf("expected error when required ID is missing, got nil")
	}
	if report.FailedCases != 1 {
		t.Fatalf("expected 1 failed case for missing ID, got %d", report.FailedCases)
	}
}

// 4. Unknown IDs injected into results -> verifier rejects
func TestMeta_UnknownIDInjected(t *testing.T) {
	contract := sampleTestContract()
	results := map[string]CaseResult{
		"C01":     {ID: "C01", ExecutedLevels: []string{"unit"}, Passed: true, Status: "PASSED"},
		"C02":     {ID: "C02", ExecutedLevels: []string{"native"}, Passed: true, Status: "PASSED"},
		"C03":     {ID: "C03", ExecutedLevels: []string{"ios-runtime"}, Passed: true, Status: "PASSED"},
		"UNKNOWN": {ID: "UNKNOWN", ExecutedLevels: []string{"unit"}, Passed: true, Status: "PASSED"},
	}
	_, err := ValidateAndAggregate(contract, results, "test_fp")
	if err == nil {
		t.Fatalf("expected error when unknown ID is injected, got nil")
	}
}

// 5. Contract hash mismatch or modification -> verifier rejects
func TestMeta_ContractHashMismatch(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "fake_contract.json")
	if err := os.WriteFile(tmpFile, []byte(`{"case_count": 0, "cases": []}`), 0644); err != nil {
		t.Fatal(err)
	}
	_, _, err := ResolveAndLoadContract(tmpFile)
	if err == nil {
		t.Fatalf("expected error loading modified acceptance contract, got nil")
	}
	_, _, err = ResolveAndLoadRepairContract(tmpFile)
	if err == nil {
		t.Fatalf("expected error loading modified repair contract, got nil")
	}
	_, _, err = ResolveAndLoadGatesContract(tmpFile)
	if err == nil {
		t.Fatalf("expected error loading modified gates contract, got nil")
	}
}

// 6. Selected test matches zero tests / empty test_id in manifest -> verifier rejects
func TestMeta_ZeroMatchingTestsDetected(t *testing.T) {
	contracts := sampleContractsSet()
	manifest := &EvidenceManifest{
		SchemaVersion:     "1",
		ImplementationSHA: "commit123",
		SchemaFingerprint: "fp123",
		Records: []EvidenceRecord{
			{
				EvidenceID:        "ev1",
				Requirements:      []string{"C01"},
				Tier:              "unit",
				ImplementationSHA: "commit123",
				TestID:            "", // Zero matching tests!
				ExitCode:          0,
				Result:            "passed",
			},
		},
	}
	_, err := ValidateEvidenceManifest(manifest, contracts, "local")
	if err == nil {
		t.Fatalf("expected error when record has empty test_id (zero matching tests), got nil")
	}
}

// 7. Only parsing ran (or ios-build instead of ios-runtime) -> reject as incomplete
func TestMeta_WeakTestLevelRejection(t *testing.T) {
	contract := sampleTestContract() // C03 requires "iOS" -> ios-runtime
	results := map[string]CaseResult{
		"C01": {ID: "C01", ExecutedLevels: []string{"unit"}, Passed: true, Status: "PASSED"},
		"C02": {ID: "C02", ExecutedLevels: []string{"native-structure"}, Passed: true, Status: "PASSED"},
		// C03 executed only "ios-build" instead of "ios-runtime"
		"C03": {ID: "C03", ExecutedLevels: []string{"ios-build"}, Passed: true, Status: "PASSED"},
	}
	report, err := ValidateAndAggregate(contract, results, "test_fp")
	if err == nil {
		t.Fatalf("expected validation error when ios-build is offered for ios requirement, got nil")
	}
	if report.FailedCases != 1 {
		t.Fatalf("expected C03 to fail tier check, got FailedCases=%d", report.FailedCases)
	}
}

// 8. Result evidence belongs to a different implementation commit SHA -> verifier rejects
func TestMeta_MismatchedEvidenceDigest(t *testing.T) {
	contracts := sampleContractsSet()
	manifest := &EvidenceManifest{
		SchemaVersion:     "1",
		ImplementationSHA: "expected-commit-sha-456",
		SchemaFingerprint: "fp123",
		Records: []EvidenceRecord{
			{
				EvidenceID:        "ev1",
				Requirements:      []string{"C01"},
				Tier:              "unit",
				ImplementationSHA: "stale-commit-sha-789", // Stale SHA!
				TestID:            "TestUnit",
				ExitCode:          0,
				Result:            "passed",
			},
		},
	}
	_, err := ValidateEvidenceManifest(manifest, contracts, "local")
	if err == nil {
		t.Fatalf("expected error on mismatched implementation_sha in record, got nil")
	}
}

// 9. Referenced artifact file is missing -> verifier rejects
func TestMeta_MissingReferencedArtifact(t *testing.T) {
	contracts := sampleContractsSet()
	missingPath := filepath.Join(t.TempDir(), "nonexistent_file.shortcut")
	manifest := &EvidenceManifest{
		SchemaVersion:     "1",
		ImplementationSHA: "commit123",
		SchemaFingerprint: "fp123",
		Records: []EvidenceRecord{
			{
				EvidenceID:        "ev1",
				Requirements:      []string{"C01"},
				Tier:              "unit",
				ImplementationSHA: "commit123",
				TestID:            "TestUnit",
				ExitCode:          0,
				Result:            "passed",
				Artifacts: []EvidenceArtifact{
					{Path: missingPath, SHA256: "somehash"},
				},
			},
		},
	}
	_, err := ValidateEvidenceManifest(manifest, contracts, "local")
	if err == nil {
		t.Fatalf("expected error on nonexistent referenced artifact, got nil")
	}
}

// 10. Referenced artifact has wrong digest -> verifier rejects
func TestMeta_ArtifactHashMismatch(t *testing.T) {
	contracts := sampleContractsSet()
	dir := t.TempDir()
	artFile := filepath.Join(dir, "artifact.shortcut")
	if err := os.WriteFile(artFile, []byte("real content"), 0644); err != nil {
		t.Fatal(err)
	}
	manifest := &EvidenceManifest{
		SchemaVersion:     "1",
		ImplementationSHA: "commit123",
		SchemaFingerprint: "fp123",
		Records: []EvidenceRecord{
			{
				EvidenceID:        "ev1",
				Requirements:      []string{"C01"},
				Tier:              "unit",
				ImplementationSHA: "commit123",
				TestID:            "TestUnit",
				ExitCode:          0,
				Result:            "passed",
				Artifacts: []EvidenceArtifact{
					{Path: artFile, SHA256: "corrupted_hash_that_does_not_match"},
				},
			},
		},
	}
	_, err := ValidateEvidenceManifest(manifest, contracts, "local")
	if err == nil {
		t.Fatalf("expected error on artifact SHA256 mismatch, got nil")
	}
}

// 11. Required CI run is pending, cancelled, or failed -> verifier rejects
func TestMeta_FailedOrPendingCIRejection(t *testing.T) {
	contracts := sampleContractsSet()
	for _, conclusion := range []string{"failure", "cancelled", "skipped", "timed_out"} {
		manifest := &EvidenceManifest{
			SchemaVersion:     "1",
			ImplementationSHA: "commit123",
			SchemaFingerprint: "fp123",
			Records: []EvidenceRecord{
				{
					EvidenceID:        "ev1",
					Requirements:      []string{"C01"},
					Tier:              "unit",
					ImplementationSHA: "commit123",
					TestID:            "TestUnit",
					ExitCode:          0,
					Result:            "passed",
				},
			},
			Runs: []CIRunRecord{
				{
					RunID:      "run-123",
					HeadSHA:    "commit123",
					Conclusion: conclusion,
				},
			},
		}
		_, err := ValidateEvidenceManifest(manifest, contracts, "local")
		if err == nil {
			t.Fatalf("expected error for CI conclusion %q, got nil", conclusion)
		}
	}
}

// 12. Duplicate evidence ID injected -> verifier rejects
func TestMeta_DuplicateEvidenceID(t *testing.T) {
	contracts := sampleContractsSet()
	manifest := &EvidenceManifest{
		SchemaVersion:     "1",
		ImplementationSHA: "commit123",
		SchemaFingerprint: "fp123",
		Records: []EvidenceRecord{
			{
				EvidenceID:        "DUP01",
				Requirements:      []string{"C01"},
				Tier:              "unit",
				ImplementationSHA: "commit123",
				TestID:            "TestUnit",
				ExitCode:          0,
				Result:            "passed",
			},
			{
				EvidenceID:        "DUP01", // Duplicate ID!
				Requirements:      []string{"C02"},
				Tier:              "native-structure",
				ImplementationSHA: "commit123",
				TestID:            "TestNative",
				ExitCode:          0,
				Result:            "passed",
			},
		},
	}
	_, err := ValidateEvidenceManifest(manifest, contracts, "local")
	if err == nil {
		t.Fatalf("expected error for duplicate evidence_id, got nil")
	}
}

// 13. Real contract files load and verify their immutable digests
func TestRealContractsLoadAndVerifyDigests(t *testing.T) {
	contracts, err := LoadAllContracts("", "", "")
	if err != nil {
		t.Fatalf("failed loading real contracts: %v", err)
	}

	if contracts.Acceptance.CaseCount != 94 || len(contracts.Acceptance.Cases) != 94 {
		t.Fatalf("expected 94 acceptance cases, got declared=%d, cases=%d",
			contracts.Acceptance.CaseCount, len(contracts.Acceptance.Cases))
	}
	if contracts.Repair.CaseCount != 52 || len(contracts.Repair.Cases) != 52 {
		t.Fatalf("expected 52 repair cases, got declared=%d, cases=%d",
			contracts.Repair.CaseCount, len(contracts.Repair.Cases))
	}
	if contracts.Gates.GateCount != 33 || len(contracts.Gates.Gates) != 33 {
		t.Fatalf("expected 33 recovery gates, got declared=%d, gates=%d",
			contracts.Gates.GateCount, len(contracts.Gates.Gates))
	}

	// Verify verified artifact SHA256 matches
	dir := t.TempDir()
	testFile := filepath.Join(dir, "test.txt")
	os.WriteFile(testFile, []byte("ok"), 0644)
	h := sha256.Sum256([]byte("ok"))
	expHex := hex.EncodeToString(h[:])

	manifest := &EvidenceManifest{
		SchemaVersion:     "1",
		ImplementationSHA: "sha1",
		SchemaFingerprint: "fp1",
		Records: []EvidenceRecord{
			{
				EvidenceID:        "rec1",
				Requirements:      []string{"C01"},
				Tier:              "unit",
				ImplementationSHA: "sha1",
				TestID:            "TestC01",
				ExitCode:          0,
				Result:            "passed",
				Artifacts: []EvidenceArtifact{
					{Path: testFile, SHA256: expHex},
				},
			},
		},
	}
	testContracts := &ContractsSet{
		Acceptance: &ContractFile{
			CaseCount: 1,
			Cases:     []CaseSpec{{ID: "C01", MinimumTestLevel: "unit"}},
		},
	}
	report, err := ValidateEvidenceManifest(manifest, testContracts, "local")
	if err != nil {
		t.Fatalf("unexpected validation error on valid evidence: %v", err)
	}
	if report.PassedCases != 1 {
		t.Fatalf("expected 1 passed case, got %d", report.PassedCases)
	}
}

// 14. Requirements-evidence map covers every single acceptance, repair, and gate requirement
func TestRequirementsEvidenceMap_CompleteCoverage(t *testing.T) {
	contracts, err := LoadAllContracts("", "", "")
	if err != nil {
		t.Fatalf("could not load real contracts: %v", err)
	}
	doc, err := BuildRequirementsEvidenceMap(contracts)
	if err != nil {
		t.Fatalf("failed to build requirements-evidence map: %v", err)
	}

	if doc.AcceptanceCount != 94 {
		t.Errorf("expected 94 acceptance cases, got %d", doc.AcceptanceCount)
	}
	if doc.RepairCount != 52 {
		t.Errorf("expected 52 repair cases, got %d", doc.RepairCount)
	}
	if doc.GatesCount != 33 {
		t.Errorf("expected 33 recovery gates, got %d", doc.GatesCount)
	}
	expectedTotal := 94 + 52 + 33
	if doc.TotalRequirements != expectedTotal {
		t.Errorf("expected %d total requirements, got %d", expectedTotal, doc.TotalRequirements)
	}
	if len(doc.Records) != expectedTotal {
		t.Errorf("expected %d records in list, got %d", expectedTotal, len(doc.Records))
	}

	seenIDs := make(map[string]bool)
	for _, rec := range doc.Records {
		if seenIDs[rec.ID] {
			t.Errorf("duplicate ID in records: %s", rec.ID)
		}
		seenIDs[rec.ID] = true

		if len(rec.RequiredTiers) == 0 {
			t.Errorf("requirement %s has no required tiers", rec.ID)
		}
		if len(rec.TestIDs) == 0 {
			t.Errorf("requirement %s has no mapped test IDs", rec.ID)
		}
		if len(rec.Assertions) == 0 {
			t.Errorf("requirement %s has no assertions attached", rec.ID)
		}
		if len(rec.FixtureIdentities) == 0 {
			t.Errorf("requirement %s has no fixture identities", rec.ID)
		}
	}

	// Verify all acceptance IDs are present
	for _, c := range contracts.Acceptance.Cases {
		if _, ok := doc.Mappings[c.ID]; !ok {
			t.Errorf("missing acceptance requirement ID in mappings: %s", c.ID)
		}
	}
	// Verify all repair IDs are present
	for _, c := range contracts.Repair.Cases {
		if _, ok := doc.Mappings[c.ID]; !ok {
			t.Errorf("missing repair requirement ID in mappings: %s", c.ID)
		}
	}
	// Verify all gate IDs are present
	for _, g := range contracts.Gates.Gates {
		if _, ok := doc.Mappings[g.ID]; !ok {
			t.Errorf("missing recovery gate ID in mappings: %s", g.ID)
		}
	}

	tmpFile := filepath.Join(t.TempDir(), "requirements-evidence-map.json")
	if err := WriteRequirementsEvidenceMap(doc, tmpFile); err != nil {
		t.Fatalf("failed to write requirements-evidence map: %v", err)
	}
	readBytes, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("failed to read back requirements-evidence map: %v", err)
	}
	var roundTrip RequirementsEvidenceMapFile
	if err := json.Unmarshal(readBytes, &roundTrip); err != nil {
		t.Fatalf("failed to unmarshal written map: %v", err)
	}
	if roundTrip.TotalRequirements != expectedTotal {
		t.Errorf("roundtrip total mismatch: %d vs %d", roundTrip.TotalRequirements, expectedTotal)
	}
}

// 15. A requirement requiring unit+native+iOS cannot become final PASS with only unit/native evidence
func TestMeta_UnitAndNativeCannotSatisfyIOSInFinal(t *testing.T) {
	contract := &ContractFile{
		CaseCount: 1,
		Cases: []CaseSpec{
			{ID: "FN01", Group: "functions", Requirement: "AST function call", MinimumTestLevel: "unit+native+iOS"},
		},
	}

	// 1. In local runner with only unit+native executed:
	localResults := map[string]CaseResult{
		"FN01": {
			ID:             "FN01",
			ExecutedLevels: []string{"unit", "native"},
			Passed:         true,
			Status:         "PENDING_EXTERNAL",
		},
	}
	localReport, err := ValidateAndAggregateWithPhase(contract, localResults, "fp1", "local")
	if err != nil {
		t.Fatalf("unexpected validation error in local phase: %v", err)
	}
	if localReport.PassedCases != 0 {
		t.Fatalf("local phase must NOT mark FN01 as passed without iOS evidence, got passed=%d", localReport.PassedCases)
	}
	if localReport.SkippedCases != 1 {
		t.Fatalf("local phase should record FN01 as pending/skipped, got skipped=%d", localReport.SkippedCases)
	}
	if localReport.Results[0].Status != "PENDING_EXTERNAL" {
		t.Fatalf("expected PENDING_EXTERNAL status for FN01 in local phase, got %s", localReport.Results[0].Status)
	}

	// 2. In final phase with only unit+native executed:
	finalReport, err := ValidateAndAggregateWithPhase(contract, localResults, "fp1", "final")
	if err == nil {
		t.Fatalf("expected final phase to reject FN01 without iOS evidence, got nil err")
	}
	if finalReport.FailedCases != 1 {
		t.Fatalf("expected FN01 to fail in final phase without iOS evidence, got failed=%d", finalReport.FailedCases)
	}
	if finalReport.ClosureComplete {
		t.Fatalf("closure_complete must be false when mandatory tier is missing")
	}

	// 3. In manifest validation: only unit and native records provided for FN01 -> rejected in final phase
	manifest := &EvidenceManifest{
		SchemaVersion:     "1",
		ImplementationSHA: "sha1",
		SchemaFingerprint: "fp1",
		Runs: []CIRunRecord{
			{RunID: "101", Repository: "davidpovarsky/cherri", HeadSHA: "sha1", Conclusion: "success", WorkflowName: "Build & Test"},
			{RunID: "102", Repository: "davidpovarsky/cherri", HeadSHA: "sha1", Conclusion: "success", WorkflowName: "OpenMinis Skill"},
			{RunID: "103", Repository: "davidpovarsky/cherri", HeadSHA: "sha1", Conclusion: "success", WorkflowName: "iOS Build"},
			{RunID: "104", Repository: "davidpovarsky/cherri", HeadSHA: "sha1", Conclusion: "success", WorkflowName: "iOS 27 Shortcuts Runtime PoC"},
		},
		Records: []EvidenceRecord{
			{
				EvidenceID:        "rec1",
				Requirements:      []string{"FN01"},
				Tier:              "unit",
				ImplementationSHA: "sha1",
				TestID:            "tools/language-acceptance:runFunctions",
				ExitCode:          0,
				Result:            "passed",
			},
			{
				EvidenceID:        "rec2",
				Requirements:      []string{"FN01"},
				Tier:              "native-structure",
				ImplementationSHA: "sha1",
				TestID:            "tools/language-acceptance:runFunctions",
				ExitCode:          0,
				Result:            "passed",
			},
		},
	}
	contractsSet := &ContractsSet{
		Acceptance: contract,
		Repair:     &RepairContractFile{},
		Gates:      &GatesContractFile{},
	}
	manifestReport, err := ValidateEvidenceManifest(manifest, contractsSet, "final")
	if err == nil {
		t.Fatalf("expected ValidateEvidenceManifest to reject FN01 missing ios tier in final phase, got nil err")
	}
	if manifestReport.FailedCases != 1 {
		t.Fatalf("expected 1 failed case in manifest validation, got %d", manifestReport.FailedCases)
	}
	if manifestReport.ClosureComplete {
		t.Fatalf("closure_complete must be false when ios tier missing in manifest")
	}

	// 4. In manifest validation: add genuine ios-runtime record -> succeeds!
	manifest.Records = append(manifest.Records, EvidenceRecord{
		EvidenceID:        "rec3",
		Requirements:      []string{"FN01"},
		Tier:              "ios-runtime",
		ImplementationSHA: "sha1",
		TestID:            "ios27-runtime-poc:functions",
		ExitCode:          0,
		Result:            "passed",
		Assertions: map[string]string{
			"function_result": "21",
		},
	})
	fullReport, err := ValidateEvidenceManifest(manifest, contractsSet, "final")
	if err != nil {
		t.Fatalf("expected full evidence to pass all tiers, got error: %v", err)
	}
	if fullReport.PassedCases != 1 {
		t.Fatalf("expected 1 passed case with full tiers, got %d", fullReport.PassedCases)
	}
	if !fullReport.ClosureComplete {
		t.Fatalf("expected closure_complete=true with all tiers satisfied")
	}
}

// 16. Claiming PASSED when required tiers are missing is strictly rejected
func TestMeta_FalsePassedClaimWithMissingTiersRejected(t *testing.T) {
	contract := &ContractFile{
		CaseCount: 1,
		Cases: []CaseSpec{
			{ID: "F01", Group: "flow", Requirement: "Branch condition", MinimumTestLevel: "unit+native+iOS"},
		},
	}
	results := map[string]CaseResult{
		"F01": {
			ID:             "F01",
			ExecutedLevels: []string{"unit", "native"},
			Passed:         true,
			Status:         "PASSED", // Claiming PASSED without iOS
		},
	}
	// In local phase:
	reportLocal, errLocal := ValidateAndAggregateWithPhase(contract, results, "fp1", "local")
	if errLocal == nil {
		t.Fatalf("expected validation error when case claims PASSED without satisfying required tiers in local phase, got nil")
	}
	if reportLocal.FailedCases != 1 {
		t.Fatalf("expected 1 failed case for false PASSED claim in local phase, got %d", reportLocal.FailedCases)
	}

	// In final phase:
	reportFinal, errFinal := ValidateAndAggregateWithPhase(contract, results, "fp1", "final")
	if errFinal == nil {
		t.Fatalf("expected validation error when case claims PASSED without satisfying required tiers in final phase, got nil")
	}
	if reportFinal.FailedCases != 1 {
		t.Fatalf("expected 1 failed case for false PASSED claim in final phase, got %d", reportFinal.FailedCases)
	}
}

func baseValidManifestAndContracts(t *testing.T) (*EvidenceManifest, *ContractsSet) {
	t.Helper()
	contracts := &ContractsSet{
		Acceptance: &ContractFile{
			CaseCount: 1,
			Cases: []CaseSpec{
				{ID: "FN01", Group: "functions", Requirement: "Declare function", MinimumTestLevel: "unit+native+iOS"},
			},
		},
		Repair: &RepairContractFile{
			CaseCount: 1,
			Cases: []RepairCaseSpec{
				{ID: "RP01", Title: "Repair 1", RequiredTestLevels: []string{"unit"}},
			},
		},
		Gates: &GatesContractFile{
			GateCount: 1,
			Gates: []RecoveryGate{
				{ID: "BRG01", Title: "Gate 1", Mandatory: true, MinimumEvidenceTiers: []string{"repository"}},
			},
		},
	}

	manifest := &EvidenceManifest{
		SchemaVersion:     "1",
		ImplementationSHA: "commit123456789012345678901234567890123",
		SchemaFingerprint: "fp1",
		Runs: []CIRunRecord{
			{RunID: "101", Repository: "davidpovarsky/cherri", HeadSHA: "commit123456789012345678901234567890123", Conclusion: "success", WorkflowName: "Build & Test"},
			{RunID: "102", Repository: "davidpovarsky/cherri", HeadSHA: "commit123456789012345678901234567890123", Conclusion: "success", WorkflowName: "OpenMinis Skill"},
			{RunID: "103", Repository: "davidpovarsky/cherri", HeadSHA: "commit123456789012345678901234567890123", Conclusion: "success", WorkflowName: "iOS Build"},
			{RunID: "104", Repository: "davidpovarsky/cherri", HeadSHA: "commit123456789012345678901234567890123", Conclusion: "success", WorkflowName: "iOS 27 Shortcuts Runtime PoC"},
		},
		Records: []EvidenceRecord{
			{
				EvidenceID:        "ev-fn01-unit",
				Requirements:      []string{"FN01"},
				Tier:              "unit",
				TestID:            "tools/language-acceptance:runFunctions",
				ImplementationSHA: "commit123456789012345678901234567890123",
				ExitCode:          0,
				Result:            "passed",
			},
			{
				EvidenceID:        "ev-fn01-native",
				Requirements:      []string{"FN01"},
				Tier:              "native-structure",
				TestID:            "tools/language-acceptance:runFunctions",
				ImplementationSHA: "commit123456789012345678901234567890123",
				ExitCode:          0,
				Result:            "passed",
			},
			{
				EvidenceID:        "ev-fn01-runtime",
				Requirements:      []string{"FN01"},
				Tier:              "ios-runtime",
				TestID:            "ios27-runtime-poc:functions",
				ImplementationSHA: "commit123456789012345678901234567890123",
				ExitCode:          0,
				Result:            "passed",
				Assertions: map[string]string{
					"function_result": "21",
				},
			},
			{
				EvidenceID:        "ev-rp01-unit",
				Requirements:      []string{"RP01"},
				Tier:              "unit",
				TestID:            "github.com/electrikmilk/cherri/tools/language-acceptance:TestMeta_ContradictoryPassedStatus",
				ImplementationSHA: "commit123456789012345678901234567890123",
				ExitCode:          0,
				Result:            "passed",
			},
			{
				EvidenceID:        "ev-brg01-repo",
				Requirements:      []string{"BRG01"},
				Tier:              "repository",
				TestID:            "scripts/verify-backend-recovery.sh",
				ImplementationSHA: "commit123456789012345678901234567890123",
				ExitCode:          0,
				Result:            "passed",
			},
		},
	}
	return manifest, contracts
}

// 1. Missing runtime record fails closed
func TestMeta_Negative01_MissingRuntimeRecordFailsClosed(t *testing.T) {
	manifest, contracts := baseValidManifestAndContracts(t)
	// Filter out the runtime record
	var filtered []EvidenceRecord
	for _, r := range manifest.Records {
		if r.Tier != "ios-runtime" {
			filtered = append(filtered, r)
		}
	}
	manifest.Records = filtered

	report, err := ValidateEvidenceManifest(manifest, contracts, "final")
	if err == nil && report.ClosureComplete {
		t.Fatalf("expected missing runtime record to fail closed, but closure succeeded")
	}
	if report.FailedCases == 0 {
		t.Fatalf("expected failed cases > 0 when runtime record missing, got %d", report.FailedCases)
	}
}

// 2. Wrong runtime TestID fails closed
func TestMeta_Negative02_WrongRuntimeTestIDFailsClosed(t *testing.T) {
	manifest, contracts := baseValidManifestAndContracts(t)
	for i := range manifest.Records {
		if manifest.Records[i].Tier == "ios-runtime" {
			manifest.Records[i].TestID = "unauthorized-test-id:fake"
		}
	}

	report, err := ValidateEvidenceManifest(manifest, contracts, "final")
	if err == nil && report.ClosureComplete {
		t.Fatalf("expected wrong runtime TestID to fail closed, but closure succeeded")
	}
}

// 3. Required assertion identity absent fails closed
func TestMeta_Negative03_RequiredAssertionIdentityAbsentFailsClosed(t *testing.T) {
	manifest, contracts := baseValidManifestAndContracts(t)
	for i := range manifest.Records {
		if manifest.Records[i].Tier == "ios-runtime" {
			manifest.Records[i].Assertions = map[string]string{} // Missing function_result
		}
	}

	report, err := ValidateEvidenceManifest(manifest, contracts, "final")
	if err == nil && report.ClosureComplete {
		t.Fatalf("expected absent required assertion to fail closed, but closure succeeded")
	}
}

// 4. CI SHA differs fails closed
func TestMeta_Negative04_CIShaDiffersFailsClosed(t *testing.T) {
	manifest, contracts := baseValidManifestAndContracts(t)
	manifest.Runs[0].HeadSHA = "different-sha-9999999999999999999999999"

	report, err := ValidateEvidenceManifest(manifest, contracts, "final")
	if err == nil && report.ClosureComplete {
		t.Fatalf("expected mismatched CI HeadSHA to fail closed, but closure succeeded")
	}
}

// 5. CI run does not exist fails closed
func TestMeta_Negative05_CIRunDoesNotExistFailsClosed(t *testing.T) {
	manifest, contracts := baseValidManifestAndContracts(t)
	// Remove required iOS 27 Shortcuts Runtime PoC run
	manifest.Runs = manifest.Runs[:3]

	report, err := ValidateEvidenceManifest(manifest, contracts, "final")
	if err == nil && report.ClosureComplete {
		t.Fatalf("expected missing required CI workflow run to fail closed, but closure succeeded")
	}
}

// 6. CI run failed fails closed
func TestMeta_Negative06_CIRunFailedFailsClosed(t *testing.T) {
	manifest, contracts := baseValidManifestAndContracts(t)
	manifest.Runs[0].Conclusion = "failure"

	report, err := ValidateEvidenceManifest(manifest, contracts, "final")
	if err == nil && report.ClosureComplete {
		t.Fatalf("expected failed CI run to fail closed, but closure succeeded")
	}
}

// 7. Artifact hash differs fails closed
func TestMeta_Negative07_ArtifactHashDiffersFailsClosed(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "test-artifact.txt")
	_ = os.WriteFile(tmpFile, []byte("genuine content"), 0644)

	manifest, contracts := baseValidManifestAndContracts(t)
	manifest.Records[0].Artifacts = []EvidenceArtifact{
		{
			Path:   tmpFile,
			SHA256: "0000000000000000000000000000000000000000000000000000000000000000", // mismatched
		},
	}

	report, err := ValidateEvidenceManifest(manifest, contracts, "final")
	if err == nil && report.ClosureComplete {
		t.Fatalf("expected artifact hash mismatch to fail closed, but closure succeeded")
	}
}

// 8. Artifact belongs to another run fails closed
func TestMeta_Negative08_ArtifactBelongsToAnotherRunFailsClosed(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "test-artifact.txt")
	content := []byte("run artifact content")
	_ = os.WriteFile(tmpFile, content, 0644)
	h := sha256.Sum256(content)

	manifest, contracts := baseValidManifestAndContracts(t)
	manifest.Records[0].Artifacts = []EvidenceArtifact{
		{
			Path:   tmpFile,
			SHA256: hex.EncodeToString(h[:]),
			RunID:  "nonexistent-run-99999", // not in manifest.Runs
		},
	}

	report, err := ValidateEvidenceManifest(manifest, contracts, "final")
	if err == nil && report.ClosureComplete {
		t.Fatalf("expected artifact from foreign run to fail closed, but closure succeeded")
	}
}

// 9. Only iOS build exists but iOS runtime is required fails closed
func TestMeta_Negative09_OnlyIOSBuildExistsWhenIOSRuntimeRequiredFailsClosed(t *testing.T) {
	manifest, contracts := baseValidManifestAndContracts(t)
	// Mutate the runtime record into an ios-ui record
	for i := range manifest.Records {
		if manifest.Records[i].Tier == "ios-runtime" {
			manifest.Records[i].Tier = "ios-ui"
			manifest.Records[i].TestID = "CherriCoreIntegrationTests/testV2LanguageLetAndFStringCompile"
		}
	}

	report, err := ValidateEvidenceManifest(manifest, contracts, "final")
	if err == nil && report.ClosureComplete {
		t.Fatalf("expected ios-ui alone to fail when ios-runtime is required, but closure succeeded")
	}
	if report.FailedCases == 0 {
		t.Fatalf("expected failed cases > 0 when ios-runtime tier unsatisfied, got %d", report.FailedCases)
	}
}

// 10. Generic runtime smoke tries to satisfy a function requirement fails closed
func TestMeta_Negative10_GenericSmokeTriesToSatisfyFunctionRequirementFailsClosed(t *testing.T) {
	manifest, contracts := baseValidManifestAndContracts(t)
	for i := range manifest.Records {
		if manifest.Records[i].Tier == "ios-runtime" {
			manifest.Records[i].TestID = "ios27-runtime-poc:smoke"
			manifest.Records[i].Assertions = map[string]string{
				"status": "CHERRI_IOS27_RUNTIME_OK",
			}
		}
	}

	report, err := ValidateEvidenceManifest(manifest, contracts, "final")
	if err == nil && report.ClosureComplete {
		t.Fatalf("expected smoke test claiming function requirement to fail closed, but closure succeeded")
	}
}

// 11. Generic marker exists but exact expected function result is absent fails closed
func TestMeta_Negative11_GenericMarkerExistsButExpectedFunctionResultAbsentFailsClosed(t *testing.T) {
	manifest, contracts := baseValidManifestAndContracts(t)
	for i := range manifest.Records {
		if manifest.Records[i].Tier == "ios-runtime" {
			// Has status marker but lacks required "function_result" = "21"
			manifest.Records[i].Assertions = map[string]string{
				"status": "CHERRI_IOS27_RUNTIME_OK",
			}
		}
	}

	report, err := ValidateEvidenceManifest(manifest, contracts, "final")
	if err == nil && report.ClosureComplete {
		t.Fatalf("expected generic marker without exact function assertion to fail closed, but closure succeeded")
	}
}

// 12. Evidence manifest says PASS without test-produced record fails closed
func TestMeta_Negative12_ManifestClaimsPassWithoutTestProducedRecordFailsClosed(t *testing.T) {
	manifest, contracts := baseValidManifestAndContracts(t)
	// Remove all records for FN01
	manifest.Records = manifest.Records[3:]

	report, err := ValidateEvidenceManifest(manifest, contracts, "final")
	if err == nil && report.ClosureComplete {
		t.Fatalf("expected missing test-produced records to fail closed, but closure succeeded")
	}
	if report.FailedCases == 0 {
		t.Fatalf("expected failed cases > 0 when records are missing, got %d", report.FailedCases)
	}
}
