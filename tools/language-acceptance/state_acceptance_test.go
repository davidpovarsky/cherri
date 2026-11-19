package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const stateHistoricalSource = "4e56dbcfe895baa8f944b83715ecff0d07bfd24e"

func stateSelectedContracts(t *testing.T, all *ContractsSet) *ContractsSet {
	t.Helper()
	selected := &ContractsSet{Acceptance: &ContractFile{}, Repair: &RepairContractFile{}, Gates: &GatesContractFile{}}
	for _, contract := range all.Repair.Cases {
		if contract.ID == "RP51" {
			selected.Repair.Cases = append(selected.Repair.Cases, contract)
		}
	}
	for _, contract := range all.Gates.Gates {
		if contract.ID == "BRG06" {
			selected.Gates.Gates = append(selected.Gates.Gates, contract)
		}
	}
	selected.Repair.CaseCount, selected.Gates.GateCount = len(selected.Repair.Cases), len(selected.Gates.Gates)
	if selected.Repair.CaseCount != 1 || selected.Gates.GateCount != 1 {
		t.Fatal("original state contract selection changed")
	}
	return selected
}

func TestBackendStateProducerInventory(t *testing.T) {
	all, err := LoadAllContracts("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	selected := stateSelectedContracts(t, all)
	if len(stateAcceptanceScenarios) != 2 {
		t.Fatal("state inventory expanded without review")
	}
	rp, brg := selected.Repair.Cases[0], selected.Gates.Gates[0]
	if rp.Title != stateAcceptanceScenarios[rp.ID].Title || brg.Title != stateAcceptanceScenarios[brg.ID].Title ||
		!reflect.DeepEqual(parseRequiredTiers(strings.Join(rp.RequiredTestLevels, "+")), []string{"benchmark", "race"}) ||
		!reflect.DeepEqual(parseRequiredTiers(strings.Join(brg.MinimumEvidenceTiers, "+")), []string{"unit", "integration", "security"}) {
		t.Fatal("state producer altered immutable requirements")
	}
	for _, id := range []string{"RP51", "BRG06"} {
		var mapped []string
		if id == "RP51" {
			mapped = deriveRepairTestIDs(id, stateAcceptanceScenarios[id].Title)
		} else {
			mapped = deriveGateTestIDs(id, "unrelated-prefix")
		}
		if !reflect.DeepEqual(mapped, stateAcceptanceTestIDs(id)) {
			t.Fatal("state mapping is detached", id)
		}
		for _, path := range stateAcceptanceSourcePaths(id) {
			info, err := os.Stat(filepath.Join(resolveRepoRoot(), filepath.FromSlash(path)))
			if err != nil || !info.Mode().IsRegular() {
				t.Fatal("state producer source absent", path, err)
			}
		}
	}
	runner := NewRunner()
	for _, id := range []string{"E05", "E08", "S03", "RP04", "CL02", "RP29", "BRG32"} {
		if runner.runBackendStateRequirement(id, "borrowed scope", []string{"unit"}) {
			t.Fatal("linked requirement inherited state evidence", id)
		}
	}
	for id, scenario := range stateAcceptanceScenarios {
		if runner.runBackendStateRequirement(id, scenario.Title, scenario.Tiers[:1]) || runner.runBackendStateRequirement(id, "changed contract", scenario.Tiers) {
			t.Fatal("weakened state contract executed", id)
		}
	}
	if len(runner.results) != 0 || len(runner.records) != 0 || runner.stateRace != nil {
		t.Fatal("unselected state scenario executed")
	}
}

// This is the original archived Linux process, used only to test strict intake.
// Its source/run identity is never relabeled as this checkout's execution.
func stateArchivedRaceData(t *testing.T) map[string][]byte {
	t.Helper()
	path := filepath.Join(resolveRepoRoot(), "docs/language-v2/evidence/p15-state-race-r2-20261119/artifact.zip")
	raw, err := os.ReadFile(path)
	if err != nil || editorSHA(raw) != "3f79a62f5be1820ef9676929e90b9dc8573e15436cfde50738af54c9163d8fb3" {
		t.Fatal("historical actual race archive changed", err)
	}
	archive, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatal(err)
	}
	data := map[string][]byte{}
	for _, entry := range archive.File {
		if _, exists := data[entry.Name]; exists || entry.UncompressedSize64 > 4<<20 {
			t.Fatal("invalid original race archive member")
		}
		reader, err := entry.Open()
		if err != nil {
			t.Fatal(err)
		}
		content, err := io.ReadAll(reader)
		reader.Close()
		if err != nil {
			t.Fatal(err)
		}
		data[entry.Name] = content
	}
	if len(data) != 5 || editorSHA(data["go-test.jsonl"]) != "33962ed1b0e071976800d35e346221000b890c8c1335e573cb8757f1fffef15f" {
		t.Fatal("original race observations lost")
	}
	return data
}

