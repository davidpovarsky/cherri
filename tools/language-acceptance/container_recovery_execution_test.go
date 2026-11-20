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

func TestContainerRecoveryProductionEvidence(t *testing.T) {
	all, err := LoadAllContracts("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	selected := &ContractsSet{Acceptance: &ContractFile{}, Repair: &RepairContractFile{}, Gates: &GatesContractFile{}}
	for _, gate := range all.Gates.Gates {
		if gate.ID == "BRG09" {
			selected.Gates.Gates = append(selected.Gates.Gates, gate)
		}
	}
	selected.Gates.GateCount = len(selected.Gates.Gates)
	if selected.Gates.GateCount != 1 || !reflect.DeepEqual(selected.Gates.Gates[0].MinimumEvidenceTiers, containerRecoveryScenario.Tiers) {
		t.Fatal("BRG09 immutable requirement or tier selection changed")
	}
	runner := NewRunner()
	runner.runRecoveryGates(selected.Gates)
	if len(runner.results) != 1 || len(runner.records) != 2 || len(runner.observations) != 0 {
		t.Fatal("BRG09 actual execution expanded into other requirements or omitted a tier")
	}
	result := runner.results["BRG09"]
	if !result.Passed || result.Status != "PASSED" || !reflect.DeepEqual(result.ExecutedLevels, containerRecoveryScenario.Tiers) {
		t.Fatalf("complete BRG09 production execution failed: %+v", result)
	}
	for _, path := range containerRecoveryScenario.ProductionPaths {
		info, err := os.Stat(filepath.Join(resolveRepoRoot(), path))
		if err != nil || !info.Mode().IsRegular() {
			t.Fatalf("actual production source path missing %s: %v", path, err)
		}
	}
	for _, record := range runner.records {
		if !reflect.DeepEqual(record.Requirements, []string{"BRG09"}) || record.ImplementationSHA != runner.implSHA || record.Result != "passed" || record.ExitCode != 0 || record.TestID != containerRecoveryProducer(record.Tier) {
			t.Fatalf("BRG09 receipt is detached from its executed source/tier: %+v", record)
		}
		if err := validateSupplementalProductionRecord("BRG09", record); err != nil {
			t.Fatal(err)
		}
		for _, name := range containerRecoveryScenario.TestNames {
			if !strings.Contains(record.Detail, name) {
				t.Fatalf("actual test discovery is missing %s", name)
			}
		}
	}
	manifest := &EvidenceManifest{SchemaVersion: "1", ImplementationSHA: runner.implSHA, Records: runner.records}
	report, err := ValidateEvidenceManifest(manifest, selected, "local")
	if err != nil || report.PassedCases != 1 || report.FailedCases != 0 || report.SkippedCases != 0 || report.ClosureComplete {
		t.Fatalf("BRG09 evidence rejected or expanded to closure: %+v %v", report, err)
	}
	encoded, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	for _, fault := range []string{"missing_structure", "missing_roundtrip", "wrong_producer", "partial_selection", "missing_assertion", "missing_scope", "stale_sha", "invented_runtime", "inherited_requirement", "added_requirement", "failed_result", "nonzero_exit"} {
		t.Run(fault, func(t *testing.T) {
			var changed EvidenceManifest
			if err := json.Unmarshal(encoded, &changed); err != nil {
				t.Fatal(err)
			}
			record := &changed.Records[0]
			switch fault {
			case "missing_structure", "missing_roundtrip":
				tier := "native-structure"
				if fault == "missing_roundtrip" {
					tier = "native-roundtrip"
				}
				for index, candidate := range changed.Records {
					if candidate.Tier == tier {
						changed.Records = append(changed.Records[:index], changed.Records[index+1:]...)
						break
					}
				}
			case "wrong_producer":
				tier := "native-roundtrip"
				if record.Tier == tier {
					tier = "native-structure"
				}
				record.TestID = containerRecoveryProducer(tier)
			case "partial_selection":
				record.Command = productionTestCommand(containerRecoveryScenario.TestNames[:1])
			case "missing_assertion":
				delete(record.Assertions, "required_production_tests")
			case "missing_scope":
				delete(record.Assertions, "container_scope")
			case "stale_sha":
				record.ImplementationSHA = strings.Repeat("f", 40)
			case "invented_runtime":
				record.Tier = "ios-runtime"
			case "inherited_requirement":
				record.Requirements = []string{"L01"}
			case "added_requirement":
				record.Requirements = []string{"BRG09", "L01"}
			case "failed_result":
				record.Result = "failed"
			case "nonzero_exit":
				record.ExitCode = 1
			}
			if _, err := ValidateEvidenceManifest(&changed, selected, "local"); err == nil {
				t.Fatal("incomplete or detached BRG09 evidence accepted: " + fault)
			}
		})
	}
	auditPath := filepath.Join(t.TempDir(), "container-audit.json")
	if err := writeExecutionAudit(runner, all, auditPath); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatal(err)
	}
	var audit moduleAuditEnvelope
	if err := json.Unmarshal(raw, &audit); err != nil {
		t.Fatal(err)
	}
	if audit.ClosureComplete || len(audit.Rows) != 179 {
		t.Fatal("BRG09 changed immutable 179-row audit accounting")
	}
	for _, row := range audit.Rows {
		if row.ID == "BRG09" {
			if row.Status != "PASS" || !row.PositiveAndNegative || !reflect.DeepEqual(row.TestNames, containerRecoveryScenario.TestNames) || !reflect.DeepEqual(row.ProductionPath, containerRecoveryScenario.ProductionPaths) {
				t.Fatalf("BRG09 audit lost actual source/selection: %+v", row)
			}
		} else if row.Status == "PASS" || row.PositiveAndNegative {
			t.Fatalf("unexecuted requirement inherited BRG09: %s", row.ID)
		}
	}
	status, statusErr := exec.Command("git", "-C", resolveRepoRoot(), "status", "--porcelain", "--untracked-files=normal").Output()
	receipt, err := json.Marshal(map[string]any{
		"source_sha": runner.implSHA, "source_tree_clean": statusErr == nil && len(status) == 0,
		"scope": containerRecoveryScope, "report": report, "records": runner.records,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Log("P15_CONTAINER_RECOVERY_RECEIPT:" + string(receipt))
}
