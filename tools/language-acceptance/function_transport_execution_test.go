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

func functionTransportSelectedContracts(t *testing.T) (*ContractsSet, *ContractsSet) {
	t.Helper()
	var all, err = LoadAllContracts("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	var selected = &ContractsSet{Acceptance: &ContractFile{}, Repair: &RepairContractFile{}, Gates: &GatesContractFile{}}
	for _, row := range all.Acceptance.Cases {
		if row.ID == "FN05" {
			selected.Acceptance.Cases = append(selected.Acceptance.Cases, row)
		}
	}
	if len(selected.Acceptance.Cases) != 1 {
		t.Fatal("missing immutable FN05 contract")
	}
	selected.Acceptance.CaseCount = 1
	return all, selected
}

func TestFunctionTransportProducerInventory(t *testing.T) {
	var _, selected = functionTransportSelectedContracts(t)
	var row = selected.Acceptance.Cases[0]
	if row.Group != "functions" || row.Requirement != functionTransportTitle || row.Expected != functionTransportExpected || !reflect.DeepEqual(parseRequiredTiers(row.MinimumTestLevel), functionTransportTiers) {
		t.Fatal("FN05 producer changed its original requirement")
	}
	var names = functionTransportRequiredEvents()
	if len(names) != 358 || !sort.StringsAreSorted(names) {
		t.Fatal("changed complete FN05 event inventory", len(names))
	}
	var seen = map[string]bool{}
	for _, name := range names {
		if name == "" || seen[name] {
			t.Fatal("invalid fixed function transport event", name)
		}
		seen[name] = true
	}
	if len(functionTransportProductionPaths) == 0 || len(functionTransportProductionPaths) > 128 {
		t.Fatal("unbounded function transport source inventory")
	}
	for _, path := range functionTransportProductionPaths {
		if seen[path] {
			t.Fatal("duplicate source path", path)
		}
		seen[path] = true
		var info, err = os.Stat(filepath.Join(resolveRepoRoot(), filepath.FromSlash(path)))
		if err != nil || !info.Mode().IsRegular() {
			t.Fatal("missing source witness", path, err)
		}
	}
	if !reflect.DeepEqual(deriveAcceptanceTestIDs(row.ID, row.Group), functionTransportTestIDs()) || !reflect.DeepEqual(deriveAcceptanceFixtures(row.ID, row.Group), functionTransportProductionPaths) {
		t.Fatal("FN05 production mapping is detached")
	}
	var runner = NewRunner()
	for _, id := range []string{"T05", "L01", "F04", "FN01", "FN02", "FN03", "FN04", "RP20", "RP24", "RP25", "RP26", "BRG11", "BRG15"} {
		if runner.runFunctionTransportRequirement(id) {
			t.Fatal("related requirement inherited FN05 production", id)
		}
	}
	if len(runner.records) != 0 || len(runner.results) != 0 || len(runner.observations) != 0 {
		t.Fatal("unexecuted related requirement minted evidence")
	}
	var old = EvidenceRecord{Requirements: []string{"FN05"}, Tier: "unit", TestID: "tools/language-acceptance:runFunctions", Result: "passed", ExitCode: 0}
	if validateFunctionTransportRecord("FN05", old) == nil {
		t.Fatal("historical Text/Number smoke became structured transport proof")
	}
}

func TestFunctionTransportActualProductionEvidence(t *testing.T) {
	var all, selected = functionTransportSelectedContracts(t)
	var runner = NewRunner()
	runner.artifactDir = os.Getenv("CHERRI_FUNCTION_TRANSPORT_EVIDENCE_OUTPUT")
	if !runner.runFunctionTransportRequirement("FN05") || len(runner.results) != 1 || len(runner.records) != 2 || len(runner.observations) != 0 {
		t.Fatalf("incomplete actual FN05 producer: results=%+v records=%d", runner.results, len(runner.records))
	}
	var result = runner.results["FN05"]
	if !result.Passed || result.Status != "PASSED" || !reflect.DeepEqual(result.ExecutedLevels, functionTransportTiers) {
		t.Fatalf("FN05 local contract not satisfied: %+v", result)
	}
	var manifest = &EvidenceManifest{SchemaVersion: "1", ImplementationSHA: runner.implSHA, Records: runner.records}
	var report, err = ValidateEvidenceManifest(manifest, selected, "local")
	if err != nil || report == nil || report.PassedCases != 1 || report.FailedCases != 0 || report.SkippedCases != 0 || report.ClosureComplete {
		t.Fatalf("FN05 local proof rejected or expanded: %+v %v", report, err)
	}
	for _, original := range runner.records {
		if err := validateFunctionTransportRecord("FN05", original); err != nil {
			t.Fatal(err)
		}
		if original.Tier != "unit" {
			continue
		}
		var relocated = editorContractCopyRecord(t, original)
		if err := validateFunctionTransportRecord("FN05", relocated); err != nil {
			t.Fatal("unchanged relocated proof rejected", err)
		}
		for _, fault := range []string{"wrong_tier", "wrong_requirement", "cross_tier_producer", "partial_command", "missing_artifact", "dirty_source", "wrong_source_digest", "missing_source_file", "missing_child", "skipped_child", "foreign_package", "early_case_parent"} {
			t.Run(fault, func(t *testing.T) {
				var broken = editorContractCopyRecord(t, original)
				switch fault {
				case "wrong_tier":
					broken.Tier = "ios-runtime"
				case "wrong_requirement":
					broken.Requirements = []string{"T05"}
				case "cross_tier_producer":
					broken.TestID = functionTransportProducer("native-structure")
				case "partial_command":
					broken.Command = productionTestCommand(functionTransportTests[:1])
				case "missing_artifact":
					broken.Artifacts = broken.Artifacts[:2]
				case "dirty_source", "wrong_source_digest", "missing_source_file":
					editorContractMutateArtifact(t, &broken, functionTransportPrefix+"-process.json", func(raw []byte) []byte {
						var receipt editorProcessReceipt
						if err := json.Unmarshal(raw, &receipt); err != nil {
							t.Fatal(err)
						}
						switch fault {
						case "dirty_source":
							receipt.SourceCleanAfter = false
						case "wrong_source_digest":
							receipt.SourceFiles[0].SHA256 = strings.Repeat("f", 64)
						case "missing_source_file":
							var retained []editorSourceFile
							for _, file := range receipt.SourceFiles {
								if file.Path != "p15_fn05_combined_transport_test.go" {
									retained = append(retained, file)
								}
							}
							if len(retained) != len(receipt.SourceFiles)-1 {
								t.Fatal("causal FN05 source witness not found")
							}
							receipt.SourceFiles = retained
						}
						var out, _ = json.Marshal(receipt)
						return out
					})
				default:
					var changed []byte
					editorContractMutateArtifact(t, &broken, functionTransportPrefix+"-go.jsonl", func(raw []byte) []byte {
						changed = functionTransportBrokenEvents(t, raw, fault)
						return changed
					})
					editorContractMutateArtifact(t, &broken, functionTransportPrefix+"-process.json", func(raw []byte) []byte {
						var receipt editorProcessReceipt
						if err := json.Unmarshal(raw, &receipt); err != nil {
							t.Fatal(err)
						}
						receipt.RawSHA256, receipt.RawBytes = editorSHA(changed), len(changed)
						var out, _ = json.Marshal(receipt)
						return out
					})
				}
				var err = validateFunctionTransportRecord("FN05", broken)
				if err == nil {
					t.Fatal("accepted broken function transport proof", fault)
				}
				if fault == "early_case_parent" && !strings.Contains(err.Error(), "parent passed before required child") {
					t.Fatal("nested-parent control missed the intended causal guard", err)
				}
			})
		}
	}
	var auditPath = filepath.Join(t.TempDir(), "audit.json")
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
		t.Fatal("FN05 changed full-contract accounting", err)
	}
	for _, row := range audit.Rows {
		if row.ID == "FN05" {
			if row.Status != "PASS" || !row.PositiveAndNegative || !reflect.DeepEqual(row.TestNames, functionTransportTests) || !reflect.DeepEqual(row.ProductionPath, functionTransportProductionPaths) || row.Scope != functionTransportScope {
				t.Fatal("actual FN05 scope lost", row)
			}
		} else if row.PositiveAndNegative || row.Status == "PASS" {
			t.Fatal("another requirement inherited FN05 evidence", row.ID)
		}
	}
	if runner.artifactDir != "" {
		var proof, _ = json.MarshalIndent(map[string]any{"source": runner.implSHA, "report": report, "results": runner.results, "records": runner.records}, "", "  ")
		if err = os.WriteFile(filepath.Join(runner.artifactDir, "function-transport-evidence.json"), append(proof, '\n'), 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("FN05 executed 358 fixed events on %s; two local tier records; no Apple, related-ID or final179 completion", runner.implSHA)
}

func functionTransportBrokenEvents(t *testing.T, raw []byte, fault string) []byte {
	t.Helper()
	var parent = functionTransportTests[1] + "/map_reserved_nested"
	var target = parent + "/oracle/missing_action"
	var events []map[string]any
	var parentPass map[string]any
	for _, line := range bytes.Split(bytes.TrimSpace(raw), []byte{'\n'}) {
		var event map[string]any
		if err := json.Unmarshal(line, &event); err != nil {
			t.Fatal(err)
		}
		events = append(events, event)
		if event["Test"] == parent && event["Action"] == "pass" {
			parentPass = event
		}
	}
	if parentPass == nil {
		t.Fatal("actual case parent absent")
	}
	var output bytes.Buffer
	var emit = func(event map[string]any) {
		var line, _ = json.Marshal(event)
		output.Write(line)
		output.WriteByte('\n')
	}
	for _, event := range events {
		if fault == "early_case_parent" {
			if event["Test"] == parent && (event["Action"] == "pass" || event["Action"] == "output") {
				continue
			}
			if event["Test"] == target && event["Action"] == "run" {
				parentPass["Time"] = event["Time"]
				emit(parentPass)
			}
		} else if event["Test"] == target {
			switch fault {
			case "missing_child":
				continue
			case "skipped_child":
				if event["Action"] == "pass" {
					event["Action"] = "skip"
					delete(event, "Elapsed")
				}
			case "foreign_package":
				event["Package"] = "foreign/package"
			}
		}
		emit(event)
	}
	if bytes.Equal(output.Bytes(), raw) {
		t.Fatal("control did not alter original events")
	}
	return output.Bytes()
}
