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

func previewEditSelectedContracts(t *testing.T) (*ContractsSet, *ContractsSet) {
	t.Helper()
	all, err := LoadAllContracts("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	selected := &ContractsSet{Acceptance: &ContractFile{}, Repair: &RepairContractFile{}, Gates: &GatesContractFile{}}
	for _, row := range all.Repair.Cases {
		if row.ID == "RP41" {
			selected.Repair.Cases = append(selected.Repair.Cases, row)
		}
	}
	if len(selected.Repair.Cases) != 1 {
		t.Fatal("missing immutable RP41 contract")
	}
	selected.Repair.CaseCount = 1
	return all, selected
}

func TestPreviewEditProducerInventory(t *testing.T) {
	_, selected := previewEditSelectedContracts(t)
	row := selected.Repair.Cases[0]
	if row.Title != previewEditTitle || row.Scenario != previewEditScenario ||
		row.ExpectedObservation != previewEditExpected ||
		!reflect.DeepEqual(parseRequiredTiers(strings.Join(row.RequiredTestLevels, "+")), previewEditRequiredTiers) ||
		!reflect.DeepEqual(previewEditRequiredTiers, []string{"integration", "ios-ui"}) ||
		!reflect.DeepEqual(previewEditLocalTiers, []string{"integration"}) {
		t.Fatal("RP41 contract or local/external boundary changed")
	}
	events := previewEditRequiredEvents()
	if len(previewEditTests) != 5 || len(events) != 40 || !sort.StringsAreSorted(events) ||
		previewEditProcessBudget != 300*time.Second ||
		!reflect.DeepEqual(previewEditCommand(), []string{"go", "test", "-json", "-count=1", "-timeout=60s",
			"-run", "^(" + strings.Join(previewEditTests, "|") + ")$", "."}) {
		t.Fatal("RP41 complete discovery inventory or original deadlines changed")
	}
	seen := map[string]bool{}
	for _, name := range events {
		if name == "" || seen[name] {
			t.Fatal("invalid preview event inventory", name)
		}
		seen[name] = true
	}
	for _, path := range previewEditProductionPaths {
		if seen[path] {
			t.Fatal("duplicate preview source witness", path)
		}
		seen[path] = true
		info, err := os.Stat(filepath.Join(resolveRepoRoot(), filepath.FromSlash(path)))
		if err != nil || !info.Mode().IsRegular() {
			t.Fatal("missing preview source witness", path, err)
		}
	}
	if !reflect.DeepEqual(deriveRepairTestIDs(row.ID, row.Title), previewEditTestIDs()) ||
		!reflect.DeepEqual(deriveRepairFixtures(row.ID), previewEditProductionPaths) {
		t.Fatal("dedicated RP41 mapping is incomplete")
	}
	var ids []string
	assertions, ui := map[string]string{}, map[string]string{}
	applyRuntimeAndUIMappings("RP41", previewEditRequiredTiers, &ids, assertions, ui)
	if len(ids) != 0 || len(assertions) != 0 || len(ui) != 0 {
		t.Fatal("generic E07 UI metadata supplied the distinct joined RP41 scenario")
	}
	runner := NewRunner()
	for _, id := range []string{"E07", "RP39", "RP40", "RP51", "BRG27", "BRG28"} {
		foreign := row
		foreign.ID = id
		if runner.runPreviewEditRequirement(foreign) {
			t.Fatal("related requirement inherited RP41 producer", id)
		}
	}
	for _, field := range []string{"title", "scenario", "expected", "tiers"} {
		t.Run(field, func(t *testing.T) {
			changed := row
			switch field {
			case "title":
				changed.Title = "different preview"
			case "scenario":
				changed.Scenario = "AST edit only"
			case "expected":
				changed.ExpectedObservation = "generated action may be edited"
			case "tiers":
				changed.RequiredTestLevels = []string{"integration"}
			}
			if !runner.runPreviewEditRequirement(changed) || runner.results["RP41"].Status != "FAILED" || len(runner.records) != 0 {
				t.Fatal("changed immutable RP41 scenario accepted", field)
			}
		})
	}
}

// Synthetic events are controls for the strict parser only. They never create a
// source/process receipt, EvidenceRecord, runtime result or acceptance tier.
func previewEditParserFixture(t *testing.T) []map[string]any {
	t.Helper()
	var events []map[string]any
	clock := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	add := func(action, name string) {
		clock = clock.Add(time.Millisecond)
		event := map[string]any{"Time": clock.Format(time.RFC3339Nano), "Action": action, "Package": "github.com/electrikmilk/cherri"}
		if name != "" {
			event["Test"] = name
		}
		if action == "pass" {
			event["Elapsed"] = 0.0
		}
		events = append(events, event)
	}
	add("start", "")
	for _, parent := range previewEditTests {
		add("run", parent)
		for _, child := range previewEditRequiredEvents() {
			if strings.HasPrefix(child, parent+"/") {
				add("run", child)
				add("pass", child)
			}
		}
		add("pass", parent)
	}
	add("pass", "")
	return events
}

func previewEditEncodeEvents(t *testing.T, events []map[string]any) []byte {
	t.Helper()
	var raw bytes.Buffer
	for _, event := range events {
		line, err := json.Marshal(event)
		if err != nil {
			t.Fatal(err)
		}
		raw.Write(line)
		raw.WriteByte('\n')
	}
	return raw.Bytes()
}

func TestPreviewEditStrictEventIntake(t *testing.T) {
	healthy := previewEditEncodeEvents(t, previewEditParserFixture(t))
	if err := parseStrictNamedProductionOutput(previewEditRequiredEvents(), previewEditProcessBudget, healthy); err != nil {
		t.Fatal("healthy synthetic parser control rejected", err)
	}
	target := previewEditTests[1] + "/generated_condition_value_read_only"
	for _, fault := range []string{"missing_joined_parent", "missing_generated_child", "child_failed", "child_skipped",
		"duplicate_child", "foreign_package", "unselected_test", "missing_elapsed", "negative_elapsed",
		"backwards_time", "output_after_package"} {
		t.Run(fault, func(t *testing.T) {
			events := previewEditParserFixture(t)
			want := ""
			switch fault {
			case "missing_joined_parent", "missing_generated_child":
				missing := previewEditTests[0]
				if fault == "missing_generated_child" {
					missing = target
				}
				var kept []map[string]any
				for _, event := range events {
					if event["Test"] != missing {
						kept = append(kept, event)
					}
				}
				events = kept
				want = "without complete scenario"
				if fault == "missing_generated_child" {
					want = "before required child"
				}
			case "child_failed", "child_skipped", "duplicate_child", "missing_elapsed", "negative_elapsed":
				for i, event := range events {
					if event["Test"] != target || event["Action"] != "pass" {
						continue
					}
					switch fault {
					case "child_failed", "child_skipped":
						event["Action"] = "fail"
						if fault == "child_skipped" {
							event["Action"] = "skip"
						}
						delete(event, "Elapsed")
						want = "failed, skipped"
					case "duplicate_child":
						events = append(events[:i], append([]map[string]any{event}, events[i:]...)...)
						want = "without unique prior run"
					case "missing_elapsed":
						delete(event, "Elapsed")
						want = "lacks observed elapsed"
					case "negative_elapsed":
						event["Elapsed"] = -1.0
						want = "invalid bounded"
					}
					break
				}
			case "foreign_package":
				events[0]["Package"] = "foreign/module"
				want = "foreign or unselected"
			case "unselected_test":
				events[1]["Test"] = "TestUnselectedPreview"
				want = "foreign or unselected"
			case "backwards_time":
				events[1]["Time"] = "2025-12-31T23:59:59Z"
				want = "backwards production"
			case "output_after_package":
				events = append(events, map[string]any{"Action": "output"})
				want = "continues after package"
			}
			err := parseStrictNamedProductionOutput(previewEditRequiredEvents(), previewEditProcessBudget, previewEditEncodeEvents(t, events))
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("causal %s rejection absent: want %q, got %v", fault, want, err)
			}
		})
	}
}

