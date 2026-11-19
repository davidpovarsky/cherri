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

func TestStdfuncSupplementalProductionEvidence(t *testing.T) {
	var all, err = LoadAllContracts("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	var selected = &ContractsSet{Acceptance: &ContractFile{}, Repair: &RepairContractFile{}, Gates: &GatesContractFile{}}
	for _, contract := range all.Repair.Cases {
		if contract.ID == "RP28" {
			selected.Repair.Cases = append(selected.Repair.Cases, contract)
		}
	}
	for _, contract := range all.Gates.Gates {
		if contract.ID == "BRG20" {
			selected.Gates.Gates = append(selected.Gates.Gates, contract)
		}
	}
	selected.Repair.CaseCount = len(selected.Repair.Cases)
	selected.Gates.GateCount = len(selected.Gates.Gates)
	if selected.Repair.CaseCount != 1 || selected.Gates.GateCount != 1 {
		t.Fatal("immutable stdfunc scenario selection changed")
	}
	var runner = NewRunner()
	runner.runRepairCases(selected.Repair)
	runner.runRecoveryGates(selected.Gates)
	if len(runner.results) != 2 || len(runner.records) != 5 || len(runner.observations) != 0 {
		t.Fatalf("supplemental stdfunc scope changed: results=%d records=%d observations=%d",
			len(runner.results), len(runner.records), len(runner.observations))
	}
	for _, id := range []string{"RP28", "BRG20"} {
		var result = runner.results[id]
		var scenario = supplementalProductionScenarios[id]
		if !result.Passed || result.Status != "PASSED" || !reflect.DeepEqual(result.ExecutedLevels, scenario.Tiers) {
			t.Fatalf("complete %s production scenario failed: %+v", id, result)
		}
		for _, path := range scenario.ProductionPaths {
			var info, err = os.Stat(filepath.Join(resolveRepoRoot(), path))
			if err != nil || !info.Mode().IsRegular() {
				t.Fatalf("required real production path missing %s: %v", path, err)
			}
		}
	}
	for _, record := range runner.records {
		if len(record.Requirements) != 1 || record.ImplementationSHA != runner.implSHA || record.Result != "passed" || record.ExitCode != 0 {
			t.Fatalf("invalid actual production receipt: %+v", record)
		}
		var id = record.Requirements[0]
		if err := validateSupplementalProductionRecord(id, record); err != nil {
			t.Fatal(err)
		}
		for _, name := range supplementalProductionScenarios[id].TestNames {
			if !strings.Contains(record.Detail, name) {
				t.Fatalf("%s receipt omitted actual test discovery %s", id, name)
			}
		}
	}
	var manifest = &EvidenceManifest{SchemaVersion: "1", ImplementationSHA: runner.implSHA, Records: runner.records}
	var report, reportErr = ValidateEvidenceManifest(manifest, selected, "local")
	if reportErr != nil || report.PassedCases != 2 || report.FailedCases != 0 || report.SkippedCases != 0 || report.ClosureComplete {
		t.Fatalf("stdfunc supplemental evidence rejected or expanded: %+v %v", report, reportErr)
	}
	var encoded, marshalErr = json.Marshal(manifest)
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}
	for _, id := range []string{"RP28", "BRG20"} {
		for _, mutation := range []string{"missing_tier", "wrong_producer", "partial_selection", "missing_assertion", "stale_sha", "invented_runtime", "inherited_requirement"} {
			t.Run(id+"/"+mutation, func(t *testing.T) {
				var changed EvidenceManifest
				if err := json.Unmarshal(encoded, &changed); err != nil {
					t.Fatal(err)
				}
				var target = -1
				for index, record := range changed.Records {
					if record.Requirements[0] == id {
						target = index
					}
				}
				if target < 0 {
					t.Fatal("required independent scenario receipt missing")
				}
				var record = &changed.Records[target]
				switch mutation {
				case "missing_tier":
					changed.Records = append(changed.Records[:target], changed.Records[target+1:]...)
				case "wrong_producer":
					record.TestID = "github.com/electrikmilk/cherri:TestP15IM03DirectServiceWithoutMain"
				case "partial_selection":
					record.Command = productionTestCommand(supplementalProductionScenarios[id].TestNames[:1])
				case "missing_assertion":
					delete(record.Assertions, "required_production_tests")
				case "stale_sha":
					record.ImplementationSHA = strings.Repeat("f", 40)
				case "invented_runtime":
					record.Tier = "ios-runtime"
				case "inherited_requirement":
					record.Requirements = []string{"IM03"}
				}
				if _, err := ValidateEvidenceManifest(&changed, selected, "local"); err == nil {
					t.Fatal("incomplete or detached stdfunc evidence was accepted")
				}
			})
		}
	}
	var auditPath = filepath.Join(t.TempDir(), "stdfunc-audit.json")
	if err := writeExecutionAudit(runner, all, auditPath); err != nil {
		t.Fatal(err)
	}
	var raw, readErr = os.ReadFile(auditPath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	var audit moduleAuditEnvelope
	if err := json.Unmarshal(raw, &audit); err != nil {
		t.Fatal(err)
	}
	if audit.ClosureComplete || len(audit.Rows) != 179 {
		t.Fatal("supplemental stdfunc proof changed immutable 179-row accounting")
	}
	for _, row := range audit.Rows {
		if row.ID == "RP28" || row.ID == "BRG20" {
			var scenario = supplementalProductionScenarios[row.ID]
			if row.Status != "PASS" || !row.PositiveAndNegative || !reflect.DeepEqual(row.TestNames, scenario.TestNames) ||
				!reflect.DeepEqual(row.ProductionPath, scenario.ProductionPaths) {
				t.Fatalf("stdfunc audit detached from executed scenario: %+v", row)
			}
		} else if row.Status == "PASS" || row.PositiveAndNegative {
			t.Fatalf("unexecuted requirement inherited supplemental success: %s", row.ID)
		}
	}
	var status, statusErr = exec.Command("git", "-C", resolveRepoRoot(), "status", "--porcelain", "--untracked-files=normal").Output()
	var receipt, receiptErr = json.Marshal(map[string]any{
		"source_sha": runner.implSHA, "source_tree_clean": statusErr == nil && len(status) == 0,
		"scope":  "RP28 actual generator and raw native stdfunc contracts; BRG20 independently executes actual aliases/cycles/runtime-init/include migration/service/native scenarios. Native means serialized structure; no Apple execution, linked-ID inheritance or release closure.",
		"report": report, "records": runner.records,
	})
	if receiptErr != nil {
		t.Fatal(receiptErr)
	}
	t.Log("P15_STDFUNC_SUPPLEMENTAL_RECEIPT:" + string(receipt))
}
