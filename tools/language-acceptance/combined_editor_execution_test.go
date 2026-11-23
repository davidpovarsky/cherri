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
	"time"
)

func combinedEditorSelectedContracts(t *testing.T) (*ContractsSet, *ContractsSet) {
	t.Helper()
	var all, err = LoadAllContracts("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	var selected = &ContractsSet{Acceptance: &ContractFile{}, Repair: &RepairContractFile{}, Gates: &GatesContractFile{}}
	for _, row := range all.Repair.Cases {
		if row.ID == "RP39" {
			selected.Repair.Cases = append(selected.Repair.Cases, row)
		}
	}
	if len(selected.Repair.Cases) != 1 {
		t.Fatal("missing immutable RP39 contract")
	}
	selected.Repair.CaseCount = 1
	return all, selected
}

func TestCombinedEditorProducerInventory(t *testing.T) {
	var _, selected = combinedEditorSelectedContracts(t)
	var row = selected.Repair.Cases[0]
	if row.Title != combinedEditorTitle || row.Scenario != combinedEditorScenario || row.ExpectedObservation != combinedEditorExpected ||
		!reflect.DeepEqual(parseRequiredTiers(strings.Join(row.RequiredTestLevels, "+")), combinedEditorRequiredTiers) {
		t.Fatal("RP39 original same-document scenario or tiers changed")
	}
	var names = combinedEditorRequiredEvents()
	if len(names) != 24 || !sort.StringsAreSorted(names) {
		t.Fatal("changed fixed combined-service event inventory", len(names))
	}
	var seen = map[string]bool{}
	for _, name := range names {
		if name == "" || seen[name] {
			t.Fatal("invalid combined-service event", name)
		}
		seen[name] = true
	}
	for _, path := range combinedEditorProductionPaths {
		if seen[path] {
			t.Fatal("duplicate combined-service source witness", path)
		}
		seen[path] = true
		var info, err = os.Stat(filepath.Join(resolveRepoRoot(), filepath.FromSlash(path)))
		if err != nil || !info.Mode().IsRegular() {
			t.Fatal("missing combined-service source witness", path, err)
		}
	}
	if len(combinedEditorProductionPaths) != 51 || !reflect.DeepEqual(deriveRepairTestIDs(row.ID, row.Title), combinedEditorTestIDs()) ||
		!reflect.DeepEqual(deriveRepairFixtures(row.ID), combinedEditorProductionPaths) {
		t.Fatal("complete combined-service producer mapping changed")
	}
	if productionTestTimeout(combinedEditorTests) != 3*time.Minute {
		t.Fatal("RP39 changed original default Go test deadline")
	}
	var ids []string
	var assertions, ui = map[string]string{}, map[string]string{}
	applyRuntimeAndUIMappings("RP39", combinedEditorRequiredTiers, &ids, assertions, ui)
	if len(ids) != 0 || len(assertions) != 0 || len(ui) != 0 {
		t.Fatal("uncombined E03/E04 mapping supplied RP39 UI evidence")
	}
	var runner = NewRunner()
	for _, id := range []string{"E03", "E04", "E05", "P05", "RP40", "RP41", "BRG27"} {
		var foreign = row
		foreign.ID = id
		if runner.runCombinedEditorRequirement(foreign) {
			t.Fatal("related requirement inherited RP39 producer", id)
		}
	}
	if len(runner.records) != 0 || len(runner.results) != 0 || len(runner.observations) != 0 {
		t.Fatal("foreign contract received combined-service credit")
	}
	for _, field := range []string{"title", "scenario", "expected", "tiers"} {
		var changed = row
		switch field {
		case "title":
			changed.Title = "uncombined editor checks"
		case "scenario":
			changed.Scenario = "format one document and rename another"
		case "expected":
			changed.ExpectedObservation = "nonempty service output"
		case "tiers":
			changed.RequiredTestLevels = []string{"unit", "service"}
		}
		if !runner.runCombinedEditorRequirement(changed) || runner.results["RP39"].Status != "FAILED" ||
			len(runner.records) != 0 || len(runner.observations) != 0 {
			t.Fatal("changed immutable combined contract accepted", field)
		}
	}
}

// Repin the changed original and the process's stream digest together. A
// negative must reach the intended event/chronology predicate, not just fail an
// unrelated stale artifact hash.
func combinedEditorRebindRaw(t *testing.T, record *EvidenceRecord, mutate func([]byte) []byte) {
	t.Helper()
	var changed []byte
	editorContractMutateArtifact(t, record, combinedEditorPrefix+"-go.jsonl", func(raw []byte) []byte {
		changed = mutate(raw)
		if bytes.Equal(raw, changed) {
			t.Fatal("combined-service raw mutation changed nothing")
		}
		return changed
	})
	editorContractMutateArtifact(t, record, combinedEditorPrefix+"-process.json", func(raw []byte) []byte {
		var receipt editorProcessReceipt
		if err := json.Unmarshal(raw, &receipt); err != nil {
			t.Fatal(err)
		}
		receipt.RawSHA256, receipt.RawBytes = editorSHA(changed), len(changed)
		var out, err = json.Marshal(receipt)
		if err != nil {
			t.Fatal(err)
		}
		return out
	})
}

func TestCombinedEditorActualProductionEvidence(t *testing.T) {
	var all, selected = combinedEditorSelectedContracts(t)
	var runner = NewRunner()
	runner.artifactDir = os.Getenv("CHERRI_COMBINED_EDITOR_EVIDENCE_OUTPUT")
	// One actual source-bound command executes both distinct RP39 parents.
	runner.runRepairCases(selected.Repair)
	var result = runner.results["RP39"]
	if len(runner.results) != 1 || len(runner.records) != 2 || len(runner.observations) != 0 ||
		result.Passed || result.Status != "PENDING_EXTERNAL" || !reflect.DeepEqual(result.ExecutedLevels, combinedEditorLocalTiers) {
		t.Fatalf("RP39 local production incomplete or expanded: %+v records=%d", runner.results, len(runner.records))
	}
	var manifest = &EvidenceManifest{SchemaVersion: "1", ImplementationSHA: runner.implSHA, Records: runner.records}
	var report, err = ValidateEvidenceManifest(manifest, selected, "local")
	if err != nil || report == nil || report.PassedCases != 0 || report.FailedCases != 0 || report.SkippedCases != 1 || report.ClosureComplete {
		t.Fatalf("RP39 local service closed missing UI execution: %+v %v", report, err)
	}
	for _, original := range runner.records {
		if err := validateCombinedEditorRecord("RP39", original); err != nil {
			t.Fatal(err)
		}
		if original.Tier != "unit" {
			continue
		}
		for _, fault := range []string{
			"borrowed_e03", "borrowed_e04", "fake_ui", "partial_command", "scope_changed",
			"wrong_fixture_blob", "wrong_source", "dirty_source", "timeout", "nonzero_exit",
			"missing_delayed_control", "missing_utf16_control", "foreign_package", "duplicate_run", "outside_process_interval",
		} {
			t.Run(fault, func(t *testing.T) {
				var broken = editorContractCopyRecord(t, original)
				if err := validateCombinedEditorRecord("RP39", broken); err != nil {
					t.Fatal("unchanged original clone rejected", err)
				}
				switch fault {
				case "borrowed_e03":
					broken.TestID = combinedEditorPackageID + ":TestP15E03ExactScopedSymbols"
				case "borrowed_e04":
					broken.Requirements = []string{"E04"}
				case "fake_ui":
					broken.Tier = "ios-ui"
					broken.TestID = "CherriUITests/testE05EditorResponsivenessStaleEditsUI"
				case "partial_command":
					broken.Command = productionTestCommandInPackage(combinedEditorPackage, combinedEditorTests[:1])
				case "scope_changed":
					broken.Assertions["scope"] = "all UI requirements proved"
				case "wrong_fixture_blob", "wrong_source", "dirty_source", "timeout", "nonzero_exit":
					editorContractMutateArtifact(t, &broken, combinedEditorPrefix+"-process.json", func(raw []byte) []byte {
						var receipt editorProcessReceipt
						if err := json.Unmarshal(raw, &receipt); err != nil {
							t.Fatal(err)
						}
						switch fault {
						case "wrong_fixture_blob":
							var changed = false
							for index := range receipt.SourceFiles {
								if receipt.SourceFiles[index].Path == "internal/language/service/p15_editor_combined_contract_test.go" {
									receipt.SourceFiles[index].SHA256 = strings.Repeat("e", 64)
									changed = true
								}
							}
							if !changed {
								t.Fatal("RP39 authored fixture binding absent")
							}
						case "wrong_source":
							receipt.SourceCommit = strings.Repeat("e", 40)
						case "dirty_source":
							receipt.SourceCleanAfter = false
						case "timeout":
							var value = true
							receipt.TimedOut = &value
						case "nonzero_exit":
							var value = 1
							receipt.ExitCode = &value
						}
						var out, err = json.Marshal(receipt)
						if err != nil {
							t.Fatal(err)
						}
						return out
					})
				default:
					var target = combinedEditorTests[1] + "/delayed_old_mutated"
					var kind = fault
					if fault == "missing_delayed_control" || fault == "missing_utf16_control" {
						kind = "missing_event"
					}
					if fault == "missing_utf16_control" {
						target = combinedEditorTests[1] + "/utf16_span"
					}
					combinedEditorRebindRaw(t, &broken, func(raw []byte) []byte {
						if fault != "outside_process_interval" {
							return structuralMigrationCorruptRaw(t, kind, target, raw)
						}
						// Shift all real timestamps equally. Strict event order,
						// duration and inventory still pass; only their join to
						// the retained process interval must fail.
						var lines [][]byte
						for _, line := range bytes.Split(bytes.TrimSpace(raw), []byte("\n")) {
							var event map[string]any
							if err := json.Unmarshal(line, &event); err != nil {
								t.Fatal(err)
							}
							var stamp, ok = event["Time"].(string)
							var instant, err = time.Parse(time.RFC3339Nano, stamp)
							if !ok || err != nil {
								t.Fatal("original event time missing", err)
							}
							event["Time"] = instant.Add(-24 * time.Hour).Format(time.RFC3339Nano)
							var out []byte
							out, err = json.Marshal(event)
							if err != nil {
								t.Fatal(err)
							}
							lines = append(lines, out)
						}
						var shifted = append(bytes.Join(lines, []byte("\n")), '\n')
						if err := parseStrictNamedProductionOutputInPackage(combinedEditorPackageID, combinedEditorRequiredEvents(),
							productionTestTimeout(combinedEditorTests)+time.Minute, shifted); err != nil {
							t.Fatal("chronology-only control damaged the original event shape", err)
						}
						return shifted
					})
				}
				if err := validateCombinedEditorRecord("RP39", broken); err == nil {
					t.Fatal("broken same-document RP39 evidence accepted", fault)
				}
			})
		}
	}
	if final, err := ValidateEvidenceManifest(manifest, selected, "final"); err == nil || final != nil && final.ClosureComplete {
		t.Fatal("RP39 closed without its actual combined iOS UI scenario")
	}
	var directory = t.TempDir()
	if runner.artifactDir != "" {
		directory = runner.artifactDir
	}
	var auditPath = filepath.Join(directory, "combined-editor-audit.json")
	if err = writeExecutionAudit(runner, all, auditPath); err != nil {
		t.Fatal(err)
	}
	var raw []byte
	raw, err = os.ReadFile(auditPath)
	if err != nil {
		t.Fatal(err)
	}
	var audit struct {
		Rows            []ExecutionAuditRow `json:"rows"`
		ClosureComplete bool                `json:"closure_complete"`
	}
	if err = json.Unmarshal(raw, &audit); err != nil || len(audit.Rows) != 179 || audit.ClosureComplete {
		t.Fatal("RP39 changed immutable full179 accounting", err)
	}
	for _, row := range audit.Rows {
		if row.Status == "PASS" || row.ID != "RP39" && row.PositiveAndNegative {
			t.Fatal("RP39 local proof completed an unproved requirement", row.ID)
		}
		if row.ID == "RP39" && (!row.PositiveAndNegative || !reflect.DeepEqual(row.TestNames, combinedEditorTests) ||
			!reflect.DeepEqual(row.ProductionPath, combinedEditorProductionPaths) || row.Scope != combinedEditorScope) {
			t.Fatal("actual combined-service scope lost")
		}
	}
	var proof []byte
	proof, err = json.MarshalIndent(map[string]any{"source": runner.implSHA, "report": report, "results": runner.results, "records": runner.records}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(directory, "combined-editor-evidence.json"), append(proof, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
	t.Logf("RP39 executed all 24 fixed events on %s; only unit/service, actual combined iOS UI scenario remains pending", runner.implSHA)
}
