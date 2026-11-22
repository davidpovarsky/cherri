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

func functionReturnSelectedContracts(t *testing.T) (*ContractsSet, *ContractsSet) {
	t.Helper()
	var all, err = LoadAllContracts("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	var selected = &ContractsSet{Acceptance: &ContractFile{}, Repair: &RepairContractFile{}, Gates: &GatesContractFile{}}
	for _, row := range all.Repair.Cases {
		if row.ID == "RP24" {
			selected.Repair.Cases = append(selected.Repair.Cases, row)
		}
	}
	if len(selected.Repair.Cases) != 1 {
		t.Fatal("missing immutable RP24 contract")
	}
	selected.Repair.CaseCount = 1
	return all, selected
}

func TestFunctionReturnProducerInventory(t *testing.T) {
	var _, selected = functionReturnSelectedContracts(t)
	var row = selected.Repair.Cases[0]
	if row.Title != functionReturnTitle || row.Scenario != functionReturnScenario || row.ExpectedObservation != functionReturnExpected || !reflect.DeepEqual(parseRequiredTiers(strings.Join(row.RequiredTestLevels, "+")), functionReturnRequiredTiers) {
		t.Fatal("RP24 original scenario or tiers changed")
	}
	var names = functionReturnRequiredEvents()
	if len(names) != 40 || !sort.StringsAreSorted(names) {
		t.Fatal("changed fixed return/recursion inventory", len(names))
	}
	var seen = map[string]bool{}
	for _, name := range names {
		if name == "" || seen[name] {
			t.Fatal("invalid return/recursion event", name)
		}
		seen[name] = true
	}
	for _, path := range functionReturnProductionPaths {
		if seen[path] {
			t.Fatal("duplicate return/recursion source witness", path)
		}
		seen[path] = true
		var info, err = os.Stat(filepath.Join(resolveRepoRoot(), filepath.FromSlash(path)))
		if err != nil || !info.Mode().IsRegular() {
			t.Fatal("missing source witness", path, err)
		}
	}
	if len(functionReturnProductionPaths) != 72 || !reflect.DeepEqual(deriveRepairTestIDs(row.ID, row.Title), functionReturnTestIDs()) || !reflect.DeepEqual(deriveRepairFixtures(row.ID), functionReturnProductionPaths) {
		t.Fatal("complete return/recursion producer mapping changed")
	}
	var ids []string
	var assertions, ui = map[string]string{}, map[string]string{}
	applyRuntimeAndUIMappings("RP24", functionReturnRequiredTiers, &ids, assertions, ui)
	if len(ids) != 0 || len(assertions) != 0 || len(ui) != 0 {
		t.Fatal("historical short function trace supplied complete RP24 runtime evidence")
	}
	var runner = NewRunner()
	for _, id := range []string{"FN01", "FN03", "FN05", "RP25", "RP26", "BRG11", "BRG15"} {
		var foreign = row
		foreign.ID = id
		if runner.runFunctionReturnRequirement(foreign) {
			t.Fatal("related requirement inherited RP24 producer", id)
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
			changed.ExpectedObservation = "no continuation needed"
		case "tiers":
			changed.RequiredTestLevels = []string{"unit", "native"}
		}
		if !runner.runFunctionReturnRequirement(changed) || runner.results["RP24"].Status != "FAILED" || len(runner.records) != 0 {
			t.Fatal("changed immutable scenario accepted", field)
		}
	}
}

func TestFunctionReturnActualProductionEvidence(t *testing.T) {
	var all, selected = functionReturnSelectedContracts(t)
	var runner = NewRunner()
	runner.artifactDir = os.Getenv("CHERRI_FUNCTION_RETURN_EVIDENCE_OUTPUT")
	runner.runRepairCases(selected.Repair)
	var result = runner.results["RP24"]
	if len(runner.results) != 1 || len(runner.records) != 2 || len(runner.observations) != 0 || result.Passed || result.Status != "PENDING_EXTERNAL" || !reflect.DeepEqual(result.ExecutedLevels, functionTransportTiers) {
		t.Fatalf("RP24 local production incomplete or expanded: %+v records=%d", runner.results, len(runner.records))
	}
	var manifest = &EvidenceManifest{SchemaVersion: "1", ImplementationSHA: runner.implSHA, Records: runner.records}
	var report, err = ValidateEvidenceManifest(manifest, selected, "local")
	if err != nil || report == nil || report.PassedCases != 0 || report.FailedCases != 0 || report.SkippedCases != 1 || report.ClosureComplete {
		t.Fatalf("RP24 local graphs closed missing Apple execution: %+v %v", report, err)
	}
	for _, original := range runner.records {
		if err := validateFunctionReturnRecord("RP24", original); err != nil {
			t.Fatal(err)
		}
		if original.Tier != "unit" {
			continue
		}
		for _, fault := range []string{"borrowed_fn01", "borrowed_fn05", "fake_runtime", "partial_tests", "missing_recursion_control", "missing_capture_control", "wrong_fixture_blob"} {
			t.Run(fault, func(t *testing.T) {
				var broken = editorContractCopyRecord(t, original)
				switch fault {
				case "borrowed_fn01":
					broken.TestID = "github.com/electrikmilk/cherri:TestP15FN01FunctionTyping"
				case "borrowed_fn05":
					broken.Requirements = []string{"FN05"}
				case "fake_runtime":
					broken.Tier = "ios-runtime"
					broken.TestID = "ios27-runtime-poc:functions"
				case "partial_tests":
					broken.Command = productionTestCommand(functionReturnTests[:1])
				case "wrong_fixture_blob":
					editorContractMutateArtifact(t, &broken, functionReturnPrefix+"-process.json", func(raw []byte) []byte {
						var receipt editorProcessReceipt
						if err := json.Unmarshal(raw, &receipt); err != nil {
							t.Fatal(err)
						}
						var changed = false
						for index := range receipt.SourceFiles {
							if receipt.SourceFiles[index].Path == "tests/language-v2/v2-rp24-return-recursion.cherri" {
								receipt.SourceFiles[index].SHA256 = strings.Repeat("e", 64)
								changed = true
							}
						}
						if !changed {
							t.Fatal("RP24 fixture binding absent")
						}
						var out, _ = json.Marshal(receipt)
						return out
					})
				default:
					var target = functionReturnTests[1] + "/oracle/decrement_operator"
					if fault == "missing_capture_control" {
						target = functionReturnTests[0] + "/mutable_capture"
					}
					var changed []byte
					editorContractMutateArtifact(t, &broken, functionReturnPrefix+"-go.jsonl", func(raw []byte) []byte {
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
							t.Fatal("required RP24 event absent from healthy original")
						}
						return changed
					})
					editorContractMutateArtifact(t, &broken, functionReturnPrefix+"-process.json", func(raw []byte) []byte {
						var receipt editorProcessReceipt
						if err := json.Unmarshal(raw, &receipt); err != nil {
							t.Fatal(err)
						}
						receipt.RawSHA256, receipt.RawBytes = editorSHA(changed), len(changed)
						var out, _ = json.Marshal(receipt)
						return out
					})
				}
				if err := validateFunctionReturnRecord("RP24", broken); err == nil {
					t.Fatal("broken RP24 evidence accepted", fault)
				}
			})
		}
	}
	if final, err := ValidateEvidenceManifest(manifest, selected, "final"); err == nil || final != nil && final.ClosureComplete {
		t.Fatal("RP24 closed without its actual Apple result")
	}
	var directory = t.TempDir()
	if runner.artifactDir != "" {
		directory = runner.artifactDir
	}
	var auditPath = filepath.Join(directory, "function-return-audit.json")
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
		t.Fatal("RP24 changed immutable full179 accounting", err)
	}
	for _, row := range audit.Rows {
		if row.Status == "PASS" || row.ID != "RP24" && row.PositiveAndNegative {
			t.Fatal("RP24 local proof completed an unproved requirement", row.ID)
		}
		if row.ID == "RP24" && (!row.PositiveAndNegative || !reflect.DeepEqual(row.TestNames, functionReturnTests) || !reflect.DeepEqual(row.ProductionPath, functionReturnProductionPaths) || row.Scope != functionReturnScope) {
			t.Fatal("actual return/recursion scope lost")
		}
	}
	var proof, _ = json.MarshalIndent(map[string]any{"source": runner.implSHA, "report": report, "results": runner.results, "records": runner.records}, "", "  ")
	if err = os.WriteFile(filepath.Join(directory, "function-return-evidence.json"), append(proof, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
	t.Logf("RP24 executed all40 fixed events on %s; only two local tiers, actual Apple result remains pending", runner.implSHA)
}
