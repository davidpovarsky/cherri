package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestSchemaSupplementalProductionEvidence(t *testing.T) {
	all, err := LoadAllContracts("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	wanted := map[string]bool{"RP06": true, "RP07": true, "BRG22": true, "BRG23": true}
	selected := &ContractsSet{Acceptance: &ContractFile{}, Repair: &RepairContractFile{}, Gates: &GatesContractFile{}}
	expectedRecords := 0
	for _, scenario := range all.Repair.Cases {
		if wanted[scenario.ID] {
			selected.Repair.Cases = append(selected.Repair.Cases, scenario)
			tiers := parseRequiredTiers(strings.Join(scenario.RequiredTestLevels, "+"))
			if !reflect.DeepEqual(tiers, supplementalProductionScenarios[scenario.ID].Tiers) {
				t.Fatal("schema repair tiers differ from immutable contract")
			}
			expectedRecords += len(tiers)
		}
	}
	for _, scenario := range all.Gates.Gates {
		if wanted[scenario.ID] {
			selected.Gates.Gates = append(selected.Gates.Gates, scenario)
			tiers := parseRequiredTiers(strings.Join(scenario.MinimumEvidenceTiers, "+"))
			if !reflect.DeepEqual(tiers, supplementalProductionScenarios[scenario.ID].Tiers) {
				t.Fatal("schema recovery tiers differ from immutable contract")
			}
			expectedRecords += len(tiers)
		}
	}
	selected.Repair.CaseCount = len(selected.Repair.Cases)
	selected.Gates.GateCount = len(selected.Gates.Gates)
	if selected.Repair.CaseCount != 2 || selected.Gates.GateCount != 2 {
		t.Fatal("schema producer contract selection changed")
	}
	runner := NewRunner()
	runner.runRepairCases(selected.Repair)
	runner.runRecoveryGates(selected.Gates)
	if len(runner.results) != 4 || len(runner.records) != expectedRecords || len(runner.observations) != 0 {
		t.Fatalf("unexpected schema scope: results=%d records=%d observations=%d", len(runner.results), len(runner.records), len(runner.observations))
	}
	for id, result := range runner.results {
		scenario := supplementalProductionScenarios[id]
		if !wanted[id] || !result.Passed || result.Status != "PASSED" || !reflect.DeepEqual(result.ExecutedLevels, scenario.Tiers) {
			t.Fatalf("complete schema producer failed: %+v", result)
		}
	}
	for _, record := range runner.records {
		if len(record.Requirements) != 1 || record.Result != "passed" || record.ExitCode != 0 || record.ImplementationSHA != runner.implSHA {
			t.Fatalf("invalid schema record: %+v", record)
		}
		id := record.Requirements[0]
		if err := validateSupplementalProductionRecord(id, record); err != nil {
			t.Fatal(err)
		}
		for _, name := range supplementalProductionScenarios[id].TestNames {
			if !strings.Contains(record.Detail, name) {
				t.Fatalf("actual discovery receipt missing %s", name)
			}
		}
	}
	manifest := &EvidenceManifest{SchemaVersion: "1", ImplementationSHA: runner.implSHA, Records: runner.records}
	report, err := ValidateEvidenceManifest(manifest, selected, "local")
	if err != nil || report.PassedCases != 4 || report.FailedCases != 0 || report.SkippedCases != 0 || report.ClosureComplete {
		t.Fatalf("complete schema receipts rejected or expanded to release closure: %+v %v", report, err)
	}
	encoded, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"RP06", "RP07", "BRG22", "BRG23"} {
		for _, fault := range []string{"zero_selection", "missing_assertion", "wrong_producer", "stale_source", "invented_runtime", "missing_generation_tier", "wrong_requirement"} {
			t.Run(id+"/"+fault, func(t *testing.T) {
				var changed EvidenceManifest
				if err := json.Unmarshal(encoded, &changed); err != nil {
					t.Fatal(err)
				}
				index := -1
				for i, r := range changed.Records {
					if r.Requirements[0] == id {
						index = i
						break
					}
				}
				if index < 0 {
					t.Fatal("actual schema record missing")
				}
				switch fault {
				case "zero_selection":
					changed.Records[index].Command = productionTestCommand([]string{"TestSchemaNoSelectedProducer"})
				case "missing_assertion":
					delete(changed.Records[index].Assertions, "required_production_tests")
				case "wrong_producer":
					changed.Records[index].TestID = "github.com/electrikmilk/cherri:TestAlphaRenamingPasses"
				case "stale_source":
					changed.Records[index].ImplementationSHA = strings.Repeat("f", 40)
				case "invented_runtime":
					changed.Records[index].Tier = "ios-runtime"
				case "wrong_requirement":
					changed.Records[index].Requirements = []string{"T01"}
				case "missing_generation_tier":
					kept := make([]EvidenceRecord, 0)
					for _, r := range changed.Records {
						if r.Requirements[0] != id || r.Tier != "generation" {
							kept = append(kept, r)
						}
					}
					changed.Records = kept
				}
				rejected, validationErr := ValidateEvidenceManifest(&changed, selected, "local")
				if p15VerificationResult(t, rejected, id).Passed || rejected.ClosureComplete {
					t.Fatal("detached or incomplete schema receipt passed")
				}
				if fault != "missing_generation_tier" && validationErr == nil {
					t.Fatal("invalid schema receipt did not fail validation")
				}
			})
		}
	}
	auditPath := filepath.Join(t.TempDir(), "audit.json")
	if err := writeExecutionAudit(runner, all, auditPath); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatal(err)
	}
	var audit struct {
		ClosureComplete bool                `json:"closure_complete"`
		Rows            []ExecutionAuditRow `json:"rows"`
	}
	if err := json.Unmarshal(data, &audit); err != nil {
		t.Fatal(err)
	}
	if audit.ClosureComplete || len(audit.Rows) != 179 {
		t.Fatal("bounded schema run changed the original179 accounting")
	}
	for _, row := range audit.Rows {
		if wanted[row.ID] {
			scenario := supplementalProductionScenarios[row.ID]
			if row.Status != "PASS" || !row.PositiveAndNegative || !reflect.DeepEqual(row.TestNames, scenario.TestNames) || !reflect.DeepEqual(row.ProductionPath, scenario.ProductionPaths) {
				t.Fatalf("schema audit lost actual producer scope: %+v", row)
			}
		} else if row.Status == "PASS" || row.PositiveAndNegative {
			t.Fatalf("unexecuted row inherited schema evidence: %+v", row)
		}
	}
	status, statusErr := exec.Command("git", "-C", resolveRepoRoot(), "status", "--porcelain", "--untracked-files=normal").Output()
	receipt, err := json.Marshal(map[string]any{
		"source_sha": runner.implSHA, "source_tree_clean": statusErr == nil && len(status) == 0,
		"scope":  "RP06/RP07/BRG22/BRG23 execute complete actual detached-worktree generation, failures and consumer propagation; no linked-row, Apple runtime or release inheritance.",
		"report": report, "records": runner.records,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Log("P15_SCHEMA_PRODUCER_RECEIPT:" + string(receipt))
}
