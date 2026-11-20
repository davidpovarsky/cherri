package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func functionDefaultSelectedContracts(t *testing.T, all *ContractsSet) *ContractsSet {
	t.Helper()
	selected := &ContractsSet{Acceptance: &ContractFile{}, Repair: &RepairContractFile{}, Gates: &GatesContractFile{}}
	for _, row := range all.Acceptance.Cases {
		if _, found := functionDefaultScenarios[row.ID]; found {
			selected.Acceptance.Cases = append(selected.Acceptance.Cases, row)
		}
	}
	selected.Acceptance.CaseCount = len(selected.Acceptance.Cases)
	if selected.Acceptance.CaseCount != 2 {
		t.Fatal("missing immutable T01/FN04 contracts")
	}
	return selected
}

func TestFunctionDefaultProducerInventory(t *testing.T) {
	all, err := LoadAllContracts("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	selected := functionDefaultSelectedContracts(t, all)
	counts := map[string]int{"T01": 290, "FN04": 33}
	for _, row := range selected.Acceptance.Cases {
		s := functionDefaultScenarios[row.ID]
		if row.Group != s.Group || row.Requirement != s.Title || row.Expected != s.Expected || row.MinimumTestLevel != s.Level ||
			!reflect.DeepEqual(parseRequiredTiers(row.MinimumTestLevel), append(append([]string{}, s.LocalTiers...), "ios-runtime")) {
			t.Fatal("function-boundary scenario drifted from immutable requirement", row)
		}
		if _, smoke := incompleteLocalAssertions[row.ID]; smoke {
			t.Fatal("actual complete local producer remains historical smoke", row.ID)
		}
		if !reflect.DeepEqual(deriveAcceptanceTestIDs(row.ID, row.Group), s.testIDs()) ||
			!reflect.DeepEqual(deriveAcceptanceFixtures(row.ID, row.Group), functionDefaultProductionPaths) {
			t.Fatal("function-boundary mapping lost tests or immutable sources", row.ID)
		}
		ids := append([]string{}, s.testIDs()...)
		assertions, ui := map[string]string{}, map[string]string{}
		applyRuntimeAndUIMappings(row.ID, parseRequiredTiers(row.MinimumTestLevel), &ids, assertions, ui)
		if !reflect.DeepEqual(ids, s.testIDs()) || len(assertions) != 0 || len(ui) != 0 || s.producer("ios-runtime") != "" {
			t.Fatal("historical short runtime trace inherited complete function-boundary credit", row.ID)
		}
		names, seen := functionDefaultRequiredEvents(row.ID), map[string]bool{}
		if len(names) != counts[row.ID] {
			t.Fatal("complete scenario event count changed", row.ID, len(names))
		}
		for _, name := range names {
			if seen[name] {
				t.Fatal("duplicate required function event", name)
			}
			seen[name] = true
		}
		for _, tier := range s.LocalTiers {
			producer := strings.TrimPrefix(s.producer(tier), "github.com/electrikmilk/cherri:")
			if !seen[producer] {
				t.Fatal("tier producer is not in complete actual selection", row.ID, tier)
			}
		}
	}
	if len(functionDefaultScenarios) != 2 || len(functionDefaultProductionPaths) > 64 {
		t.Fatal("function-boundary scope or bounded source request changed")
	}
	seen := map[string]bool{}
	for _, path := range functionDefaultProductionPaths {
		info, err := os.Stat(filepath.Join(resolveRepoRoot(), filepath.FromSlash(path)))
		if seen[path] || err != nil || !info.Mode().IsRegular() {
			t.Fatal("missing or duplicate actual function source", path, err)
		}
		seen[path] = true
	}
	runner := NewRunner()
	for _, id := range []string{"FN01", "FN02", "FN03", "FN05", "T05", "RP25", "RP26", "BRG11", "BRG15"} {
		if runner.runFunctionDefaultRequirement(id) {
			t.Fatal("related obligation inherited an unexecuted scenario", id)
		}
	}
	if len(runner.records) != 0 || len(runner.results) != 0 || len(runner.observations) != 0 {
		t.Fatal("unexecuted requirement received evidence")
	}
}

// Synthetic streams exercise parser control flow only. They are never used as
// execution evidence; the positive intake below launches the actual compiler.
func functionDefaultControlEvents(id string) []byte {
	names := functionDefaultRequiredEvents(id)
	required := map[string]bool{}
	for _, name := range names {
		required[name] = true
	}
	children := map[string][]string{}
	for _, name := range names {
		parent := name
		for {
			slash := strings.LastIndexByte(parent, '/')
			if slash < 0 {
				parent = ""
				break
			}
			parent = parent[:slash]
			if required[parent] {
				break
			}
		}
		children[parent] = append(children[parent], name)
	}
	var lines [][]byte
	emit := func(action, name string) {
		event := map[string]any{"Time": "2026-11-20T00:00:00Z", "Action": action, "Package": "github.com/electrikmilk/cherri"}
		if name != "" {
			event["Test"] = name
		}
		if action == "pass" {
			event["Elapsed"] = 0.001
		}
		line, _ := json.Marshal(event)
		lines = append(lines, line)
	}
	var walk func(string)
	walk = func(parent string) {
		for _, name := range children[parent] {
			emit("run", name)
			walk(name)
			emit("pass", name)
		}
	}
	emit("start", "")
	walk("")
	emit("pass", "")
	return append(bytes.Join(lines, []byte("\n")), '\n')
}

func functionDefaultCorruptRaw(t *testing.T, kind, target string, raw []byte) []byte {
	t.Helper()
	switch kind {
	case "invalid_utf8":
		return append(append([]byte{}, raw...), 0xff)
	case "duplicate_json_key":
		return bytes.Replace(raw, []byte("{"), []byte("{\"Action\":\"start\","), 1)
	case "trailing_json":
		return append(append([]byte{}, raw...), []byte("{}\n")...)
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
		if kind == "outside_process_interval" {
			observed, err := time.Parse(time.RFC3339Nano, event["Time"].(string))
			if err != nil {
				t.Fatal(err)
			}
			event["Time"] = observed.Add(-24 * time.Hour).Format(time.RFC3339Nano)
			changed = true
		}
		if !changed {
			switch kind {
			case "orphan_pass":
				if test == target && action == "run" {
					changed = true
					continue
				}
			case "skipped_child", "failed_child":
				if test == target && action == "pass" {
					if kind == "skipped_child" {
						event["Action"] = "skip"
					} else {
						event["Action"] = "fail"
					}
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
			case "parent_before_child", "nested_parent_before_child":
				if test == target && action == "run" {
					parent := strings.SplitN(target, "/", 2)[0]
					if kind == "nested_parent_before_child" {
						parent = target[:strings.LastIndexByte(target, '/')]
					}
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
			case "missing_elapsed", "negative_elapsed", "oversized_elapsed":
				if action == "pass" {
					switch kind {
					case "missing_elapsed":
						delete(event, "Elapsed")
					case "negative_elapsed":
						event["Elapsed"] = -1
					default:
						event["Elapsed"] = 3600
					}
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
		t.Fatal("raw fault did not change a targeted event", kind, target)
	}
	return append(bytes.Join(result, []byte("\n")), '\n')
}

func TestFunctionDefaultEventParserControls(t *testing.T) {
	for _, id := range []string{"T01", "FN04"} {
		t.Run(id, func(t *testing.T) {
			raw := functionDefaultControlEvents(id)
			if err := parseFunctionDefaultOutput(id, raw); err != nil {
				t.Fatal(err)
			}
			for _, name := range functionDefaultRequiredEvents(id) {
				t.Run("omitted/"+strings.ReplaceAll(name, "/", "_"), func(t *testing.T) {
					if parseFunctionDefaultOutput(id, functionDefaultCorruptRaw(t, "missing_event", name, raw)) == nil {
						t.Fatal("missing required scenario or causal control accepted", name)
					}
				})
			}
			target := functionDefaultScenarios[id].Tests[1] + "/false/supplied_absent"
			if id == "FN04" {
				target = functionDefaultScenarios[id].Tests[2] + "/nested_true/nested_payload_loses_caller"
			}
			for _, fault := range []string{"orphan_pass", "skipped_child", "failed_child", "foreign_package", "unknown_child", "duplicate_run", "parent_before_child", "nested_parent_before_child", "missing_time", "invalid_time", "backwards_time", "missing_elapsed", "negative_elapsed", "oversized_elapsed", "invalid_utf8", "duplicate_json_key", "trailing_json"} {
				t.Run(fault, func(t *testing.T) {
					if parseFunctionDefaultOutput(id, functionDefaultCorruptRaw(t, fault, target, raw)) == nil {
						t.Fatal("damaged parser control accepted", fault)
					}
				})
			}
		})
	}
	if parseFunctionDefaultOutput("FN05", functionDefaultControlEvents("T01")) == nil {
		t.Fatal("unknown scenario accepted")
	}
}

func functionDefaultRewriteArtifacts(t *testing.T, record *EvidenceRecord, mutate func(map[string][]byte, *editorProcessReceipt)) {
	t.Helper()
	data, err := stateReadArtifacts(record.Artifacts, functionDefaultArtifactNames)
	if err != nil {
		t.Fatal(err)
	}
	var receipt editorProcessReceipt
	if err := decodeEditorJSON(data["function-process.json"], &receipt); err != nil {
		t.Fatal(err)
	}
	mutate(data, &receipt)
	receipt.RawSHA256, receipt.RawBytes = editorSHA(data["function-go.jsonl"]), len(data["function-go.jsonl"])
	receipt.StderrSHA256, receipt.StderrBytes = editorSHA(data["function-stderr.log"]), len(data["function-stderr.log"])
	encoded, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	data["function-process.json"] = append(encoded, '\n')
	root := t.TempDir()
	record.Artifacts = nil
	for _, name := range functionDefaultArtifactNames {
		path := filepath.Join(root, name)
		if err := os.WriteFile(path, data[name], 0600); err != nil {
			t.Fatal(err)
		}
		record.Artifacts = append(record.Artifacts, EvidenceArtifact{Path: path, SHA256: editorSHA(data[name])})
	}
}

func TestFunctionDefaultActualProductionEvidence(t *testing.T) {
	all, err := LoadAllContracts("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	selected := functionDefaultSelectedContracts(t, all)
	runner := NewRunner()
	runner.artifactDir = os.Getenv("CHERRI_FUNCTION_DEFAULT_EVIDENCE_OUTPUT")
	if runner.artifactDir == "" {
		runner.artifactDir = t.TempDir()
	}
	for _, id := range []string{"T01", "FN04"} {
		s := functionDefaultScenarios[id]
		if !runner.runFunctionDefaultRequirement(id) {
			t.Fatal("missing function producer", id)
		}
		result := runner.results[id]
		if result.Passed || result.Status != "PENDING_EXTERNAL" || result.Group != s.Group || result.Requirement != s.Title ||
			result.TestLevel != s.Level || !reflect.DeepEqual(result.ExecutedLevels, s.LocalTiers) {
			t.Fatal("actual function-boundary scope lost or expanded", result)
		}
	}
	if len(runner.results) != 2 || len(runner.records) != 3 || len(runner.observations) != 0 {
		t.Fatal("actual function-boundary accounting drifted", runner.results, len(runner.records))
	}
	for _, record := range runner.records {
		if len(record.Requirements) != 1 || record.ImplementationSHA != runner.implSHA {
			t.Fatal("actual function-boundary requirement/source binding lost", record)
		}
		if err := validateFunctionDefaultRecord(record.Requirements[0], record); err != nil {
			t.Fatal(err)
		}
	}
	manifest := &EvidenceManifest{SchemaVersion: "1", ImplementationSHA: runner.implSHA, Records: runner.records}
	report, err := ValidateEvidenceManifest(manifest, selected, "local")
	if err != nil || report == nil || report.TotalCases != 2 || report.PassedCases != 0 || report.FailedCases != 0 || report.SkippedCases != 2 || report.ClosureComplete {
		t.Fatal("local function-boundary evidence falsely closed native execution", report, err)
	}
	for _, row := range report.Results {
		if row.Status != "NOT_RUN" || row.Passed || !reflect.DeepEqual(row.ExecutedLevels, functionDefaultScenarios[row.ID].LocalTiers) {
			t.Fatal("missing actual Apple tier was concealed", row)
		}
	}
	if final, err := ValidateEvidenceManifest(manifest, selected, "final"); err == nil || final == nil || final.ClosureComplete || final.PassedCases != 0 {
		t.Fatal("local function proof bypassed final Apple/release gates", final, err)
	}
	if err := os.MkdirAll(runner.artifactDir, 0755); err != nil {
		t.Fatal(err)
	}
	for name, value := range map[string]any{"function-manifest.json": manifest, "function-report.json": report} {
		data, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(runner.artifactDir, name), append(data, '\n'), 0600); err != nil {
			t.Fatal(err)
		}
	}
	auditPath := filepath.Join(runner.artifactDir, "function-execution-audit.json")
	if err := writeExecutionAudit(runner, all, auditPath); err != nil {
		t.Fatal(err)
	}
	auditData, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatal(err)
	}
	var audit struct{ Rows []ExecutionAuditRow }
	if err := json.Unmarshal(auditData, &audit); err != nil {
		t.Fatal(err)
	}
	if len(audit.Rows) != 179 {
		t.Fatal("full original 179 accounting changed", len(audit.Rows))
	}
	for _, row := range audit.Rows {
		if s, found := functionDefaultScenarios[row.ID]; found {
			if row.Status != "BLOCKED_EXTERNAL" || !row.PositiveAndNegative || !reflect.DeepEqual(row.Tier, s.LocalTiers) ||
				!reflect.DeepEqual(row.TestNames, s.Tests) || !reflect.DeepEqual(row.ProductionPath, functionDefaultProductionPaths) || row.Scope != s.Scope {
				t.Fatal("actual local function scope lost in complete audit", row)
			}
		} else if row.Status == "PASS" || row.Status == "BLOCKED_EXTERNAL" || row.PositiveAndNegative {
			t.Fatal("unselected requirement inherited function-boundary credit", row)
		}
	}
	original, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	faults := []string{
		"wrong_producer", "borrowed_id", "fake_ios", "wrong_tier", "truncated_command", "missing_child_binding", "wrong_scope", "stale_source", "failed_result", "failed_exit",
		"missing_artifact", "duplicate_artifact", "bad_artifact_digest", "missing_tier", "process_source", "dirty_before", "dirty_after", "missing_exit", "missing_timeout", "timeout",
		"missing_discovery", "missing_required_test", "bad_time", "forged_source_hash", "missing_source_path", "process_requirement", "process_command", "forged_benchmark",
		"raw_missing_graph", "raw_missing_corruption", "raw_orphan_pass", "raw_skipped_child", "raw_failed_child", "raw_foreign_package", "raw_unknown_child", "raw_duplicate_run", "raw_parent_before_child", "raw_nested_parent_before_child",
		"raw_invalid_utf8", "raw_duplicate_json_key", "raw_trailing_json", "raw_missing_time", "raw_invalid_time", "raw_backwards_time", "raw_missing_elapsed", "raw_negative_elapsed", "raw_oversized_elapsed", "raw_outside_process_interval", "stderr_invalid_utf8",
	}
	for _, actual := range runner.records {
		id := actual.Requirements[0]
		s := functionDefaultScenarios[id]
		t.Run(id+"/"+actual.Tier, func(t *testing.T) {
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
							changed.TestID = "tools/language-acceptance:runFunctions"
						case "borrowed_id":
							changed.Requirements = append(changed.Requirements, "FN05")
						case "fake_ios":
							changed.Tier = "ios-runtime"
						case "wrong_tier":
							changed.Tier = "native-roundtrip"
						case "truncated_command":
							changed.Command = productionTestCommand(s.Tests[:1])
						case "missing_child_binding":
							delete(changed.Assertions, "required_go_events")
						case "wrong_scope":
							changed.Assertions["scope"] = "All linked function/native requirements complete"
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
							functionDefaultRewriteArtifacts(t, changed, func(data map[string][]byte, receipt *editorProcessReceipt) {
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
									receipt.Requirement = "FN05"
								case "process_command":
									receipt.Command = productionTestCommand(s.Tests[:1])
								case "forged_benchmark":
									receipt.BenchmarkSHA256, receipt.BenchmarkBytes = strings.Repeat("a", 64), 1
								case "stderr_invalid_utf8":
									data["function-stderr.log"] = []byte{0xff}
								default:
									target := s.Tests[1] + "/false/supplied_absent"
									if id == "FN04" {
										target = s.Tests[2] + "/nested_true/nested_payload_loses_caller"
									}
									kind := strings.TrimPrefix(fault, "raw_")
									if fault == "raw_missing_graph" {
										kind, target = "missing_event", s.Tests[0]
									}
									if fault == "raw_missing_corruption" {
										kind = "missing_event"
									}
									data["function-go.jsonl"] = functionDefaultCorruptRaw(t, kind, target, data["function-go.jsonl"])
								}
							})
						}
						break
					}
					damaged, err := ValidateEvidenceManifest(&broken, selected, "local")
					if err == nil || damaged == nil || damaged.ClosureComplete || damaged.PassedCases != 0 {
						t.Fatal("damaged actual function evidence accepted", fault, damaged, err)
					}
				})
			}
		})
	}
	for _, id := range []string{"T01", "FN04"} {
		var actual EvidenceRecord
		for _, record := range runner.records {
			if record.Requirements[0] == id {
				actual = record
				break
			}
		}
		for index, path := range functionDefaultProductionPaths {
			t.Run(id+"/source_digest/"+strings.ReplaceAll(path, "/", "_"), func(t *testing.T) {
				var changed EvidenceRecord
				encoded, _ := json.Marshal(actual)
				if err := json.Unmarshal(encoded, &changed); err != nil {
					t.Fatal(err)
				}
				functionDefaultRewriteArtifacts(t, &changed, func(_ map[string][]byte, receipt *editorProcessReceipt) {
					receipt.SourceFiles[index].SHA256 = strings.Repeat("0", 64)
				})
				if validateFunctionDefaultRecord(id, changed) == nil {
					t.Fatal("hash-reconciled immutable source corruption accepted", path)
				}
			})
		}
	}
	for _, id := range []string{"FN01", "FN02", "FN03", "FN05", "T05", "RP25", "RP26", "BRG11", "BRG15"} {
		t.Run("no_inheritance/"+id, func(t *testing.T) {
			related := &ContractsSet{Acceptance: &ContractFile{}, Repair: &RepairContractFile{}, Gates: &GatesContractFile{}}
			for _, row := range all.Acceptance.Cases {
				if row.ID == id {
					related.Acceptance.Cases = append(related.Acceptance.Cases, row)
				}
			}
			for _, row := range all.Repair.Cases {
				if row.ID == id {
					related.Repair.Cases = append(related.Repair.Cases, row)
				}
			}
			for _, row := range all.Gates.Gates {
				if row.ID == id {
					related.Gates.Gates = append(related.Gates.Gates, row)
				}
			}
			related.Acceptance.CaseCount = len(related.Acceptance.Cases)
			if related.Acceptance.CaseCount+len(related.Repair.Cases)+len(related.Gates.Gates) != 1 {
				t.Fatal("missing related immutable requirement", id)
			}
			var rebound EvidenceManifest
			if err := json.Unmarshal(original, &rebound); err != nil {
				t.Fatal(err)
			}
			for index := range rebound.Records {
				rebound.Records[index].Requirements = []string{id}
			}
			report, err := ValidateEvidenceManifest(&rebound, related, "local")
			if err == nil || report == nil || report.PassedCases != 0 || report.ClosureComplete {
				t.Fatal("related unexecuted requirement inherited function evidence", id, report, err)
			}
		})
	}
}
