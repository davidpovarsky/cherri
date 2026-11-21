package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func documentationRecoverySelected(t *testing.T) *ContractsSet {
	t.Helper()
	all, err := LoadAllContracts("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	selected := &ContractsSet{Acceptance: &ContractFile{}, Repair: &RepairContractFile{}, Gates: &GatesContractFile{}}
	for _, row := range all.Repair.Cases {
		if row.ID == "RP46" {
			selected.Repair.Cases = append(selected.Repair.Cases, row)
		}
	}
	for _, row := range all.Gates.Gates {
		if row.ID == "BRG29" {
			selected.Gates.Gates = append(selected.Gates.Gates, row)
		}
	}
	if len(selected.Repair.Cases) != 1 || len(selected.Gates.Gates) != 1 {
		t.Fatal("missing immutable documentation contracts")
	}
	selected.Repair.CaseCount, selected.Gates.GateCount = 1, 1
	return selected
}

func TestDocumentationRecoveryContractAndNoInheritance(t *testing.T) {
	selected := documentationRecoverySelected(t)
	repair, gate := selected.Repair.Cases[0], selected.Gates.Gates[0]
	for _, check := range []struct {
		id, title string
		tiers     []string
	}{
		{repair.ID, repair.Title, repair.RequiredTestLevels},
		{gate.ID, gate.Title, gate.MinimumEvidenceTiers},
	} {
		scenario := documentationRecoveryScenarios[check.id]
		if scenario.Title != check.title || !reflect.DeepEqual(scenario.Tiers, parseRequiredTiers(strings.Join(check.tiers, "+"))) {
			t.Fatal("documentation producer changed immutable title/tiers", check.id)
		}
		if !reflect.DeepEqual(deriveRepairFixtures("RP46"), documentationRecoveryProductionPaths) ||
			!reflect.DeepEqual(deriveGateFixtures("BRG29"), documentationRecoveryProductionPaths) {
			t.Fatal("source mapping incomplete")
		}
		runner := NewRunner()
		if !runner.runDocumentationRecoveryRequirement(check.id, "different contract", check.tiers) ||
			runner.results[check.id].Status != "FAILED" || len(runner.records) != 0 {
			t.Fatal("foreign scenario accepted")
		}
		if !runner.runDocumentationRecoveryRequirement(check.id, check.title, []string{"docs"}) ||
			runner.results[check.id].Status != "FAILED" || len(runner.records) != 0 {
			t.Fatal("weaker tiers accepted")
		}
	}
	runner := NewRunner()
	for _, id := range []string{"D01", "D02", "D03", "K05", "BRG30"} {
		if runner.runDocumentationRecoveryRequirement(id, "related only", []string{"docs"}) {
			t.Fatal("linked requirement inherited docs proof", id)
		}
	}
	if len(runner.records) != 0 || len(runner.results) != 0 {
		t.Fatal("unexecuted requirement received credit")
	}
	for _, path := range documentationRecoveryProductionPaths {
		if info, err := os.Stat(filepath.Join(resolveRepoRoot(), filepath.FromSlash(path))); err != nil || !info.Mode().IsRegular() {
			t.Fatal("missing production source", path, err)
		}
	}
}

func documentationRecoveryRewrite(t *testing.T, record *EvidenceRecord, mutate func(map[string][]byte, *editorProcessReceipt)) {
	t.Helper()
	scenario := documentationRecoveryScenarios[record.Requirements[0]]
	names := scenario.artifacts()
	data, err := stateReadArtifacts(record.Artifacts, names)
	if err != nil {
		t.Fatal(err)
	}
	var receipt editorProcessReceipt
	if err = decodeEditorJSON(data[names[0]], &receipt); err != nil {
		t.Fatal(err)
	}
	mutate(data, &receipt)
	receipt.RawSHA256, receipt.RawBytes = editorSHA(data[names[1]]), len(data[names[1]])
	receipt.StderrSHA256, receipt.StderrBytes = editorSHA(data[names[2]]), len(data[names[2]])
	encoded, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	data[names[0]] = append(encoded, '\n')
	root := t.TempDir()
	record.Artifacts = nil
	for _, name := range names {
		path := filepath.Join(root, name)
		if err := os.WriteFile(path, data[name], 0600); err != nil {
			t.Fatal(err)
		}
		record.Artifacts = append(record.Artifacts, EvidenceArtifact{Path: path, SHA256: editorSHA(data[name])})
	}
}

func TestDocumentationRecoveryActualProductionEvidence(t *testing.T) {
	if os.Getenv("CHERRI_DOCUMENTATION_SOURCE_ROOT") == "" || os.Getenv("CHERRI_D03_DOCS_REPO") == "" || os.Getenv("CHERRI_DOCUMENTATION_COMPILER") == "" {
		t.Skip("actual external immutable tuple must be supplied; no documentation acceptance granted")
	}
	selected := documentationRecoverySelected(t)
	runner := NewRunner()
	runner.artifactDir = os.Getenv("CHERRI_DOCS_RECOVERY_ACCEPTANCE_OUTPUT")
	if runner.artifactDir == "" {
		runner.artifactDir = t.TempDir()
	}
	runner.runRepairCases(selected.Repair)
	runner.runRecoveryGates(selected.Gates)
	if len(runner.records) != 5 || len(runner.results) != 2 || len(runner.observations) != 0 {
		t.Fatalf("actual documentation accounting differs: records=%d results=%+v observations=%v", len(runner.records), runner.results, runner.observations)
	}
	for id, scenario := range documentationRecoveryScenarios {
		result := runner.results[id]
		if !result.Passed || result.Status != "PASSED" || result.Group != scenario.Group || !reflect.DeepEqual(result.ExecutedLevels, scenario.Tiers) {
			t.Fatalf("actual %s did not pass: %+v", id, result)
		}
	}
	manifest := &EvidenceManifest{SchemaVersion: "1", ImplementationSHA: runner.implSHA, Records: runner.records}
	report, err := ValidateEvidenceManifest(manifest, selected, "local")
	if err != nil || report == nil || report.PassedCases != 2 || report.FailedCases != 0 || report.ClosureComplete {
		t.Fatal("actual documentation proof rejected", report, err)
	}
	checked := map[string]bool{}
	for _, original := range runner.records {
		id := original.Requirements[0]
		if err := validateDocumentationRecoveryRecord(id, original); err != nil {
			t.Fatal(err)
		}
		if checked[id] {
			continue
		}
		checked[id] = true
		encoded, _ := json.Marshal(original)
		for _, fault := range []string{"borrowed_id", "wrong_tier", "wrong_producer", "weaker_command", "missing_artifact", "dirty_before", "dirty_after", "stale_source", "forged_source_bytes", "missing_source", "missing_child", "skipped_child", "raw_bad_utf8", "elapsed_timeout", "unbound_required_events"} {
			t.Run(id+"/"+fault, func(t *testing.T) {
				var changed EvidenceRecord
				if err := json.Unmarshal(encoded, &changed); err != nil {
					t.Fatal(err)
				}
				switch fault {
				case "borrowed_id":
					changed.Requirements = []string{"D03"}
				case "wrong_tier":
					changed.Tier = "ios-runtime"
				case "wrong_producer":
					changed.TestID = "tools/language-acceptance:runDocs"
				case "weaker_command":
					changed.Command = []string{"go", "test", "."}
				case "missing_artifact":
					changed.Artifacts = changed.Artifacts[:2]
				default:
					documentationRecoveryRewrite(t, &changed, func(data map[string][]byte, receipt *editorProcessReceipt) {
						scenario := documentationRecoveryScenarios[id]
						switch fault {
						case "dirty_before":
							receipt.SourceCleanBefore = false
						case "dirty_after":
							receipt.SourceCleanAfter = false
						case "stale_source":
							receipt.SourceCommit = strings.Repeat("e", 40)
						case "forged_source_bytes":
							receipt.SourceFiles[0].SHA256 = strings.Repeat("0", 64)
						case "missing_source":
							receipt.SourceFiles = receipt.SourceFiles[1:]
						case "elapsed_timeout":
							receipt.ElapsedSeconds = 3600
						case "unbound_required_events":
							receipt.RequiredTests = receipt.RequiredTests[:1]
						default:
							target := scenario.events()[1]
							kind := "missing_event"
							if fault == "skipped_child" {
								kind = "skipped_child"
							}
							if fault == "raw_bad_utf8" {
								kind = "invalid_utf8"
							}
							name := scenario.artifacts()[1]
							raw := numberWireCorruptRaw(t, kind, target, data[name])
							if bytes.Equal(raw, data[name]) {
								t.Fatal("control did not corrupt original output")
							}
							data[name] = raw
						}
					})
				}
				if validateDocumentationRecoveryRecord(id, changed) == nil {
					t.Fatal("corrupted real documentation proof accepted", fault)
				}
			})
		}
	}
	final, finalErr := ValidateEvidenceManifest(manifest, selected, "final")
	if finalErr == nil || final == nil || final.ClosureComplete {
		t.Fatal("local documentation bypassed final release gates", final, finalErr)
	}
	raw, err := json.Marshal(map[string]any{"source_sha": runner.implSHA, "report": report, "records": runner.records})
	if err != nil {
		t.Fatal(err)
	}
	t.Log("P15_DOCUMENTATION_RECOVERY_ACCEPTANCE:" + string(raw))
}
