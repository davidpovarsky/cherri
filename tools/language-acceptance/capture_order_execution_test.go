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

func captureOrderSelectedContracts(t *testing.T) (*ContractsSet, *ContractsSet) {
	t.Helper()
	all, err := LoadAllContracts("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	selected := &ContractsSet{Acceptance: &ContractFile{}, Repair: &RepairContractFile{}, Gates: &GatesContractFile{}}
	for _, row := range all.Repair.Cases {
		if row.ID == "RP12" {
			selected.Repair.Cases = append(selected.Repair.Cases, row)
		}
	}
	for _, row := range all.Gates.Gates {
		if row.ID == "BRG10" {
			selected.Gates.Gates = append(selected.Gates.Gates, row)
		}
	}
	selected.Repair.CaseCount = len(selected.Repair.Cases)
	selected.Gates.GateCount = len(selected.Gates.Gates)
	if selected.Repair.CaseCount != 1 || selected.Gates.GateCount != 1 {
		t.Fatal("missing distinct immutable contracts")
	}
	return all, selected
}
func TestCaptureOrderInventoryAndUnprovedRuntime(t *testing.T) {
	_, selected := captureOrderSelectedContracts(t)
	for _, id := range []string{"RP12", "BRG10"} {
		spec := captureOrderScenarios[id]
		var title, group string
		var tiers []string
		if id == "RP12" {
			row := selected.Repair.Cases[0]
			title = row.Title
			group = "repair"
			tiers = row.RequiredTestLevels
		} else {
			row := selected.Gates.Gates[0]
			title = row.Title
			group = "recovery_gate"
			tiers = row.MinimumEvidenceTiers
		}
		if spec.Title != title || spec.Group != group || !reflect.DeepEqual(parseRequiredTiers(strings.Join(tiers, "+")), captureOrderRequiredTiers) {
			t.Fatal("changed immutable combined requirement")
		}
		expected := 58
		if id == "BRG10" {
			expected = 120
		}
		events := captureOrderEvidence.Events[id]
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
			if validateCaptureOrderRecord(id, EvidenceRecord{Requirements: []string{id}, Tier: tier, TestID: "ios27-runtime-poc:variables", Result: "passed"}) == nil {
				t.Fatal("incomplete historical trace supplied combined Apple tier")
			}
		}
		runner := NewRunner()
		if !runner.runCaptureOrderRequirement(id, group, title, []string{"unit", "native"}) || len(runner.records) != 0 || runner.results[id].Status != "FAILED" {
			t.Fatal("weaker required tiers accepted")
		}
	}
	runner := NewRunner()
	if runner.runCaptureOrderRequirement("V01", "binding", "related only", []string{"unit"}) || len(runner.results) != 0 {
		t.Fatal("related requirement inherited combined proof")
	}
	for _, path := range captureOrderEvidence.SourcePaths {
		if _, err := os.Stat(resolveRepoPath(path)); err != nil {
			t.Fatal(err)
		}
	}
}
func TestCaptureOrderActualProductionEvidence(t *testing.T) {
	all, selected := captureOrderSelectedContracts(t)
	runner := NewRunner()
	runner.artifactDir = os.Getenv("CHERRI_CAPTURE_ORDER_EVIDENCE_OUTPUT")
	runner.runRepairCases(selected.Repair)
	runner.runRecoveryGates(selected.Gates)
	if len(runner.records) != 4 || len(runner.results) != 2 || len(runner.observations) != 0 {
		t.Fatalf("incomplete independent producers: %+v records=%d", runner.results, len(runner.records))
	}
	for id, result := range runner.results {
		if result.Passed || result.Status != "PENDING_EXTERNAL" || !reflect.DeepEqual(result.ExecutedLevels, captureOrderLocalTiers) {
			t.Fatalf("unproved Apple scope escaped %s: %+v", id, result)
		}
	}
	manifest := &EvidenceManifest{SchemaVersion: "1", ImplementationSHA: runner.implSHA, Records: runner.records}
	report, err := ValidateEvidenceManifest(manifest, selected, "local")
	if err != nil || report == nil || report.PassedCases != 0 || report.FailedCases != 0 || report.SkippedCases != 2 || report.ClosureComplete {
		t.Fatalf("local combined proof expanded/rejected: %+v %v", report, err)
	}
	for _, original := range runner.records {
		id := original.Requirements[0]
		if err := validateCaptureOrderRecord(id, original); err != nil {
			t.Fatal(err)
		}
		if original.Tier != "unit" {
			continue
		}
		relocated := editorContractCopyRecord(t, original)
		if err := validateCaptureOrderRecord(id, relocated); err != nil {
			t.Fatal("unchanged relocated proof rejected", err)
		}
		for _, fault := range []string{"wrong_tier", "wrong_requirement", "cross_tier_producer", "partial_command", "missing_artifact", "dirty_source", "wrong_source_digest", "missing_child", "skipped_child", "foreign_package"} {
			t.Run(id+"/"+fault, func(t *testing.T) {
				broken := editorContractCopyRecord(t, original)
				prefix := captureOrderPrefix(id)
				switch fault {
				case "wrong_tier":
					broken.Tier = "ios-runtime"
				case "wrong_requirement":
					broken.Requirements = []string{"V01"}
				case "cross_tier_producer":
					broken.TestID = captureOrderScenarios[id].producer("native-structure")
				case "partial_command":
					broken.Command = productionTestCommand(captureOrderScenarios[id].tests()[:1])
				case "missing_artifact":
					broken.Artifacts = broken.Artifacts[:2]
				case "dirty_source", "wrong_source_digest":
					editorContractMutateArtifact(t, &broken, prefix+"-process.json", func(raw []byte) []byte {
						var receipt editorProcessReceipt
						json.Unmarshal(raw, &receipt)
						if fault == "dirty_source" {
							receipt.SourceCleanAfter = false
						} else {
							receipt.SourceFiles[0].SHA256 = strings.Repeat("f", 64)
						}
						out, _ := json.Marshal(receipt)
						return out
					})
				default:
					target := captureOrderEvidence.Events[id][1]
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
				if err := validateCaptureOrderRecord(id, broken); err == nil {
					t.Fatalf("accepted combined evidence mutation %s", fault)
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
		if row.ID == "RP12" || row.ID == "BRG10" {
			if !row.PositiveAndNegative || !reflect.DeepEqual(row.TestNames, captureOrderScenarios[row.ID].tests()) {
				t.Fatal("actual combined scope lost")
			}
		} else if row.PositiveAndNegative {
			t.Fatal("related requirement inherited combined evidence", row.ID)
		}
	}
	proof, _ := json.MarshalIndent(map[string]any{"source": runner.implSHA, "report": report, "results": runner.results, "records": runner.records}, "", "  ")
	if runner.artifactDir != "" {
		if err = os.WriteFile(filepath.Join(runner.artifactDir, "capture-order-evidence.json"), append(proof, '\n'), 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("Distinct RP12 and BRG10 actual local execution on %s; 4 unit/native records, 0 full PASS, 2 pending Apple runtime; no related-ID inheritance", runner.implSHA)
}
