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

func explicitSigningSelectedContracts(t *testing.T) (*ContractsSet, *ContractsSet) {
	t.Helper()
	all, err := LoadAllContracts("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	selected := &ContractsSet{Acceptance: &ContractFile{}, Repair: &RepairContractFile{}, Gates: &GatesContractFile{}}
	for _, row := range all.Gates.Gates {
		if row.ID == "BRG26" {
			selected.Gates.Gates = append(selected.Gates.Gates, row)
		}
	}
	if len(selected.Gates.Gates) != 1 {
		t.Fatal("missing immutable BRG26 contract")
	}
	selected.Gates.GateCount = 1
	return all, selected
}

func TestExplicitSigningProducerInventory(t *testing.T) {
	_, selected := explicitSigningSelectedContracts(t)
	row := selected.Gates.Gates[0]
	if row.Title != explicitSigningTitle || row.Scenario != explicitSigningScenario ||
		row.ExpectedObservation != explicitSigningExpected ||
		!reflect.DeepEqual(parseRequiredTiers(strings.Join(row.MinimumEvidenceTiers, "+")), explicitSigningRequiredTiers) {
		t.Fatal("original BRG26 contract changed")
	}
	names := explicitSigningRequiredEvents()
	if len(names) != 13 || !sort.StringsAreSorted(names) {
		t.Fatal("not the complete original 13-case inventory", names)
	}
	seen := map[string]bool{}
	for _, name := range names {
		if name == "" || seen[name] {
			t.Fatal("duplicate or empty actual event", name)
		}
		seen[name] = true
	}
	for _, path := range explicitSigningProductionPaths {
		if seen[path] {
			t.Fatal("duplicate source witness", path)
		}
		seen[path] = true
		info, err := os.Stat(filepath.Join(resolveRepoRoot(), filepath.FromSlash(path)))
		if err != nil || !info.Mode().IsRegular() {
			t.Fatal("missing actual signing source witness", path, err)
		}
	}
	if !reflect.DeepEqual(deriveGateTestIDs(row.ID, row.ProposedTestPrefix), explicitSigningTestIDs()) ||
		!reflect.DeepEqual(deriveGateFixtures(row.ID), explicitSigningProductionPaths) {
		t.Fatal("dedicated signing mapping lost")
	}
	var ids []string
	assertions, ui := map[string]string{}, map[string]string{}
	applyRuntimeAndUIMappings("BRG26", explicitSigningRequiredTiers, &ids, assertions, ui)
	if len(ids) != 0 || len(assertions) != 0 || len(ui) != 0 {
		t.Fatal("generic runtime supplied explicit signing evidence")
	}
	runner := NewRunner()
	for _, id := range []string{"RP43", "K05", "CL01", "CI02", "CI04", "RP48", "BRG31"} {
		foreign := row
		foreign.ID = id
		if runner.runExplicitSigningRequirement(foreign) {
			t.Fatal("related requirement inherited signing producer", id)
		}
	}
	for _, field := range []string{"title", "scenario", "expected", "tiers", "mandatory"} {
		t.Run("contract/"+field, func(t *testing.T) {
			changed := row
			switch field {
			case "mandatory":
				changed.Mandatory = false
			case "title":
				changed.Title = "different"
			case "scenario":
				changed.Scenario = "only a signed header"
			case "expected":
				changed.ExpectedObservation = "ambient names allowed"
			case "tiers":
				changed.MinimumEvidenceTiers = []string{"unit", "integration"}
			}
			if !runner.runExplicitSigningRequirement(changed) || runner.results["BRG26"].Status != "FAILED" || len(runner.records) != 0 {
				t.Fatal("changed original signing contract admitted", field)
			}
		})
	}
}

func explicitSigningMutateProcess(t *testing.T, record *EvidenceRecord, mutation func(*editorProcessReceipt)) {
	t.Helper()
	editorContractMutateArtifact(t, record, explicitSigningPrefix+"-process.json", func(raw []byte) []byte {
		var receipt editorProcessReceipt
		if err := json.Unmarshal(raw, &receipt); err != nil {
			t.Fatal(err)
		}
		mutation(&receipt)
		changed, err := json.Marshal(receipt)
		if err != nil {
			t.Fatal(err)
		}
		return changed
	})
}

func explicitSigningMutateEvents(t *testing.T, record *EvidenceRecord, fault string) {
	t.Helper()
	target := explicitSigningTests[0] + "/multipart/legacy"
	var changed []byte
	editorContractMutateArtifact(t, record, explicitSigningPrefix+"-go.jsonl", func(raw []byte) []byte {
		var out bytes.Buffer
		found := false
		for _, line := range bytes.Split(bytes.TrimSpace(raw), []byte{'\n'}) {
			var event map[string]any
			if err := json.Unmarshal(line, &event); err != nil {
				t.Fatal(err)
			}
			if event["Test"] == target {
				found = true
				if fault == "missing_child" {
					continue
				}
				if event["Action"] == "pass" {
					switch fault {
					case "skip_child":
						event["Action"] = "skip"
						line, _ = json.Marshal(event)
					case "failed_child":
						event["Action"] = "fail"
						line, _ = json.Marshal(event)
					case "duplicate_child":
						out.Write(line)
						out.WriteByte('\n')
					case "unknown_child":
						event["Test"] = target + "_unreviewed"
						line, _ = json.Marshal(event)
					}
				}
			}
			out.Write(line)
			out.WriteByte('\n')
		}
		if !found || bytes.Equal(out.Bytes(), raw) {
			t.Fatal("causal mutation did not change the original selected child", fault)
		}
		changed = out.Bytes()
		return changed
	})
	// Reconcile bytes and sizes deliberately: the strict semantic event
	// inventory must reject the forgery even after its surrounding hashes agree.
	explicitSigningMutateProcess(t, record, func(receipt *editorProcessReceipt) {
		receipt.RawSHA256, receipt.RawBytes = editorSHA(changed), len(changed)
	})
}

func TestExplicitSigningActualProductionEvidence(t *testing.T) {
	all, selected := explicitSigningSelectedContracts(t)
	runner := NewRunner()
	runner.artifactDir = os.Getenv("CHERRI_EXPLICIT_SIGNING_EVIDENCE_OUTPUT")
	runner.runRecoveryGates(selected.Gates)
	result := runner.results["BRG26"]
	if len(runner.results) != 1 || len(runner.records) != 2 || len(runner.observations) != 0 ||
		result.Passed || result.Status != "PENDING_EXTERNAL" ||
		!reflect.DeepEqual(result.ExecutedLevels, explicitSigningLocalTiers) {
		t.Fatalf("BRG26 full local execution failed or claimed Apple: %+v records=%d", runner.results, len(runner.records))
	}
	manifest := &EvidenceManifest{SchemaVersion: "1", ImplementationSHA: runner.implSHA, Records: runner.records}
	report, err := ValidateEvidenceManifest(manifest, selected, "local")
	if err != nil || report == nil || report.PassedCases != 0 || report.FailedCases != 0 ||
		report.SkippedCases != 1 || report.ClosureComplete {
		t.Fatalf("local protocol was accepted as real Apple signing/runtime: %+v %v", report, err)
	}
	for _, original := range runner.records {
		if err := validateExplicitSigningRecord("BRG26", original); err != nil {
			t.Fatal(err)
		}
		if original.Tier != "unit" {
			continue
		}
		for _, fault := range []string{
			"borrowed_producer", "borrowed_requirement", "added_requirement", "fake_runtime",
			"partial_tests", "missing_events_assertion", "missing_scope", "invented_signature",
			"invented_runtime", "nonzero_record", "stale_record_source", "missing_artifact", "duplicate_artifact",
			"dirty_source", "wrong_process_source", "wrong_process_command", "timed_out",
			"missing_exit", "missing_source_witness", "wrong_scenario_blob", "outside_process_time",
			"missing_child", "skip_child", "failed_child", "duplicate_child", "unknown_child",
		} {
			t.Run(fault, func(t *testing.T) {
				broken := editorContractCopyRecord(t, original)
				switch fault {
				case "borrowed_producer":
					broken.TestID = "github.com/electrikmilk/cherri:TestP15RP43OutputSigning"
				case "borrowed_requirement":
					broken.Requirements = []string{"RP43"}
				case "added_requirement":
					broken.Requirements = []string{"BRG26", "K05"}
				case "fake_runtime":
					broken.Tier, broken.TestID = "ios-runtime", "ios27-runtime-poc:functions"
				case "partial_tests":
					broken.Command = productionTestCommand(explicitSigningTests[:1])
				case "missing_events_assertion":
					delete(broken.Assertions, "required_go_events")
				case "missing_scope":
					delete(broken.Assertions, "scope")
				case "invented_signature":
					broken.Assertions["apple_signature_authenticated"] = "true"
				case "invented_runtime":
					broken.Assertions["apple_runtime_observed"] = "true"
				case "nonzero_record":
					broken.ExitCode = 1
				case "stale_record_source":
					broken.ImplementationSHA = strings.Repeat("f", 40)
				case "missing_artifact":
					broken.Artifacts = broken.Artifacts[:len(broken.Artifacts)-1]
				case "duplicate_artifact":
					broken.Artifacts = append(broken.Artifacts, broken.Artifacts[0])
				case "missing_child", "skip_child", "failed_child", "duplicate_child", "unknown_child":
					explicitSigningMutateEvents(t, &broken, fault)
				default:
					explicitSigningMutateProcess(t, &broken, func(receipt *editorProcessReceipt) {
						switch fault {
						case "dirty_source":
							receipt.SourceCleanAfter = false
						case "wrong_process_source":
							receipt.SourceCommit = strings.Repeat("a", 40)
						case "wrong_process_command":
							receipt.Command = productionTestCommand(explicitSigningTests[1:])
						case "timed_out":
							value := true
							receipt.TimedOut = &value
						case "missing_exit":
							receipt.ExitCode = nil
						case "missing_source_witness":
							receipt.SourceFiles = receipt.SourceFiles[1:]
						case "wrong_scenario_blob":
							found := false
							for i := range receipt.SourceFiles {
								if receipt.SourceFiles[i].Path == "p15_signing_explicit_joined_test.go" {
									receipt.SourceFiles[i].SHA256 = strings.Repeat("e", 64)
									found = true
								}
							}
							if !found {
								t.Fatal("actual joined signing source witness absent")
							}
						case "outside_process_time":
							receipt.StartedUTC = "2000-01-01T00:00:00Z"
							receipt.FinishedUTC = "2000-01-01T00:00:01Z"
							receipt.ElapsedSeconds = 1
						default:
							t.Fatal("unknown negative control", fault)
						}
					})
				}
				if err := validateExplicitSigningRecord("BRG26", broken); err == nil {
					t.Fatal("incomplete or forged exact signing evidence accepted", fault)
				}
			})
		}
	}
	if final, err := ValidateEvidenceManifest(manifest, selected, "final"); err == nil || final != nil && final.ClosureComplete {
		t.Fatal("BRG26 finalized without actual Apple import/execution")
	}
	directory := runner.artifactDir
	if directory == "" {
		directory = t.TempDir()
	}
	auditPath := filepath.Join(directory, "explicit-signing-audit.json")
	if err := writeExecutionAudit(runner, all, auditPath); err != nil {
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
	if err := json.Unmarshal(raw, &audit); err != nil || len(audit.Rows) != 179 || audit.ClosureComplete {
		t.Fatal("explicit signing changed immutable179 audit accounting", err)
	}
	for _, row := range audit.Rows {
		if row.Status == "PASS" || row.ID != "BRG26" && row.PositiveAndNegative {
			t.Fatal("local explicit signing completed an unproved requirement", row.ID)
		}
		if row.ID == "BRG26" && (!row.PositiveAndNegative ||
			!reflect.DeepEqual(row.TestNames, explicitSigningTests) ||
			!reflect.DeepEqual(row.ProductionPath, explicitSigningProductionPaths) ||
			row.Scope != explicitSigningScope) {
			t.Fatal("actual BRG26 local scope lost", row)
		}
	}
	proof, err := json.MarshalIndent(map[string]any{
		"source": runner.implSHA, "scope": explicitSigningScope,
		"report": report, "results": runner.results, "records": runner.records,
	}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "explicit-signing-evidence.json"), append(proof, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
	t.Logf("BRG26 executed all13 original events on %s; actual Apple signing/import/runtime remains pending", runner.implSHA)
}
