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

func purePublicSelectedContracts(t *testing.T, all *ContractsSet) *ContractsSet {
	t.Helper()
	selected := &ContractsSet{Acceptance: &ContractFile{}, Repair: &RepairContractFile{}, Gates: &GatesContractFile{}}
	for _, row := range all.Gates.Gates {
		if row.ID == "BRG21" {
			selected.Gates.Gates = append(selected.Gates.Gates, row)
		}
	}
	if len(selected.Gates.Gates) != 1 {
		t.Fatal("missing immutable BRG21 contract")
	}
	selected.Gates.GateCount = 1
	return selected
}
func TestPurePublicProducerInventory(t *testing.T) {
	all, err := LoadAllContracts("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	row := purePublicSelectedContracts(t, all).Gates.Gates[0]
	if row.Title != purePublicTitle || row.Scenario != purePublicScenario || row.ExpectedObservation != purePublicExpected || !reflect.DeepEqual(parseRequiredTiers(strings.Join(row.MinimumEvidenceTiers, "+")), purePublicTiers) {
		t.Fatal("immutable full BRG21 contract changed", row)
	}
	names := purePublicRequiredEvents()
	if len(names) != 33 || len(purePublicProductionPaths) != 56 {
		t.Fatal("unreviewed BRG21 event/source inventory", len(names), len(purePublicProductionPaths))
	}
	seen := map[string]bool{}
	for _, name := range names {
		if name == "" || seen[name] {
			t.Fatal("invalid event inventory", name)
		}
		seen[name] = true
	}
	if !reflect.DeepEqual(deriveGateTestIDs("BRG21", row.ProposedTestPrefix), purePublicTestIDs()) || !reflect.DeepEqual(deriveGateFixtures("BRG21"), purePublicProductionPaths) {
		t.Fatal("missing complete production mapping")
	}
	for _, path := range purePublicProductionPaths {
		if seen[path] {
			t.Fatal("duplicate source path", path)
		}
		seen[path] = true
		info, err := os.Stat(filepath.Join(resolveRepoRoot(), filepath.FromSlash(path)))
		if err != nil || !info.Mode().IsRegular() {
			t.Fatal("missing production source", path, err)
		}
	}
	runner := NewRunner()
	for _, foreign := range []string{"IM04", "IM05", "CL02", "RP29", "BRG18"} {
		other := row
		other.ID = foreign
		if runner.runPurePublicRequirement(other) {
			t.Fatal("related ID inherited producer", foreign)
		}
	}
	if len(runner.records) != 0 || len(runner.results) != 0 || len(runner.observations) != 0 {
		t.Fatal("unexecuted ID minted evidence")
	}
	for _, field := range []string{"title", "scenario", "expected", "tiers"} {
		changed := row
		switch field {
		case "title":
			changed.Title = "different title"
		case "scenario":
			changed.Scenario = "different scenario"
		case "expected":
			changed.ExpectedObservation = "different expected"
		case "tiers":
			changed.MinimumEvidenceTiers = []string{"integration"}
		}
		if !runner.runPurePublicRequirement(changed) || runner.results["BRG21"].Status != "FAILED" || len(runner.records) != 0 {
			t.Fatal("altered immutable scenario accepted", field)
		}
	}
}

// Synthetic streams exercise parser rejection only, never production credit.
func purePublicControlEvents() []byte {
	var lines [][]byte
	emit := func(action, name string) {
		e := map[string]any{"Time": "2026-11-20T00:00:00Z", "Action": action, "Package": "github.com/electrikmilk/cherri"}
		if name != "" {
			e["Test"] = name
		}
		if action == "pass" {
			e["Elapsed"] = 0.001
		}
		b, _ := json.Marshal(e)
		lines = append(lines, b)
	}
	names := purePublicRequiredEvents()
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
	for _, name := range purePublicTests {
		execute(name)
	}
	emit("pass", "")
	return append(bytes.Join(lines, []byte("\n")), '\n')
}
func TestPurePublicEventParserControls(t *testing.T) {
	raw := purePublicControlEvents()
	if err := parsePurePublicOutput(raw); err != nil {
		t.Fatal(err)
	}
	for _, name := range purePublicRequiredEvents() {
		t.Run("omitted/"+strings.ReplaceAll(name, "/", "_"), func(t *testing.T) {
			if parsePurePublicOutput(structuralMigrationCorruptRaw(t, "missing_event", name, raw)) == nil {
				t.Fatal("required real case omitted", name)
			}
		})
	}
	target := purePublicTests[0] + "/service_remote_import"
	for _, fault := range []string{"orphan_pass", "skipped_child", "failed_child", "foreign_package", "unknown_child", "duplicate_run", "parent_before_child", "missing_time", "invalid_time", "backwards_time", "missing_elapsed", "negative_elapsed", "oversized_elapsed", "invalid_utf8", "duplicate_json_key", "trailing_json"} {
		t.Run(fault, func(t *testing.T) {
			if parsePurePublicOutput(structuralMigrationCorruptRaw(t, fault, target, raw)) == nil {
				t.Fatal("damaged parser control accepted", fault)
			}
		})
	}

}
func purePublicRewriteArtifacts(t *testing.T, record *EvidenceRecord, mutate func(map[string][]byte, *editorProcessReceipt)) {
	t.Helper()
	names := purePublicArtifacts
	data, err := stateReadArtifacts(record.Artifacts, names)
	if err != nil {
		t.Fatal(err)
	}
	var receipt editorProcessReceipt
	if err = decodeEditorJSON(data[names[0]], &receipt); err != nil {
		t.Fatal(err)
	}
	mutate(data, &receipt)
	receipt.RawSHA256 = editorSHA(data[names[1]])
	receipt.RawBytes = len(data[names[1]])
	receipt.StderrSHA256 = editorSHA(data[names[2]])
	receipt.StderrBytes = len(data[names[2]])
	encoded, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	data[names[0]] = append(encoded, '\n')
	root := t.TempDir()
	record.Artifacts = nil
	for _, name := range names {
		path := filepath.Join(root, name)
		if err = os.WriteFile(path, data[name], 0600); err != nil {
			t.Fatal(err)
		}
		record.Artifacts = append(record.Artifacts, EvidenceArtifact{Path: path, SHA256: editorSHA(data[name])})
	}
}
func TestPurePublicActualProductionEvidence(t *testing.T) {
	const id = "BRG21"
	all, err := LoadAllContracts("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	selected := purePublicSelectedContracts(t, all)
	scenario := struct {
		Tests, Tiers []string
		Scope        string
	}{purePublicTests, purePublicTiers, purePublicScope}
	runner := NewRunner()
	runner.artifactDir = os.Getenv("CHERRI_NATIVE_PATHS_EVIDENCE_OUTPUT")
	if runner.artifactDir == "" {
		runner.artifactDir = t.TempDir()
	}
	runner.runRecoveryGates(selected.Gates)
	if len(runner.records) != 3 || len(runner.results) != 1 || len(runner.observations) != 0 {
		t.Fatalf("actual %s accounting: records=%d results=%+v observations=%v", id, len(runner.records), runner.results, runner.observations)
	}
	result := runner.results[id]
	if !result.Passed || result.Status != "PASSED" || result.Group != "recovery_gate" || !reflect.DeepEqual(result.ExecutedLevels, scenario.Tiers) {
		t.Fatalf("actual full %s production did not pass: %+v", id, result)
	}
	for _, record := range runner.records {
		if err = validatePurePublicRecord(id, record); err != nil {
			t.Fatal(err)
		}
	}
	manifest := &EvidenceManifest{SchemaVersion: "1", ImplementationSHA: runner.implSHA, Records: runner.records}
	report, err := ValidateEvidenceManifest(manifest, selected, "local")
	if err != nil || report == nil || report.PassedCases != 1 || report.FailedCases != 0 || report.SkippedCases != 0 || report.ClosureComplete {
		t.Fatal("complete authorized-loader/pure-public scenario rejected", report, err)
	}
	final, finalErr := ValidateEvidenceManifest(manifest, selected, "final")
	if finalErr == nil || final == nil || final.PassedCases != 1 || final.ClosureComplete || !strings.Contains(finalErr.Error(), "missing required successful CI workflow run") {
		t.Fatal("local component bypassed global final release gate", final, finalErr)
	}
	original, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	faults := []string{"wrong_producer", "borrowed_id", "fake_ios", "wrong_tier", "truncated_command", "missing_child_binding", "wrong_scope", "stale_source", "failed_result", "failed_exit", "missing_artifact", "duplicate_artifact", "bad_artifact_digest", "missing_tier", "process_source", "dirty_before", "dirty_after", "missing_exit", "missing_timeout", "timeout", "missing_discovery", "missing_required_test", "bad_time", "forged_source_hash", "missing_source_path", "process_requirement", "process_command", "forged_benchmark", "stderr_invalid_utf8", "raw_missing_parent", "raw_orphan_pass", "raw_skipped_child", "raw_failed_child", "raw_foreign_package", "raw_unknown_child", "raw_duplicate_run", "raw_parent_before_child", "raw_invalid_utf8", "raw_duplicate_json_key", "raw_trailing_json", "raw_missing_time", "raw_invalid_time", "raw_backwards_time", "raw_missing_elapsed", "raw_negative_elapsed", "raw_oversized_elapsed", "raw_outside_process_interval"}
	names := purePublicArtifacts
	target := purePublicTests[0] + "/service_remote_import"
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
							changed.Requirements = append(changed.Requirements, "K01")
						case "fake_ios":
							changed.Tier = "ios-runtime"
						case "wrong_tier":
							changed.Tier = "repository"
						case "truncated_command":
							changed.Command = productionTestCommand(scenario.Tests[:1])
						case "missing_child_binding":
							delete(changed.Assertions, "required_go_events")
						case "wrong_scope":
							changed.Assertions["scope"] = "All linked requirements complete"
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
							purePublicRewriteArtifacts(t, changed, func(data map[string][]byte, receipt *editorProcessReceipt) {
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
									receipt.Requirement = "K01"
								case "process_command":
									receipt.Command = productionTestCommand(scenario.Tests[:1])
								case "forged_benchmark":
									receipt.BenchmarkSHA256 = strings.Repeat("a", 64)
									receipt.BenchmarkBytes = 1
								case "stderr_invalid_utf8":
									data[names[2]] = []byte{0xff}
								default:
									kind := strings.TrimPrefix(fault, "raw_")
									changedTarget := target
									if fault == "raw_missing_parent" {
										kind = "missing_event"
										changedTarget = scenario.Tests[0]
									}
									var raw = structuralMigrationCorruptRaw(t, kind, changedTarget, data[names[1]])
									if bytes.Equal(raw, data[names[1]]) {
										t.Fatal("actual byte corruption did not occur", fault)
									}
									data[names[1]] = raw
								}
							})
						}
						break
					}
					corrupted, err := ValidateEvidenceManifest(&broken, selected, "local")
					if corrupted == nil || corrupted.ClosureComplete || corrupted.PassedCases != 0 {
						t.Fatal("damaged actual authorization/pure-public proof accepted", fault, corrupted, err)
					}
					if err == nil {
						t.Fatal("malformed or incomplete actual BRG21 proof lacked required error", fault, corrupted)
					}
				})
			}
		})
	}
	for index, path := range purePublicProductionPaths {
		t.Run("source_digest/"+strings.ReplaceAll(path, "/", "_"), func(t *testing.T) {
			var changed EvidenceRecord
			encoded, _ := json.Marshal(runner.records[0])
			if err := json.Unmarshal(encoded, &changed); err != nil {
				t.Fatal(err)
			}
			purePublicRewriteArtifacts(t, &changed, func(_ map[string][]byte, receipt *editorProcessReceipt) {
				receipt.SourceFiles[index].SHA256 = strings.Repeat("0", 64)
			})
			if validatePurePublicRecord(id, changed) == nil {
				t.Fatal("hash-reconciled source corruption accepted", path)
			}
		})
	}
	for _, other := range []string{"IM04", "IM05", "CL02", "RP29", "BRG18"} {
		if other == id {
			continue
		}
		t.Run("no_inheritance/"+other, func(t *testing.T) {
			related := &ContractsSet{Acceptance: &ContractFile{}, Repair: &RepairContractFile{}, Gates: &GatesContractFile{}}
			for _, row := range all.Acceptance.Cases {
				if row.ID == other {
					related.Acceptance.Cases = append(related.Acceptance.Cases, row)
				}
			}
			for _, row := range all.Repair.Cases {
				if row.ID == other {
					related.Repair.Cases = append(related.Repair.Cases, row)
				}
			}
			for _, row := range all.Gates.Gates {
				if row.ID == other {
					related.Gates.Gates = append(related.Gates.Gates, row)
				}
			}
			if len(related.Acceptance.Cases)+len(related.Repair.Cases)+len(related.Gates.Gates) != 1 {
				t.Fatal("missing linked immutable contract", other)
			}
			related.Acceptance.CaseCount = len(related.Acceptance.Cases)
			related.Repair.CaseCount = len(related.Repair.Cases)
			related.Gates.GateCount = len(related.Gates.Gates)
			var rebound EvidenceManifest
			if err := json.Unmarshal(original, &rebound); err != nil {
				t.Fatal(err)
			}
			for index := range rebound.Records {
				rebound.Records[index].Requirements = []string{other}
			}
			borrowed, err := ValidateEvidenceManifest(&rebound, related, "local")
			if err == nil || borrowed == nil || borrowed.PassedCases != 0 || borrowed.ClosureComplete {
				t.Fatal("linked requirement inherited component proof", other, borrowed, err)
			}
		})
	}
	auditPath := filepath.Join(t.TempDir(), "audit.json")
	if err = writeExecutionAudit(runner, all, auditPath); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatal(err)
	}
	var audit bindingCaptureAuditEnvelope
	if err = json.Unmarshal(raw, &audit); err != nil || len(audit.Rows) != 179 || audit.ClosureComplete {
		t.Fatal("component changed formal 179-row scope", err)
	}
	for _, row := range audit.Rows {
		if row.ID == id {
			if row.Status != "PASS" || !row.PositiveAndNegative || !reflect.DeepEqual(row.TestNames, scenario.Tests) || !reflect.DeepEqual(row.ProductionPath, purePublicProductionPaths) || !reflect.DeepEqual(row.Tier, scenario.Tiers) || row.Scope != scenario.Scope {
				t.Fatalf("component audit lost actual scope: %+v", row)
			}
		} else if row.Status == "PASS" {
			t.Fatal("unexecuted requirement inherited component proof", row.ID)
		}
	}
	receipt, err := json.Marshal(map[string]any{"requirement": id, "source_sha": runner.implSHA, "scope": scenario.Scope, "report": report, "records": runner.records})
	if err != nil {
		t.Fatal(err)
	}
	t.Log("P15_PURE_PUBLIC_PRODUCTION_RECEIPT:" + string(receipt))
}
