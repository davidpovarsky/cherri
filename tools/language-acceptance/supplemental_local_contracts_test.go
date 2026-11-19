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

func TestSupplementalLocalContractsProductionEvidence(t *testing.T) {
	all, err := LoadAllContracts("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	wanted := map[string]bool{"RP05": true, "RP09": true, "RP10": true, "RP30": true, "RP31": true, "RP32": true, "BRG05": true, "BRG17": true}
	selected := &ContractsSet{Acceptance: &ContractFile{}, Repair: &RepairContractFile{}, Gates: &GatesContractFile{}}
	for _, scenario := range all.Repair.Cases {
		if wanted[scenario.ID] {
			selected.Repair.Cases = append(selected.Repair.Cases, scenario)
		}
	}
	for _, scenario := range all.Gates.Gates {
		if wanted[scenario.ID] {
			selected.Gates.Gates = append(selected.Gates.Gates, scenario)
		}
	}
	selected.Repair.CaseCount = len(selected.Repair.Cases)
	selected.Gates.GateCount = len(selected.Gates.Gates)
	if selected.Repair.CaseCount != 6 || selected.Gates.GateCount != 2 {
		t.Fatal("eight distinct immutable contracts not selected")
	}
	runner := NewRunner()
	runner.runRepairCases(selected.Repair)
	runner.runRecoveryGates(selected.Gates)
	if len(runner.results) != 8 || len(runner.records) != 17 || len(runner.observations) != 0 {
		t.Fatalf("local scope mismatch: results=%d records=%d observations=%d", len(runner.results), len(runner.records), len(runner.observations))
	}
	for id, result := range runner.results {
		scenario := supplementalProductionScenarios[id]
		if !wanted[id] || !result.Passed || result.Status != "PASSED" || !reflect.DeepEqual(result.ExecutedLevels, scenario.Tiers) {
			t.Fatalf("incomplete scenario %s: %+v", id, result)
		}
	}
	for _, record := range runner.records {
		if len(record.Requirements) != 1 || record.Result != "passed" || record.ExitCode != 0 || record.ImplementationSHA != runner.implSHA {
			t.Fatalf("unbound source result: %+v", record)
		}
		id := record.Requirements[0]
		if err := validateSupplementalProductionRecord(id, record); err != nil {
			t.Fatal(err)
		}
		for _, name := range supplementalProductionScenarios[id].TestNames {
			if !strings.Contains(record.Detail, name) {
				t.Fatalf("actual named production discovery missing %s", name)
			}
		}
	}
	manifest := &EvidenceManifest{SchemaVersion: "1", ImplementationSHA: runner.implSHA, Records: runner.records}
	report, err := ValidateEvidenceManifest(manifest, selected, "local")
	if err != nil || report.PassedCases != 8 || report.FailedCases != 0 || report.SkippedCases != 0 || report.ClosureComplete {
		t.Fatalf("bounded source report rejected or promoted to release: %+v %v", report, err)
	}
	raw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"RP05", "RP09", "RP10", "RP30", "RP31", "RP32", "BRG05", "BRG17"} {
		for _, fault := range []string{"partial_selection", "missing_assertion", "wrong_producer", "stale_source", "invented_runtime", "missing_required_tier"} {
			t.Run(id+"/"+fault, func(t *testing.T) {
				var changed EvidenceManifest
				if err := json.Unmarshal(raw, &changed); err != nil {
					t.Fatal(err)
				}
				index := -1
				for i, record := range changed.Records {
					if record.Requirements[0] == id {
						index = i
						break
					}
				}
				if index < 0 {
					t.Fatal("scenario has no actual evidence")
				}
				scenario := supplementalProductionScenarios[id]
				switch fault {
				case "partial_selection":
					names := scenario.TestNames[:len(scenario.TestNames)-1]
					changed.Records[index].Command = productionTestCommand(names)
				case "missing_assertion":
					delete(changed.Records[index].Assertions, "required_production_tests")
				case "wrong_producer":
					changed.Records[index].TestID = "github.com/electrikmilk/cherri:TestSupplementalUnrelated"
				case "stale_source":
					changed.Records[index].ImplementationSHA = strings.Repeat("f", 40)
				case "invented_runtime":
					changed.Records[index].Tier = "ios-runtime"
				case "missing_required_tier":
					var kept []EvidenceRecord
					removed := scenario.Tiers[len(scenario.Tiers)-1]
					for _, record := range changed.Records {
						if record.Requirements[0] != id || record.Tier != removed {
							kept = append(kept, record)
						}
					}
					changed.Records = kept
				}
				rejected, err := ValidateEvidenceManifest(&changed, selected, "local")
				if p15VerificationResult(t, rejected, id).Passed || rejected.ClosureComplete {
					t.Fatal("damaged source evidence satisfied complete scenario")
				}
				if fault != "missing_required_tier" && err == nil {
					t.Fatal("malformed manifest did not fail validation")
				}
			})
		}
	}
	auditPath := filepath.Join(t.TempDir(), "execution-audit.json")
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
		t.Fatal("local selection changed immutable179 accounting")
	}
	for _, row := range audit.Rows {
		if wanted[row.ID] {
			scenario := supplementalProductionScenarios[row.ID]
			if row.Status != "PASS" || !row.PositiveAndNegative || !reflect.DeepEqual(row.TestNames, scenario.TestNames) || !reflect.DeepEqual(row.ProductionPath, scenario.ProductionPaths) {
				t.Fatalf("audit lost executed scenario %+v", row)
			}
		} else if row.Status == "PASS" || row.PositiveAndNegative {
			t.Fatalf("unexecuted requirement inherited pass: %+v", row)
		}
	}
	status, statusErr := exec.Command("git", "-C", resolveRepoRoot(), "status", "--porcelain", "--untracked-files=normal").Output()
	receipt, err := json.Marshal(map[string]any{
		"source_sha": runner.implSHA, "source_tree_clean": statusErr == nil && len(status) == 0,
		"scope":  "Eight independently executed immutable local contracts: RP05/RP09/RP10/RP30/RP31/RP32/BRG05/BRG17. Actual CLI/mobile/service, canonical native differential, metadata/setup/trigger exact roundtrip and causal controls. Native format data is not Apple runtime or automation installation. RP20 remains unverified. No release closure.",
		"report": report, "records": runner.records,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Log("P15_LOCAL_CONTRACT_RECEIPT:" + string(receipt))
}
