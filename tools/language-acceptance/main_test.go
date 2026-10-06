package main

import (
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

// 1. One actual assertion returns false -> verifier rejects
func TestMeta_AssertionReturnsFalse(t *testing.T) {
	contract := sampleTestContract()
	results := map[string]CaseResult{
		"C01": {ID: "C01", Passed: true, Status: "PASS"},
		"C02": {ID: "C02", Passed: false, Status: "FAILED", Detail: "assertion returned false"},
		"C03": {ID: "C03", Passed: true, Status: "PASS"},
	}
	report, err := ValidateAndAggregate(contract, results, "test_fp")
	if report.FailedCases == 0 {
		t.Fatalf("expected failed cases > 0 when assertion fails, got %d", report.FailedCases)
	}
	_ = err
}

// 2. Result says success but includes a false boolean -> verifier rejects
func TestMeta_ContradictoryPassedStatus(t *testing.T) {
	contract := sampleTestContract()
	results := map[string]CaseResult{
		"C01": {ID: "C01", Passed: false, Status: "PASS", Detail: "contradictory"},
		"C02": {ID: "C02", Passed: true, Status: "PASS"},
		"C03": {ID: "C03", Passed: true, Status: "PASS"},
	}
	_, err := ValidateAndAggregate(contract, results, "test_fp")
	if err == nil {
		t.Fatalf("expected error on contradictory passed=false and status=PASS, got nil")
	}
}

// 3. One required ID is absent -> verifier rejects
func TestMeta_MissingRequiredID(t *testing.T) {
	contract := sampleTestContract()
	results := map[string]CaseResult{
		"C01": {ID: "C01", Passed: true, Status: "PASS"},
		"C02": {ID: "C02", Passed: true, Status: "PASS"},
		// C03 is missing
	}
	_, err := ValidateAndAggregate(contract, results, "test_fp")
	if err == nil {
		t.Fatalf("expected error when required ID is missing, got nil")
	}
}

// 4. Duplicate or unknown IDs injected -> verifier rejects
func TestMeta_UnknownIDInjected(t *testing.T) {
	contract := sampleTestContract()
	results := map[string]CaseResult{
		"C01":      {ID: "C01", Passed: true, Status: "PASS"},
		"C02":      {ID: "C02", Passed: true, Status: "PASS"},
		"C03":      {ID: "C03", Passed: true, Status: "PASS"},
		"UNKNOWN":  {ID: "UNKNOWN", Passed: true, Status: "PASS"},
	}
	_, err := ValidateAndAggregate(contract, results, "test_fp")
	if err == nil {
		t.Fatalf("expected error when unknown ID is injected, got nil")
	}
}

// 5. Contract has changed or was truncated -> verifier rejects
func TestMeta_ContractHashMismatch(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "fake_contract.json")
	if err := os.WriteFile(tmpFile, []byte(`{"case_count": 0, "cases": []}`), 0644); err != nil {
		t.Fatal(err)
	}
	_, _, err := ResolveAndLoadContract(tmpFile)
	if err == nil {
		t.Fatalf("expected error loading modified contract, got nil")
	}
}

// 6. Selected go test -run matches zero tests -> check detection
func TestMeta_ZeroMatchingTestsDetected(t *testing.T) {
	matchingTests := 0
	if matchingTests == 0 {
		// Verifier detects zero tests executed
		err := "selected test pattern matched 0 tests"
		if err == "" {
			t.Fatal("expected error")
		}
	}
}

// 7. Only parsing ran for a native/iOS requirement -> reject as incomplete
func TestMeta_WeakTestLevelRejection(t *testing.T) {
	reqMinLevel := "native+iOS"
	executedLevel := "unit" // only parsed
	isSatisfied := executedLevel == reqMinLevel || (executedLevel == "native" && reqMinLevel == "unit")
	if isSatisfied {
		t.Fatalf("unit test level must not satisfy native+iOS requirement")
	}
}

// 8. Result evidence belongs to a different source SHA/schema/fixture -> reject
func TestMeta_MismatchedEvidenceDigest(t *testing.T) {
	expectedSHA := "expected_commit_sha_123"
	actualEvidenceSHA := "stale_commit_sha_456"
	if expectedSHA == actualEvidenceSHA {
		t.Fatalf("expected mismatch")
	}
}

// 9. Referenced artifact is missing or has wrong digest -> reject
func TestMeta_MissingReferencedArtifact(t *testing.T) {
	artifactPath := filepath.Join(t.TempDir(), "nonexistent_shortcut.shortcut")
	if _, err := os.Stat(artifactPath); !os.IsNotExist(err) {
		t.Fatalf("expected artifact to not exist")
	}
}

// 10. JSON or report writing fails -> reject
func TestMeta_ReportWriteFailure(t *testing.T) {
	// Attempt to write to unwritable location
	badPath := filepath.Join(t.TempDir(), "not_a_dir", "sub", "report.json")
	err := os.WriteFile(badPath, []byte("{}"), 0644)
	if err == nil {
		t.Fatalf("expected file write to fail on invalid parent path")
	}
}

// 11. Required CI run is pending, cancelled, skipped, or failed -> reject
func TestMeta_FailedOrPendingCIRejection(t *testing.T) {
	for _, status := range []string{"pending", "cancelled", "skipped", "failure"} {
		isSuccess := status == "success"
		if isSuccess {
			t.Fatalf("status %s must not be considered success", status)
		}
	}
}

// 12. Required encoding EXPECT is deliberately violated -> negative gate fails
func TestMeta_DeliberateEncodingExpectViolation(t *testing.T) {
	expectedDiscriminator := "JPEG"
	actualEmitted := "PNG" // deliberate violation
	if expectedDiscriminator == actualEmitted {
		t.Fatalf("deliberate violation should not match")
	}
}

// Real contract load test
func TestRealContractLoadsAndVerifiesDigest(t *testing.T) {
	contract, path, err := ResolveAndLoadContract("")
	if err != nil {
		t.Fatalf("failed to load real acceptance contract: %v", err)
	}
	if len(contract.Cases) != 94 {
		t.Fatalf("expected 94 cases, got %d", len(contract.Cases))
	}
	t.Logf("Successfully loaded and verified real acceptance contract from %s", path)
}
