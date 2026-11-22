package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func functionCombinedDefaultSelectedContracts(t *testing.T) (*ContractsSet, *ContractsSet) {
	t.Helper()
	var all, err = LoadAllContracts("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	var selected = &ContractsSet{Acceptance: &ContractFile{}, Repair: &RepairContractFile{}, Gates: &GatesContractFile{}}
	for _, row := range all.Repair.Cases {
		if row.ID == "RP25" {
			selected.Repair.Cases = append(selected.Repair.Cases, row)
		}
	}
	if len(selected.Repair.Cases) != 1 {
		t.Fatal("missing immutable RP25 contract")
	}
	selected.Repair.CaseCount = 1
	return all, selected
}

func TestFunctionCombinedDefaultProducerInventory(t *testing.T) {
	var _, selected = functionCombinedDefaultSelectedContracts(t)
	var row = selected.Repair.Cases[0]
	if row.Title != functionCombinedDefaultTitle || row.Scenario != functionCombinedDefaultScenario || row.ExpectedObservation != functionCombinedDefaultExpected || !reflect.DeepEqual(parseRequiredTiers(strings.Join(row.RequiredTestLevels, "+")), functionCombinedDefaultRequiredTiers) {
		t.Fatal("RP25 original scenario or tiers changed")
	}
	var names = functionCombinedDefaultRequiredEvents()
	if len(names) != 431 || !sort.StringsAreSorted(names) {
		t.Fatal("changed fixed typed default/transport inventory", len(names))
	}
	var seen = map[string]bool{}
	for _, name := range names {
		if name == "" || seen[name] {
			t.Fatal("invalid typed default/transport event", name)
		}
		seen[name] = true
	}
	for _, path := range functionCombinedDefaultProductionPaths {
		if seen[path] {
			t.Fatal("duplicate typed default/transport source witness", path)
		}
		seen[path] = true
		var info, err = os.Stat(filepath.Join(resolveRepoRoot(), filepath.FromSlash(path)))
		if err != nil || !info.Mode().IsRegular() {
			t.Fatal("missing source witness", path, err)
		}
	}
	if len(functionCombinedDefaultProductionPaths) != 72 || !reflect.DeepEqual(deriveRepairTestIDs(row.ID, row.Title), functionCombinedDefaultTestIDs()) || !reflect.DeepEqual(deriveRepairFixtures(row.ID), functionCombinedDefaultProductionPaths) {
		t.Fatal("complete typed default/transport producer mapping changed")
	}
	var ids []string
	var assertions, ui = map[string]string{}, map[string]string{}
	applyRuntimeAndUIMappings("RP25", functionCombinedDefaultRequiredTiers, &ids, assertions, ui)
	if len(ids) != 0 || len(assertions) != 0 || len(ui) != 0 {
		t.Fatal("historical short function trace supplied complete RP25 runtime evidence")
	}
	var runner = NewRunner()
	for _, id := range []string{"FN01", "FN02", "FN05", "RP24", "RP26", "BRG11", "BRG15"} {
		var foreign = row
		foreign.ID = id
		if runner.runFunctionCombinedDefaultRequirement(foreign) {
			t.Fatal("related requirement inherited RP25 producer", id)
		}
	}
	for _, field := range []string{"title", "scenario", "expected", "tiers"} {
		var changed = row
		switch field {
		case "title":
			changed.Title = "different scenario"
		case "scenario":
			changed.Scenario = "only function declaration"
		case "expected":
			changed.ExpectedObservation = "explicit None may select the fallback"
		case "tiers":
			changed.RequiredTestLevels = []string{"unit", "native"}
		}
		if !runner.runFunctionCombinedDefaultRequirement(changed) || runner.results["RP25"].Status != "FAILED" || len(runner.records) != 0 {
			t.Fatal("changed immutable scenario accepted", field)
		}
	}
}

func TestFunctionCombinedDefaultActualProductionEvidence(t *testing.T) {
	var all, selected = functionCombinedDefaultSelectedContracts(t)
	var runner = NewRunner()
	runner.artifactDir = os.Getenv("CHERRI_FUNCTION_DEFAULT_EVIDENCE_OUTPUT")
	runner.runRepairCases(selected.Repair)
	var result = runner.results["RP25"]
	if len(runner.results) != 1 || len(runner.records) != 2 || len(runner.observations) != 0 || result.Passed || result.Status != "PENDING_EXTERNAL" || !reflect.DeepEqual(result.ExecutedLevels, functionTransportTiers) {
		t.Fatalf("RP25 local production incomplete or expanded: %+v records=%d", runner.results, len(runner.records))
	}
	var manifest = &EvidenceManifest{SchemaVersion: "1", ImplementationSHA: runner.implSHA, Records: runner.records}
	var report, err = ValidateEvidenceManifest(manifest, selected, "local")
	if err != nil || report == nil || report.PassedCases != 0 || report.FailedCases != 0 || report.SkippedCases != 1 || report.ClosureComplete {
		t.Fatalf("RP25 local graphs closed missing Apple execution: %+v %v", report, err)
	}
	for _, original := range runner.records {
		if err := validateFunctionCombinedDefaultRecord("RP25", original); err != nil {
			t.Fatal(err)
		}
		if original.Tier != "unit" {
			continue
		}
		for _, fault := range []string{"borrowed_fn02", "borrowed_fn05", "fake_runtime", "partial_tests", "missing_presence_control", "missing_empty_default_control", "wrong_scenario_blob", "early_case_parent"} {
			t.Run(fault, func(t *testing.T) {
				var broken = editorContractCopyRecord(t, original)
				switch fault {
				case "borrowed_fn02":
					broken.TestID = "github.com/electrikmilk/cherri:TestP15FN02DefaultTyping"
				case "borrowed_fn05":
					broken.Requirements = []string{"FN05"}
				case "fake_runtime":
					broken.Tier = "ios-runtime"
					broken.TestID = "ios27-runtime-poc:functions"
				case "partial_tests":
					broken.Command = productionTestCommand(functionCombinedDefaultTests[:1])
				case "wrong_scenario_blob":
					editorContractMutateArtifact(t, &broken, functionCombinedDefaultPrefix+"-process.json", func(raw []byte) []byte {
						var receipt editorProcessReceipt
						if err := json.Unmarshal(raw, &receipt); err != nil {
							t.Fatal(err)
						}
						var changed = false
						for index := range receipt.SourceFiles {
							if receipt.SourceFiles[index].Path == "p15_combined_function_defaults_test.go" {
								receipt.SourceFiles[index].SHA256 = strings.Repeat("e", 64)
								changed = true
							}
						}
						if !changed {
							t.Fatal("RP25 scenario binding absent")
						}
						var out, _ = json.Marshal(receipt)
						return out
					})
				default:
					var target = functionCombinedDefaultTests[1] + "/optional_false/oracle/fallback_bool_as_number"
					if fault == "missing_empty_default_control" {
						target = functionCombinedDefaultTests[1] + "/empty_list_default/oracle/empty_inherits_input"
					}
					var changed []byte
					editorContractMutateArtifact(t, &broken, functionCombinedDefaultPrefix+"-go.jsonl", func(raw []byte) []byte {
						if fault == "early_case_parent" {
							changed = functionScenarioBrokenEvents(t, raw, fault, functionCombinedDefaultTests[1]+"/optional_false")
							return changed
						}
						var output bytes.Buffer
						for _, line := range bytes.Split(bytes.TrimSpace(raw), []byte{'\n'}) {
							var event struct{ Test string }
							if err := json.Unmarshal(line, &event); err != nil {
								t.Fatal(err)
							}
							if event.Test != target {
								output.Write(line)
								output.WriteByte('\n')
							}
						}
						changed = output.Bytes()
						if bytes.Equal(changed, raw) {
							t.Fatal("required RP25 event absent from healthy original")
						}
						return changed
					})
					editorContractMutateArtifact(t, &broken, functionCombinedDefaultPrefix+"-process.json", func(raw []byte) []byte {
						var receipt editorProcessReceipt
						if err := json.Unmarshal(raw, &receipt); err != nil {
							t.Fatal(err)
						}
						receipt.RawSHA256, receipt.RawBytes = editorSHA(changed), len(changed)
						var out, _ = json.Marshal(receipt)
						return out
					})
				}
				var err = validateFunctionCombinedDefaultRecord("RP25", broken)
				if err == nil {
					t.Fatal("broken RP25 evidence accepted", fault)
				}
				if fault == "early_case_parent" && !strings.Contains(err.Error(), "parent passed before required child") {
					t.Fatal("nested-parent control missed the intended causal guard", err)
				}
			})
		}
	}
	if final, err := ValidateEvidenceManifest(manifest, selected, "final"); err == nil || final != nil && final.ClosureComplete {
		t.Fatal("RP25 closed without its actual Apple result")
	}
	var directory = t.TempDir()
	if runner.artifactDir != "" {
		directory = runner.artifactDir
	}
	var auditPath = filepath.Join(directory, "function-default-audit.json")
	if err = writeExecutionAudit(runner, all, auditPath); err != nil {
		t.Fatal(err)
	}
	var raw, readErr = os.ReadFile(auditPath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	var audit struct {
		Rows            []ExecutionAuditRow `json:"rows"`
		ClosureComplete bool                `json:"closure_complete"`
	}
	if err = json.Unmarshal(raw, &audit); err != nil || len(audit.Rows) != 179 || audit.ClosureComplete {
		t.Fatal("RP25 changed immutable full179 accounting", err)
	}
	for _, row := range audit.Rows {
		if row.Status == "PASS" || row.ID != "RP25" && row.PositiveAndNegative {
			t.Fatal("RP25 local proof completed an unproved requirement", row.ID)
		}
		if row.ID == "RP25" && (!row.PositiveAndNegative || !reflect.DeepEqual(row.TestNames, functionCombinedDefaultTests) || !reflect.DeepEqual(row.ProductionPath, functionCombinedDefaultProductionPaths) || row.Scope != functionCombinedDefaultScope) {
			t.Fatal("actual typed default/transport scope lost")
		}
	}
	var proof, _ = json.MarshalIndent(map[string]any{"source": runner.implSHA, "report": report, "results": runner.results, "records": runner.records}, "", "  ")
	if err = os.WriteFile(filepath.Join(directory, "function-default-evidence.json"), append(proof, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
	t.Logf("RP25 executed all 431 fixed events on %s; only two local tiers, actual Apple result remains pending", runner.implSHA)
}
