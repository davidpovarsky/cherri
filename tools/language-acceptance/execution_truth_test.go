package main

import "testing"

func TestRecordRejectsContradictorySuccessfulLabel(t *testing.T) {
	var runner = NewRunner()
	runner.Record("F01", "control", "negative mutation", "unit+native+iOS", []string{"unit", "native"}, false, "PASSED", "RHS escaped its guard")
	var result = runner.results["F01"]
	if result.Passed || result.Status != "FAILED" {
		t.Fatalf("false assertion got status %q, passed=%t", result.Status, result.Passed)
	}
	for _, record := range runner.records {
		if record.Result != "failed" || record.ExitCode == 0 {
			t.Fatalf("failed assertion emitted positive evidence: %+v", record)
		}
	}
}

func TestRecordPreservesOnlyExecutedLocalPasses(t *testing.T) {
	var runner = NewRunner()
	runner.Record("F01", "control", "local pass", "unit+native+iOS", []string{"unit", "native"}, true, "PASSED", "structural assertions ran")
	var result = runner.results["F01"]
	if result.Passed || result.Status != "PENDING_EXTERNAL" {
		t.Fatalf("missing runtime was closed: %+v", result)
	}
	for _, record := range runner.records {
		if record.Tier == "ios-runtime" || record.Result != "passed" || record.ExitCode != 0 {
			t.Fatalf("wrong local evidence: %+v", record)
		}
	}
}

func TestSupplementalRequirementsNeedExecutedScenarios(t *testing.T) {
	var runner = NewRunner()
	runner.results["P01"] = CaseResult{ID: "P01", Passed: true, Status: "PASSED"}
	runner.runRepairCases(&RepairContractFile{Cases: []RepairCaseSpec{{ID: "RP01", Title: "Independent repair", OriginalRequirementIDs: []string{"P01"}, RequiredTestLevels: []string{"unit"}}}})
	runner.runRecoveryGates(&GatesContractFile{Gates: []RecoveryGate{{ID: "BRG01", Title: "Independent gate", RelatedOriginalIDs: []string{"P01"}, MinimumEvidenceTiers: []string{"unit"}}}})
	for _, id := range []string{"RP01", "BRG01"} {
		var result = runner.results[id]
		if result.Status != "NOT_RUN" || result.Passed || len(result.ExecutedLevels) != 0 {
			t.Fatalf("%s inherited an unexecuted pass: %+v", id, result)
		}
	}
	for _, record := range runner.records {
		if record.Result == "passed" {
			t.Fatalf("linked requirement fabricated pass: %+v", record)
		}
	}
}
