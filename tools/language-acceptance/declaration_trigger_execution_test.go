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

func TestDeclarationTriggerProductionEvidence(t *testing.T) {
	all, err := LoadAllContracts("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	selected := &ContractsSet{Acceptance: &ContractFile{}, Repair: &RepairContractFile{}, Gates: &GatesContractFile{}}
	for _, c := range all.Acceptance.Cases {
		if c.ID == "AC03" || c.ID == "M05" {
			selected.Acceptance.Cases = append(selected.Acceptance.Cases, c)
		}
	}
	selected.Acceptance.CaseCount = len(selected.Acceptance.Cases)
	if selected.Acceptance.CaseCount != 2 {
		t.Fatal("missing immutable declaration/trigger contracts")
	}
	runner := NewRunner()
	for _, id := range []string{"AC03", "M05"} {
		runner.runDeclarationTriggerRequirement(id)
	}
	if len(runner.results) != 2 || len(runner.records) != 3 || len(runner.observations) != 0 {
		t.Fatalf("unexpected scope: %d results, %d records, %d observations", len(runner.results), len(runner.records), len(runner.observations))
	}
	for _, c := range selected.Acceptance.Cases {
		result := runner.results[c.ID]
		scenario := declarationTriggerScenarios[c.ID]
		if !result.Passed || result.Status != "PASSED" || result.Requirement != c.Requirement || result.TestLevel != c.MinimumTestLevel || !reflect.DeepEqual(result.ExecutedLevels, scenario.Tiers) {
			t.Fatalf("unexecuted complete contract: %+v", result)
		}
	}
	for _, record := range runner.records {
		if len(record.Requirements) != 1 || record.Result != "passed" || record.ExitCode != 0 || record.ImplementationSHA != runner.implSHA {
			t.Fatalf("unbound production evidence: %+v", record)
		}
		id := record.Requirements[0]
		if err := validateDeclarationTriggerRecord(id, record); err != nil {
			t.Fatal(err)
		}
		for _, name := range declarationTriggerScenarios[id].TestNames {
			if !strings.Contains(record.Detail, name) {
				t.Fatalf("actual test discovery omitted %s", name)
			}
		}
	}
	manifest := &EvidenceManifest{SchemaVersion: "1", ImplementationSHA: runner.implSHA, Records: runner.records}
	report, err := ValidateEvidenceManifest(manifest, selected, "local")
	if err != nil || report.PassedCases != 2 || report.FailedCases != 0 || report.SkippedCases != 0 || report.ClosureComplete {
		t.Fatalf("bounded declaration/trigger evidence invalid: %+v %v", report, err)
	}
	raw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"AC03", "M05"} {
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
					t.Fatal("actual requirement evidence absent")
				}
				scenario := declarationTriggerScenarios[id]
				switch fault {
				case "partial_selection":
					changed.Records[index].Command = productionTestCommand(scenario.TestNames[:len(scenario.TestNames)-1])
				case "missing_assertion":
					delete(changed.Records[index].Assertions, "required_production_tests")
				case "wrong_producer":
					changed.Records[index].TestID = "tools/language-acceptance:runMetadata"
				case "stale_source":
					changed.Records[index].ImplementationSHA = strings.Repeat("f", 40)
				case "invented_runtime":
					changed.Records[index].Tier = "ios-runtime"
				case "missing_required_tier":
					var kept []EvidenceRecord
					tier := scenario.Tiers[len(scenario.Tiers)-1]
					for _, record := range changed.Records {
						if record.Requirements[0] != id || record.Tier != tier {
							kept = append(kept, record)
						}
					}
					changed.Records = kept
				}
				rejected, err := ValidateEvidenceManifest(&changed, selected, "local")
				if p15VerificationResult(t, rejected, id).Passed || rejected.ClosureComplete {
					t.Fatal("damaged declaration/trigger evidence minted a pass")
				}
				if fault != "missing_required_tier" && err == nil {
					t.Fatal("malformed evidence was not rejected")
				}
			})
		}
	}
	path := filepath.Join(t.TempDir(), "execution-audit.json")
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
		t.Fatal("changed full contract accounting")
	}
	for _, row := range audit.Rows {
		if scenario, ok := declarationTriggerScenarios[row.ID]; ok {
			if row.Status != "PASS" || !row.PositiveAndNegative || !reflect.DeepEqual(row.TestNames, scenario.TestNames) || !reflect.DeepEqual(row.ProductionPath, scenario.ProductionPaths) {
				t.Fatalf("audit lost production scope: %+v", row)
			}
		} else if row.Status == "PASS" || row.PositiveAndNegative {
			t.Fatalf("unexecuted requirement inherited a pass: %+v", row)
		}
	}
	status, statusErr := exec.Command("git", "-C", resolveRepoRoot(), "status", "--porcelain", "--untracked-files=normal").Output()
	receipt, err := json.Marshal(map[string]any{"source_sha": runner.implSHA, "source_tree_clean": statusErr == nil && len(status) == 0,
		"scope":  "Only AC03 hard rejection/generation and M05 actual seven-trigger serialized native data. No automation installation, Apple execution, AC02 success, or release closure.",
		"report": report, "records": runner.records})
	if err != nil {
		t.Fatal(err)
	}
	t.Log("P15_DECLARATION_TRIGGER_RECEIPT:" + string(receipt))
}
