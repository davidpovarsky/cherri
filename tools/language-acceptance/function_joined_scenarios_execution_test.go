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

func distinctFunctionSelectedContracts(t *testing.T, id string) (*ContractsSet, *ContractsSet) {
	t.Helper()
	var all, err = LoadAllContracts("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	var selected = &ContractsSet{Acceptance: &ContractFile{}, Repair: &RepairContractFile{}, Gates: &GatesContractFile{}}
	switch id {
	case "RP26":
		for _, row := range all.Repair.Cases {
			if row.ID == id {
				selected.Repair.Cases = append(selected.Repair.Cases, row)
			}
		}
		if len(selected.Repair.Cases) != 1 {
			t.Fatal("missing immutable RP26")
		}
		selected.Repair.CaseCount = 1
	case "BRG15":
		for _, row := range all.Gates.Gates {
			if row.ID == id {
				selected.Gates.Gates = append(selected.Gates.Gates, row)
			}
		}
		if len(selected.Gates.Gates) != 1 {
			t.Fatal("missing immutable BRG15")
		}
		selected.Gates.GateCount = 1
	default:
		t.Fatal("unknown distinct function requirement", id)
	}
	return all, selected
}

func TestDistinctFunctionProducerInventory(t *testing.T) {
	for _, id := range []string{"RP26", "BRG15"} {
		t.Run(id, func(t *testing.T) {
			var _, selected = distinctFunctionSelectedContracts(t, id)
			var s = distinctFunctionScenarios[id]
			var names = distinctFunctionRequiredEvents(id)
			var counts = map[string]int{"RP26": 84, "BRG15": 47}
			if len(names) != counts[id] || !sort.StringsAreSorted(names) {
				t.Fatal("distinct function event inventory changed", id, len(names))
			}
			var seen = map[string]bool{}
			for _, name := range names {
				if name == "" || seen[name] {
					t.Fatal("duplicate or empty distinct scenario event", name)
				}
				seen[name] = true
			}
			var paths = distinctFunctionSourcePaths(id)
			if len(paths) > 128 || len(paths) == 0 {
				t.Fatal("unbounded source witnesses")
			}
			for _, path := range paths {
				if seen[path] {
					t.Fatal("duplicate source witness", path)
				}
				seen[path] = true
				var info, err = os.Stat(filepath.Join(resolveRepoRoot(), filepath.FromSlash(path)))
				if err != nil || !info.Mode().IsRegular() {
					t.Fatal("missing actual source witness", path, err)
				}
			}
			var ids []string
			var assertions, ui = map[string]string{}, map[string]string{}
			applyRuntimeAndUIMappings(id, s.RequiredTiers, &ids, assertions, ui)
			if len(ids)+len(assertions)+len(ui) != 0 {
				t.Fatal("historical runtime smoke granted the distinct joined scenario")
			}
			if id == "RP26" {
				var row = selected.Repair.Cases[0]
				if !reflect.DeepEqual(deriveRepairTestIDs(id, row.Title), s.testIDs()) || !reflect.DeepEqual(deriveRepairFixtures(id), paths) {
					t.Fatal("RP26 mapping detached from actual producer")
				}
				for _, field := range []string{"title", "scenario", "expected", "tiers", "related"} {
					var changed = row
					switch field {
					case "title":
						changed.Title = "only a function"
					case "scenario":
						changed.Scenario = "reuse M02"
					case "expected":
						changed.ExpectedObservation = "incidental Void output allowed"
					case "tiers":
						changed.RequiredTestLevels = []string{"native"}
					case "related":
						changed.OriginalRequirementIDs = []string{"FN05"}
					}
					var runner = NewRunner()
					if !runner.runCallerInputVoidRequirement(changed) || runner.results[id].Status != "FAILED" || len(runner.records) != 0 {
						t.Fatal("changed immutable RP26 accepted", field)
					}
				}
				for _, other := range []string{"M02", "FN04", "FN05", "RP24", "RP25", "BRG15"} {
					var changed = row
					changed.ID = other
					var runner = NewRunner()
					if runner.runCallerInputVoidRequirement(changed) || len(runner.results)+len(runner.records) != 0 {
						t.Fatal("another ID inherited RP26", other)
					}
				}
			} else {
				var row = selected.Gates.Gates[0]
				if !reflect.DeepEqual(deriveGateTestIDs(id, row.ProposedTestPrefix), s.testIDs()) || !reflect.DeepEqual(deriveGateFixtures(id), paths) {
					t.Fatal("BRG15 mapping detached from actual producer")
				}
				for _, field := range []string{"title", "scenario", "expected", "tiers", "related", "prefix", "mandatory"} {
					var changed = row
					switch field {
					case "title":
						changed.Title = "only a function"
					case "scenario":
						changed.Scenario = "reuse FN05"
					case "expected":
						changed.ExpectedObservation = "lowering failure may be dropped"
					case "tiers":
						changed.MinimumEvidenceTiers = []string{"unit", "native-structure"}
					case "related":
						changed.RelatedRepairIDs = []string{"RP25"}
					case "prefix":
						changed.ProposedTestPrefix = "TestP15FN05"
					case "mandatory":
						changed.Mandatory = false
					}
					var runner = NewRunner()
					if !runner.runJoinedFunctionTransportRequirement(changed) || runner.results[id].Status != "FAILED" || len(runner.records) != 0 {
						t.Fatal("changed immutable BRG15 accepted", field)
					}
				}
				for _, other := range []string{"FN01", "FN02", "FN03", "FN04", "FN05", "RP24", "RP25", "RP26", "BRG11"} {
					var changed = row
					changed.ID = other
					var runner = NewRunner()
					if runner.runJoinedFunctionTransportRequirement(changed) || len(runner.results)+len(runner.records) != 0 {
						t.Fatal("another ID inherited BRG15", other)
					}
				}
			}
		})
	}
}

func TestRP26ActualDistinctProductionEvidence(t *testing.T) {
	distinctFunctionActualProductionEvidence(t, "RP26")
}

func TestBRG15ActualDistinctProductionEvidence(t *testing.T) {
	distinctFunctionActualProductionEvidence(t, "BRG15")
}

func distinctFunctionActualProductionEvidence(t *testing.T, id string) {
	t.Helper()
	var all, selected = distinctFunctionSelectedContracts(t, id)
	var s = distinctFunctionScenarios[id]
	var runner = NewRunner()
	var directory = t.TempDir()
	if root := os.Getenv("CHERRI_DISTINCT_FUNCTION_EVIDENCE_OUTPUT"); root != "" {
		directory = filepath.Join(root, strings.ToLower(id))
		if err := os.Mkdir(directory, 0700); err != nil {
			t.Fatal(err)
		}
	}
	runner.artifactDir = directory
	if id == "RP26" {
		runner.runRepairCases(selected.Repair)
	} else {
		runner.runRecoveryGates(selected.Gates)
	}
	var result = runner.results[id]
	if len(runner.results) != 1 || len(runner.records) != len(s.LocalTiers) || len(runner.observations) != 0 ||
		result.Passed || result.Status != "PENDING_EXTERNAL" || !reflect.DeepEqual(result.ExecutedLevels, s.LocalTiers) {
		t.Fatalf("%s distinct local producer incomplete or expanded: %+v records=%d", id, runner.results, len(runner.records))
	}
	var manifest = &EvidenceManifest{SchemaVersion: "1", ImplementationSHA: runner.implSHA, Records: runner.records}
	var report, err = ValidateEvidenceManifest(manifest, selected, "local")
	if err != nil || report == nil || report.PassedCases != 0 || report.FailedCases != 0 || report.SkippedCases != 1 || report.ClosureComplete {
		t.Fatalf("%s local evidence closed missing native runtime: %+v %v", id, report, err)
	}
	for index, original := range runner.records {
		if err := validateDistinctFunctionRecord(id, original); err != nil {
			t.Fatal(err)
		}
		if index != 0 {
			continue
		}
		var relocated = editorContractCopyRecord(t, original)
		if err := validateDistinctFunctionRecord(id, relocated); err != nil {
			t.Fatal("byte-identical relocated originals rejected", err)
		}
		for _, fault := range []string{
			"borrowed_related", "borrowed_other_scenario", "fake_runtime", "partial_command",
			"missing_artifact", "dirty_source", "wrong_source_digest", "missing_source_file",
			"missing_child", "skipped_child", "foreign_package", "early_case_parent",
			"missing_entry_or_recursive_input", "missing_lowering_or_main_terminal",
		} {
			t.Run(fault, func(t *testing.T) {
				var broken = editorContractCopyRecord(t, original)
				switch fault {
				case "borrowed_related":
					broken.TestID = "github.com/electrikmilk/cherri:TestP15FN05CombinedStructuredTransportNative"
				case "borrowed_other_scenario":
					var other = "RP26"
					if id == other {
						other = "BRG15"
					}
					broken.Requirements = []string{other}
				case "fake_runtime":
					broken.Tier = "ios-runtime"
					broken.TestID = "ios27-runtime-poc:functions"
				case "partial_command":
					broken.Command = productionTestCommand(s.Tests[:1])
				case "missing_artifact":
					broken.Artifacts = broken.Artifacts[:2]
				case "dirty_source", "wrong_source_digest", "missing_source_file":
					editorContractMutateArtifact(t, &broken, s.Prefix+"-process.json", func(raw []byte) []byte {
						var receipt editorProcessReceipt
						if err := json.Unmarshal(raw, &receipt); err != nil {
							t.Fatal(err)
						}
						if fault == "dirty_source" {
							receipt.SourceCleanAfter = false
						} else {
							var changed = false
							var kept []editorSourceFile
							for _, file := range receipt.SourceFiles {
								if file.Path == s.Sources[0] {
									changed = true
									if fault == "missing_source_file" {
										continue
									}
									file.SHA256 = strings.Repeat("e", 64)
								}
								kept = append(kept, file)
							}
							if !changed {
								t.Fatal("scenario source witness absent from healthy original")
							}
							receipt.SourceFiles = kept
						}
						var out, _ = json.Marshal(receipt)
						return out
					})
				default:
					var changed []byte
					editorContractMutateArtifact(t, &broken, s.Prefix+"-go.jsonl", func(raw []byte) []byte {
						var parent = s.Tests[0] + "/value"
						if id == "BRG15" {
							parent = s.Tests[1]
						}
						if fault != "missing_entry_or_recursive_input" && fault != "missing_lowering_or_main_terminal" {
							changed = functionScenarioBrokenEvents(t, raw, fault, parent)
							return changed
						}
						var target string
						if id == "RP26" {
							target = s.Tests[1] + "/marker_only"
							if fault == "missing_lowering_or_main_terminal" {
								target = s.Tests[0] + "/bare/oracle/main_terminal"
							}
						} else {
							target = s.Tests[1] + "/oracle/recursive_caller"
							if fault == "missing_lowering_or_main_terminal" {
								target = s.Tests[2] + "/recursive_body"
							}
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
							t.Fatal("causal scenario event absent from healthy original")
						}
						return changed
					})
					editorContractMutateArtifact(t, &broken, s.Prefix+"-process.json", func(raw []byte) []byte {
						var receipt editorProcessReceipt
						if err := json.Unmarshal(raw, &receipt); err != nil {
							t.Fatal(err)
						}
						receipt.RawSHA256, receipt.RawBytes = editorSHA(changed), len(changed)
						var out, _ = json.Marshal(receipt)
						return out
					})
				}
				var rejected = validateDistinctFunctionRecord(id, broken)
				if rejected == nil {
					t.Fatal("damaged distinct scenario proof accepted", id, fault)
				}
				if fault == "early_case_parent" && !strings.Contains(rejected.Error(), "parent passed before required child") {
					t.Fatal("ancestor control missed its intended causal guard", rejected)
				}
			})
		}
	}
	if final, err := ValidateEvidenceManifest(manifest, selected, "final"); err == nil || final != nil && final.ClosureComplete {
		t.Fatal("local distinct scenario evidence closed missing native runtime", id)
	}
	var auditPath = filepath.Join(directory, s.Prefix+"-audit.json")
	if err := writeExecutionAudit(runner, all, auditPath); err != nil {
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
	if err := json.Unmarshal(raw, &audit); err != nil || len(audit.Rows) != 179 || audit.ClosureComplete {
		t.Fatal("distinct scenario changed immutable full179 accounting", err)
	}
	for _, row := range audit.Rows {
		if row.Status == "PASS" || row.ID != id && row.PositiveAndNegative {
			t.Fatal("local scenario completed an unproved requirement", row.ID)
		}
		if row.ID == id && (!row.PositiveAndNegative || !reflect.DeepEqual(row.TestNames, s.Tests) ||
			!reflect.DeepEqual(row.ProductionPath, distinctFunctionSourcePaths(id)) || row.Scope != s.Scope) {
			t.Fatal("distinct production scope lost", id)
		}
	}
	var proof, _ = json.MarshalIndent(map[string]any{"source": runner.implSHA, "report": report, "results": runner.results, "records": runner.records}, "", "  ")
	if err := os.WriteFile(filepath.Join(directory, s.Prefix+"-evidence.json"), append(proof, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
	t.Logf("%s executed its %d distinct fixed events on %s; local tiers=%v; actual native/iOS obligations remain pending", id, len(distinctFunctionRequiredEvents(id)), runner.implSHA, s.LocalTiers)
}
