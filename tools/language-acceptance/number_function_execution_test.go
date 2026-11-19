package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestNumberFunctionProducerInventory(t *testing.T) {
	var all, err = LoadAllContracts("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	var seen = map[string]bool{}
	for _, contract := range all.Acceptance.Cases {
		var scenario, present = numberFunctionScenarios[contract.ID]
		if !present {
			continue
		}
		seen[contract.ID] = true
		if scenario.Group != contract.Group || scenario.Title != contract.Requirement ||
			!reflect.DeepEqual(parseRequiredTiers(contract.MinimumTestLevel), []string{"unit", "native-structure", "ios-runtime"}) {
			t.Fatal("numeric/function producer changed its immutable contract", contract.ID)
		}
		if _, smoke := incompleteLocalAssertions[contract.ID]; smoke {
			t.Fatal("executed producer remains a historical smoke probe", contract.ID)
		}
		if !reflect.DeepEqual(deriveAcceptanceTestIDs(contract.ID, contract.Group), scenario.testIDs()) {
			t.Fatal("numeric/function producer IDs detached", contract.ID)
		}
		for _, path := range scenario.ProductionPaths {
			if info, err := os.Stat(filepath.Join(resolveRepoRoot(), filepath.FromSlash(path))); err != nil || !info.Mode().IsRegular() {
				t.Fatal("missing actual numeric/function implementation path", path, err)
			}
		}
	}
	if len(seen) != len(numberFunctionScenarios) || !seen["N01"] || !seen["FN01"] || !seen["FN02"] || !seen["FN03"] {
		t.Fatal("missing numeric/function production scenario")
	}
	var runner = NewRunner()
	for _, id := range []string{"RP15", "RP17", "RP24", "RP25", "RP26", "BRG11", "BRG15", "BRG16", "FN04", "FN05", "T01"} {
		runner.runNumberFunctionRequirement(id)
	}
	if len(runner.records) != 0 || len(runner.results) != 0 || len(runner.observations) != 0 {
		t.Fatal("related requirement inherited numeric/function execution")
	}
}

func TestNumberFunctionActualProductionEvidence(t *testing.T) {
	var all, err = LoadAllContracts("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	var selected = &ContractsSet{Acceptance: &ContractFile{}, Repair: &RepairContractFile{}, Gates: &GatesContractFile{}}
	var runner = NewRunner()
	for _, contract := range all.Acceptance.Cases {
		var scenario, present = numberFunctionScenarios[contract.ID]
		if !present {
			continue
		}
		selected.Acceptance.Cases = append(selected.Acceptance.Cases, contract)
		runner.runNumberFunctionRequirement(contract.ID)
		var result = runner.results[contract.ID]
		if result.Passed || result.Status != "PENDING_EXTERNAL" || result.Group != scenario.Group ||
			result.Requirement != scenario.Title || result.TestLevel != contract.MinimumTestLevel ||
			!reflect.DeepEqual(result.ExecutedLevels, []string{"unit", "native-structure"}) {
			t.Fatalf("actual numeric/function scope lost or expanded: %+v", result)
		}
	}
	selected.Acceptance.CaseCount = len(selected.Acceptance.Cases)
	if selected.Acceptance.CaseCount != len(numberFunctionScenarios) || len(runner.results) != len(numberFunctionScenarios) ||
		len(runner.records) != 2*len(numberFunctionScenarios) || len(runner.observations) != 0 {
		t.Fatal("numeric/function accounting drifted")
	}
	for _, record := range runner.records {
		if len(record.Requirements) != 1 || record.Result != "passed" || record.ExitCode != 0 || record.ImplementationSHA != runner.implSHA {
			t.Fatal("invalid actual numeric/function process receipt")
		}
		var id = record.Requirements[0]
		if err := validateNumberFunctionRecord(id, record); err != nil {
			t.Fatal(err)
		}
		for _, name := range numberFunctionScenarios[id].tests() {
			if !strings.Contains(record.Detail, name) {
				t.Fatal("missing successful required test", name)
			}
		}
	}
	var manifest = &EvidenceManifest{SchemaVersion: "1", ImplementationSHA: runner.implSHA, Records: runner.records}
	var report, validateErr = ValidateEvidenceManifest(manifest, selected, "local")
	if validateErr != nil || report.PassedCases != 0 || report.FailedCases != 0 || report.SkippedCases != len(numberFunctionScenarios) || report.ClosureComplete {
		t.Fatal("local graph or diagnostics falsely closed Apple runtime", report, validateErr)
	}
	if final, err := ValidateEvidenceManifest(manifest, selected, "final"); err == nil || final != nil && final.ClosureComplete {
		t.Fatal("missing Apple execution passed final acceptance")
	}
	var encoded, marshalErr = json.Marshal(manifest)
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}
	for _, record := range runner.records {
		t.Run(record.Requirements[0]+"/"+record.Tier, func(t *testing.T) {
			for _, fault := range []string{"wrong_producer", "fake_ios", "borrowed_id", "truncated_command", "missing_test_binding", "wrong_scope", "stale_source", "failed_process", "missing_tier"} {
				t.Run(fault, func(t *testing.T) {
					var broken EvidenceManifest
					if err := json.Unmarshal(encoded, &broken); err != nil {
						t.Fatal(err)
					}
					for index := range broken.Records {
						var changed = &broken.Records[index]
						if changed.EvidenceID != record.EvidenceID {
							continue
						}
						switch fault {
						case "wrong_producer":
							changed.TestID = "tools/language-acceptance:runNumbers"
						case "fake_ios":
							changed.Tier = "ios-runtime"
						case "borrowed_id":
							changed.Requirements = append(changed.Requirements, "T01")
						case "truncated_command":
							changed.Command = productionTestCommand(numberFunctionScenarios[record.Requirements[0]].tests()[:1])
						case "missing_test_binding":
							delete(changed.Assertions, "required_production_tests")
						case "wrong_scope":
							changed.Assertions["scope"] = "actual Apple execution"
						case "stale_source":
							changed.ImplementationSHA = strings.Repeat("e", 40)
						case "failed_process":
							changed.ExitCode = 1
						case "missing_tier":
							broken.Records = append(broken.Records[:index], broken.Records[index+1:]...)
						}
						break
					}
					var invalid, err = ValidateEvidenceManifest(&broken, selected, "local")
					if err == nil || invalid == nil || invalid.ClosureComplete || invalid.PassedCases != 0 {
						t.Fatal("damaged numeric/function evidence accepted", fault, err)
					}
				})
			}
		})
	}
	var auditPath = filepath.Join(t.TempDir(), "audit.json")
	if err := writeExecutionAudit(runner, all, auditPath); err != nil {
		t.Fatal(err)
	}
	var raw, readErr = os.ReadFile(auditPath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	var audit bindingCaptureAuditEnvelope
	if err := json.Unmarshal(raw, &audit); err != nil || len(audit.Rows) != 179 || audit.ClosureComplete {
		t.Fatal("numeric/function producer altered the full179 scope", err)
	}
	for _, row := range audit.Rows {
		if scenario, found := numberFunctionScenarios[row.ID]; found {
			if row.Status != "BLOCKED_EXTERNAL" || !row.PositiveAndNegative || row.Blocker == "" ||
				!reflect.DeepEqual(row.TestNames, scenario.tests()) || !reflect.DeepEqual(row.ProductionPath, scenario.ProductionPaths) ||
				!reflect.DeepEqual(row.Tier, []string{"unit", "native-structure"}) {
				t.Fatalf("numeric/function audit lost its actual local scope: %+v", row)
			}
		} else if row.Status == "PASS" {
			t.Fatal("unexecuted requirement inherited numeric/function evidence", row.ID)
		}
	}
	var receipt, receiptErr = json.Marshal(map[string]any{"source_sha": runner.implSHA, "scope": "Actual local numeric/function unit and native-structure tests only; Apple execution remains required", "report": report, "records": runner.records})
	if receiptErr != nil {
		t.Fatal(receiptErr)
	}
	t.Log("P15_NUMBER_FUNCTION_PRODUCTION_RECEIPT:" + string(receipt))
}
