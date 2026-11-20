package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func safeImportEditSelectedContracts(t *testing.T, all *ContractsSet) *ContractsSet {
	t.Helper()
	selected := &ContractsSet{Acceptance: &ContractFile{}, Repair: &RepairContractFile{}, Gates: &GatesContractFile{}}
	for _, c := range all.Gates.Gates {
		if c.ID == "BRG19" {
			selected.Gates.Gates = append(selected.Gates.Gates, c)
		}
	}
	if len(selected.Gates.Gates) != 1 {
		t.Fatal("missing immutable BRG19 contract")
	}
	selected.Gates.GateCount = 1
	return selected
}

func TestSafeImportEditProducerInventory(t *testing.T) {
	all, err := LoadAllContracts("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	selected := safeImportEditSelectedContracts(t, all)
	row := selected.Gates.Gates[0]
	if row.Title != "Safe import editing and structural migration" || !reflect.DeepEqual(parseRequiredTiers(strings.Join(row.MinimumEvidenceTiers, "+")), safeImportEditTiers) ||
		row.Scenario != "Migrate legacy metadata/references/includes, fail an unsupported conversion, then perform one authorized parameter edit on imported data." ||
		row.ExpectedObservation != "Originals survive failed conversion; intended delta only; no regex directive deletion; public output is v2, not legacy retry." {
		t.Fatal("BRG19 producer drifted from immutable contract", row)
	}
	if len(safeImportEditTests) != 2 || len(safeImportEditRequiredEvents()) != 53 || len(safeImportEditNativeControls) != 15 || len(safeImportEditPolicyControls) != 8 || len(safeImportEditSyntaxControls) != 3 {
		t.Fatal("complete migration/import/edit scenario inventory changed")
	}
	seen := map[string]bool{}
	for _, name := range safeImportEditRequiredEvents() {
		if seen[name] {
			t.Fatal("duplicate required event", name)
		}
		seen[name] = true
	}
	if !reflect.DeepEqual(deriveGateTestIDs(row.ID, row.ProposedTestPrefix), safeImportEditTestIDs()) || !reflect.DeepEqual(deriveGateFixtures(row.ID), safeImportEditProductionPaths) {
		t.Fatal("BRG19 mapping omitted actual producers or sources")
	}
	for _, path := range safeImportEditProductionPaths {
		info, err := os.Stat(filepath.Join(resolveRepoRoot(), filepath.FromSlash(path)))
		if err != nil || !info.Mode().IsRegular() {
			t.Fatal("missing safe-import-edit production source", path, err)
		}
	}
	runner := NewRunner()
	for _, id := range []string{"R07", "CL03", "RP33", "RP34", "RP37", "BRG20"} {
		other := row
		other.ID = id
		if runner.runSafeImportEditRequirement(other) {
			t.Fatal("related requirement inherited BRG19 producer", id)
		}
	}
	if len(runner.records) != 0 || len(runner.results) != 0 || len(runner.observations) != 0 {
		t.Fatal("unexecuted related requirement received credit")
	}
	for _, field := range []string{"tiers", "title", "scenario", "expected"} {
		changed := row
		switch field {
		case "tiers":
			changed.MinimumEvidenceTiers = []string{"integration"}
		case "title":
			changed.Title = "different title"
		case "scenario":
			changed.Scenario = "different scenario"
		case "expected":
			changed.ExpectedObservation = "different observation"
		}
		if !runner.runSafeImportEditRequirement(changed) || runner.results["BRG19"].Status != "FAILED" || len(runner.records) != 0 {
			t.Fatal("different or incomplete immutable contract accepted", field)
		}
	}
}

// Synthetic streams are parser controls only; actual evidence below always
// comes from a fresh source-bound subprocess executing the production compiler.
func safeImportEditControlEvents() []byte {
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
	names := safeImportEditRequiredEvents()
	selected := map[string]bool{}
	for _, name := range names {
		selected[name] = true
	}
	parent := func(name string) string {
		for {
			slash := strings.LastIndexByte(name, '/')
			if slash < 0 {
				return ""
			}
			name = name[:slash]
			if selected[name] {
				return name
			}
		}
	}
	var execute func(string)
	execute = func(name string) {
		emit("run", name)
		for _, child := range names {
			if parent(child) == name {
				execute(child)
			}
		}
		emit("pass", name)
	}
	emit("start", "")
	for _, name := range safeImportEditTests {
		execute(name)
	}
	emit("pass", "")
	return append(bytes.Join(lines, []byte("\n")), '\n')
}

func safeImportEditMoveNestedParent(t *testing.T, raw []byte) []byte {
	t.Helper()
	parent := safeImportEditTests[1] + "/xml"
	child := parent + "/candidate_metadata_loss"
	var events []map[string]any
	var pass map[string]any
	var childTime any
	for _, line := range bytes.Split(bytes.TrimSpace(raw), []byte("\n")) {
		var event map[string]any
		if err := json.Unmarshal(line, &event); err != nil {
			t.Fatal(err)
		}
		events = append(events, event)
		if event["Action"] == "pass" && event["Test"] == parent {
			pass = event
		}
		if event["Action"] == "run" && event["Test"] == child {
			childTime = event["Time"]
		}
	}
	if pass == nil || childTime == nil {
		t.Fatal("nested control target was not observed")
	}
	pass["Time"] = childTime
	var lines [][]byte
	for _, event := range events {
		if event["Action"] == "pass" && event["Test"] == parent {
			continue
		}
		if event["Action"] == "run" && event["Test"] == child {
			encoded, _ := json.Marshal(pass)
			lines = append(lines, encoded)
		}
		encoded, _ := json.Marshal(event)
		lines = append(lines, encoded)
	}
	return append(bytes.Join(lines, []byte("\n")), '\n')
}

func safeImportEditCorruptRaw(t *testing.T, kind, target string, raw []byte) []byte {
	t.Helper()
	if kind == "nested_parent_before_child" {
		return safeImportEditMoveNestedParent(t, raw)
	}
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

func TestSafeImportEditEventParserControls(t *testing.T) {
	raw := safeImportEditControlEvents()
	if err := parseSafeImportEditOutput(raw); err != nil {
		t.Fatal(err)
	}
	for _, name := range safeImportEditRequiredEvents() {
		t.Run("omitted/"+strings.ReplaceAll(name, "/", "_"), func(t *testing.T) {
			if parseSafeImportEditOutput(safeImportEditCorruptRaw(t, "missing_event", name, raw)) == nil {
				t.Fatal("omitted required producer or child accepted", name)
			}
		})
	}
	// The inherited generic parser covers top-level parents. This narrower
	// control is otherwise valid and proves the BRG19 nested-parent guard.
	nested := safeImportEditMoveNestedParent(t, raw)
	if err := parseStrictNamedProductionOutput(safeImportEditRequiredEvents(), productionTestTimeout(safeImportEditTests), nested); err != nil {
		t.Fatal("nested-only control unexpectedly damaged another generic invariant", err)
	}
	if parseSafeImportEditOutput(nested) == nil {
		t.Fatal("nested case passed before its required children")
	}
	target := safeImportEditTests[0] + "/metadata"
	for _, fault := range []string{"orphan_pass", "skipped_child", "failed_child", "foreign_package", "unknown_child", "duplicate_run", "parent_before_child", "nested_parent_before_child", "missing_time", "invalid_time", "backwards_time", "missing_elapsed", "negative_elapsed", "oversized_elapsed", "invalid_utf8", "duplicate_json_key", "trailing_json"} {
		t.Run(fault, func(t *testing.T) {
			if parseSafeImportEditOutput(safeImportEditCorruptRaw(t, fault, target, raw)) == nil {
				t.Fatal("damaged synthetic parser control accepted", fault)
			}
		})
	}
}

// Synthetic Git-size metadata exercises only the aggregate-bound parser. The
// production path always obtains metadata from the exact immutable Git commit.
func TestSafeImportEditSourceBudgetControls(t *testing.T) {
	metadata := func(sizes []int64) []byte {
		var out strings.Builder
		for _, size := range sizes {
			fmt.Fprintf(&out, "%s blob %d\n", strings.Repeat("a", 40), size)
		}
		return []byte(out.String())
	}
	healthy := make([]int64, 68)
	for index := range healthy {
		healthy[index] = 1024
	}
	healthy[0], healthy[1], healthy[2] = 4<<20, 4<<20, 4<<20
	if err := safeImportEditSourceBudget(metadata(healthy), 68); err != nil {
		t.Fatal(err)
	}
	for _, fault := range []string{"aggregate_across_batch", "headers_cross_limit", "oversize_object", "negative_size", "overflow_size", "wrong_type", "wrong_hash", "missing_object", "extra_object", "truncated"} {
		t.Run(fault, func(t *testing.T) {
			sizes := append([]int64(nil), healthy...)
			switch fault {
			case "aggregate_across_batch":
				sizes[64] = 4 << 20
			case "headers_cross_limit":
				for index := range sizes {
					sizes[index] = 0
				}
				sizes[0], sizes[1], sizes[2], sizes[64] = 4<<20, 4<<20, 4<<20, 4<<20
			case "oversize_object":
				sizes[67] = (4 << 20) + 1
			case "negative_size":
				sizes[67] = -1
			case "missing_object":
				sizes = sizes[:67]
			case "extra_object":
				sizes = append(sizes, 1)
			}
			raw := metadata(sizes)
			switch fault {
			case "overflow_size":
				raw = bytes.Replace(raw, []byte("blob 4194304"), []byte("blob 9223372036854775808"), 1)
			case "wrong_type":
				raw = bytes.Replace(raw, []byte(" blob "), []byte(" tree "), 1)
			case "wrong_hash":
				raw[0] = 'z'
			case "truncated":
				raw = raw[:len(raw)-1]
			}
			if safeImportEditSourceBudget(raw, 68) == nil {
				t.Fatal("invalid aggregate/source metadata accepted", fault)
			}
		})
	}
}

func safeImportEditRewriteArtifacts(t *testing.T, record *EvidenceRecord, mutate func(map[string][]byte, *editorProcessReceipt)) {
	t.Helper()
	data, err := stateReadArtifacts(record.Artifacts, safeImportEditArtifacts)
	if err != nil {
		t.Fatal(err)
	}
	var receipt editorProcessReceipt
	if err := decodeEditorJSON(data["brg19-process.json"], &receipt); err != nil {
		t.Fatal(err)
	}
	mutate(data, &receipt)
	receipt.RawSHA256 = editorSHA(data["brg19-go.jsonl"])
	receipt.RawBytes = len(data["brg19-go.jsonl"])
	receipt.StderrSHA256 = editorSHA(data["brg19-stderr.log"])
	receipt.StderrBytes = len(data["brg19-stderr.log"])
	encoded, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	data["brg19-process.json"] = append(encoded, '\n')
	root := t.TempDir()
	record.Artifacts = nil
	for _, name := range safeImportEditArtifacts {
		path := filepath.Join(root, name)
		if err := os.WriteFile(path, data[name], 0600); err != nil {
			t.Fatal(err)
		}
		record.Artifacts = append(record.Artifacts, EvidenceArtifact{Path: path, SHA256: editorSHA(data[name])})
	}
}

func TestSafeImportEditActualProductionEvidence(t *testing.T) {
	all, err := LoadAllContracts("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	selected := safeImportEditSelectedContracts(t, all)
	runner := NewRunner()
	runner.artifactDir = os.Getenv("CHERRI_BRG19_EVIDENCE_OUTPUT")
	if runner.artifactDir == "" {
		runner.artifactDir = t.TempDir()
	}
	runner.runRecoveryGates(selected.Gates)
	if len(runner.records) != 2 || len(runner.results) != 1 || len(runner.observations) != 0 {
		t.Fatalf("BRG19 actual accounting: records=%d results=%+v observations=%v", len(runner.records), runner.results, runner.observations)
	}
	result := runner.results["BRG19"]
	if !result.Passed || result.Status != "PASSED" || result.Group != "recovery_gate" || !reflect.DeepEqual(result.ExecutedLevels, safeImportEditTiers) {
		t.Fatalf("actual full BRG19 production did not pass: %+v", result)
	}
	for _, record := range runner.records {
		if err := validateSafeImportEditRecord("BRG19", record); err != nil {
			t.Fatal(err)
		}
	}
	manifest := &EvidenceManifest{SchemaVersion: "1", ImplementationSHA: runner.implSHA, Records: runner.records}
	report, err := ValidateEvidenceManifest(manifest, selected, "local")
	if err != nil || report == nil || report.PassedCases != 1 || report.FailedCases != 0 || report.SkippedCases != 0 || report.ClosureComplete {
		t.Fatal("complete BRG19-only source-bound production rejected", report, err)
	}
	// Final release validation independently requires the global consumer tuple
	// and real CI runs, even when selecting one requirement. Do not fabricate
	// those unrelated proofs to make this local producer appear release-ready.
	final, finalErr := ValidateEvidenceManifest(manifest, selected, "final")
	if finalErr == nil || final == nil || final.PassedCases != 1 || final.ClosureComplete ||
		!strings.Contains(finalErr.Error(), "missing required successful CI workflow run") {
		t.Fatal("local BRG19 proof bypassed the global final release gate", final, finalErr)
	}
	original, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	faults := []string{
		"wrong_producer", "borrowed_id", "fake_ios", "wrong_tier", "truncated_command", "missing_child_binding", "wrong_scope", "stale_source", "failed_result", "failed_exit",
		"missing_artifact", "duplicate_artifact", "bad_artifact_digest", "missing_tier", "process_source", "dirty_before", "dirty_after", "missing_exit", "missing_timeout", "timeout",
		"missing_discovery", "missing_required_test", "bad_time", "forged_source_hash", "missing_source_path", "process_requirement", "process_command", "forged_benchmark",
		"raw_missing_integration", "raw_missing_native", "raw_orphan_pass", "raw_skipped_child", "raw_failed_child", "raw_foreign_package", "raw_unknown_child", "raw_duplicate_run", "raw_parent_before_child", "raw_nested_parent_before_child",
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
							changed.Requirements = append(changed.Requirements, "CL03")
						case "fake_ios":
							changed.Tier = "ios-runtime"
						case "wrong_tier":
							changed.Tier = "native-structure"
						case "truncated_command":
							changed.Command = productionTestCommand(safeImportEditTests[:1])
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
							safeImportEditRewriteArtifacts(t, changed, func(data map[string][]byte, receipt *editorProcessReceipt) {
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
									receipt.Requirement = "CL03"
								case "process_command":
									receipt.Command = productionTestCommand(safeImportEditTests[:1])
								case "forged_benchmark":
									receipt.BenchmarkSHA256 = strings.Repeat("a", 64)
									receipt.BenchmarkBytes = 1
								case "stderr_invalid_utf8":
									data["brg19-stderr.log"] = []byte{0xff}
								default:
									target := safeImportEditTests[0] + "/metadata"
									kind := strings.TrimPrefix(fault, "raw_")
									if fault == "raw_missing_integration" {
										kind = "missing_event"
									}
									if fault == "raw_missing_native" {
										kind = "missing_event"
										target = safeImportEditTests[1] + "/xml"
									}
									raw := safeImportEditCorruptRaw(t, kind, target, data["brg19-go.jsonl"])
									if bytes.Equal(raw, data["brg19-go.jsonl"]) {
										t.Fatal("actual raw corruption did not occur")
									}
									data["brg19-go.jsonl"] = raw
								}
							})
						}
						break
					}
					report, err := ValidateEvidenceManifest(&broken, selected, "local")
					if err == nil || report == nil || report.ClosureComplete || report.PassedCases != 0 {
						t.Fatal("damaged actual BRG19 evidence accepted", fault, report, err)
					}
				})
			}
		})
	}
	for index, path := range safeImportEditProductionPaths {
		t.Run("source_digest/"+strings.ReplaceAll(path, "/", "_"), func(t *testing.T) {
			var changed EvidenceRecord
			encoded, _ := json.Marshal(runner.records[0])
			if err := json.Unmarshal(encoded, &changed); err != nil {
				t.Fatal(err)
			}
			safeImportEditRewriteArtifacts(t, &changed, func(_ map[string][]byte, receipt *editorProcessReceipt) {
				receipt.SourceFiles[index].SHA256 = strings.Repeat("0", 64)
			})
			if validateSafeImportEditRecord("BRG19", changed) == nil {
				t.Fatal("hash-reconciled immutable source corruption accepted", path)
			}
		})
	}
	for _, id := range []string{"R07", "CL03", "RP33", "RP34", "RP37", "BRG20"} {
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
			if len(related.Acceptance.Cases)+len(related.Repair.Cases)+len(related.Gates.Gates) != 1 {
				t.Fatal("missing original related contract", id)
			}
			related.Acceptance.CaseCount = len(related.Acceptance.Cases)
			related.Repair.CaseCount = len(related.Repair.Cases)
			related.Gates.GateCount = len(related.Gates.Gates)
			var rebound EvidenceManifest
			if err := json.Unmarshal(original, &rebound); err != nil {
				t.Fatal(err)
			}
			for index := range rebound.Records {
				rebound.Records[index].Requirements = []string{id}
			}
			report, err := ValidateEvidenceManifest(&rebound, related, "local")
			if err == nil || report == nil || report.PassedCases != 0 || report.ClosureComplete {
				t.Fatal("linked requirement inherited actual BRG19 proof", id, report, err)
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
		t.Fatal("BRG19 altered 179-row formal scope", err)
	}
	for _, row := range audit.Rows {
		if row.ID == "BRG19" {
			if row.Status != "PASS" || !row.PositiveAndNegative || !reflect.DeepEqual(row.TestNames, safeImportEditTests) ||
				!reflect.DeepEqual(row.ProductionPath, safeImportEditProductionPaths) || !reflect.DeepEqual(row.Tier, safeImportEditTiers) || row.Scope != safeImportEditScope {
				t.Fatalf("BRG19 audit lost exact actual scope: %+v", row)
			}
		} else if row.Status == "PASS" {
			t.Fatal("unexecuted requirement inherited BRG19 proof", row.ID)
		}
	}
	receipt, err := json.Marshal(map[string]any{"source_sha": runner.implSHA, "scope": safeImportEditScope, "report": report, "records": runner.records})
	if err != nil {
		t.Fatal(err)
	}
	t.Log("P15_BRG19_PRODUCTION_RECEIPT:" + string(receipt))
}
