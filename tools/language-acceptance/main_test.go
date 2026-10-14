package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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
		CodecFingerprint:  "codec_fp1",
		Records: []EvidenceRecord{
			{
				EvidenceID:        "rec1",
				Requirements:      []string{"C01"},
				Tier:              "unit",
				ImplementationSHA: "sha1",
				TestID:            "TestC01",
				ExitCode:          0,
				Result:            "passed",
				Assertions:        map[string]string{"assertion": "ok"},
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

	artPath := filepath.Join(t.TempDir(), "clipboard-result.txt")
	artContent := []byte("CHERRI_IOS27_RUNTIME_OK\nFUNCTION=21\n")
	_ = os.WriteFile(artPath, artContent, 0644)
	artSum := sha256.Sum256(artContent)
	artHex := hex.EncodeToString(artSum[:])

	// 3. In manifest validation: only unit and native records provided for FN01 -> rejected in final phase
	manifest := &EvidenceManifest{
		SchemaVersion:     "1",
		ImplementationSHA: "sha1",
		SchemaFingerprint: "fp1",
		CodecFingerprint:  "codec_fp1",
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
				Assertions:        map[string]string{"assertion": "unit ok"},
			},
			{
				EvidenceID:        "rec2",
				Requirements:      []string{"FN01"},
				Tier:              "native-structure",
				ImplementationSHA: "sha1",
				TestID:            "tools/language-acceptance:runFunctions",
				ExitCode:          0,
				Result:            "passed",
				Assertions:        map[string]string{"assertion": "native ok"},
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
			"function_result":       "21",
			"caller_continuation":   "22",
			"defaults_explicit":     "0:0::",
			"defaults_omitted":      "1:9:def:",
			"recursion_fact3":       "6",
			"void_output":           "",
			"partial_envelope_safe": "multiply",
		},
		Artifacts: []EvidenceArtifact{
			{Path: artPath, SHA256: artHex, RunID: "104"},
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
	artPath := filepath.Join(t.TempDir(), "clipboard-result.txt")
	artContent := []byte("CHERRI_IOS27_RUNTIME_OK\nFUNCTION=21\n")
	_ = os.WriteFile(artPath, artContent, 0644)
	artSum := sha256.Sum256(artContent)
	artHex := hex.EncodeToString(artSum[:])

	uiArtPath := filepath.Join(t.TempDir(), "xcodetest.log")
	uiArtContent := []byte("Test Case '-[CherriUITests.CherriEditorUITests testE01CompletionDisplayAndInsertionUI]' passed\n")
	_ = os.WriteFile(uiArtPath, uiArtContent, 0644)
	uiSum := sha256.Sum256(uiArtContent)
	uiHex := hex.EncodeToString(uiSum[:])

	contracts := &ContractsSet{
		Acceptance: &ContractFile{
			CaseCount: 2,
			Cases: []CaseSpec{
				{ID: "FN01", Group: "functions", Requirement: "Declare function", MinimumTestLevel: "unit+native+iOS"},
				{ID: "E01", Group: "editor", Requirement: "Contextual completion", MinimumTestLevel: "service+UI"},
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
		DocsSHA:           "docs12345678901234567890123456789012345",
		SchemaFingerprint: "fp1",
		CodecFingerprint:  "codec_fp1",
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
				Assertions:        map[string]string{"assertion": "unit ok"},
			},
			{
				EvidenceID:        "ev-fn01-native",
				Requirements:      []string{"FN01"},
				Tier:              "native-structure",
				TestID:            "tools/language-acceptance:runFunctions",
				ImplementationSHA: "commit123456789012345678901234567890123",
				ExitCode:          0,
				Result:            "passed",
				Assertions:        map[string]string{"assertion": "native ok"},
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
					"function_result":       "21",
					"caller_continuation":   "22",
					"defaults_explicit":     "0:0::",
					"defaults_omitted":      "1:9:def:",
					"recursion_fact3":       "6",
					"void_output":           "",
					"partial_envelope_safe": "multiply",
				},
				Artifacts: []EvidenceArtifact{
					{Path: artPath, SHA256: artHex, RunID: "104"},
				},
			},
			{
				EvidenceID:        "ev-e01-service",
				Requirements:      []string{"E01"},
				Tier:              "service",
				TestID:            "tools/language-acceptance:runEditor",
				ImplementationSHA: "commit123456789012345678901234567890123",
				ExitCode:          0,
				Result:            "passed",
				Assertions:        map[string]string{"assertion": "service ok"},
			},
			{
				EvidenceID:        "ev-e01-ui",
				Requirements:      []string{"E01"},
				Tier:              "ios-ui",
				TestID:            "CherriUITests/testE01CompletionDisplayAndInsertionUI",
				ImplementationSHA: "commit123456789012345678901234567890123",
				ExitCode:          0,
				Result:            "passed",
				Assertions:        map[string]string{"completion_contextual": "true"},
				Artifacts: []EvidenceArtifact{
					{Path: uiArtPath, SHA256: uiHex, RunID: "103"},
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
				Assertions:        map[string]string{"assertion": "rp01 ok"},
			},
			{
				EvidenceID:        "ev-brg01-repo",
				Requirements:      []string{"BRG01"},
				Tier:              "repository",
				TestID:            "scripts/verify-backend-recovery.sh",
				ImplementationSHA: "commit123456789012345678901234567890123",
				ExitCode:          0,
				Result:            "passed",
				Assertions:        map[string]string{"assertion": "brg01 ok"},
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

// Section 1E: End-to-End Aggregator Adversarial Meta-Tests (BuildFinalEvidenceManifest + ValidateEvidenceManifest)
func TestEndToEndAggregator_All12AdversarialAttacks(t *testing.T) {
	origDiscover := ciRunDiscoveryFunc
	origPaths := defaultEvidencePathsFunc
	t.Cleanup(func() {
		ciRunDiscoveryFunc = origDiscover
		defaultEvidencePathsFunc = origPaths
	})

	runEndToEnd := func(t *testing.T, mutate func(dir string, baseManifest *EvidenceManifest, contracts *ContractsSet, evFile string)) (*AcceptanceReport, error) {
		t.Helper()
		baseManifest, contracts := baseValidManifestAndContracts(t)
		dir := t.TempDir()
		evFile := filepath.Join(dir, "producer-evidence.json")
		raw, err := json.MarshalIndent(baseManifest.Records, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(evFile, raw, 0644); err != nil {
			t.Fatal(err)
		}

		ciRunDiscoveryFunc = func(commitSHA string) ([]CIRunRecord, error) {
			return baseManifest.Runs, nil
		}
		defaultEvidencePathsFunc = func() []string {
			if _, err := os.Stat(evFile); err == nil {
				return []string{evFile}
			}
			return nil
		}

		if mutate != nil {
			mutate(dir, baseManifest, contracts, evFile)
			if _, err := os.Stat(evFile); err == nil {
				rawMut, _ := json.MarshalIndent(baseManifest.Records, "", "  ")
				_ = os.WriteFile(evFile, rawMut, 0644)
			}
		}

		outManifestPath := filepath.Join(dir, "final-manifest.json")
		built, err := BuildFinalEvidenceManifest(contracts, baseManifest.ImplementationSHA, baseManifest.DocsSHA, outManifestPath)
		if err != nil {
			return nil, err
		}
		return ValidateEvidenceManifest(built, contracts, "final")
	}

	// Sanity check: unmodified end-to-end aggregator passes
	repOk, errOk := runEndToEnd(t, nil)
	if errOk != nil || !repOk.ClosureComplete {
		t.Fatalf("expected unmodified end-to-end aggregator to pass, got err=%v, closure=%v", errOk, repOk)
	}

	// Attack 1: Delete all evidence files while leaving all four successful CI run IDs
	t.Run("Attack01_DeleteAllEvidenceFilesWithGreenCI", func(t *testing.T) {
		rep, err := runEndToEnd(t, func(dir string, bm *EvidenceManifest, c *ContractsSet, evFile string) {
			_ = os.Remove(evFile)
		})
		if err == nil || (rep != nil && rep.ClosureComplete) || (rep != nil && rep.FailedCases == 0) {
			t.Fatalf("Attack 1 must fail closed when all evidence files are deleted: err=%v, rep=%+v", err, rep)
		}
	})

	// Attack 2: Evidence record with exit_code != 0 and result = "passed"
	t.Run("Attack02_NonZeroExitCodeWithPassedResult", func(t *testing.T) {
		rep, err := runEndToEnd(t, func(dir string, bm *EvidenceManifest, c *ContractsSet, evFile string) {
			bm.Records[0].ExitCode = 2
			bm.Records[0].Result = "passed"
		})
		if err == nil || (rep != nil && rep.ClosureComplete) {
			t.Fatalf("Attack 2 must reject exit_code != 0 with passed result")
		}
	})

	// Attack 3: Contradictory exit_code == 0 with result = "failed"
	t.Run("Attack03_ContradictoryFailedResultWithZeroExit", func(t *testing.T) {
		rep, err := runEndToEnd(t, func(dir string, bm *EvidenceManifest, c *ContractsSet, evFile string) {
			bm.Records[0].ExitCode = 0
			bm.Records[0].Result = "failed"
		})
		if err == nil || (rep != nil && rep.ClosureComplete) {
			t.Fatalf("Attack 3 must reject contradictory failed status")
		}
	})

	// Attack 4: Missing any single acceptance ID, repair ID, or gate ID
	t.Run("Attack04_MissingAcceptanceRepairOrGateID", func(t *testing.T) {
		rep, err := runEndToEnd(t, func(dir string, bm *EvidenceManifest, c *ContractsSet, evFile string) {
			// Drop BRG01 record
			bm.Records = bm.Records[:len(bm.Records)-1]
		})
		if err == nil || (rep != nil && rep.ClosureComplete) {
			t.Fatalf("Attack 4 must reject missing gate ID")
		}
	})

	// Attack 5: Injected unknown requirement ID
	t.Run("Attack05_InjectedUnknownRequirementID", func(t *testing.T) {
		rep, err := runEndToEnd(t, func(dir string, bm *EvidenceManifest, c *ContractsSet, evFile string) {
			bm.Records[0].Requirements = append(bm.Records[0].Requirements, "BOGUS_REQ_999")
		})
		if err == nil || (rep != nil && rep.ClosureComplete) {
			t.Fatalf("Attack 5 must reject unknown requirement ID")
		}
	})

	// Attack 6: Modified contract file hash
	t.Run("Attack06_ModifiedContractHash", func(t *testing.T) {
		tmpContract := filepath.Join(t.TempDir(), "corrupt-acceptance.json")
		_ = os.WriteFile(tmpContract, []byte(`{"case_count":94}`), 0644)
		if _, err := LoadAllContracts(tmpContract, "", ""); err == nil {
			t.Fatalf("Attack 6 must reject modified contract hash")
		}
	})

	// Attack 7: Unauthorized test_id including prefix/substring spoofing
	t.Run("Attack07_UnauthorizedTestIDPrefixAndSubstringSpoofing", func(t *testing.T) {
		rep, err := runEndToEnd(t, func(dir string, bm *EvidenceManifest, c *ContractsSet, evFile string) {
			// Prefix spoof without "/" boundary and substring spoof
			bm.Records[0].TestID = "spoof/tools/language-acceptance:runFunctionsEvil"
		})
		if err == nil || (rep != nil && rep.ClosureComplete) {
			t.Fatalf("Attack 7 must reject spoofed test_id")
		}
	})

	// Attack 8: Weak tier substitution (service supplied where ios-ui is required)
	t.Run("Attack08_WeakTierSubstitutionServiceForIOSUI", func(t *testing.T) {
		rep, err := runEndToEnd(t, func(dir string, bm *EvidenceManifest, c *ContractsSet, evFile string) {
			for i := range bm.Records {
				if bm.Records[i].EvidenceID == "ev-e01-ui" {
					bm.Records[i].Tier = "service"
				}
			}
		})
		if err == nil || (rep != nil && rep.ClosureComplete) {
			t.Fatalf("Attack 8 must reject service tier substituted for ios-ui")
		}
	})

	// Attack 9: Empty or mismatched implementation_sha on evidence record (no silent backfilling!)
	t.Run("Attack09_EmptyImplementationSHANotBackfilled", func(t *testing.T) {
		rep, err := runEndToEnd(t, func(dir string, bm *EvidenceManifest, c *ContractsSet, evFile string) {
			bm.Records[0].ImplementationSHA = ""
		})
		if err == nil || (rep != nil && rep.ClosureComplete) {
			t.Fatalf("Attack 9 must reject empty implementation_sha without backfilling")
		}
	})

	// Attack 10: Missing referenced artifact or corrupt artifact SHA-256
	t.Run("Attack10_MissingOrCorruptArtifact", func(t *testing.T) {
		rep, err := runEndToEnd(t, func(dir string, bm *EvidenceManifest, c *ContractsSet, evFile string) {
			for i := range bm.Records {
				if bm.Records[i].Tier == "ios-runtime" {
					bm.Records[i].Artifacts = nil
				}
			}
		})
		if err == nil || (rep != nil && rep.ClosureComplete) {
			t.Fatalf("Attack 10 must reject ios-runtime record with missing artifacts")
		}
	})

	// Attack 11: Wrong RequiredAssertions value (function_result = "0" instead of "21")
	t.Run("Attack11_WrongRequiredAssertionValue", func(t *testing.T) {
		rep, err := runEndToEnd(t, func(dir string, bm *EvidenceManifest, c *ContractsSet, evFile string) {
			for i := range bm.Records {
				if bm.Records[i].Tier == "ios-runtime" {
					bm.Records[i].Assertions["function_result"] = "0"
				}
			}
		})
		if err == nil || (rep != nil && rep.ClosureComplete) {
			t.Fatalf("Attack 11 must reject wrong function_result assertion value")
		}
	})

	// Attack 12: Duplicate evidence_id across ingested files
	t.Run("Attack12_DuplicateEvidenceIDAcrossIngestedFiles", func(t *testing.T) {
		rep, err := runEndToEnd(t, func(dir string, bm *EvidenceManifest, c *ContractsSet, evFile string) {
			dup := bm.Records[0]
			bm.Records = append(bm.Records, dup)
		})
		if err == nil || (rep != nil && rep.ClosureComplete) {
			t.Fatalf("Attack 12 must reject duplicate evidence_id in ingested files")
		}
	})
}

func TestFull179ContractRequirements_EndToEndClosureAndProducerIsolation(t *testing.T) {
	origDiscover := ciRunDiscoveryFunc
	origPaths := defaultEvidencePathsFunc
	t.Cleanup(func() {
		ciRunDiscoveryFunc = origDiscover
		defaultEvidencePathsFunc = origPaths
	})

	contracts, err := LoadAllContracts("", "", "")
	if err != nil {
		t.Fatalf("LoadAllContracts failed: %v", err)
	}

	dir := t.TempDir()
	implSHA := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	docsSHA := "4218b152f6aea5e8e17912a8b4a8924ffd819f00"

	runs := []CIRunRecord{
		{RunID: "201", Repository: "davidpovarsky/cherri", HeadSHA: implSHA, CheckoutSHA: implSHA, Conclusion: "success", WorkflowName: "Build & Test"},
		{RunID: "202", Repository: "davidpovarsky/cherri", HeadSHA: implSHA, CheckoutSHA: implSHA, Conclusion: "success", WorkflowName: "OpenMinis Skill"},
		{RunID: "203", Repository: "davidpovarsky/cherri", HeadSHA: implSHA, CheckoutSHA: implSHA, Conclusion: "success", WorkflowName: "iOS Build"},
		{RunID: "204", Repository: "davidpovarsky/cherri", HeadSHA: implSHA, CheckoutSHA: implSHA, Conclusion: "success", WorkflowName: "iOS 27 Shortcuts Runtime PoC"},
	}
	ciRunDiscoveryFunc = func(commitSHA string) ([]CIRunRecord, error) {
		return runs, nil
	}

	// 1. Run real local Runner for all 22 groups + repair + gates + CI
	runner := NewRunner()
	runner.implSHA = implSHA
	runner.runBaseline()
	runner.runUpstream()
	runner.runParser()
	runner.runBinding()
	runner.runCalls()
	runner.runTypes()
	runner.runText()
	runner.runCollections()
	runner.runNumbers()
	runner.runControl()
	runner.runFunctions()
	runner.runMetadata()
	runner.runModules()
	runner.runSchema()
	runner.runNative()
	runner.runEditor()
	runner.runCLI()
	runner.runDocs()
	runner.runSkill()
	runner.runVerification()
	runner.runEvaluation()
	runner.runDelivery()
	runner.runRepairCases(contracts.Repair)
	runner.runRecoveryGates(contracts.Gates)
	runner.runCIVerification()

	localEvPath := filepath.Join(dir, "local-runner-evidence.json")
	localBytes, _ := json.MarshalIndent(runner.records, "", "  ")
	_ = os.WriteFile(localEvPath, localBytes, 0644)

	// 2. Skill producer file (matches self-test.sh output)
	skillArtPath := filepath.Join(dir, "test.shortcut")
	skillArtBytes := []byte("bplist00-skill")
	_ = os.WriteFile(skillArtPath, skillArtBytes, 0644)
	skillArtSum := sha256.Sum256(skillArtBytes)
	skillArt := EvidenceArtifact{Path: skillArtPath, SHA256: hex.EncodeToString(skillArtSum[:]), RunID: "202"}

	skillRecs := []EvidenceRecord{
		{
			EvidenceID:        "ev-skill-package-self-test",
			Requirements:      []string{"K01", "RP42", "RP44", "RP45", "RP49", "BRG30"},
			Tier:              "skill-package",
			ImplementationSHA: implSHA,
			TestID:            "skills/cherri-shortcuts/scripts/self-test.sh",
			ExitCode:          0,
			Result:            "passed",
			Assertions:        map[string]string{"scenarios_passed": "10"},
			Artifacts:         []EvidenceArtifact{skillArt},
		},
		{
			EvidenceID:        "ev-skill-integration-self-test",
			Requirements:      []string{"K02", "K03", "K04", "K05", "D03", "RP44", "RP45", "RP46", "BRG30"},
			Tier:              "integration",
			ImplementationSHA: implSHA,
			TestID:            "skills/cherri-shortcuts/scripts/self-test.sh",
			ExitCode:          0,
			Result:            "passed",
			Assertions:        map[string]string{"scenarios_passed": "10"},
			Artifacts:         []EvidenceArtifact{skillArt},
		},
	}
	skillEvPath := filepath.Join(dir, "skill-evidence.json")
	skillEvBytes, _ := json.MarshalIndent(skillRecs, "", "  ")
	_ = os.WriteFile(skillEvPath, skillEvBytes, 0644)

	// 3. iOS Build Simulator XCTests producer file (matches ios-build.yml output)
	uiArtPath := filepath.Join(dir, "xcodetest.log")
	uiArtBytes := []byte("TEST SUCCEEDED")
	_ = os.WriteFile(uiArtPath, uiArtBytes, 0644)
	uiArtSum := sha256.Sum256(uiArtBytes)
	uiArt := EvidenceArtifact{Path: uiArtPath, SHA256: hex.EncodeToString(uiArtSum[:]), RunID: "203"}

	iosRecs := []EvidenceRecord{
		{EvidenceID: "ev-ios-ui-E01", Requirements: []string{"E01"}, Tier: "ios-ui", ImplementationSHA: implSHA, TestID: "CherriUITests/testE01CompletionDisplayAndInsertionUI", ExitCode: 0, Result: "passed", Assertions: map[string]string{"completion_contextual": "true"}, Artifacts: []EvidenceArtifact{uiArt}},
		{EvidenceID: "ev-ios-ui-E02", Requirements: []string{"E02"}, Tier: "ios-ui", ImplementationSHA: implSHA, TestID: "CherriUITests/testE02DiagnosticsRenderingAndNavigationUI", ExitCode: 0, Result: "passed", Assertions: map[string]string{"multi_diagnostics": "true"}, Artifacts: []EvidenceArtifact{uiArt}},
		{EvidenceID: "ev-ios-ui-E03", Requirements: []string{"E03", "RP39"}, Tier: "ios-ui", ImplementationSHA: implSHA, TestID: "CherriUITests/testE03EditorActionsHoverRenameUI", ExitCode: 0, Result: "passed", Assertions: map[string]string{"scoped_rename": "true"}, Artifacts: []EvidenceArtifact{uiArt}},
		{EvidenceID: "ev-ios-ui-E05", Requirements: []string{"E05"}, Tier: "ios-ui", ImplementationSHA: implSHA, TestID: "CherriUITests/testE05EditorResponsivenessStaleEditsUI", ExitCode: 0, Result: "passed", Assertions: map[string]string{"versioned_edits_isolated": "true"}, Artifacts: []EvidenceArtifact{uiArt}},
		{EvidenceID: "ev-ios-ui-E06", Requirements: []string{"E06"}, Tier: "ios-ui", ImplementationSHA: implSHA, TestID: "CherriUITests/testE06BidirectionalHebrewEmojiUI", ExitCode: 0, Result: "passed", Assertions: map[string]string{"utf16_spans_exact": "true"}, Artifacts: []EvidenceArtifact{uiArt}},
		{EvidenceID: "ev-ios-ui-E07", Requirements: []string{"E07", "RP41", "BRG28"}, Tier: "ios-ui", ImplementationSHA: implSHA, TestID: "CherriUITests/testE07VisualParameterEditingCanvasUI", ExitCode: 0, Result: "passed", Assertions: map[string]string{"visual_source_edit_bounded": "true"}, Artifacts: []EvidenceArtifact{uiArt}},
		{EvidenceID: "ev-ios-ui-BRG27", Requirements: []string{"BRG27", "BRG25"}, Tier: "ios-ui", ImplementationSHA: implSHA, TestID: "CherriUITests/testBRG27EditorRealWorldWorkflowUI", ExitCode: 0, Result: "passed", Assertions: map[string]string{"multi_diagnostics": "true", "completion_contextual": "true", "shared_analysis_bridge": "true"}, Artifacts: []EvidenceArtifact{uiArt}},
		{EvidenceID: "ev-ios-ui-RP40", Requirements: []string{"RP40"}, Tier: "ios-ui", ImplementationSHA: implSHA, TestID: "CherriUITests/testE01CompletionDisplayAndInsertionUI", ExitCode: 0, Result: "passed", Assertions: map[string]string{"completion_contextual": "true"}, Artifacts: []EvidenceArtifact{uiArt}},
		{EvidenceID: "ev-ios-service-E04", Requirements: []string{"E04", "RP39"}, Tier: "service", ImplementationSHA: implSHA, TestID: "CherriCoreIntegrationTests/testE04FormatterPreservesCommentsAndSemantics", ExitCode: 0, Result: "passed", Assertions: map[string]string{"formatter_idempotent_comments": "true"}, Artifacts: []EvidenceArtifact{uiArt}},
		{EvidenceID: "ev-ios-service-BRG25", Requirements: []string{"BRG25"}, Tier: "service", ImplementationSHA: implSHA, TestID: "CherriCoreIntegrationTests/testBRG25SharedAnalysisBridgeContract", ExitCode: 0, Result: "passed", Assertions: map[string]string{"shared_analysis_bridge": "true"}, Artifacts: []EvidenceArtifact{uiArt}},
		{EvidenceID: "ev-ios-build-BRG25", Requirements: []string{"BRG25"}, Tier: "ios-build", ImplementationSHA: implSHA, TestID: "CherriCoreIntegrationTests/testBRG25SharedAnalysisBridgeContract", ExitCode: 0, Result: "passed", Assertions: map[string]string{"shared_analysis_bridge": "true"}, Artifacts: []EvidenceArtifact{uiArt}},
	}
	iosEvPath := filepath.Join(dir, "ios-test-evidence.json")
	iosEvBytes, _ := json.MarshalIndent(iosRecs, "", "  ")
	_ = os.WriteFile(iosEvPath, iosEvBytes, 0644)

	// 4. iOS 27 Shortcuts Runtime PoC producer file (matches scripts/ios27_runtime_poc.sh output)
	rtArtPath := filepath.Join(dir, "clipboard-result.txt")
	rtArtBytes := []byte("CHERRI_IOS27_RUNTIME_OK\n")
	_ = os.WriteFile(rtArtPath, rtArtBytes, 0644)
	rtArtSum := sha256.Sum256(rtArtBytes)
	rtArt := EvidenceArtifact{Path: rtArtPath, SHA256: hex.EncodeToString(rtArtSum[:]), RunID: "204"}

	runtimeRecs := []EvidenceRecord{
		{EvidenceID: "ev-runtime-smoke", Requirements: []string{"CI02", "RP48", "BRG26", "BRG31"}, Tier: "ios-runtime", ImplementationSHA: implSHA, TestID: "ios27-runtime-poc:smoke", ExitCode: 0, Result: "passed", Assertions: map[string]string{"status": "CHERRI_IOS27_RUNTIME_OK"}, Artifacts: []EvidenceArtifact{rtArt}},
		{EvidenceID: "ev-runtime-variables", Requirements: []string{"V01", "V03", "RP12", "RP14", "BRG10"}, Tier: "ios-runtime", ImplementationSHA: implSHA, TestID: "ios27-runtime-poc:variables", ExitCode: 0, Result: "passed", Assertions: map[string]string{"variable_result": "updated", "captured_before": "initial", "num_captured": "1", "clip_captured": "clip_initial"}, Artifacts: []EvidenceArtifact{rtArt}},
		{EvidenceID: "ev-runtime-unicode", Requirements: []string{"S01", "RP19", "BRG08"}, Tier: "ios-runtime", ImplementationSHA: implSHA, TestID: "ios27-runtime-poc:unicode", ExitCode: 0, Result: "passed", Assertions: map[string]string{"unicode_output": "שָׁלוֹם:🌍:1"}, Artifacts: []EvidenceArtifact{rtArt}},
		{EvidenceID: "ev-runtime-indexing", Requirements: []string{"L02", "L04", "RP21"}, Tier: "ios-runtime", ImplementationSHA: implSHA, TestID: "ios27-runtime-poc:indexing", ExitCode: 0, Result: "passed", Assertions: map[string]string{"index_trace": "alpha:gamma:::::beta"}, Artifacts: []EvidenceArtifact{rtArt}},
		{EvidenceID: "ev-runtime-numbers", Requirements: []string{"N01", "RP15", "RP17", "BRG16"}, Tier: "ios-runtime", ImplementationSHA: implSHA, TestID: "ios27-runtime-poc:numbers", ExitCode: 0, Result: "passed", Assertions: map[string]string{"math_trace": "2.5:1:11:0:1:0", "index_trace": "alpha:gamma:::::beta"}, Artifacts: []EvidenceArtifact{rtArt}},
		{EvidenceID: "ev-runtime-conditionals", Requirements: []string{"F01", "RP18", "BRG13"}, Tier: "ios-runtime", ImplementationSHA: implSHA, TestID: "ios27-runtime-poc:conditionals", ExitCode: 0, Result: "passed", Assertions: map[string]string{"conditional_result": "true", "short_circuit_trace": "START|RAN_RHS:0:1:1"}, Artifacts: []EvidenceArtifact{rtArt}},
		{EvidenceID: "ev-runtime-loops", Requirements: []string{"F02", "F04", "RP22", "BRG12", "BRG14"}, Tier: "ios-runtime", ImplementationSHA: implSHA, TestID: "ios27-runtime-poc:nested-loops", ExitCode: 0, Result: "passed", Assertions: map[string]string{"loop_trace": "0:A:0:1|0:A:1:2|1:B:0:1|1:B:1:2|", "value_blocks": "then_val:<X><Y>"}, Artifacts: []EvidenceArtifact{rtArt}},
		{EvidenceID: "ev-runtime-functions", Requirements: []string{"FN01", "FN02", "FN03", "FN04", "T01", "M02", "RP24", "RP25", "RP26", "BRG11", "BRG15"}, Tier: "ios-runtime", ImplementationSHA: implSHA, TestID: "ios27-runtime-poc:functions", ExitCode: 0, Result: "passed", Assertions: map[string]string{"function_result": "21", "caller_continuation": "22", "defaults_explicit": "0:0::", "defaults_omitted": "1:9:def:", "recursion_fact3": "6", "void_output": "", "partial_envelope_safe": "multiply"}, Artifacts: []EvidenceArtifact{rtArt}},
	}
	runtimeEvPath := filepath.Join(dir, "runtime-evidence.json")
	runtimeEvBytes, _ := json.MarshalIndent(runtimeRecs, "", "  ")
	_ = os.WriteFile(runtimeEvPath, runtimeEvBytes, 0644)

	defaultEvidencePathsFunc = func() []string {
		return []string{localEvPath, skillEvPath, iosEvPath, runtimeEvPath}
	}

	manifest, err := BuildFinalEvidenceManifest(contracts, implSHA, docsSHA, filepath.Join(dir, "final-manifest.json"))
	if err != nil {
		t.Fatalf("BuildFinalEvidenceManifest failed: %v", err)
	}
	report, err := ValidateEvidenceManifest(manifest, contracts, "final")
	if err != nil {
		t.Fatalf("ValidateEvidenceManifest failed on full 179 requirement set: %v", err)
	}
	if report.TotalCases != 179 || report.PassedCases != 178 || report.SkippedCases != 1 || report.FailedCases != 0 || !report.ClosureComplete {
		t.Fatalf("unexpected report counts: total=%d passed=%d skipped=%d failed=%d closure=%t",
			report.TotalCases, report.PassedCases, report.SkippedCases, report.FailedCases, report.ClosureComplete)
	}
}

// Phase 2.4 Anti-False-Positive Regression Test Suite
// Proves that bridge-only results cannot satisfy UI requirements, swapped acceptance IDs are rejected,
// and missing UI evidence causes the acceptance suite to report requirements open or blocked rather than passing.

// AFP-1: Bridge-only results cannot satisfy UI requirements
func TestAntiFalsePositive_BridgeOnlyCannotSatisfyUIRequirement(t *testing.T) {
	manifest, contracts := baseValidManifestAndContracts(t)

	// Replace the legitimate UI test with a bridge-only test purporting to be ios-ui
	for i := range manifest.Records {
		if manifest.Records[i].Tier == "ios-ui" {
			manifest.Records[i].TestID = "CherriCoreIntegrationTests/testE01ContextualCompletionService"
		}
	}

	report, err := ValidateEvidenceManifest(manifest, contracts, "final")
	if err == nil && report.ClosureComplete {
		t.Fatalf("expected validation error when bridge test claims ios-ui tier, but got success")
	}
	foundBridgeError := false
	for _, vErr := range report.ValidationErrors {
		if strings.Contains(vErr, "claims UI tier with bridge-only test") {
			foundBridgeError = true
			break
		}
	}
	if !foundBridgeError {
		t.Errorf("expected validation error mentioning bridge-only test on UI tier, got errors: %v", report.ValidationErrors)
	}
}

// AFP-2: Bridge record with service tier does not satisfy UI level
func TestAntiFalsePositive_BridgeRecordCannotSatisfyUILevel(t *testing.T) {
	manifest, contracts := baseValidManifestAndContracts(t)

	// Demote UI record to service tier
	var filtered []EvidenceRecord
	for _, r := range manifest.Records {
		if r.Tier != "ios-ui" {
			filtered = append(filtered, r)
		}
	}
	// Add service record from bridge
	filtered = append(filtered, EvidenceRecord{
		EvidenceID:        "ev-bridge-service",
		Requirements:      []string{"E01"},
		Tier:              "service",
		TestID:            "CherriCoreIntegrationTests/testE01ContextualCompletionService",
		ExitCode:          0,
		Result:            "passed",
		ImplementationSHA: manifest.ImplementationSHA,
	})
	manifest.Records = filtered

	report, _ := ValidateEvidenceManifest(manifest, contracts, "final")
	if report.ClosureComplete {
		t.Fatalf("expected closure to fail when UI tier is missing and only service tier is present")
	}
	if report.FailedCases == 0 {
		t.Fatalf("expected failed cases when requirement E01 lacks UI evidence, got 0")
	}
}

// AFP-3: Swapped acceptance IDs (E01 completion vs E02 diagnostics) are rejected
func TestAntiFalsePositive_SwappedAcceptanceIDs_E01_E02(t *testing.T) {
	manifest, contracts := baseValidManifestAndContracts(t)

	// Mutate E01 record to assert multi_diagnostics instead of completion_contextual
	for i := range manifest.Records {
		if manifest.Records[i].Tier == "ios-ui" {
			manifest.Records[i].Assertions = map[string]string{"multi_diagnostics": "true"}
		}
	}

	report, err := ValidateEvidenceManifest(manifest, contracts, "final")
	if err == nil && report.ClosureComplete {
		t.Fatalf("expected rejection when E01 provides diagnostics assertion instead of completion assertion")
	}
	foundMismatch := false
	for _, vErr := range report.ValidationErrors {
		if strings.Contains(vErr, "missing required UI assertion") || strings.Contains(vErr, "assertion") {
			foundMismatch = true
			break
		}
	}
	if !foundMismatch {
		t.Errorf("expected assertion mismatch error for swapped ID, got: %v", report.ValidationErrors)
	}
}

// AFP-4: Missing UI evidence leaves requirement unsatisfied (open or failed)
func TestAntiFalsePositive_MissingUIEvidenceLeavesRequirementUnsatisfied(t *testing.T) {
	manifest, contracts := baseValidManifestAndContracts(t)

	// Strip all UI records
	var stripped []EvidenceRecord
	for _, r := range manifest.Records {
		if r.Tier != "ios-ui" {
			stripped = append(stripped, r)
		}
	}
	manifest.Records = stripped

	// In local phase: reports NOT_RUN / PENDING_EXTERNAL
	reportLocal, _ := ValidateEvidenceManifest(manifest, contracts, "local")
	var e01Res CaseResult
	for _, r := range reportLocal.Results {
		if r.ID == "E01" {
			e01Res = r
			break
		}
	}
	if e01Res.Passed {
		t.Errorf("E01 must not pass locally without UI evidence")
	}

	// In final phase: reports FAILED
	reportFinal, err := ValidateEvidenceManifest(manifest, contracts, "final")
	if err == nil && reportFinal.ClosureComplete {
		t.Fatalf("final closure must not succeed when UI evidence is missing")
	}
}

// AFP-5: Invalid UI assertion value is rejected
func TestAntiFalsePositive_InvalidUIAssertionValueRejected(t *testing.T) {
	manifest, contracts := baseValidManifestAndContracts(t)

	for i := range manifest.Records {
		if manifest.Records[i].Tier == "ios-ui" {
			manifest.Records[i].Assertions = map[string]string{"completion_contextual": "false"}
		}
	}

	report, err := ValidateEvidenceManifest(manifest, contracts, "final")
	if err == nil && report.ClosureComplete {
		t.Fatalf("expected rejection when UI assertion has false value")
	}
	foundMismatch := false
	for _, vErr := range report.ValidationErrors {
		if strings.Contains(vErr, "value mismatch") {
			foundMismatch = true
			break
		}
	}
	if !foundMismatch {
		t.Errorf("expected value mismatch error, got: %v", report.ValidationErrors)
	}
}

// AFP-6: Unauthorized test ID claiming requirement is rejected
func TestAntiFalsePositive_UnauthorizedTestIDForRequirementRejected(t *testing.T) {
	manifest, contracts := baseValidManifestAndContracts(t)

	for i := range manifest.Records {
		if manifest.Records[i].Tier == "ios-ui" {
			manifest.Records[i].TestID = "UnauthorizedSuite/testInjectFakeUI"
		}
	}

	report, err := ValidateEvidenceManifest(manifest, contracts, "final")
	if err == nil && report.ClosureComplete {
		t.Fatalf("expected rejection when unauthorized test ID claims requirement")
	}
	foundAuthError := false
	for _, vErr := range report.ValidationErrors {
		if strings.Contains(vErr, "is not authorized for requirement") {
			foundAuthError = true
			break
		}
	}
	if !foundAuthError {
		t.Errorf("expected unauthorized test ID error, got: %v", report.ValidationErrors)
	}
}

// AFP-7: Empty manifest leaves all external requirements unsatisfied
func TestAntiFalsePositive_EmptyManifestLeavesAllExternalRequirementsUnsatisfied(t *testing.T) {
	_, contracts := baseValidManifestAndContracts(t)
	emptyManifest := &EvidenceManifest{
		SchemaVersion:     "1",
		ImplementationSHA: "commit123456789012345678901234567890123",
		DocsSHA:           "docs12345678901234567890123456789012345",
		SchemaFingerprint: "fp1",
		CodecFingerprint:  "codec_fp1",
		Records:           []EvidenceRecord{},
	}

	report, err := ValidateEvidenceManifest(emptyManifest, contracts, "final")
	if err == nil && report.ClosureComplete {
		t.Fatalf("empty manifest must not achieve closure")
	}
	if report.PassedCases != 0 {
		t.Errorf("expected 0 passed cases for empty manifest in final phase, got %d", report.PassedCases)
	}
}
