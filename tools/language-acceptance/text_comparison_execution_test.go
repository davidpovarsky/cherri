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

func textComparisonSelectedContracts(t *testing.T) (*ContractsSet, *ContractsSet, CaseSpec) {
	t.Helper()
	all, err := LoadAllContracts("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	selected := &ContractsSet{Acceptance: &ContractFile{}, Repair: &RepairContractFile{}, Gates: &GatesContractFile{}}
	var actual CaseSpec
	for _, row := range all.Acceptance.Cases {
		if row.ID == textComparisonRequirement {
			actual = row
			selected.Acceptance.Cases = append(selected.Acceptance.Cases, row)
		}
	}
	selected.Acceptance.CaseCount = len(selected.Acceptance.Cases)
	if selected.Acceptance.CaseCount != 1 {
		t.Fatal("missing or duplicated immutable N03 contract")
	}
	return all, selected, actual
}

func TestTextComparisonInventoryAndTierAuthority(t *testing.T) {
	all, _, contract := textComparisonSelectedContracts(t)
	if contract.Group != textComparisonGroup || contract.Requirement != textComparisonTitle ||
		contract.MinimumTestLevel != textComparisonLevel || contract.Expected != "New contract tested; old semantics preserved explicitly" ||
		contract.NegativeCase || !reflect.DeepEqual(parseRequiredTiers(contract.MinimumTestLevel), textComparisonTiers) {
		t.Fatal("N03 producer differs from immutable unit/native-structure authority")
	}
	if err := validateTextComparisonInventory(textComparisonEvidence); err != nil {
		t.Fatal(err)
	}
	t.Run("source_count_before_launch", func(t *testing.T) {
		if err := validateTextComparisonSourceInventory(make([]string, 129)); err == nil || !strings.Contains(err.Error(), "invalid bounded") {
			t.Fatal("source bound was not enforced before launch", err)
		}
	})
	t.Run("migration_dispatcher_required", func(t *testing.T) {
		var names []string
		for _, name := range textComparisonEvidence.SourcePaths {
			if name != "internal/language/migrate/migrate.go" {
				names = append(names, name)
			}
		}
		if len(names) != len(textComparisonEvidence.SourcePaths)-1 {
			t.Fatal("control did not remove the actual migration dispatcher")
		}
		if err := validateTextComparisonSourceInventory(names); err == nil || !strings.Contains(err.Error(), "reviewed source witnesses") {
			t.Fatal("migration source omission did not reach its exact witness gate", err)
		}
	})
	t.Run("complete_parent_required", func(t *testing.T) {
		value := textComparisonEvidence
		value.Tests = append([]string{}, value.Tests...)
		for index, name := range value.Tests {
			if name == textComparisonNativeTest {
				value.Tests = append(value.Tests[:index], value.Tests[index+1:]...)
				break
			}
		}
		if err := validateTextComparisonInventory(value); err == nil {
			t.Fatal("joined complete scenario was optional")
		}
	})
	t.Run("causal_child_required", func(t *testing.T) {
		value := textComparisonEvidence
		value.Events = append([]string{}, value.Events...)
		for index, name := range value.Events {
			if name == textComparisonNativeTest+"/legacy_subject" {
				value.Events = append(value.Events[:index], value.Events[index+1:]...)
				break
			}
		}
		if err := validateTextComparisonInventory(value); err == nil {
			t.Fatal("joined causal event was optional")
		}
	})
	for _, level := range []string{"unit", "unit+native+iOS"} {
		t.Run("immutable_level_"+strings.ReplaceAll(level, "+", "_"), func(t *testing.T) {
			runner := NewRunner()
			if !runner.runTextComparisonRequirement(textComparisonRequirement, textComparisonGroup, textComparisonTitle, level) ||
				len(runner.records) != 0 || runner.results[textComparisonRequirement].Status != "FAILED" {
				t.Fatal("changed required tier set was silently accepted")
			}
		})
	}
	for _, tier := range []string{"ios-runtime", "ios-ui", "ios-build"} {
		t.Run("no_"+tier+"_claim", func(t *testing.T) {
			if err := validateTextComparisonRecord(textComparisonRequirement, EvidenceRecord{
				Requirements: []string{textComparisonRequirement}, Tier: tier, TestID: "ios27-runtime-poc:conditionals", Result: "passed",
			}); err == nil {
				t.Fatal("local producer granted an unexecuted platform tier")
			}
		})
	}
	runner := NewRunner()
	if runner.runTextComparisonRequirement("RP33", "repair", "related migration", textComparisonLevel) || len(runner.results) != 0 {
		t.Fatal("related migration requirement inherited N03 proof")
	}
	mapping, err := BuildRequirementsEvidenceMap(all)
	if err != nil {
		t.Fatal(err)
	}
	actual := mapping.Mappings[textComparisonRequirement]
	if !reflect.DeepEqual(actual.TestIDs, textComparisonTestIDs()) ||
		!reflect.DeepEqual(actual.FixtureIdentities, textComparisonEvidence.SourcePaths) ||
		len(actual.RequiredAssertions) != 0 || len(actual.RequiredUIAssertions) != 0 {
		t.Fatal("N03 mappings do not name only the complete local producer and exact witness")
	}
	if incompleteLocalAssertions[textComparisonRequirement] != "" {
		t.Fatal("reviewed complete producer is still routed through the historical weak-probe guard")
	}
	for _, path := range textComparisonEvidence.SourcePaths {
		if _, err := os.Stat(resolveRepoPath(path)); err != nil {
			t.Fatal(err)
		}
	}
	if productionTestTimeout(textComparisonEvidence.Tests) != 3*time.Minute {
		t.Fatal("original default production deadline changed")
	}
}

// This parent runs the complete actual production selection once. Every intake
// fault below mutates owned copies of that same original; none replays a producer.
func TestTextComparisonActualProductionEvidence(t *testing.T) {
	all, selected, contract := textComparisonSelectedContracts(t)
	runner := NewRunner()
	runner.artifactDir = os.Getenv("CHERRI_TEXT_COMPARISON_EVIDENCE_OUTPUT")
	if !runner.runTextComparisonRequirement(contract.ID, contract.Group, contract.Requirement, contract.MinimumTestLevel) {
		t.Fatal("N03 producer was not selected")
	}
	if len(runner.results) != 1 || len(runner.records) != 2 || len(runner.observations) != 0 {
		t.Fatalf("incomplete N03 local producer: results=%+v records=%d observations=%d", runner.results, len(runner.records), len(runner.observations))
	}
	actual := runner.results[textComparisonRequirement]
	if !actual.Passed || actual.Status != "PASSED" || !reflect.DeepEqual(actual.ExecutedLevels, textComparisonTiers) {
		t.Fatalf("complete original local tiers did not pass: %+v", actual)
	}
	manifest := &EvidenceManifest{SchemaVersion: "1", ImplementationSHA: runner.implSHA, Records: runner.records}
	report, err := ValidateEvidenceManifest(manifest, selected, "local")
	if err != nil || report == nil || report.TotalCases != 1 || report.PassedCases != 1 || report.FailedCases != 0 || report.SkippedCases != 0 {
		t.Fatalf("N03 original local tiers rejected or expanded: %+v %v", report, err)
	}
	var original EvidenceRecord
	for _, record := range runner.records {
		if err := validateTextComparisonRecord(textComparisonRequirement, record); err != nil {
			t.Fatal(err)
		}
		if record.Assertions["apple_runtime_executed"] != "false" {
			t.Fatal("local graph was relabelled as an Apple execution")
		}
		if record.Tier == "unit" {
			original = record
		}
	}
	if original.TestID != textComparisonProducer("unit") {
		t.Fatal("actual unit record missing")
	}
	if err := validateTextComparisonRecord(textComparisonRequirement, editorContractCopyRecord(t, original)); err != nil {
		t.Fatal("unchanged relocated original rejected", err)
	}
	faults := []string{
		"wrong_tier", "wrong_requirement", "borrowed_producer", "cross_tier_producer", "partial_command",
		"partial_event_assertion", "runtime_claim", "missing_artifact", "extra_artifact",
		"dirty_before", "dirty_after", "wrong_source_commit", "wrong_source_digest", "missing_migration_source",
		"missing_child", "skipped_child", "duplicate_child", "foreign_package", "events_outside_process",
	}
	for _, fault := range faults {
		t.Run(fault, func(t *testing.T) {
			broken := editorContractCopyRecord(t, original)
			switch fault {
			case "wrong_tier":
				broken.Tier = "ios-runtime"
			case "wrong_requirement":
				broken.Requirements = []string{"RP33"}
			case "borrowed_producer":
				broken.TestID = "language-acceptance:runNumbers"
			case "cross_tier_producer":
				broken.TestID = textComparisonProducer("native-structure")
			case "partial_command":
				broken.Command = productionTestCommand(textComparisonEvidence.Tests[:len(textComparisonEvidence.Tests)-1])
			case "partial_event_assertion":
				broken.Assertions["required_go_events"] = strings.Join(textComparisonEvidence.Events[:len(textComparisonEvidence.Events)-1], "\n")
			case "runtime_claim":
				broken.Assertions["apple_runtime_executed"] = "true"
			case "missing_artifact":
				broken.Artifacts = broken.Artifacts[:2]
			case "extra_artifact":
				broken.Artifacts = append(broken.Artifacts, broken.Artifacts[0])
			case "dirty_before", "dirty_after", "wrong_source_commit", "wrong_source_digest", "missing_migration_source":
				editorContractMutateArtifact(t, &broken, textComparisonPrefix+"-process.json", func(raw []byte) []byte {
					var receipt editorProcessReceipt
					if err := json.Unmarshal(raw, &receipt); err != nil {
						t.Fatal(err)
					}
					switch fault {
					case "dirty_before":
						receipt.SourceCleanBefore = false
					case "dirty_after":
						receipt.SourceCleanAfter = false
					case "wrong_source_commit":
						receipt.SourceCommit = strings.Repeat("f", 40)
					case "wrong_source_digest":
						receipt.SourceFiles[0].SHA256 = strings.Repeat("f", 64)
					case "missing_migration_source":
						before := len(receipt.SourceFiles)
						var retained []editorSourceFile
						for _, file := range receipt.SourceFiles {
							if file.Path != "internal/language/migrate/migrate.go" {
								retained = append(retained, file)
							}
						}
						if len(retained) != before-1 {
							t.Fatal("control did not remove the actual migration dispatcher")
						}
						receipt.SourceFiles = retained
					}
					out, err := json.Marshal(receipt)
					if err != nil {
						t.Fatal(err)
					}
					return out
				})
			default:
				target := textComparisonNativeTest + "/legacy_subject"
				var changed []byte
				editorContractMutateArtifact(t, &broken, textComparisonPrefix+"-go.jsonl", func(raw []byte) []byte {
					var output bytes.Buffer
					for _, line := range bytes.Split(raw, []byte{'\n'}) {
						if len(bytes.TrimSpace(line)) == 0 {
							continue
						}
						var event map[string]any
						if err := json.Unmarshal(line, &event); err != nil {
							t.Fatal(err)
						}
						if fault == "events_outside_process" {
							value, okay := event["Time"].(string)
							when, err := time.Parse(time.RFC3339Nano, value)
							if !okay || err != nil {
								t.Fatal("actual event timestamp missing", err)
							}
							event["Time"] = when.Add(-24 * time.Hour).Format(time.RFC3339Nano)
						}
						if event["Test"] == target {
							if fault == "missing_child" {
								continue
							}
							if fault == "skipped_child" && event["Action"] == "pass" {
								event["Action"] = "skip"
							}
							if fault == "foreign_package" {
								event["Package"] = "foreign/package"
							}
						}
						data, err := json.Marshal(event)
						if err != nil {
							t.Fatal(err)
						}
						output.Write(data)
						output.WriteByte('\n')
						if fault == "duplicate_child" && event["Test"] == target && event["Action"] == "pass" {
							output.Write(data)
							output.WriteByte('\n')
						}
					}
					changed = output.Bytes()
					if bytes.Equal(changed, raw) {
						t.Fatal("control did not change the original event stream")
					}
					return changed
				})
				editorContractMutateArtifact(t, &broken, textComparisonPrefix+"-process.json", func(raw []byte) []byte {
					var receipt editorProcessReceipt
					if err := json.Unmarshal(raw, &receipt); err != nil {
						t.Fatal(err)
					}
					receipt.RawSHA256, receipt.RawBytes = editorSHA(changed), len(changed)
					data, err := json.Marshal(receipt)
					if err != nil {
						t.Fatal(err)
					}
					return data
				})
			}
			err := validateTextComparisonRecord(textComparisonRequirement, broken)
			if err == nil {
				t.Fatal("accepted detached/incomplete N03 evidence", fault)
			}
			if fault == "missing_migration_source" && !strings.Contains(err.Error(), "source manifest mismatch") {
				t.Fatal("missing source control failed before its intended immutable source gate", err)
			}
			if fault == "events_outside_process" && !strings.Contains(err.Error(), "outside the observed process interval") {
				t.Fatal("chronology control failed before its intended process-time join", err)
			}
		})
	}

	// Final phase retains the pre-existing global successful-workflow gates.
	// N03's local required tiers may pass without granting global release closure.
	final, finalErr := ValidateEvidenceManifest(manifest, selected, "final")
	if finalErr == nil || final == nil || final.ClosureComplete || final.PassedCases != 1 ||
		!strings.Contains(finalErr.Error(), "missing required successful CI workflow") {
		t.Fatal("N03 local proof bypassed or acquired global CI authority", final, finalErr)
	}
	auditPath := filepath.Join(t.TempDir(), "n03-audit.json")
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
	if err := json.Unmarshal(raw, &audit); err != nil {
		t.Fatal(err)
	}
	if len(audit.Rows) != 179 || audit.ClosureComplete {
		t.Fatal("N03 changed immutable full-contract accounting")
	}
	for _, row := range audit.Rows {
		if row.ID == textComparisonRequirement {
			if row.Status != "PASS" || !row.PositiveAndNegative || !reflect.DeepEqual(row.TestNames, textComparisonEvidence.Tests) ||
				!reflect.DeepEqual(row.ProductionPath, textComparisonEvidence.SourcePaths) || row.Scope != textComparisonScope {
				t.Fatal("actual complete N03 scope lost from execution audit", row)
			}
		} else if row.Status == "PASS" || row.PositiveAndNegative {
			t.Fatal("related or unexecuted requirement inherited N03 proof", row.ID)
		}
	}
	if runner.artifactDir != "" {
		proof, err := json.MarshalIndent(map[string]any{
			"source": runner.implSHA, "report": report, "results": runner.results, "records": runner.records,
			"formal_requirement": "N03", "apple_runtime_executed": false, "global_closure_granted": false,
		}, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(runner.artifactDir, "n03-complete-local-evidence.json"), append(proof, '\n'), 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("Actual source %s: complete N03 unit/native-structure selection passed; all 179 audit rows retained, no Apple-runtime or related-requirement credit", runner.implSHA)
}