func TestPreviewEditActualProductionEvidence(t *testing.T) {
	all, selected := previewEditSelectedContracts(t)
	runner := NewRunner()
	runner.artifactDir = os.Getenv("CHERRI_PREVIEW_EDIT_EVIDENCE_OUTPUT")
	runner.runRepairCases(selected.Repair)
	result := runner.results["RP41"]
	if len(runner.results) != 1 || len(runner.records) != 1 || len(runner.observations) != 0 ||
		result.Passed || result.Status != "PENDING_EXTERNAL" ||
		!reflect.DeepEqual(result.ExecutedLevels, previewEditLocalTiers) {
		t.Fatalf("RP41 local production missing or expanded: %+v records=%d", result, len(runner.records))
	}
	original := runner.records[0]
	if err := validatePreviewEditRecord("RP41", original); err != nil {
		t.Fatal(err)
	}
	manifest := &EvidenceManifest{SchemaVersion: "1", ImplementationSHA: runner.implSHA, Records: runner.records}
	report, err := ValidateEvidenceManifest(manifest, selected, "local")
	if err != nil || report == nil || report.PassedCases != 0 || report.FailedCases != 0 ||
		report.SkippedCases != 1 || report.ClosureComplete {
		t.Fatalf("RP41 integration acquired absent UI credit: %+v %v", report, err)
	}
	for _, fault := range []string{"borrowed_e07", "borrowed_brg28", "fake_ui", "partial_tests", "changed_scope",
		"wrong_source_blob", "nonzero_process", "timeout_process", "missing_joined_parent",
		"missing_generated_control", "missing_stale_control", "events_outside_process"} {
		t.Run(fault, func(t *testing.T) {
			broken := editorContractCopyRecord(t, original)
			switch fault {
			case "borrowed_e07":
				broken.TestID = "github.com/electrikmilk/cherri/internal/language/service:TestE07BoundedVisualSourceEditAndRejection"
			case "borrowed_brg28":
				broken.Requirements = []string{"BRG28"}
			case "fake_ui":
				broken.Tier = "ios-ui"
				broken.TestID = "CherriUITests/testE07VisualParameterEditingCanvasUI"
			case "partial_tests":
				broken.Command[6] = "^(" + previewEditTests[0] + ")$"
			case "changed_scope":
				broken.Assertions["scope"] = "Swift UI passed"
			case "wrong_source_blob", "nonzero_process", "timeout_process":
				editorContractMutateArtifact(t, &broken, previewEditPrefix+"-process.json", func(raw []byte) []byte {
					var receipt editorProcessReceipt
					if err := json.Unmarshal(raw, &receipt); err != nil {
						t.Fatal(err)
					}
					switch fault {
					case "wrong_source_blob":
						changed := false
						for i := range receipt.SourceFiles {
							if receipt.SourceFiles[i].Path == "p15_preview_source_mapping_contract_test.go" {
								receipt.SourceFiles[i].SHA256 = strings.Repeat("e", 64)
								changed = true
							}
						}
						if !changed {
							t.Fatal("actual combined source witness absent")
						}
					case "nonzero_process":
						code := 1
						receipt.ExitCode = &code
					case "timeout_process":
						timedOut := true
						receipt.TimedOut = &timedOut
					}
					out, err := json.Marshal(receipt)
					if err != nil {
						t.Fatal(err)
					}
					return out
				})
			default:
				target := previewEditTests[0]
				if fault == "missing_generated_control" {
					target = previewEditTests[1] + "/generated_condition_value_read_only"
				} else if fault == "missing_stale_control" {
					target = previewEditTests[3] + "/stale_source"
				}
				var changed []byte
				editorContractMutateArtifact(t, &broken, previewEditPrefix+"-go.jsonl", func(raw []byte) []byte {
					var output bytes.Buffer
					found := false
					for _, line := range bytes.Split(bytes.TrimSpace(raw), []byte{'\n'}) {
						var event map[string]any
						if err := json.Unmarshal(line, &event); err != nil {
							t.Fatal(err)
						}
						if fault == "events_outside_process" {
							at, err := time.Parse(time.RFC3339Nano, event["Time"].(string))
							if err != nil {
								t.Fatal(err)
							}
							event["Time"] = at.Add(time.Hour).Format(time.RFC3339Nano)
							line, err = json.Marshal(event)
							if err != nil {
								t.Fatal(err)
							}
							found = true
						} else if event["Test"] == target {
							found = true
							continue
						}
						output.Write(line)
						output.WriteByte('\n')
					}
					changed = output.Bytes()
					if !found || bytes.Equal(changed, raw) {
						t.Fatal("causal mutation did not alter its selected original events")
					}
					return changed
				})
				// Re-pin the original raw-stream binding coherently. A rejection
				// must come from actual event completeness/time, not a hash typo.
				editorContractMutateArtifact(t, &broken, previewEditPrefix+"-process.json", func(raw []byte) []byte {
					var receipt editorProcessReceipt
					if err := json.Unmarshal(raw, &receipt); err != nil {
						t.Fatal(err)
					}
					receipt.RawSHA256, receipt.RawBytes = editorSHA(changed), len(changed)
					out, err := json.Marshal(receipt)
					if err != nil {
						t.Fatal(err)
					}
					return out
				})
			}
			err := validatePreviewEditRecord("RP41", broken)
			if err == nil {
				t.Fatal("invalid RP41 evidence accepted", fault)
			}
			if fault == "missing_joined_parent" && !strings.Contains(err.Error(), "without complete scenario") ||
				(fault == "missing_generated_control" || fault == "missing_stale_control") && !strings.Contains(err.Error(), "before required child") ||
				fault == "events_outside_process" && !strings.Contains(err.Error(), "outside the observed process") {
				t.Fatalf("causal event/time mutation reached the wrong guard: %s: %v", fault, err)
			}
		})
	}
	if final, err := ValidateEvidenceManifest(manifest, selected, "final"); err == nil || final != nil && final.ClosureComplete {
		t.Fatal("local RP41 producer closed missing actual iOS UI")
	}
	directory := runner.artifactDir
	if directory == "" {
		directory = t.TempDir()
	}
	auditPath := filepath.Join(directory, "preview-edit-audit.json")
	if err := writeExecutionAudit(runner, all, auditPath); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatal(err)
	}
	var audit struct {
		ClosureComplete bool                `json:"closure_complete"`
		Rows            []ExecutionAuditRow `json:"rows"`
	}
	if err := json.Unmarshal(raw, &audit); err != nil {
		t.Fatal(err)
	}
	if audit.ClosureComplete || len(audit.Rows) != 179 {
		t.Fatal("RP41 producer changed complete obligation accounting")
	}
	seen := false
	for _, row := range audit.Rows {
		if row.ID == "RP41" {
			seen = true
			if row.Status != "BLOCKED_EXTERNAL" || !row.PositiveAndNegative ||
				!reflect.DeepEqual(row.Tier, previewEditLocalTiers) ||
				!reflect.DeepEqual(row.RequiredTiers, previewEditRequiredTiers) ||
				!reflect.DeepEqual(row.TestNames, previewEditTests) ||
				!reflect.DeepEqual(row.ProductionPath, previewEditProductionPaths) ||
				row.Scope != previewEditScope || row.Detail != result.Detail {
				t.Fatalf("audit lost the exact local/external RP41 boundary: %+v", row)
			}
		} else if row.Status == "PASS" || row.PositiveAndNegative {
			t.Fatalf("related/unexecuted requirement inherited preview credit: %+v", row)
		}
	}
	if !seen {
		t.Fatal("RP41 audit row missing")
	}
	proof, err := json.Marshal(map[string]any{
		"source_sha": runner.implSHA, "scope": previewEditScope,
		"report": report, "results": runner.results, "records": runner.records,
		"native_ui_executed": false, "formal_acceptance_granted": false, "closure_complete": false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "preview-edit-evidence.json"), append(proof, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
	t.Logf("RP41 executed all 40 fixed local events on %s; integration only, actual iOS UI remains pending", runner.implSHA)
}
