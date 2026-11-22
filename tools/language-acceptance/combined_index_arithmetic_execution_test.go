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

func combinedIndexArithmeticSelectedContracts(t *testing.T, ids ...string) (*ContractsSet, *ContractsSet) {
	t.Helper()
	if len(ids) == 0 {
		ids = []string{"RP21", "BRG16"}
	}
	wanted := map[string]bool{}
	for _, id := range ids {
		if wanted[id] {
			t.Fatal("duplicate selected contract")
		}
		wanted[id] = true
	}
	all, err := LoadAllContracts("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	selected := &ContractsSet{Acceptance: &ContractFile{}, Repair: &RepairContractFile{}, Gates: &GatesContractFile{}}
	for _, row := range all.Repair.Cases {
		if wanted[row.ID] {
			selected.Repair.Cases = append(selected.Repair.Cases, row)
		}
	}
	for _, row := range all.Gates.Gates {
		if wanted[row.ID] {
			selected.Gates.Gates = append(selected.Gates.Gates, row)
		}
	}
	selected.Repair.CaseCount = len(selected.Repair.Cases)
	selected.Gates.GateCount = len(selected.Gates.Gates)
	if selected.Repair.CaseCount+selected.Gates.GateCount != len(ids) {
		t.Fatal("missing distinct immutable contracts")
	}
	return all, selected
}
func TestCombinedIndexArithmeticInventoryAndUnprovedRuntime(t *testing.T) {
	if err := validateCombinedIndexArithmeticSourceInventory(combinedIndexArithmeticEvidence.SourcePaths); err != nil {
		t.Fatal(err)
	}
	t.Run("source_count_before_launch", func(t *testing.T) {
		err := validateCombinedIndexArithmeticSourceInventory(make([]string, 129))
		if err == nil || !strings.Contains(err.Error(), "invalid bounded") {
			t.Fatal("oversized source inventory did not reach the prelaunch size gate", err)
		}
	})
	t.Run("required_source_before_launch", func(t *testing.T) {
		var missing []string
		for _, path := range combinedIndexArithmeticEvidence.SourcePaths {
			if path != "p15_combined_index_arithmetic_test.go" {
				missing = append(missing, path)
			}
		}
		if len(missing) != len(combinedIndexArithmeticEvidence.SourcePaths)-1 {
			t.Fatal("required-source control did not remove the actual oracle")
		}
		err := validateCombinedIndexArithmeticSourceInventory(missing)
		if err == nil || !strings.Contains(err.Error(), "reviewed source witnesses") {
			t.Fatal("removed required source did not reach the prelaunch witness gate", err)
		}
	})
	_, selected := combinedIndexArithmeticSelectedContracts(t, "RP21", "BRG16")
	for _, id := range []string{"RP21", "BRG16"} {
		spec := combinedIndexArithmeticScenarios[id]
		var title, group string
		var tiers []string
		for _, row := range selected.Repair.Cases {
			if row.ID == id {
				title = row.Title
				group = "repair"
				tiers = row.RequiredTestLevels
			}
		}
		for _, row := range selected.Gates.Gates {
			if row.ID == id {
				title = row.Title
				group = "recovery_gate"
				tiers = row.MinimumEvidenceTiers
			}
		}
		if spec.Title != title || spec.Group != group || !reflect.DeepEqual(parseRequiredTiers(strings.Join(tiers, "+")), combinedIndexArithmeticRequiredTiers) {
			t.Fatal("changed immutable combined requirement")
		}
		expected := map[string]int{"RP21": 234, "BRG16": 213}[id]
		events := combinedIndexArithmeticEvidence.Events[id]
		if len(events) != expected || !sort.StringsAreSorted(events) {
			t.Fatal("fixed causal child inventory changed")
		}
		seen := map[string]bool{}
		for _, event := range events {
			if seen[event] || !(event == spec.Unit || event == spec.Native || strings.HasPrefix(event, spec.Unit+"/") || strings.HasPrefix(event, spec.Native+"/")) {
				t.Fatal("duplicate or foreign combined event")
			}
			seen[event] = true
		}
		for _, tier := range []string{"ios-runtime", "ios-ui", "ios-build"} {
			if validateCombinedIndexArithmeticRecord(id, EvidenceRecord{Requirements: []string{id}, Tier: tier, TestID: "ios27-runtime-poc:conditionals", Result: "passed"}) == nil {
				t.Fatal("incomplete historical trace supplied combined Apple tier")
			}
		}
		runner := NewRunner()
		if !runner.runCombinedIndexArithmeticRequirement(id, group, title, []string{"unit", "native"}) || len(runner.records) != 0 || runner.results[id].Status != "FAILED" {
			t.Fatal("weaker required tiers accepted")
		}
	}
	runner := NewRunner()
	if runner.runCombinedIndexArithmeticRequirement("L02", "binding", "related only", []string{"unit"}) || len(runner.results) != 0 {
		t.Fatal("related requirement inherited combined proof")
	}
	all, err := LoadAllContracts("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	mapping, err := BuildRequirementsEvidenceMap(all)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"RP21", "BRG16"} {
		if !reflect.DeepEqual(mapping.Mappings[id].TestIDs, combinedIndexArithmeticScenarios[id].testIDs()) {
			t.Fatal("combined scenario inherited a related or old runtime producer", id)
		}
		if len(mapping.Mappings[id].RequiredAssertions) != 0 {
			t.Fatal("historical short trace still supplies combined assertions", id)
		}
	}
	for _, path := range combinedIndexArithmeticEvidence.SourcePaths {
		if _, err := os.Stat(resolveRepoPath(path)); err != nil {
			t.Fatal(err)
		}
	}
}
func TestCombinedIndexArithmeticActualProductionEvidence(t *testing.T) {
	combinedIndexArithmeticActualProductionEvidence(t, []string{"RP21", "BRG16"})
}
func combinedIndexArithmeticActualProductionEvidence(t *testing.T, ids []string) {
	t.Helper()
	all, selected := combinedIndexArithmeticSelectedContracts(t, ids...)
	runner := NewRunner()
	runner.artifactDir = os.Getenv("CHERRI_COMBINED_INDEX_ARITHMETIC_EVIDENCE_OUTPUT")
	runner.runRepairCases(selected.Repair)
	runner.runRecoveryGates(selected.Gates)
	if len(runner.records) != 2*len(ids) || len(runner.results) != len(ids) || len(runner.observations) != 0 {
		t.Fatalf("incomplete independent producers: %+v records=%d", runner.results, len(runner.records))
	}
	for id, result := range runner.results {
		if result.Passed || result.Status != "PENDING_EXTERNAL" || !reflect.DeepEqual(result.ExecutedLevels, combinedIndexArithmeticLocalTiers) {
			t.Fatalf("unproved Apple scope escaped %s: %+v", id, result)
		}
	}
	manifest := &EvidenceManifest{SchemaVersion: "1", ImplementationSHA: runner.implSHA, Records: runner.records}
	report, err := ValidateEvidenceManifest(manifest, selected, "local")
	if err != nil || report == nil || report.PassedCases != 0 || report.FailedCases != 0 || report.SkippedCases != len(ids) || report.ClosureComplete {
		t.Fatalf("local combined proof expanded/rejected: %+v %v", report, err)
	}
	for _, original := range runner.records {
		id := original.Requirements[0]
		if err := validateCombinedIndexArithmeticRecord(id, original); err != nil {
			t.Fatal(err)
		}
		if original.Tier != "unit" {
			continue
		}
		relocated := editorContractCopyRecord(t, original)
		if err := validateCombinedIndexArithmeticRecord(id, relocated); err != nil {
			t.Fatal("unchanged relocated proof rejected", err)
		}
		for _, fault := range []string{"wrong_tier", "wrong_requirement", "cross_tier_producer", "partial_command", "missing_artifact", "dirty_source", "wrong_source_digest", "missing_source_file", "missing_child", "skipped_child", "foreign_package"} {
			t.Run(id+"/"+fault, func(t *testing.T) {
				broken := editorContractCopyRecord(t, original)
				prefix := combinedIndexArithmeticPrefix(id)
				switch fault {
				case "wrong_tier":
					broken.Tier = "ios-runtime"
				case "wrong_requirement":
					broken.Requirements = []string{"L02"}
				case "cross_tier_producer":
					broken.TestID = combinedIndexArithmeticScenarios[id].producer("native-structure")
				case "partial_command":
					broken.Command = productionTestCommand(combinedIndexArithmeticScenarios[id].tests()[:1])
				case "missing_artifact":
					broken.Artifacts = broken.Artifacts[:2]
				case "dirty_source", "wrong_source_digest", "missing_source_file":
					editorContractMutateArtifact(t, &broken, prefix+"-process.json", func(raw []byte) []byte {
						var receipt editorProcessReceipt
						json.Unmarshal(raw, &receipt)
						if fault == "dirty_source" {
							receipt.SourceCleanAfter = false
						} else if fault == "missing_source_file" {
							originalCount := len(receipt.SourceFiles)
							var retained []editorSourceFile
							for _, file := range receipt.SourceFiles {
								if file.Path != "p15_combined_index_arithmetic_test.go" {
									retained = append(retained, file)
								}
							}
							if len(retained) != originalCount-1 {
								t.Fatal("missing-source receipt control did not remove the oracle")
							}
							receipt.SourceFiles = retained
						} else {
							receipt.SourceFiles[0].SHA256 = strings.Repeat("f", 64)
						}
						out, _ := json.Marshal(receipt)
						return out
					})
				default:
					target := combinedIndexArithmeticEvidence.Events[id][1]
					var changed []byte
					editorContractMutateArtifact(t, &broken, prefix+"-go.jsonl", func(raw []byte) []byte {
						var result bytes.Buffer
						for _, line := range bytes.Split(raw, []byte{'\n'}) {
							if len(bytes.TrimSpace(line)) == 0 {
								continue
							}
							var event map[string]any
							if err := json.Unmarshal(line, &event); err != nil {
								t.Fatal(err)
							}
							if event["Test"] == target {
								if fault == "missing_child" {
									continue
								}
								if fault == "skipped_child" && event["Action"] == "pass" {
									event["Action"] = "skip"
								}
								if fault == "foreign_package" {
									event["Package"] = "foreign/package"
								}
							}
							data, _ := json.Marshal(event)
							result.Write(data)
							result.WriteByte('\n')
						}
						changed = result.Bytes()
						if bytes.Equal(changed, raw) {
							t.Fatal("control did not change raw events")
						}
						return changed
					})
					editorContractMutateArtifact(t, &broken, prefix+"-process.json", func(raw []byte) []byte {
						var receipt editorProcessReceipt
						json.Unmarshal(raw, &receipt)
						receipt.RawSHA256 = editorSHA(changed)
						receipt.RawBytes = len(changed)
						data, _ := json.Marshal(receipt)
						return data
					})
				}
				err := validateCombinedIndexArithmeticRecord(id, broken)
				if err == nil {
					t.Fatalf("accepted combined evidence mutation %s", fault)
				}
				if fault == "missing_source_file" && !strings.Contains(err.Error(), "source manifest mismatch") {
					t.Fatal("missing-source receipt control failed before the intended source gate", err)
				}
			})
		}
	}
	final, finalErr := ValidateEvidenceManifest(manifest, selected, "final")
	if finalErr == nil || final == nil || final.ClosureComplete || final.PassedCases != 0 {
		t.Fatal("local combined proof closed final release")
	}
	auditPath := filepath.Join(t.TempDir(), "audit.json")
	if err = writeExecutionAudit(runner, all, auditPath); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatal(err)
	}
	var audit struct {
		Rows            []ExecutionAuditRow `json:"rows"`
		ClosureComplete bool                `json:"closure_complete"`
	}
	if err = json.Unmarshal(raw, &audit); err != nil {
		t.Fatal(err)
	}
	if len(audit.Rows) != 179 || audit.ClosureComplete {
		t.Fatal("combined producer changed immutable accounting")
	}
	for _, row := range audit.Rows {
		if row.Status == "PASS" {
			t.Fatal("unproved combined or related requirement received full acceptance", row.ID)
		}
		if _, executed := runner.results[row.ID]; executed {
			if !row.PositiveAndNegative || !reflect.DeepEqual(row.TestNames, combinedIndexArithmeticScenarios[row.ID].tests()) {
				t.Fatal("actual combined scope lost")
			}
		} else if row.PositiveAndNegative {
			t.Fatal("related requirement inherited combined evidence", row.ID)
		}
	}
	proof, _ := json.MarshalIndent(map[string]any{"source": runner.implSHA, "report": report, "results": runner.results, "records": runner.records}, "", "  ")
	if runner.artifactDir != "" {
		if err = os.WriteFile(filepath.Join(runner.artifactDir, "combined-index-arithmetic-evidence.json"), append(proof, '\n'), 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("Distinct %v actual local execution on %s; %d unit/native records, 0 full PASS, %d pending Apple runtime; no related-ID inheritance", ids, runner.implSHA, 2*len(ids), len(ids))
}
