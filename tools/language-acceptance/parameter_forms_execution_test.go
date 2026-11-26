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

func parameterFormsSelectedContracts(t *testing.T) (*ContractsSet, *ContractsSet) {
	t.Helper()
	var all, err = LoadAllContracts("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	var selected = &ContractsSet{Acceptance: &ContractFile{}, Repair: &RepairContractFile{}, Gates: &GatesContractFile{}}
	for _, row := range all.Gates.Gates {
		if row.ID == "BRG11" {
			selected.Gates.Gates = append(selected.Gates.Gates, row)
		}
	}
	if len(selected.Gates.Gates) != 1 {
		t.Fatal("missing immutable BRG11 contract")
	}
	selected.Gates.GateCount = 1
	return all, selected
}

func TestParameterFormsProducerInventory(t *testing.T) {
	var _, selected = parameterFormsSelectedContracts(t)
	var row = selected.Gates.Gates[0]
	if row.Title != parameterFormsTitle || row.Scenario != parameterFormsScenario || row.ExpectedObservation != parameterFormsExpected || !reflect.DeepEqual(parseRequiredTiers(strings.Join(row.MinimumEvidenceTiers, "+")), parameterFormsRequiredTiers) {
		t.Fatal("BRG11 original scenario or tiers changed")
	}
	var names = parameterFormsRequiredEvents()
	if len(names) != 80 || !sort.StringsAreSorted(names) {
		t.Fatal("changed fixed canonical parameter-forms inventory", len(names))
	}
	var seen = map[string]bool{}
	for _, name := range names {
		if name == "" || seen[name] {
			t.Fatal("invalid canonical parameter-forms event", name)
		}
		seen[name] = true
	}
	for _, path := range parameterFormsProductionPaths {
		if seen[path] {
			t.Fatal("duplicate canonical parameter-forms source witness", path)
		}
		seen[path] = true
		var info, err = os.Stat(filepath.Join(resolveRepoRoot(), filepath.FromSlash(path)))
		if err != nil || !info.Mode().IsRegular() {
			t.Fatal("missing source witness", path, err)
		}
	}
	if len(parameterFormsProductionPaths) != 85 || !seen["p15_tagged_boundary_alignment_test.go"] || !reflect.DeepEqual(deriveGateTestIDs(row.ID, row.ProposedTestPrefix), parameterFormsTestIDs()) || !reflect.DeepEqual(deriveGateFixtures(row.ID), parameterFormsProductionPaths) {
		t.Fatal("complete canonical parameter-forms producer mapping changed")
	}
	var ids []string
	var assertions, ui = map[string]string{}, map[string]string{}
	applyRuntimeAndUIMappings("BRG11", parameterFormsRequiredTiers, &ids, assertions, ui)
	if len(ids) != 0 || len(assertions) != 0 || len(ui) != 0 {
		t.Fatal("historical short function trace supplied complete BRG11 runtime evidence")
	}
	var runner = NewRunner()
	for _, id := range []string{"T01", "T02", "T03", "C02", "C03", "C04", "RP25", "BRG15"} {
		var foreign = row
		foreign.ID = id
		if runner.runParameterFormsRequirement(foreign) {
			t.Fatal("related requirement inherited BRG11 producer", id)
		}
	}
	for _, field := range []string{"title", "scenario", "expected", "tiers", "mandatory"} {
		var changed = row
		switch field {
		case "mandatory":
			changed.Mandatory = false
		case "title":
			changed.Title = "different scenario"
		case "scenario":
			changed.Scenario = "only function declaration"
		case "expected":
			changed.ExpectedObservation = "wrong forms may shift argument slots"
		case "tiers":
			changed.MinimumEvidenceTiers = []string{"unit", "native"}
		}
		if !runner.runParameterFormsRequirement(changed) || runner.results["BRG11"].Status != "FAILED" || len(runner.records) != 0 {
			t.Fatal("changed immutable scenario accepted", field)
		}
	}
}

func TestParameterFormsActualProductionEvidence(t *testing.T) {
	var all, selected = parameterFormsSelectedContracts(t)
	var runner = NewRunner()
	runner.artifactDir = os.Getenv("CHERRI_PARAMETER_FORMS_EVIDENCE_OUTPUT")
	runner.runRecoveryGates(selected.Gates)
	var result = runner.results["BRG11"]
	if len(runner.results) != 1 || len(runner.records) != 2 || len(runner.observations) != 0 || result.Passed || result.Status != "PENDING_EXTERNAL" || !reflect.DeepEqual(result.ExecutedLevels, functionTransportTiers) {
		t.Fatalf("BRG11 local production incomplete or expanded: %+v records=%d", runner.results, len(runner.records))
	}
	var manifest = &EvidenceManifest{SchemaVersion: "1", ImplementationSHA: runner.implSHA, Records: runner.records}
	var report, err = ValidateEvidenceManifest(manifest, selected, "local")
	if err != nil || report == nil || report.PassedCases != 0 || report.FailedCases != 0 || report.SkippedCases != 1 || report.ClosureComplete {
		t.Fatalf("BRG11 local graphs closed missing Apple execution: %+v %v", report, err)
	}
	for _, original := range runner.records {
		if err := validateParameterFormsRecord("BRG11", original); err != nil {
			t.Fatal(err)
		}
		if original.Tier != "unit" {
			continue
		}
		for _, fault := range []string{"borrowed_t01", "borrowed_rp25", "fake_runtime", "partial_tests", "missing_default_control", "missing_form_control", "wrong_scenario_blob"} {
			t.Run(fault, func(t *testing.T) {
				var broken = editorContractCopyRecord(t, original)
				switch fault {
				case "borrowed_t01":
					broken.TestID = "github.com/electrikmilk/cherri:TestP15T01TypedDefaultBoundaryGraphs"
				case "borrowed_rp25":
					broken.Requirements = []string{"RP25"}
				case "fake_runtime":
					broken.Tier = "ios-runtime"
					broken.TestID = "ios27-runtime-poc:functions"
				case "partial_tests":
					broken.Command = productionTestCommand(parameterFormsTests[:1])
				case "wrong_scenario_blob":
					editorContractMutateArtifact(t, &broken, parameterFormsPrefix+"-process.json", func(raw []byte) []byte {
						var receipt editorProcessReceipt
						if err := json.Unmarshal(raw, &receipt); err != nil {
							t.Fatal(err)
						}
						var changed = false
						for index := range receipt.SourceFiles {
							if receipt.SourceFiles[index].Path == "p15_combined_parameter_forms_test.go" {
								receipt.SourceFiles[index].SHA256 = strings.Repeat("e", 64)
								changed = true
							}
						}
						if !changed {
							t.Fatal("BRG11 scenario binding absent")
						}
						var out, _ = json.Marshal(receipt)
						return out
					})
				default:
					var target = parameterFormsTests[1] + "/defaults/flag/truthiness"
					if fault == "missing_form_control" {
						target = parameterFormsTests[1] + "/forms/token_as_literal"
					}
					var changed []byte
					editorContractMutateArtifact(t, &broken, parameterFormsPrefix+"-go.jsonl", func(raw []byte) []byte {
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
							t.Fatal("required BRG11 event absent from healthy original")
						}
						return changed
					})
					editorContractMutateArtifact(t, &broken, parameterFormsPrefix+"-process.json", func(raw []byte) []byte {
						var receipt editorProcessReceipt
						if err := json.Unmarshal(raw, &receipt); err != nil {
							t.Fatal(err)
						}
						receipt.RawSHA256, receipt.RawBytes = editorSHA(changed), len(changed)
						var out, _ = json.Marshal(receipt)
						return out
					})
				}
				if err := validateParameterFormsRecord("BRG11", broken); err == nil {
					t.Fatal("broken BRG11 evidence accepted", fault)
				}
			})
		}
	}
	if final, err := ValidateEvidenceManifest(manifest, selected, "final"); err == nil || final != nil && final.ClosureComplete {
		t.Fatal("BRG11 closed without its actual Apple result")
	}
	var directory = t.TempDir()
	if runner.artifactDir != "" {
		directory = runner.artifactDir
	}
	var auditPath = filepath.Join(directory, "parameter-forms-audit.json")
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
		t.Fatal("BRG11 changed immutable full179 accounting", err)
	}
	for _, row := range audit.Rows {
		if row.Status == "PASS" || row.ID != "BRG11" && row.PositiveAndNegative {
			t.Fatal("BRG11 local proof completed an unproved requirement", row.ID)
		}
		if row.ID == "BRG11" && (!row.PositiveAndNegative || !reflect.DeepEqual(row.TestNames, parameterFormsTests) || !reflect.DeepEqual(row.ProductionPath, parameterFormsProductionPaths) || row.Scope != parameterFormsScope) {
			t.Fatal("actual canonical parameter-forms scope lost")
		}
	}
	var proof, _ = json.MarshalIndent(map[string]any{"source": runner.implSHA, "report": report, "results": runner.results, "records": runner.records}, "", "  ")
	if err = os.WriteFile(filepath.Join(directory, "parameter-forms-evidence.json"), append(proof, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
	t.Logf("BRG11 executed all 80 fixed events on %s; only two local tiers, actual Apple result remains pending", runner.implSHA)
}
