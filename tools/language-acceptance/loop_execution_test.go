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

func loopSelectedContracts(t *testing.T, all *ContractsSet) *ContractsSet {
	t.Helper()
	selected := &ContractsSet{Acceptance: &ContractFile{}, Repair: &RepairContractFile{}, Gates: &GatesContractFile{}}
	for _, c := range all.Acceptance.Cases {
		if c.ID == "F02" {
			selected.Acceptance.Cases = append(selected.Acceptance.Cases, c)
		}
	}
	for _, c := range all.Repair.Cases {
		if c.ID == "RP22" {
			selected.Repair.Cases = append(selected.Repair.Cases, c)
		}
	}
	for _, c := range all.Gates.Gates {
		if c.ID == "BRG12" {
			selected.Gates.Gates = append(selected.Gates.Gates, c)
		}
	}
	if len(selected.Acceptance.Cases) != 1 || len(selected.Repair.Cases) != 1 || len(selected.Gates.Gates) != 1 {
		t.Fatal("missing immutable loop scenarios")
	}
	selected.Acceptance.CaseCount = 1
	return selected
}

func TestLoopProducerInventory(t *testing.T) {
	all, err := LoadAllContracts("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	selected := loopSelectedContracts(t, all)
	f02 := selected.Acceptance.Cases[0]
	rp22 := selected.Repair.Cases[0]
	brg12 := selected.Gates.Gates[0]
	facts := []struct {
		id, group, title string
		tiers, ids       []string
	}{
		{f02.ID, f02.Group, f02.Requirement, parseRequiredTiers(f02.MinimumTestLevel), deriveAcceptanceTestIDs(f02.ID, f02.Group)},
		{rp22.ID, "repair", rp22.Title, parseRequiredTiers(strings.Join(rp22.RequiredTestLevels, "+")), deriveRepairTestIDs(rp22.ID, rp22.Title)},
		{brg12.ID, "recovery_gate", brg12.Title, parseRequiredTiers(strings.Join(brg12.MinimumEvidenceTiers, "+")), deriveGateTestIDs(brg12.ID, brg12.ProposedTestPrefix)},
	}
	if len(loopAcceptanceScenarios) != 3 {
		t.Fatal("loop producer scope expanded")
	}
	for _, fact := range facts {
		scenario, found := loopAcceptanceScenarios[fact.id]
		if !found || scenario.Group != fact.group || scenario.Title != fact.title || !reflect.DeepEqual(fact.tiers, loopRequiredTiers) || !reflect.DeepEqual(fact.ids, scenario.testIDs()) {
			t.Fatal("loop producer drifted from immutable row", fact.id)
		}
		if _, smoke := incompleteLocalAssertions[fact.id]; smoke {
			t.Fatal("complete loop producer remains historical smoke")
		}
		ids := append([]string{}, fact.ids...)
		assertions, ui := map[string]string{}, map[string]string{}
		applyRuntimeAndUIMappings(fact.id, fact.tiers, &ids, assertions, ui)
		if !reflect.DeepEqual(ids, fact.ids) || len(assertions) != 0 || len(ui) != 0 {
			t.Fatal("simple historical trace still claims the full loop scenario", fact.id)
		}
		eventNames := loopRequiredEvents(fact.id)
		seen := map[string]bool{}
		for _, name := range eventNames {
			if seen[name] {
				t.Fatal("duplicate required loop event")
			}
			seen[name] = true
		}
		if len(eventNames) < 26 {
			t.Fatal("full causal loop scenario missing", fact.id)
		}
	}
	for _, path := range loopProductionPaths {
		info, err := os.Stat(filepath.Join(resolveRepoRoot(), filepath.FromSlash(path)))
		if err != nil || !info.Mode().IsRegular() {
			t.Fatal("missing actual loop source", path, err)
		}
	}
	runner := NewRunner()
	for _, id := range []string{"F01", "F04", "F05", "RP18", "RP23", "BRG13", "BRG14"} {
		if runner.runLoopRequirement(id, "control", "unrelated", loopRequiredTiers) {
			t.Fatal("linked scenario inherited loop execution", id)
		}
	}
	if len(runner.records) != 0 || len(runner.results) != 0 || len(runner.observations) != 0 {
		t.Fatal("unexecuted requirements received loop credit")
	}
	if !runner.runLoopRequirement("RP22", "repair", loopAcceptanceScenarios["RP22"].Title, []string{"unit"}) ||
		runner.results["RP22"].Status != "FAILED" || len(runner.records) != 0 {
		t.Fatal("incomplete requested tiers accepted")
	}
}

// Synthetic event streams are parser unit controls only. They never enter a
// Runner, produce evidence records, or stand in for the actual producer below.
func loopParserControlEvents(id string) []byte {
	scenario := loopAcceptanceScenarios[id]
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
	for _, parent := range scenario.tests() {
		emit("run", parent)
		for _, name := range loopRequiredEvents(id) {
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

func loopCorruptRaw(t *testing.T, id, kind string, raw []byte) []byte {
	t.Helper()
	if kind == "invalid_utf8" {
		return bytes.Replace(raw, []byte(`"Output":"`), []byte{'"', 'O', 'u', 't', 'p', 'u', 't', '"', ':', '"', 0xff}, 1)
	}
	var events []map[string]any
	for _, line := range bytes.Split(bytes.TrimSpace(raw), []byte("\n")) {
		var event map[string]any
		if err := json.Unmarshal(line, &event); err != nil {
			t.Fatal(err)
		}
		events = append(events, event)
	}
	child := loopAcceptanceScenarios[id].UnitTest + "/scoped_types"
	var out []map[string]any
	changed := false
	for _, event := range events {
		test, _ := event["Test"].(string)
		action, _ := event["Action"].(string)
		switch kind {
		case "outside_process_interval":
			event["Time"] = "2001-01-01T00:00:00Z"
			changed = true
		case "missing_time":
			if !changed {
				delete(event, "Time")
				changed = true
			}
		case "invalid_time":
			if !changed {
				event["Time"] = "not-a-time"
				changed = true
			}
		case "backwards_time":
			if !changed && len(out) > 0 {
				event["Time"] = "2000-01-01T00:00:00Z"
				changed = true
			}
		case "missing_elapsed":
			if !changed && action == "pass" {
				delete(event, "Elapsed")
				changed = true
			}
		case "negative_elapsed":
			if !changed && action == "pass" {
				event["Elapsed"] = -1
				changed = true
			}
		case "oversized_elapsed":
			if !changed && action == "pass" {
				event["Elapsed"] = 99999
				changed = true
			}
		case "missing_child":
			if test == child {
				changed = true
				continue
			}
		case "orphan_pass":
			if test == child && action == "run" {
				changed = true
				continue
			}
		case "skipped_child":
			if test == child && action == "pass" {
				event["Action"] = "skip"
				changed = true
			}
		case "foreign_package":
			event["Package"] = "foreign/loop/package"
			changed = true
		case "unknown_child":
			if !changed && test == child && action == "run" {
				event["Test"] = child + "_unselected"
				changed = true
			}
		case "duplicate_run":
			if !changed && test == child && action == "run" {
				out = append(out, event)
				changed = true
			}
		}
		out = append(out, event)
	}
	if kind == "after_completion" {
		out = append(out, map[string]any{"Time": "2026-11-20T00:00:00Z", "Action": "output", "Package": "github.com/electrikmilk/cherri", "Output": "late\n"})
		changed = true
	}
	if !changed {
		t.Fatal("raw control did not target an event", kind)
	}
	var lines [][]byte
	for _, event := range out {
		encoded, err := json.Marshal(event)
		if err != nil {
			t.Fatal(err)
		}
		lines = append(lines, encoded)
	}
	return append(bytes.Join(lines, []byte("\n")), '\n')
}

func TestLoopEventParserControls(t *testing.T) {
	for _, id := range []string{"F02", "RP22", "BRG12"} {
		raw := loopParserControlEvents(id)
		if err := parseLoopProductionOutput(id, raw); err != nil {
			t.Fatal(err)
		}
		for _, fault := range []string{"missing_child", "orphan_pass", "skipped_child", "foreign_package", "unknown_child", "duplicate_run", "after_completion", "missing_time", "invalid_time", "backwards_time", "missing_elapsed", "negative_elapsed", "oversized_elapsed"} {
			t.Run(id+"/"+fault, func(t *testing.T) {
				if parseLoopProductionOutput(id, loopCorruptRaw(t, id, fault, raw)) == nil {
					t.Fatal("incomplete synthetic parser control accepted")
				}
			})
		}
		t.Run(id+"/invalid_utf8", func(t *testing.T) {
			if parseLoopProductionOutput(id, append(raw, 0xff)) == nil {
				t.Fatal("invalid raw UTF-8 accepted")
			}
		})
		t.Run(id+"/duplicate_json_key", func(t *testing.T) {
			broken := bytes.Replace(raw, []byte("{"), []byte(`{"Action":"start",`), 1)
			if parseLoopProductionOutput(id, broken) == nil {
				t.Fatal("duplicate event key accepted")
			}
		})
	}
}

func loopRewriteArtifacts(t *testing.T, record *EvidenceRecord, mutate func(map[string][]byte, *editorProcessReceipt)) {
	t.Helper()
	data, err := stateReadArtifacts(record.Artifacts, loopArtifactNames)
	if err != nil {
		t.Fatal(err)
	}
	var receipt editorProcessReceipt
	if err := decodeEditorJSON(data["loop-process.json"], &receipt); err != nil {
		t.Fatal(err)
	}
	mutate(data, &receipt)
	receipt.RawSHA256 = editorSHA(data["loop-go.jsonl"])
	receipt.RawBytes = len(data["loop-go.jsonl"])
	receipt.StderrSHA256 = editorSHA(data["loop-stderr.log"])
	receipt.StderrBytes = len(data["loop-stderr.log"])
	encoded, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	data["loop-process.json"] = append(encoded, '\n')
	root := t.TempDir()
	record.Artifacts = nil
	for _, name := range loopArtifactNames {
		path := filepath.Join(root, name)
		if err := os.WriteFile(path, data[name], 0600); err != nil {
			t.Fatal(err)
		}
		record.Artifacts = append(record.Artifacts, EvidenceArtifact{Path: path, SHA256: editorSHA(data[name])})
	}
}

func TestLoopActualProductionEvidence(t *testing.T) {
	all, err := LoadAllContracts("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	selected := loopSelectedContracts(t, all)
	runner := NewRunner()
	runner.artifactDir = os.Getenv("CHERRI_LOOP_EVIDENCE_OUTPUT")
	if runner.artifactDir == "" {
		runner.artifactDir = t.TempDir()
	}
	f02 := selected.Acceptance.Cases[0]
	if !runner.runLoopRequirement(f02.ID, f02.Group, f02.Requirement, strings.Split(f02.MinimumTestLevel, "+")) {
		t.Fatal("F02 producer absent")
	}
	runner.runRepairCases(selected.Repair)
	runner.runRecoveryGates(selected.Gates)
	if len(runner.records) != 6 || len(runner.results) != 3 || len(runner.observations) != 0 {
		t.Fatalf("loop execution accounting: records=%d results=%v observations=%v", len(runner.records), runner.results, runner.observations)
	}
	for id, scenario := range loopAcceptanceScenarios {
		result := runner.results[id]
		if result.Passed || result.Status != "PENDING_EXTERNAL" || result.Group != scenario.Group || result.Requirement != scenario.Title || !reflect.DeepEqual(result.ExecutedLevels, loopLocalTiers) {
			t.Fatalf("local loop execution lost scope or invented runtime: %+v", result)
		}
	}
	for _, record := range runner.records {
		if len(record.Requirements) != 1 {
			t.Fatal("loop evidence inherited requirements")
		}
		if err := validateLoopRecord(record.Requirements[0], record); err != nil {
			t.Fatal(err)
		}
	}
	manifest := &EvidenceManifest{SchemaVersion: "1", ImplementationSHA: runner.implSHA, Records: runner.records}
	report, err := ValidateEvidenceManifest(manifest, selected, "local")
	if err != nil || report.PassedCases != 0 || report.FailedCases != 0 || report.SkippedCases != 3 || report.ClosureComplete {
		t.Fatal("loop local proof falsely closes runtime", report, err)
	}
	if final, err := ValidateEvidenceManifest(manifest, selected, "final"); err == nil || final != nil && final.ClosureComplete {
		t.Fatal("missing actual Apple loop execution accepted")
	}
	original, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	faults := []string{
		"wrong_producer", "borrowed_id", "fake_ios", "truncated_command", "missing_child_binding", "wrong_scope", "stale_source", "failed_result", "failed_exit",
		"missing_artifact", "duplicate_artifact", "missing_tier", "rebound_other_scenario", "process_source", "dirty_before", "dirty_after", "missing_exit", "missing_timeout", "timeout",
		"missing_discovery", "bad_time", "forged_source_hash", "missing_source_path",
		"raw_missing_child", "raw_orphan_pass", "raw_skipped_child", "raw_foreign_package", "raw_unknown_child", "raw_duplicate_run", "raw_after_completion", "raw_invalid_utf8", "raw_missing_time", "raw_invalid_time", "raw_backwards_time", "raw_missing_elapsed", "raw_negative_elapsed", "raw_oversized_elapsed", "raw_outside_process_interval",
	}
	for _, actual := range runner.records {
		t.Run(actual.Requirements[0]+"/"+actual.Tier, func(t *testing.T) {
			for _, fault := range faults {
				t.Run(fault, func(t *testing.T) {
					var broken EvidenceManifest
					if err := json.Unmarshal(original, &broken); err != nil {
						t.Fatal(err)
					}
					for i := range broken.Records {
						changed := &broken.Records[i]
						if changed.EvidenceID != actual.EvidenceID {
							continue
						}
						id := actual.Requirements[0]
						switch fault {
						case "wrong_producer":
							changed.TestID = "tools/language-acceptance:runControl"
						case "borrowed_id":
							changed.Requirements = append(changed.Requirements, "F04")
						case "fake_ios":
							changed.Tier = "ios-runtime"
						case "truncated_command":
							changed.Command = productionTestCommand(loopAcceptanceScenarios[id].tests()[:1])
						case "missing_child_binding":
							delete(changed.Assertions, "required_go_events")
						case "wrong_scope":
							changed.Assertions["scope"] = "actual Apple execution"
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
						case "missing_tier":
							broken.Records = append(broken.Records[:i], broken.Records[i+1:]...)
						default:
							loopRewriteArtifacts(t, changed, func(data map[string][]byte, receipt *editorProcessReceipt) {
								switch fault {
								case "rebound_other_scenario":
									for _, other := range runner.records {
										if other.Requirements[0] != id {
											foreign, err := stateReadArtifacts(other.Artifacts, loopArtifactNames)
											if err != nil {
												t.Fatal(err)
											}
											data["loop-go.jsonl"] = foreign["loop-go.jsonl"]
											break
										}
									}
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
								case "bad_time":
									receipt.ElapsedSeconds = 0
								case "forged_source_hash":
									receipt.SourceFiles[0].SHA256 = strings.Repeat("0", 64)
								case "missing_source_path":
									receipt.SourceFiles = receipt.SourceFiles[:len(receipt.SourceFiles)-1]
								default:
									raw := loopCorruptRaw(t, id, strings.TrimPrefix(fault, "raw_"), data["loop-go.jsonl"])
									if bytes.Equal(raw, data["loop-go.jsonl"]) {
										t.Fatal("actual raw mutation was not applied")
									}
									data["loop-go.jsonl"] = raw
								}
							})
						}
						break
					}
					invalid, err := ValidateEvidenceManifest(&broken, selected, "local")
					if err == nil || invalid == nil || invalid.ClosureComplete || invalid.PassedCases != 0 {
						t.Fatal("damaged full loop evidence accepted", fault, err)
					}
				})
			}
		})
	}
	// Every immutable source field is independently checked, even after the
	// attacker reconciles the outer receipt and artifact hashes.
	originalRecord := runner.records[0]
	for index, path := range loopProductionPaths {
		t.Run("source_digest/"+strings.ReplaceAll(path, "/", "_"), func(t *testing.T) {
			var changed EvidenceRecord
			encoded, _ := json.Marshal(originalRecord)
			if err := json.Unmarshal(encoded, &changed); err != nil {
				t.Fatal(err)
			}
			loopRewriteArtifacts(t, &changed, func(_ map[string][]byte, receipt *editorProcessReceipt) {
				receipt.SourceFiles[index].SHA256 = strings.Repeat("0", 64)
			})
			if validateLoopRecord("F02", changed) == nil {
				t.Fatal("forged immutable source hash accepted", path)
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
		t.Fatal("loop producer altered formal scope", err)
	}
	for _, row := range audit.Rows {
		if scenario, found := loopAcceptanceScenarios[row.ID]; found {
			if row.Status != "BLOCKED_EXTERNAL" || !row.PositiveAndNegative || row.Blocker == "" ||
				!reflect.DeepEqual(row.TestNames, scenario.tests()) || !reflect.DeepEqual(row.ProductionPath, loopProductionPaths) || !reflect.DeepEqual(row.Tier, loopLocalTiers) {
				t.Fatalf("loop audit lost actual partial scope: %+v", row)
			}
		} else if row.Status == "PASS" {
			t.Fatal("unexecuted row inherited loop proof", row.ID)
		}
	}
	receipt, err := json.Marshal(map[string]any{"source_sha": runner.implSHA, "scope": "Three independently executed local loop scenarios; no Apple execution or full179 closure", "report": report, "records": runner.records})
	if err != nil {
		t.Fatal(err)
	}
	t.Log("P15_LOOP_PRODUCTION_RECEIPT:" + string(receipt))
}