func stateCloneData(original map[string][]byte) map[string][]byte {
	data := map[string][]byte{}
	for name, raw := range original {
		data[name] = append([]byte{}, raw...)
	}
	return data
}

func stateControlArtifacts(t *testing.T, data map[string][]byte, names []string) []EvidenceArtifact {
	t.Helper()
	directory := t.TempDir()
	var artifacts []EvidenceArtifact
	for _, name := range names {
		path := filepath.Join(directory, name)
		if err := os.WriteFile(path, data[name], 0600); err != nil {
			t.Fatal(err)
		}
		artifacts = append(artifacts, EvidenceArtifact{Path: path, SHA256: editorSHA(data[name])})
	}
	return artifacts
}

func stateControlJSON(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func stateControlReconcile(t *testing.T, data map[string][]byte, alter func(*stateRaceReceipt)) {
	t.Helper()
	var receipt stateRaceReceipt
	if err := json.Unmarshal(data["race-receipt.json"], &receipt); err != nil {
		t.Fatal(err)
	}
	receipt.RawSHA256, receipt.RawBytes = editorSHA(data["go-test.jsonl"]), len(data["go-test.jsonl"])
	receipt.StderrSHA256, receipt.StderrBytes = editorSHA(data["go-test.stderr"]), len(data["go-test.stderr"])
	receipt.EnvironmentSHA256, receipt.SourceFilesSHA256 = editorSHA(data["environment.json"]), editorSHA(data["source-files.json"])
	if alter != nil {
		alter(&receipt)
	}
	data["race-receipt.json"] = stateControlJSON(t, receipt)
}

func stateAlterEvents(t *testing.T, raw []byte, alter func(map[string]any) bool) []byte {
	t.Helper()
	var output bytes.Buffer
	for _, line := range bytes.Split(bytes.TrimSpace(raw), []byte{'\n'}) {
		var event map[string]any
		if err := json.Unmarshal(line, &event); err != nil {
			t.Fatal(err)
		}
		if alter(event) {
			output.Write(stateControlJSON(t, event))
			output.WriteByte('\n')
		}
	}
	return output.Bytes()
}

func TestBackendStateRaceIntakeActualArchive(t *testing.T) {
	original := stateArchivedRaceData(t)
	artifacts := stateControlArtifacts(t, original, stateRaceArtifactNames)
	for _, claim := range []struct{ id, tier string }{{"BRG06", "unit"}, {"BRG06", "integration"}, {"RP51", "race"}} {
		if err := validateStateRaceRecord(claim.id, stateRaceRecord(claim.id, claim.tier, stateHistoricalSource, artifacts)); err != nil {
			t.Fatalf("original actual archive rejected for %s/%s: %v", claim.id, claim.tier, err)
		}
	}
	mutations := []struct {
		name  string
		alter func(map[string][]byte, *EvidenceRecord)
	}{
		{"missing_artifact", func(_ map[string][]byte, r *EvidenceRecord) { r.Artifacts = r.Artifacts[:4] }},
		{"duplicate_artifact", func(_ map[string][]byte, r *EvidenceRecord) { r.Artifacts[1] = r.Artifacts[0] }},
		{"wrong_producer", func(_ map[string][]byte, r *EvidenceRecord) { r.TestID = "tools/language-acceptance:runRecoveryGates" }},
		{"wrong_tier", func(_ map[string][]byte, r *EvidenceRecord) { r.Tier = "unit" }},
		{"borrowed_id", func(_ map[string][]byte, r *EvidenceRecord) { r.Requirements = append(r.Requirements, "E05") }},
		{"nonrace_command", func(_ map[string][]byte, r *EvidenceRecord) {
			r.Command = productionTestCommand([]string{stateRaceTests[0]})
		}},
		{"partial_selection", func(_ map[string][]byte, r *EvidenceRecord) {
			r.Assertions["required_production_tests"] = stateRaceTests[0]
		}},
		{"native_claim", func(_ map[string][]byte, r *EvidenceRecord) { r.Assertions["scope"] = "actual Apple runtime" }},
		{"run_identity", func(_ map[string][]byte, r *EvidenceRecord) { r.Assertions["race_run_id"] = "1" }},
		{"current_source_relabel", func(_ map[string][]byte, r *EvidenceRecord) { r.ImplementationSHA = NewRunner().implSHA }},
		{"failed_record", func(_ map[string][]byte, r *EvidenceRecord) { r.Result = "failed"; r.ExitCode = 1 }},
	}
	for _, fault := range []string{"exit", "timeout", "dirty_before", "dirty_after", "wrong_repo", "no_run", "partial_tests", "elapsed", "invented_claim", "receipt_scope", "missing_issues"} {
		fault := fault
		mutations = append(mutations, struct {
			name  string
			alter func(map[string][]byte, *EvidenceRecord)
		}{fault, func(data map[string][]byte, _ *EvidenceRecord) {
			stateControlReconcile(t, data, func(r *stateRaceReceipt) {
				switch fault {
				case "exit":
					*r.ExitCode = 1
				case "timeout":
					*r.TimedOut = true
				case "dirty_before":
					r.SourceCleanBefore = false
				case "dirty_after":
					r.SourceCleanAfter = false
				case "wrong_repo":
					r.Repository = "unrelated/cherri"
				case "no_run":
					r.RunID = ""
				case "partial_tests":
					r.RequiredTests = r.RequiredTests[:1]
				case "elapsed":
					r.ElapsedSeconds = 0.00001
				case "invented_claim":
					r.FormalRequirementsProved = []string{"BRG06", "RP51"}
				case "receipt_scope":
					r.Scope = stateRaceScope
				case "missing_issues":
					r.Issues = nil
				}
			})
		}})
	}
	for _, change := range []struct{ key, value string }{{"CGO_ENABLED", "0"}, {"GOOS", "windows"}, {"GOARCH", "other"}, {"GORACE", "exitcode=0 log_path=hidden"}, {"GOFLAGS", "-race=false"}, {"GOFLAGS", "-run=Nothing"}, {"GOMAXPROCS", "0"}, {"go_version", "go version fabricated"}} {
		change := change
		mutations = append(mutations, struct {
			name  string
			alter func(map[string][]byte, *EvidenceRecord)
		}{"environment_" + change.key + "_" + change.value, func(data map[string][]byte, _ *EvidenceRecord) {
			var env map[string]string
			if err := json.Unmarshal(data["environment.json"], &env); err != nil {
				t.Fatal(err)
			}
			env[change.key] = change.value
			data["environment.json"] = stateControlJSON(t, env)
			stateControlReconcile(t, data, nil)
		}})
	}
	for _, fault := range []string{"missing_child_pass", "missing_run", "skipped", "failed", "foreign_package", "duplicate_json", "duplicate_witness", "false_instrumentation", "incomplete_mixed", "incomplete_imports", "swallowed_panic", "race_warning", "invalid_utf8", "invalid_utf8_environment"} {
		fault := fault
		mutations = append(mutations, struct {
			name  string
			alter func(map[string][]byte, *EvidenceRecord)
		}{"raw_" + fault, func(data map[string][]byte, _ *EvidenceRecord) {
			data["go-test.jsonl"] = stateAlterEvents(t, data["go-test.jsonl"], func(e map[string]any) bool {
				test, _ := e["Test"].(string)
				action, _ := e["Action"].(string)
				output, _ := e["Output"].(string)
				if fault == "missing_child_pass" && test == stateRaceTests[3] && action == "pass" {
					return false
				}
				if fault == "missing_run" && test == stateRaceTests[0] && action == "run" {
					return false
				}
				if fault == "skipped" && test == stateRaceTests[0] && action == "pass" {
					e["Action"] = "skip"
				}
				if fault == "failed" && test == stateRaceTests[0] && action == "pass" {
					e["Action"] = "fail"
				}
				if fault == "foreign_package" {
					e["Package"] = "example.invalid/foreign"
				}
				if strings.Contains(output, "P15_STATE_EXERCISE=") {
					if fault == "duplicate_witness" {
						e["Output"] = output + output
					}
					if fault == "false_instrumentation" {
						e["Output"] = strings.ReplaceAll(output, `"race_instrumented":true`, `"race_instrumented":false`)
					}
					if fault == "incomplete_mixed" {
						e["Output"] = strings.ReplaceAll(output, `"completed_cases":64`, `"completed_cases":63`)
					}
					if fault == "incomplete_imports" {
						e["Output"] = strings.ReplaceAll(output, `"completed_cases":32`, `"completed_cases":31`)
					}
					if fault == "swallowed_panic" {
						e["Output"] = strings.ReplaceAll(output, `"partial_output_rejected":true`, `"partial_output_rejected":false`)
					}
				}
				return true
			})
			if fault == "duplicate_json" {
				data["go-test.jsonl"] = bytes.Replace(data["go-test.jsonl"], []byte(`"Action":"run"`), []byte(`"Action":"run","Action":"pass"`), 1)
			}
			if fault == "race_warning" {
				data["go-test.stderr"] = append(data["go-test.stderr"], []byte("\nWARNING: DATA RACE\n")...)
			}
			if fault == "invalid_utf8" {
				data["go-test.jsonl"] = bytes.Replace(data["go-test.jsonl"], []byte("=== RUN"), []byte{0xff, '=', '=', ' ', 'R', 'U', 'N'}, 1)
			}
			if fault == "invalid_utf8_environment" {
				data["environment.json"] = bytes.Replace(data["environment.json"], []byte("Linux-"), []byte{0xff, 'i', 'n', 'u', 'x', '-'}, 1)
			}
			// Reconcile every content digest. Also counterfeit the summary witness
			// to match corrupted raw values: the canonical evaluator must reject it.
			stateControlReconcile(t, data, func(r *stateRaceReceipt) {
				for name, entries := range r.Observations {
					for _, entry := range entries.([]any) {
						w := entry.(map[string]any)
						if fault == "false_instrumentation" {
							w["race_instrumented"] = false
						}
						if fault == "incomplete_mixed" && name == stateRaceTests[0] {
							w["completed_cases"] = 63
						}
						if fault == "incomplete_imports" && name == stateRaceTests[4] {
							w["completed_cases"] = 31
						}
						if fault == "swallowed_panic" && name == stateRaceTests[5] {
							w["partial_output_rejected"] = false
						}
					}
				}
			})
		}})
	}
	for index, path := range stateRaceSourcePaths {
		index, path := index, path
		mutations = append(mutations, struct {
			name  string
			alter func(map[string][]byte, *EvidenceRecord)
		}{fmt.Sprintf("immutable_source_%02d", index), func(data map[string][]byte, _ *EvidenceRecord) {
			var files []editorSourceFile
			if err := json.Unmarshal(data["source-files.json"], &files); err != nil {
				t.Fatal(err)
			}
			if files[index].Path != path {
				t.Fatal("source mutation detached")
			}
			files[index].SHA256 = strings.Repeat("f", 64)
			data["source-files.json"] = stateControlJSON(t, files)
			stateControlReconcile(t, data, nil)
		}})
	}
	for _, mutation := range mutations {
		t.Run(mutation.name, func(t *testing.T) {
			data := stateCloneData(original)
			record := stateRaceRecord("RP51", "race", stateHistoricalSource, stateControlArtifacts(t, data, stateRaceArtifactNames))
			mutation.alter(data, &record)
			// Preserve intentional outer-record changes while updating each file's
			// bytes/hash, including all forged inner-receipt reconciliations.
			for index := range record.Artifacts {
				a := &record.Artifacts[index]
				raw := data[filepath.Base(a.Path)]
				if err := os.WriteFile(a.Path, raw, 0600); err != nil {
					t.Fatal(err)
				}
				a.SHA256 = editorSHA(raw)
			}
			if err := validateStateRaceRecord("RP51", record); err == nil {
				t.Fatal("damaged or substituted actual race evidence accepted")
			}
		})
	}
}

func TestBackendStateSecurityStreamControls(t *testing.T) {
	// Synthetic parser controls, not a production execution or acceptance claim.
	var events = []map[string]any{{"Action": "start", "Package": "github.com/electrikmilk/cherri"}, {"Action": "run", "Package": "github.com/electrikmilk/cherri", "Test": stateSecurityTest}, {"Action": "pass", "Package": "github.com/electrikmilk/cherri", "Test": stateSecurityTest}, {"Action": "pass", "Package": "github.com/electrikmilk/cherri"}}
	var original []byte
	for _, e := range events {
		original = append(original, stateControlJSON(t, e)...)
		original = append(original, '\n')
	}
	if err := parseStateSecurityOutput(original); err != nil {
		t.Fatal(err)
	}
	t.Run("invalid_utf8_output", func(t *testing.T) {
		output := []byte(`{"Action":"output","Package":"github.com/electrikmilk/cherri","Test":"` + stateSecurityTest + `","Output":"`)
		output = append(output, 0xff)
		output = append(output, []byte("\"}\n")...)
		if err := parseStateSecurityOutput(append(output, original...)); err == nil {
			t.Fatal("invalid UTF-8 was silently normalized")
		}
	})
	for _, fault := range []string{"no_run", "no_pass", "no_package", "skip", "fail", "foreign", "unexpected_child", "duplicate_event", "duplicate_json", "after_package"} {
		t.Run(fault, func(t *testing.T) {
			raw := stateAlterEvents(t, original, func(e map[string]any) bool {
				if fault == "no_run" && e["Action"] == "run" {
					return false
				}
				if fault == "no_pass" && e["Action"] == "pass" && e["Test"] == stateSecurityTest {
					return false
				}
				if fault == "no_package" && e["Action"] == "pass" && e["Test"] == nil {
					return false
				}
				if (fault == "skip" || fault == "fail") && e["Test"] == stateSecurityTest && e["Action"] == "pass" {
					e["Action"] = fault
				}
				if fault == "foreign" {
					e["Package"] = "other/package"
				}
				if fault == "unexpected_child" && e["Test"] != nil {
					e["Test"] = stateSecurityTest + "/invented"
				}
				return true
			})
			if fault == "duplicate_event" {
				raw = append(stateControlJSON(t, events[1]), append([]byte{'\n'}, raw...)...)
			}
			if fault == "duplicate_json" {
				raw = bytes.Replace(raw, []byte(`"Action":"run"`), []byte(`"Action":"run","Action":"pass"`), 1)
			}
			if fault == "after_package" {
				raw = append(raw, append(stateControlJSON(t, events[0]), '\n')...)
			}
			if err := parseStateSecurityOutput(raw); err == nil {
				t.Fatal("partial or fabricated security selection accepted")
			}
		})
	}
}

func TestBackendStateActualAvailableProduction(t *testing.T) {
	all, err := LoadAllContracts("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	selected := stateSelectedContracts(t, all)
	runner := NewRunner()
	if directory := os.Getenv("CHERRI_STATE_TEST_EVIDENCE_DIR"); directory != "" {
		runner.artifactDir = directory
	} else {
		runner.artifactDir = t.TempDir()
	}
	runner.runRepairCases(selected.Repair)
	runner.runRecoveryGates(selected.Gates)
	if len(runner.results) != 2 || runner.stateRace == nil || len(runner.observations) != 0 {
		t.Fatal("state actual execution inventory drifted")
	}
	wantRecords, wantPassed := 2, 0
	if runner.stateRace.Available {
		wantRecords, wantPassed = 5, 2
	}
	if len(runner.records) != wantRecords {
		t.Fatalf("actual state scope: records=%d want=%d", len(runner.records), wantRecords)
	}
	for id, result := range runner.results {
		wantTiers := []string{"benchmark"}
		if id == "BRG06" {
			wantTiers = []string{"security"}
		}
		wantStatus := "NOT_RUN"
		if runner.stateRace.Available {
			wantStatus = "PASSED"
			wantTiers = stateAcceptanceScenarios[id].Tiers
		}
		if result.Status != wantStatus || result.Passed != runner.stateRace.Available || !reflect.DeepEqual(result.ExecutedLevels, wantTiers) {
			t.Fatalf("actual state execution failed or promoted missing race: %+v", result)
		}
	}
	for _, record := range runner.records {
		id := record.Requirements[0]
		if record.Result != "passed" || record.ExitCode != 0 || record.ImplementationSHA != runner.implSHA {
			t.Fatal("failed current actual process", record.Detail)
		}
		if err := validateBackendStateRecord(id, record); err != nil {
			t.Fatal(err)
		}
		if err := validateEditorPerformanceRecord(id, record); err != nil {
			t.Fatal(err)
		}
		for _, fault := range []string{"missing_artifact", "wrong_scope", "borrowed_requirement", "partial_selection", "stale_source", "failed_process"} {
			t.Run(id+"/"+record.Tier+"/"+fault, func(t *testing.T) {
				var changed EvidenceRecord
				if err := json.Unmarshal(stateControlJSON(t, record), &changed); err != nil {
					t.Fatal(err)
				}
				switch fault {
				case "missing_artifact":
					changed.Artifacts = changed.Artifacts[:len(changed.Artifacts)-1]
				case "wrong_scope":
					changed.Assertions["scope"] = "Apple runtime"
				case "borrowed_requirement":
					changed.Requirements = append(changed.Requirements, "E08")
				case "partial_selection":
					delete(changed.Assertions, "required_production_tests")
				case "stale_source":
					changed.ImplementationSHA = stateHistoricalSource
				case "failed_process":
					changed.ExitCode = 1
					changed.Result = "failed"
				}
				var err error
				if id == "RP51" && record.Tier == "benchmark" {
					err = validateEditorPerformanceRecord(id, changed)
				} else {
					err = validateBackendStateRecord(id, changed)
				}
				if fault == "failed_process" && id == "RP51" && record.Tier == "benchmark" { // Generic intake checks process status before specialized benchmark intake.
					_, err = ValidateEvidenceManifest(&EvidenceManifest{SchemaVersion: "1", ImplementationSHA: runner.implSHA, Records: []EvidenceRecord{changed}}, selected, "local")
				}
				if err == nil {
					t.Fatal("invalid current state evidence accepted")
				}
			})
		}
	}
	manifest := &EvidenceManifest{SchemaVersion: "1", ImplementationSHA: runner.implSHA, Records: runner.records}
	intake, intakeErr := ValidateEvidenceManifest(manifest, selected, "local")
	if runner.stateRace.Available {
		if intakeErr != nil || intake.PassedCases != 2 || intake.FailedCases != 0 || intake.ClosureComplete {
			t.Fatalf("complete actual state receipt rejected: %+v %v", intake, intakeErr)
		}
	} else if intakeErr == nil || intake.PassedCases != 0 || intake.FailedCases != 2 || intake.ClosureComplete {
		t.Fatalf("strict intake promoted missing local race/unit/integration: %+v %v", intake, intakeErr)
	}
	// The direct local runner explicitly retains NOT_RUN with observed partial
	// levels. Strict manifest intake must still reject the missing local tiers.
	report, err := ValidateAndAggregateWithPhase(allLocalContracts(selected), runner.results, runner.reg.Fingerprint(), "local")
	if err != nil || report.PassedCases != wantPassed || report.FailedCases != 0 || report.SkippedCases != 2-wantPassed || report.ClosureComplete {
		t.Fatalf("state local accounting invalid: %+v %v", report, err)
	}
	for _, id := range []string{"RP51", "BRG06"} {
		t.Run(id+"/missing_required_tier", func(t *testing.T) {
			var partial []EvidenceRecord
			for _, record := range runner.records {
				if record.Requirements[0] == id && record.Tier != "benchmark" && record.Tier != "security" {
					partial = append(partial, record)
				}
			}
			incomplete, err := ValidateEvidenceManifest(&EvidenceManifest{SchemaVersion: "1", ImplementationSHA: runner.implSHA, Records: partial}, selected, "local")
			if err == nil || incomplete == nil || incomplete.ClosureComplete || p15VerificationResult(t, incomplete, id).Passed {
				t.Fatal("partial race alone closed complete state requirement", err)
			}
		})
	}
	path := filepath.Join(t.TempDir(), "audit.json")
	if err := writeExecutionAudit(runner, all, path); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var audit bindingCaptureAuditEnvelope
	if err := json.Unmarshal(raw, &audit); err != nil || len(audit.Rows) != 179 || audit.ClosureComplete {
		t.Fatal("state producer changed full179 accounting", err)
	}
	for _, row := range audit.Rows {
		if _, selected := stateAcceptanceScenarios[row.ID]; selected {
			if row.PositiveAndNegative != runner.stateRace.Available || !reflect.DeepEqual(row.TestNames, stateAcceptanceTests(row.ID)) || !reflect.DeepEqual(row.ProductionPath, stateAcceptanceSourcePaths(row.ID)) {
				t.Fatal("state audit detached from actual producer", row)
			}
		} else if row.Status == "PASS" || row.PositiveAndNegative {
			t.Fatal("unselected requirement inherited state success", row.ID)
		}
	}
	t.Log("P15_STATE_ACCEPTANCE_PRODUCTION_RECEIPT:" + string(stateControlJSON(t, map[string]any{"source_sha": runner.implSHA, "scope": "Independently executed state security and strict cold/warm benchmark; race only when the complete actual current-source Linux bundle is available", "report": report, "records": runner.records})))
}
