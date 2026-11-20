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

func exhaustiveNodeSelectedContracts(t *testing.T, all *ContractsSet) *ContractsSet {
	t.Helper()
	selected := &ContractsSet{Acceptance: &ContractFile{}, Repair: &RepairContractFile{}, Gates: &GatesContractFile{}}
	for _, c := range all.Repair.Cases {
		if c.ID == "RP16" {
			selected.Repair.Cases = append(selected.Repair.Cases, c)
		}
	}
	if len(selected.Repair.Cases) != 1 {
		t.Fatal("missing immutable RP16 contract")
	}
	return selected
}

func TestExhaustiveNodeProducerInventory(t *testing.T) {
	all, err := LoadAllContracts("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	selected := exhaustiveNodeSelectedContracts(t, all)
	row := selected.Repair.Cases[0]
	if row.Title != "Exhaustive node handling" || !reflect.DeepEqual(parseRequiredTiers(strings.Join(row.RequiredTestLevels, "+")), exhaustiveNodeTiers) ||
		row.Scenario != "Send every approved expression/declaration/statement kind through checked lowering and inject an unknown node in a test." ||
		row.ExpectedObservation != "Required nodes work; deliberately unknown nodes fail explicitly; no silent nil success." {
		t.Fatal("RP16 producer drifted from immutable contract", row)
	}
	if len(exhaustiveNodeKinds) != 42 || len(exhaustiveNodeRequiredEvents()) != 114 || len(exhaustiveNodeNegativeContexts) != 12 || len(exhaustiveNodeTypeContexts) != 11 {
		t.Fatal("complete approved-kind/unknown-boundary inventory changed")
	}
	seen := map[string]bool{}
	for _, name := range exhaustiveNodeRequiredEvents() {
		if seen[name] {
			t.Fatal("duplicate required event", name)
		}
		seen[name] = true
	}
	if !reflect.DeepEqual(deriveRepairTestIDs(row.ID, row.Title), exhaustiveNodeTestIDs()) || !reflect.DeepEqual(deriveRepairFixtures(row.ID), exhaustiveNodeProductionPaths) {
		t.Fatal("RP16 mapping omitted actual producers or sources")
	}
	for _, path := range exhaustiveNodeProductionPaths {
		info, err := os.Stat(filepath.Join(resolveRepoRoot(), filepath.FromSlash(path)))
		if err != nil || !info.Mode().IsRegular() {
			t.Fatal("missing exhaustive-node production source", path, err)
		}
	}
	runner := NewRunner()
	for _, id := range []string{"P03", "F03", "FN01", "M03", "RP15", "RP17"} {
		if runner.runExhaustiveNodeRequirement(id, "unrelated", exhaustiveNodeTiers) {
			t.Fatal("related requirement inherited RP16 producer", id)
		}
	}
	if len(runner.records) != 0 || len(runner.results) != 0 || len(runner.observations) != 0 {
		t.Fatal("unexecuted related requirement received credit")
	}
	if !runner.runExhaustiveNodeRequirement("RP16", row.Title, []string{"unit"}) || runner.results["RP16"].Status != "FAILED" || len(runner.records) != 0 {
		t.Fatal("incomplete requested tiers accepted")
	}
	if !runner.runExhaustiveNodeRequirement("RP16", "different scenario", exhaustiveNodeTiers) || runner.results["RP16"].Status != "FAILED" || len(runner.records) != 0 {
		t.Fatal("different requested scenario accepted")
	}
}

// Synthetic streams are parser controls only; actual evidence below always
// comes from a fresh source-bound subprocess executing the production compiler.
func exhaustiveNodeControlEvents() []byte {
	var lines [][]byte
	emit := func(action, name string) {
		event := map[string]any{"Time": "2026-11-20T00:00:00Z", "Action": action, "Package": "github.com/electrikmilk/cherri"}
		if name != "" {
			event["Test"] = name
		}
		if action == "pass" {
			event["Elapsed"] = 0.001
		}
		encoded, _ := json.Marshal(event)
		lines = append(lines, encoded)
	}
	emit("start", "")
	for _, parent := range exhaustiveNodeTests {
		emit("run", parent)
		for _, name := range exhaustiveNodeRequiredEvents() {
			if strings.HasPrefix(name, parent+"/") {
				emit("run", name)
				emit("pass", name)
			}
		}
		emit("pass", parent)
	}
	emit("pass", "")
	return append(bytes.Join(lines, []byte("\n")), '\n')
}

func exhaustiveNodeCorruptRaw(t *testing.T, kind, target string, raw []byte) []byte {
	t.Helper()
	if kind == "invalid_utf8" {
		return append(append([]byte{}, raw...), 0xff)
	}
	if kind == "duplicate_json_key" {
		return bytes.Replace(raw, []byte("{"), []byte("{\"Action\":\"start\","), 1)
	}
	if kind == "trailing_json" {
		return append(append([]byte{}, raw...), []byte("{}\n")...)
	}
	if kind == "outside_process_interval" {
		return bytes.ReplaceAll(raw, []byte("\"Time\":\"2026"), []byte("\"Time\":\"2000"))
	}
	var result [][]byte
	changed := false
	for _, line := range bytes.Split(bytes.TrimSpace(raw), []byte("\n")) {
		var event map[string]any
		if err := json.Unmarshal(line, &event); err != nil {
			t.Fatal(err)
		}
		test, _ := event["Test"].(string)
		action, _ := event["Action"].(string)
		if kind == "missing_event" && test == target {
			changed = true
			continue
		}
		if !changed {
			switch kind {
			case "orphan_pass":
				if test == target && action == "run" {
					changed = true
					continue
				}
			case "skipped_child":
				if test == target && action == "pass" {
					event["Action"] = "skip"
					changed = true
				}
			case "failed_child":
				if test == target && action == "pass" {
					event["Action"] = "fail"
					changed = true
				}
			case "foreign_package":
				event["Package"] = "example.invalid/unexecuted"
				changed = true
			case "unknown_child":
				if test == target {
					event["Test"] = target + "_unselected"
					changed = true
				}
			case "duplicate_run":
				if test == target && action == "run" {
					result = append(result, append([]byte{}, line...))
					changed = true
				}
			case "parent_before_child":
				if test == target && action == "run" {
					parent := strings.SplitN(target, "/", 2)[0]
					extra := map[string]any{"Time": event["Time"], "Action": "pass", "Package": event["Package"], "Test": parent, "Elapsed": 0.001}
					encoded, _ := json.Marshal(extra)
					result = append(result, encoded)
					changed = true
				}
			case "missing_time":
				delete(event, "Time")
				changed = true
			case "invalid_time":
				event["Time"] = "not-a-time"
				changed = true
			case "backwards_time":
				if action != "start" {
					event["Time"] = "2000-01-01T00:00:00Z"
					changed = true
				}
			case "missing_elapsed":
				if action == "pass" {
					delete(event, "Elapsed")
					changed = true
				}
			case "negative_elapsed":
				if action == "pass" {
					event["Elapsed"] = -1
					changed = true
				}
			case "oversized_elapsed":
				if action == "pass" {
					event["Elapsed"] = 3600
					changed = true
				}
			}
		}
		encoded, err := json.Marshal(event)
		if err != nil {
			t.Fatal(err)
		}
		result = append(result, encoded)
	}
	if !changed {
		t.Fatal("raw control did not change a targeted event", kind, target)
	}
	return append(bytes.Join(result, []byte("\n")), '\n')
}

func TestExhaustiveNodeEventParserControls(t *testing.T) {
	raw := exhaustiveNodeControlEvents()
	if err := parseExhaustiveNodeOutput(raw); err != nil {
		t.Fatal(err)
	}
	for _, name := range exhaustiveNodeRequiredEvents() {
		t.Run("omitted/"+strings.ReplaceAll(name, "/", "_"), func(t *testing.T) {
			if parseExhaustiveNodeOutput(exhaustiveNodeCorruptRaw(t, "missing_event", name, raw)) == nil {
				t.Fatal("omitted required producer or child accepted", name)
			}
		})
	}
	target := exhaustiveNodeTests[0] + "/declaration_ImportDecl"
	for _, fault := range []string{"orphan_pass", "skipped_child", "failed_child", "foreign_package", "unknown_child", "duplicate_run", "parent_before_child", "missing_time", "invalid_time", "backwards_time", "missing_elapsed", "negative_elapsed", "oversized_elapsed", "invalid_utf8", "duplicate_json_key", "trailing_json"} {
		t.Run(fault, func(t *testing.T) {
			if parseExhaustiveNodeOutput(exhaustiveNodeCorruptRaw(t, fault, target, raw)) == nil {
				t.Fatal("damaged synthetic parser control accepted", fault)
			}
		})
	}
}

func exhaustiveNodeRewriteArtifacts(t *testing.T, record *EvidenceRecord, mutate func(map[string][]byte, *editorProcessReceipt)) {
	t.Helper()
	data, err := stateReadArtifacts(record.Artifacts, exhaustiveNodeArtifacts)
	if err != nil {
		t.Fatal(err)
	}
	var receipt editorProcessReceipt
	if err := decodeEditorJSON(data["rp16-process.json"], &receipt); err != nil {
		t.Fatal(err)
	}
	mutate(data, &receipt)
	receipt.RawSHA256 = editorSHA(data["rp16-go.jsonl"])
	receipt.RawBytes = len(data["rp16-go.jsonl"])
	receipt.StderrSHA256 = editorSHA(data["rp16-stderr.log"])
	receipt.StderrBytes = len(data["rp16-stderr.log"])
	encoded, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	data["rp16-process.json"] = append(encoded, '\n')
	root := t.TempDir()
	record.Artifacts = nil
	for _, name := range exhaustiveNodeArtifacts {
		path := filepath.Join(root, name)
		if err := os.WriteFile(path, data[name], 0600); err != nil {
			t.Fatal(err)
		}
		record.Artifacts = append(record.Artifacts, EvidenceArtifact{Path: path, SHA256: editorSHA(data[name])})
	}
}

func TestExhaustiveNodeActualProductionEvidence(t *testing.T) {
	all, err := LoadAllContracts("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	selected := exhaustiveNodeSelectedContracts(t, all)
	runner := NewRunner()
	runner.artifactDir = os.Getenv("CHERRI_RP16_EVIDENCE_OUTPUT")
	if runner.artifactDir == "" {
		runner.artifactDir = t.TempDir()
	}
	runner.runRepairCases(selected.Repair)
	if len(runner.records) != 2 || len(runner.results) != 1 || len(runner.observations) != 0 {
		t.Fatalf("RP16 actual accounting: records=%d results=%+v observations=%v", len(runner.records), runner.results, runner.observations)
	}
	result := runner.results["RP16"]
	if !result.Passed || result.Status != "PASSED" || result.Group != "repair" || !reflect.DeepEqual(result.ExecutedLevels, exhaustiveNodeTiers) {
		t.Fatalf("actual full RP16 production did not pass: %+v", result)
	}
	for _, record := range runner.records {
		if err := validateExhaustiveNodeRecord("RP16", record); err != nil {
			t.Fatal(err)
		}
	}
	manifest := &EvidenceManifest{SchemaVersion: "1", ImplementationSHA: runner.implSHA, Records: runner.records}
	report, err := ValidateEvidenceManifest(manifest, selected, "local")
	if err != nil || report == nil || report.PassedCases != 1 || report.FailedCases != 0 || report.SkippedCases != 0 || report.ClosureComplete {
		t.Fatal("complete RP16-only source-bound production rejected", report, err)
	}
	// Final release validation independently requires the global consumer tuple
	// and real CI runs, even when selecting one requirement. Do not fabricate
	// those unrelated proofs to make this local producer appear release-ready.
	final, finalErr := ValidateEvidenceManifest(manifest, selected, "final")
	if finalErr == nil || final == nil || final.PassedCases != 1 || final.ClosureComplete ||
		!strings.Contains(finalErr.Error(), "missing required successful CI workflow run") {
		t.Fatal("local RP16 proof bypassed the global final release gate", final, finalErr)
	}
	original, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	faults := []string{
		"wrong_producer", "borrowed_id", "fake_ios", "wrong_tier", "truncated_command", "missing_child_binding", "wrong_scope", "stale_source", "failed_result", "failed_exit",
		"missing_artifact", "duplicate_artifact", "bad_artifact_digest", "missing_tier", "process_source", "dirty_before", "dirty_after", "missing_exit", "missing_timeout", "timeout",
		"missing_discovery", "missing_required_test", "bad_time", "forged_source_hash", "missing_source_path", "process_requirement", "process_command", "forged_benchmark",
		"raw_missing_approved", "raw_missing_unknown", "raw_orphan_pass", "raw_skipped_child", "raw_failed_child", "raw_foreign_package", "raw_unknown_child", "raw_duplicate_run", "raw_parent_before_child",
		"raw_invalid_utf8", "raw_duplicate_json_key", "raw_trailing_json", "raw_missing_time", "raw_invalid_time", "raw_backwards_time", "raw_missing_elapsed", "raw_negative_elapsed", "raw_oversized_elapsed", "raw_outside_process_interval",
		"stderr_invalid_utf8",
	}
	for _, actual := range runner.records {
		t.Run(actual.Tier, func(t *testing.T) {
			for _, fault := range faults {
				t.Run(fault, func(t *testing.T) {
					var broken EvidenceManifest
					if err := json.Unmarshal(original, &broken); err != nil {
						t.Fatal(err)
					}
					for index := range broken.Records {
						changed := &broken.Records[index]
						if changed.EvidenceID != actual.EvidenceID {
							continue
						}
						switch fault {
						case "wrong_producer":
							changed.TestID = "tools/language-acceptance:runControl"
						case "borrowed_id":
							changed.Requirements = append(changed.Requirements, "F03")
						case "fake_ios":
							changed.Tier = "ios-runtime"
						case "wrong_tier":
							changed.Tier = "native-structure"
						case "truncated_command":
							changed.Command = productionTestCommand(exhaustiveNodeTests[:1])
						case "missing_child_binding":
							delete(changed.Assertions, "required_go_events")
						case "wrong_scope":
							changed.Assertions["scope"] = "All original linked requirements complete"
						case "stale_source":
							changed.ImplementationSHA = strings.Repeat("e", 40)
						case "failed_result":
							changed.Result = "failed"
						case "failed_exit":
							changed.ExitCode = 1
						case "missing_artifact":
							changed.Artifacts = changed.Artifacts[:2]
						case "duplicate_artifact":
							changed.Artifacts[2] = changed.Artifacts[1]
						case "bad_artifact_digest":
							changed.Artifacts[0].SHA256 = strings.Repeat("0", 64)
						case "missing_tier":
							broken.Records = append(broken.Records[:index], broken.Records[index+1:]...)
						default:
							exhaustiveNodeRewriteArtifacts(t, changed, func(data map[string][]byte, receipt *editorProcessReceipt) {
								switch fault {
								case "process_source":
									receipt.SourceCommit = strings.Repeat("a", 40)
								case "dirty_before":
									receipt.SourceCleanBefore = false
								case "dirty_after":
									receipt.SourceCleanAfter = false
								case "missing_exit":
									receipt.ExitCode = nil
								case "missing_timeout":
									receipt.TimedOut = nil
								case "timeout":
									value := true
									receipt.TimedOut = &value
								case "missing_discovery":
									receipt.DiscoveredTests = receipt.DiscoveredTests[:len(receipt.DiscoveredTests)-1]
								case "missing_required_test":
									receipt.RequiredTests = receipt.RequiredTests[:len(receipt.RequiredTests)-1]
								case "bad_time":
									receipt.ElapsedSeconds = 0
								case "forged_source_hash":
									receipt.SourceFiles[0].SHA256 = strings.Repeat("0", 64)
								case "missing_source_path":
									receipt.SourceFiles = receipt.SourceFiles[:len(receipt.SourceFiles)-1]
								case "process_requirement":
									receipt.Requirement = "F03"
								case "process_command":
									receipt.Command = productionTestCommand(exhaustiveNodeTests[:1])
								case "forged_benchmark":
									receipt.BenchmarkSHA256 = strings.Repeat("a", 64)
									receipt.BenchmarkBytes = 1
								case "stderr_invalid_utf8":
									data["rp16-stderr.log"] = []byte{0xff}
								default:
									target := exhaustiveNodeTests[0] + "/declaration_ImportDecl"
									kind := strings.TrimPrefix(fault, "raw_")
									if fault == "raw_missing_approved" {
										kind = "missing_event"
									}
									if fault == "raw_missing_unknown" {
										kind = "missing_event"
										target = exhaustiveNodeTests[3] + "/alias"
									}
									raw := exhaustiveNodeCorruptRaw(t, kind, target, data["rp16-go.jsonl"])
									if bytes.Equal(raw, data["rp16-go.jsonl"]) {
										t.Fatal("actual raw corruption did not occur")
									}
									data["rp16-go.jsonl"] = raw
								}
							})
						}
						break
					}
					report, err := ValidateEvidenceManifest(&broken, selected, "local")
					if err == nil || report == nil || report.ClosureComplete || report.PassedCases != 0 {
						t.Fatal("damaged actual RP16 evidence accepted", fault, report, err)
					}
				})
			}
		})
	}
	for index, path := range exhaustiveNodeProductionPaths {
		t.Run("source_digest/"+strings.ReplaceAll(path, "/", "_"), func(t *testing.T) {
			var changed EvidenceRecord
			encoded, _ := json.Marshal(runner.records[0])
			if err := json.Unmarshal(encoded, &changed); err != nil {
				t.Fatal(err)
			}
			exhaustiveNodeRewriteArtifacts(t, &changed, func(_ map[string][]byte, receipt *editorProcessReceipt) {
				receipt.SourceFiles[index].SHA256 = strings.Repeat("0", 64)
			})
			if validateExhaustiveNodeRecord("RP16", changed) == nil {
				t.Fatal("hash-reconciled immutable source corruption accepted", path)
			}
		})
	}
	for _, id := range []string{"P03", "F03", "FN01", "M03"} {
		t.Run("no_inheritance/"+id, func(t *testing.T) {
			related := &ContractsSet{Acceptance: &ContractFile{}, Repair: &RepairContractFile{}, Gates: &GatesContractFile{}}
			for _, row := range all.Acceptance.Cases {
				if row.ID == id {
					related.Acceptance.Cases = append(related.Acceptance.Cases, row)
				}
			}
			if len(related.Acceptance.Cases) != 1 {
				t.Fatal("missing original related contract", id)
			}
			related.Acceptance.CaseCount = 1
			var rebound EvidenceManifest
			if err := json.Unmarshal(original, &rebound); err != nil {
				t.Fatal(err)
			}
			for index := range rebound.Records {
				rebound.Records[index].Requirements = []string{id}
			}
			report, err := ValidateEvidenceManifest(&rebound, related, "local")
			if err == nil || report == nil || report.PassedCases != 0 || report.ClosureComplete {
				t.Fatal("linked requirement inherited actual RP16 proof", id, report, err)
			}
		})
	}
	auditPath := filepath.Join(t.TempDir(), "audit.json")
	if err := writeExecutionAudit(runner, all, auditPath); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatal(err)
	}
	var audit bindingCaptureAuditEnvelope
	if err := json.Unmarshal(raw, &audit); err != nil || len(audit.Rows) != 179 || audit.ClosureComplete {
		t.Fatal("RP16 altered 179-row formal scope", err)
	}
	for _, row := range audit.Rows {
		if row.ID == "RP16" {
			if row.Status != "PASS" || !row.PositiveAndNegative || !reflect.DeepEqual(row.TestNames, exhaustiveNodeTests) ||
				!reflect.DeepEqual(row.ProductionPath, exhaustiveNodeProductionPaths) || !reflect.DeepEqual(row.Tier, exhaustiveNodeTiers) || row.Scope != exhaustiveNodeScope {
				t.Fatalf("RP16 audit lost exact actual scope: %+v", row)
			}
		} else if row.Status == "PASS" {
			t.Fatal("unexecuted requirement inherited RP16 proof", row.ID)
		}
	}
	receipt, err := json.Marshal(map[string]any{"source_sha": runner.implSHA, "scope": exhaustiveNodeScope, "report": report, "records": runner.records})
	if err != nil {
		t.Fatal(err)
	}
	t.Log("P15_RP16_PRODUCTION_RECEIPT:" + string(receipt))
}
