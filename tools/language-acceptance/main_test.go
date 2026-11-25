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
				{ID: "RP00", Title: "Repair 1", RequiredTestLevels: []string{"unit"}},
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

	// 3. Synthetic validator controls only: these records do not prove FN01 execution.
	// Bind the positive model to the current producer contract so that the negative
	// control rejects the missing iOS tier, not a stale local producer identifier.
	var scenario = numberFunctionScenarios["FN01"]
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
				TestID:            scenario.producer("unit"),
				ExitCode:          0,
				Result:            "passed",
				Command:           productionTestCommand(scenario.tests()),
				Assertions: map[string]string{
					"required_production_tests": strings.Join(scenario.tests(), "\n"),
					"scope":                     scenario.Scope,
				},
			},
			{
				EvidenceID:        "rec2",
				Requirements:      []string{"FN01"},
				Tier:              "native-structure",
				ImplementationSHA: "sha1",
				TestID:            scenario.producer("native-structure"),
				ExitCode:          0,
				Result:            "passed",
				Command:           productionTestCommand(scenario.tests()),
				Assertions: map[string]string{
					"required_production_tests": strings.Join(scenario.tests(), "\n"),
					"scope":                     scenario.Scope,
				},
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
	if len(manifestReport.ValidationErrors) != 1 || manifestReport.ValidationErrors[0] != "requirement FN01 has unsatisfied required tiers in final phase" {
		t.Fatalf("synthetic model must reject only the missing iOS tier, got: %v", err)
	}
	if manifestReport.FailedCases != 1 {
		t.Fatalf("expected 1 failed case in manifest validation, got %d", manifestReport.FailedCases)
	}
	if manifestReport.ClosureComplete {
		t.Fatalf("closure_complete must be false when ios tier missing in manifest")
	}

	// 4. Add a synthetic iOS model record to exercise the all-tiers policy control.
	// Actual Apple execution is validated separately; this fixture grants no runtime credit.
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

// Historical synthetic model for rejection controls. Its E01 placeholders are
// deliberately invalid under the strict editor intake. Positive controls must
// use p15VerifiedEditorMetaFixture, which replaces them with actual service proof
// and preserves the missing UI tier.
// BRG00 is deliberately outside the original 33-gate contract. Generic model
// records must never stand in for BRG01's dedicated takeover evidence.
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
				{ID: "RP00", Title: "Repair 1", RequiredTestLevels: []string{"unit"}},
			},
		},
		Gates: &GatesContractFile{
			GateCount: 1,
			Gates: []RecoveryGate{
				{ID: "BRG00", Title: "Synthetic gate fixture", Mandatory: true, MinimumEvidenceTiers: []string{"repository"}},
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
				EvidenceID:        "ev-rp00-unit",
				Requirements:      []string{"RP00"},
				Tier:              "unit",
				TestID:            "tools/language-acceptance:runRepair",
				ImplementationSHA: "commit123456789012345678901234567890123",
				ExitCode:          0,
				Result:            "passed",
				Assertions:        map[string]string{"assertion": "synthetic repair fixture ok"},
			},
			{
				EvidenceID:        "ev-brg00-repo",
				Requirements:      []string{"BRG00"},
				Tier:              "repository",
				TestID:            "tools/language-acceptance:runGates",
				ImplementationSHA: "commit123456789012345678901234567890123",
				ExitCode:          0,
				Result:            "passed",
				Assertions:        map[string]string{"assertion": "synthetic gate validator input only"},
			},
		},
	}
	// Synthetic intake model only: use the registered local protocol fields.
	// This fixture does not execute or claim function/Apple production proof.
	var functionScenario = numberFunctionScenarios["FN01"]
	for index := 0; index < 2; index++ {
		var record = &manifest.Records[index]
		record.TestID = functionScenario.producer(record.Tier)
		record.Command = productionTestCommand(functionScenario.tests())
		record.Assertions = map[string]string{"assertion": "synthetic local intake model", "required_production_tests": strings.Join(functionScenario.tests(), "\n"), "scope": functionScenario.Scope}
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

	verifiedManifest, verifiedContracts := p15VerifiedEditorMetaFixture(t)
	runEndToEnd := func(t *testing.T, mutate func(dir string, baseManifest *EvidenceManifest, contracts *ContractsSet, evFile string)) (*AcceptanceReport, error) {
		t.Helper()
		baseManifest, contracts := p15CloneMetaFixture(t, verifiedManifest, verifiedContracts)
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

	// All supplied records validate. The actual service proof cannot close E01's
	// UI tier; each attack below must cause its own additional rejection.
	repOk, errOk := runEndToEnd(t, nil)
	p15AssertMetaFixtureBaseline(t, repOk, errOk, "final")

	// Attack 1: Delete all evidence files while leaving all four successful CI run IDs
	t.Run("Attack01_DeleteAllEvidenceFilesWithGreenCI", func(t *testing.T) {
		rep, err := runEndToEnd(t, func(dir string, bm *EvidenceManifest, c *ContractsSet, evFile string) {
			_ = os.Remove(evFile)
		})
		p15AssertCausalMetaRejection(t, rep, err, "requirement FN01 has unsatisfied required tiers in final phase")
	})

	// Attack 2: Evidence record with exit_code != 0 and result = "passed"
	t.Run("Attack02_NonZeroExitCodeWithPassedResult", func(t *testing.T) {
		rep, err := runEndToEnd(t, func(dir string, bm *EvidenceManifest, c *ContractsSet, evFile string) {
			bm.Records[0].ExitCode = 2
			bm.Records[0].Result = "passed"
		})
		p15AssertCausalMetaRejection(t, rep, err, "non-zero exit code 2 with passed result")
	})

	// Attack 3: Contradictory exit_code == 0 with result = "failed"
	t.Run("Attack03_ContradictoryFailedResultWithZeroExit", func(t *testing.T) {
		rep, err := runEndToEnd(t, func(dir string, bm *EvidenceManifest, c *ContractsSet, evFile string) {
			bm.Records[0].ExitCode = 0
			bm.Records[0].Result = "failed"
		})
		p15AssertCausalMetaRejection(t, rep, err, "zero exit code with failed result")
	})

	// Attack 4: Missing any single acceptance ID, repair ID, or gate ID
	t.Run("Attack04_MissingAcceptanceRepairOrGateID", func(t *testing.T) {
		rep, err := runEndToEnd(t, func(dir string, bm *EvidenceManifest, c *ContractsSet, evFile string) {
			// Drop the synthetic BRG00 gate record
			bm.Records = bm.Records[:len(bm.Records)-1]
		})
		p15AssertCausalMetaRejection(t, rep, err, "requirement BRG00 has unsatisfied required tiers in final phase")
	})

	// Attack 5: Injected unknown requirement ID
	t.Run("Attack05_InjectedUnknownRequirementID", func(t *testing.T) {
		rep, err := runEndToEnd(t, func(dir string, bm *EvidenceManifest, c *ContractsSet, evFile string) {
			bm.Records[0].Requirements = append(bm.Records[0].Requirements, "BOGUS_REQ_999")
		})
		p15AssertCausalMetaRejection(t, rep, err, "unknown requirement ID \"BOGUS_REQ_999\"")
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
		p15AssertCausalMetaRejection(t, rep, err, "is not authorized for requirement FN01")
	})

	// Attack 8: Weak tier substitution (service supplied where ios-ui is required)
	t.Run("Attack08_WeakTierSubstitutionServiceForIOSUI", func(t *testing.T) {
		rep, err := runEndToEnd(t, func(dir string, bm *EvidenceManifest, c *ContractsSet, evFile string) {
			for i := range bm.Records {
				if len(bm.Records[i].Requirements) == 1 && bm.Records[i].Requirements[0] == "E01" {
					// Retag the actual service receipt as UI, keeping its producer,
					// source and raw observations unchanged.
					bm.Records[i].Tier = "ios-ui"
				}
			}
		})
		p15AssertCausalMetaRejection(t, rep, err, "detached editor local contract E01")
	})

	// Attack 9: Empty or mismatched implementation_sha on evidence record (no silent backfilling!)
	t.Run("Attack09_EmptyImplementationSHANotBackfilled", func(t *testing.T) {
		rep, err := runEndToEnd(t, func(dir string, bm *EvidenceManifest, c *ContractsSet, evFile string) {
			bm.Records[0].ImplementationSHA = ""
		})
		p15AssertCausalMetaRejection(t, rep, err, "has empty implementation_sha")
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
		p15AssertCausalMetaRejection(t, rep, err, "on tier ios-runtime missing required CI artifacts")
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
		p15AssertCausalMetaRejection(t, rep, err, `assertion "function_result" value mismatch`)
	})

	// Attack 12: Duplicate evidence_id across ingested files
	t.Run("Attack12_DuplicateEvidenceIDAcrossIngestedFiles", func(t *testing.T) {
		rep, err := runEndToEnd(t, func(dir string, bm *EvidenceManifest, c *ContractsSet, evFile string) {
			dup := bm.Records[0]
			bm.Records = append(bm.Records, dup)
		})
		p15AssertCausalMetaRejection(t, rep, err, "duplicate evidence_id")
	})
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

// AFP-8: Green UI test with skipped interaction assertion fails closed
func TestAntiFalsePositive_GreenUITestWithoutBehavioralCheckpoint_Rejected(t *testing.T) {
	manifest, contracts := baseValidManifestAndContracts(t)
	for i := range manifest.Records {
		if manifest.Records[i].Tier == "ios-ui" {
			manifest.Records[i].Assertions = map[string]string{"completion_contextual": "skipped"}
		}
	}

	report, err := ValidateEvidenceManifest(manifest, contracts, "final")
	if err == nil && report.ClosureComplete {
		t.Fatalf("green UI test with skipped interaction assertion must not achieve closure")
	}
}

// AFP-9: Visual parameter edit with mutated wrong assertion key fails closed
func TestAntiFalsePositive_MutatedSourceNodeTarget_Rejected(t *testing.T) {
	manifest, contracts := baseValidManifestAndContracts(t)
	for i := range manifest.Records {
		if manifest.Records[i].Tier == "ios-ui" {
			manifest.Records[i].Assertions = map[string]string{"wrong_mutation_key": "true"}
		}
	}

	report, err := ValidateEvidenceManifest(manifest, contracts, "final")
	if err == nil && report.ClosureComplete {
		t.Fatalf("mutated UI assertion must not achieve closure")
	}
}
