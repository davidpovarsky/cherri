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

func TestCustomActionProductionEvidence(t *testing.T) {
	all, err := LoadAllContracts("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	selected := &ContractsSet{Acceptance: &ContractFile{}, Repair: &RepairContractFile{}, Gates: &GatesContractFile{}}
	for _, c := range all.Acceptance.Cases {
		if c.ID == "AC02" {
			selected.Acceptance.Cases = append(selected.Acceptance.Cases, c)
		}
	}
	selected.Acceptance.CaseCount = len(selected.Acceptance.Cases)
	if selected.Acceptance.CaseCount != 1 {
		t.Fatal("immutable AC02 contract missing")
	}
	runner := NewRunner()
	runner.runCustomActionRequirement()
	result := runner.results["AC02"]
	contract := selected.Acceptance.Cases[0]
	if len(runner.results) != 1 || len(runner.records) != 2 || len(runner.observations) != 0 || !result.Passed || result.Status != "PASSED" || result.Requirement != contract.Requirement || result.TestLevel != contract.MinimumTestLevel || !reflect.DeepEqual(result.ExecutedLevels, customActionScenario.Tiers) {
		t.Fatalf("incomplete public action execution: %+v", result)
	}
	for _, record := range runner.records {
		if !reflect.DeepEqual(record.Requirements, []string{"AC02"}) || record.ImplementationSHA != runner.implSHA || record.Result != "passed" || record.ExitCode != 0 {
			t.Fatalf("unbound production record: %+v", record)
		}
		if err := validateCustomActionRecord("AC02", record); err != nil {
			t.Fatal(err)
		}
		for _, name := range customActionScenario.TestNames {
			if !strings.Contains(record.Detail, name) {
				t.Fatalf("actual named test discovery missing %s", name)
			}
		}
	}
	manifest := &EvidenceManifest{SchemaVersion: "1", ImplementationSHA: runner.implSHA, Records: runner.records}
	report, err := ValidateEvidenceManifest(manifest, selected, "local")
	if err != nil || report.PassedCases != 1 || report.FailedCases != 0 || report.SkippedCases != 0 || report.ClosureComplete {
		t.Fatalf("bounded AC02 evidence invalid: %+v %v", report, err)
	}
	raw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	for _, fault := range []string{"partial_selection", "missing_assertion", "wrong_producer", "stale_source", "invented_runtime", "missing_native"} {
		t.Run(fault, func(t *testing.T) {
			var changed EvidenceManifest
			if err := json.Unmarshal(raw, &changed); err != nil {
				t.Fatal(err)
			}
			switch fault {
			case "partial_selection":
				changed.Records[0].Command = productionTestCommand(customActionScenario.TestNames[:len(customActionScenario.TestNames)-1])
			case "missing_assertion":
				delete(changed.Records[0].Assertions, "required_production_tests")
			case "wrong_producer":
				changed.Records[0].TestID = "tools/language-acceptance:runSchema"
			case "stale_source":
				changed.Records[0].ImplementationSHA = strings.Repeat("f", 40)
			case "invented_runtime":
				changed.Records[0].Tier = "ios-runtime"
			case "missing_native":
				var kept []EvidenceRecord
				for _, record := range changed.Records {
					if record.Tier != "native" {
						kept = append(kept, record)
					}
				}
				changed.Records = kept
			}
			rejected, err := ValidateEvidenceManifest(&changed, selected, "local")
			if p15VerificationResult(t, rejected, "AC02").Passed || rejected.ClosureComplete {
				t.Fatal("partial/corrupted custom-action proof minted a pass")
			}
			if fault != "missing_native" && err == nil {
				t.Fatal("invalid custom-action record accepted")
			}
		})
	}
	path := filepath.Join(t.TempDir(), "audit.json")
	if err := writeExecutionAudit(runner, all, path); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
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
		t.Fatal("changed full acceptance accounting")
	}
	for _, row := range audit.Rows {
		if row.ID == "AC02" {
			if row.Status != "PASS" || !row.PositiveAndNegative || !reflect.DeepEqual(row.TestNames, customActionScenario.TestNames) || !reflect.DeepEqual(row.ProductionPath, customActionScenario.ProductionPaths) {
				t.Fatalf("audit lost actual custom-action proof: %+v", row)
			}
		} else if row.Status == "PASS" || row.PositiveAndNegative {
			t.Fatalf("unexecuted requirement inherited custom-action proof: %+v", row)
		}
	}
	status, statusErr := exec.Command("git", "-C", resolveRepoRoot(), "status", "--porcelain", "--untracked-files=normal").Output()
	receipt, err := json.Marshal(map[string]any{"source_sha": runner.implSHA, "source_tree_clean": statusErr == nil && len(status) == 0, "scope": "Only AC02 public declarations through common schema and eight closed canonical codec identities; actual native-format outputs, CLI/mobile and registry/codec causal controls. No Apple runtime, plugin execution capability or release closure.", "report": report, "records": runner.records})
	if err != nil {
		t.Fatal(err)
	}
	t.Log("P15_CUSTOM_ACTION_RECEIPT:" + string(receipt))
}
