package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

func TestLocalReportIncludesAll179WithoutInheritedClosure(t *testing.T) {
	contracts, err := LoadAllContracts("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	combined := allLocalContracts(contracts)
	if len(combined.Cases) != 179 || combined.CaseCount != 179 {
		t.Fatalf("wrong obligation count: %d", len(combined.Cases))
	}
	results := make(map[string]CaseResult)
	for _, c := range combined.Cases {
		results[c.ID] = CaseResult{ID: c.ID, Status: "NOT_RUN", Detail: "No dedicated scenario has executed"}
	}
	report, err := ValidateAndAggregateWithPhase(combined, results, "fixture", "local")
	if err != nil {
		t.Fatalf("explicit unexecuted obligations are not unknown IDs: %v", err)
	}
	if report.TotalCases != 179 || report.PassedCases != 0 || report.FailedCases != 0 || report.SkippedCases != 179 || report.ClosureComplete {
		t.Fatalf("unexecuted obligations changed truth: %+v", report)
	}
	for _, result := range report.Results {
		if result.Status != "NOT_RUN" || result.Passed || len(result.ExecutedLevels) != 0 {
			t.Fatalf("false execution claim: %+v", result)
		}
	}
	final, err := ValidateAndAggregateWithPhase(combined, results, "fixture", "final")
	if err == nil || final.ClosureComplete {
		t.Fatal("unexecuted obligations must reject final closure")
	}
}

func TestExecutionAuditOnlyReferencesRecordedSmoke(t *testing.T) {
	contracts, err := LoadAllContracts("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, observed := range []bool{false, true} {
		name := "CI01_without_observation"
		if observed {
			name = "CI01_with_retained_smoke"
		}
		t.Run(name, func(t *testing.T) {
			runner := NewRunner()
			runner.runNativeEncodingRequirement()
			// A different recorded observation must not manufacture a CI01 link.
			runner.Record("B01", "baseline", "historical baseline smoke", "repository",
				[]string{"repository"}, true, "PASSED", "observed narrow baseline smoke")
			if observed {
				runner.Record("CI01", "verification", "historical negative CI smoke", "negative-ci",
					[]string{"negative-ci"}, true, "PASSED", "observed narrow encoding smoke")
			}
			before := runner.results["CI01"]
			if before.Status != "NOT_RUN" || before.Passed || len(before.ExecutedLevels) != 0 {
				t.Fatalf("CI01 must remain unexecuted before audit: %+v", before)
			}
			dir := t.TempDir()
			auditPath := filepath.Join(dir, "audit.json")
			if err := writeExecutionAudit(runner, contracts, auditPath); err != nil {
				t.Fatal(err)
			}
			var audit struct {
				ClosureComplete *bool               `json:"closure_complete"`
				Rows            []ExecutionAuditRow `json:"rows"`
			}
			data, err := os.ReadFile(auditPath)
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(data, &audit); err != nil {
				t.Fatal(err)
			}
			if audit.ClosureComplete == nil || *audit.ClosureComplete || len(audit.Rows) != 179 {
				t.Fatalf("audit must retain all unclosed obligations: %+v", audit)
			}
			var observations struct {
				Observations []LocalSmokeObservation `json:"observations"`
			}
			data, err = os.ReadFile(filepath.Join(dir, "acceptance-local-observations.json"))
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(data, &observations); err != nil {
				t.Fatal(err)
			}
			recorded := make(map[string]bool)
			for _, observation := range observations.Observations {
				recorded[observation.ID] = true
			}
			expectedObservations := 1
			if observed {
				expectedObservations++
			}
			if len(recorded) != expectedObservations || !recorded["B01"] || recorded["CI01"] != observed {
				t.Fatalf("audit changed actual observations: %+v", observations)
			}
			byID := make(map[string]ExecutionAuditRow)
			for _, row := range audit.Rows {
				byID[row.ID] = row
				for _, reference := range row.Evidence {
					const prefix = "acceptance-local-observations.json#"
					if strings.HasPrefix(reference, prefix) && !recorded[strings.TrimPrefix(reference, prefix)] {
						t.Errorf("%s links an observation absent from the emitted file: %s", row.ID, reference)
					}
				}
			}
			ci := byID["CI01"]
			if ci.Status != "UNVERIFIED" || len(ci.Tier) != 0 || ci.PositiveAndNegative ||
				len(ci.RequiredTiers) != 1 || ci.RequiredTiers[0] != "negative-ci" ||
				ci.Blocker != incompleteLocalAssertions["CI01"] {
				t.Fatalf("CI01 audit changed acceptance scope: %+v", ci)
			}
			if observed {
				if len(ci.Evidence) != 1 || ci.Evidence[0] != "acceptance-local-observations.json#CI01" {
					t.Fatalf("lost actual CI01 smoke reference: %+v", ci)
				}
			} else {
				if len(ci.Evidence) != 0 || strings.Contains(ci.Scope, "Observed smoke") {
					t.Fatalf("unexecuted CI01 fabricated observation evidence: %+v", ci)
				}
			}
			b01 := byID["B01"]
			if b01.Status != "UNVERIFIED" || len(b01.Evidence) != 1 ||
				b01.Evidence[0] != "acceptance-local-observations.json#B01" {
				t.Fatalf("lost actual B01 smoke reference: %+v", b01)
			}
			if len(runner.records) != 0 {
				t.Fatal("metadata audit minted formal evidence")
			}
			after := runner.results["CI01"]
			if after.Status != "NOT_RUN" || after.Passed || len(after.ExecutedLevels) != 0 {
				t.Fatalf("metadata audit changed original CI01 result: %+v", after)
			}
		})
	}
}
