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

func combinedValueBlockSelectedContracts(t *testing.T, ids ...string) (*ContractsSet, *ContractsSet) {
	t.Helper()
	if len(ids) == 0 {
		ids = []string{"RP23", "BRG14"}
	}
	wanted := map[string]bool{}
	for _, id := range ids {
		if wanted[id] {
			t.Fatal("duplicate selected contract")
		}
		wanted[id] = true
	}
	all, err := LoadAllContracts("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	selected := &ContractsSet{Acceptance: &ContractFile{}, Repair: &RepairContractFile{}, Gates: &GatesContractFile{}}
	for _, row := range all.Repair.Cases {
		if wanted[row.ID] {
			selected.Repair.Cases = append(selected.Repair.Cases, row)
		}
	}
	for _, row := range all.Gates.Gates {
		if wanted[row.ID] {
			selected.Gates.Gates = append(selected.Gates.Gates, row)
		}
	}
	selected.Repair.CaseCount = len(selected.Repair.Cases)
	selected.Gates.GateCount = len(selected.Gates.Gates)
	if selected.Repair.CaseCount+selected.Gates.GateCount != len(ids) {
		t.Fatal("missing distinct immutable contracts")
	}
	return all, selected
}
func TestCombinedValueBlockInventoryAndUnprovedRuntime(t *testing.T) {
	if err := validateCombinedValueBlockSourceInventory(combinedValueBlockEvidence.SourcePaths); err != nil {
		t.Fatal(err)
	}
	t.Run("source_count_before_launch", func(t *testing.T) {
		err := validateCombinedValueBlockSourceInventory(make([]string, 129))
		if err == nil || !strings.Contains(err.Error(), "invalid bounded") {
			t.Fatal("oversized source inventory did not reach the prelaunch size gate", err)
		}
	})
	t.Run("required_source_before_launch", func(t *testing.T) {
		var missing []string
		for _, path := range combinedValueBlockEvidence.SourcePaths {
			if path != "p15_combined_value_blocks_test.go" {
				missing = append(missing, path)
			}
		}
		if len(missing) != len(combinedValueBlockEvidence.SourcePaths)-1 {
			t.Fatal("required-source control did not remove the actual oracle")
		}
		err := validateCombinedValueBlockSourceInventory(missing)
		if err == nil || !strings.Contains(err.Error(), "reviewed source witnesses") {
			t.Fatal("removed required source did not reach the prelaunch witness gate", err)
		}
	})
	_, selected := combinedValueBlockSelectedContracts(t, "RP23", "BRG14")
	for _, id := range []string{"RP23", "BRG14"} {
		spec := combinedValueBlockScenarios[id]
		var title, group string
		var tiers []string
		for _, row := range selected.Repair.Cases {
			if row.ID == id {
				title = row.Title
				group = "repair"
				tiers = row.RequiredTestLevels
			}
		}
		for _, row := range selected.Gates.Gates {
			if row.ID == id {
				title = row.Title
				group = "recovery_gate"
				tiers = row.MinimumEvidenceTiers
			}
		}
		if spec.Title != title || spec.Group != group || !reflect.DeepEqual(parseRequiredTiers(strings.Join(tiers, "+")), spec.Required) {
			t.Fatal("changed immutable combined requirement")
		}
		expected := map[string]int{"RP23": 57, "BRG14": 96}[id]
		events := combinedValueBlockEvidence.Events[id]
		if len(events) != expected || !sort.StringsAreSorted(events) {
			t.Fatal("fixed causal child inventory changed")
		}
		seen := map[string]bool{}
		for _, event := range events {
			if seen[event] || !(event == spec.Unit || event == spec.Native || strings.HasPrefix(event, spec.Unit+"/") || strings.HasPrefix(event, spec.Native+"/")) {
				t.Fatal("duplicate or foreign combined event")
			}
			seen[event] = true
		}
		for _, tier := range []string{"ios-runtime", "ios-ui", "ios-build"} {
			if validateCombinedValueBlockRecord(id, EvidenceRecord{Requirements: []string{id}, Tier: tier, TestID: "ios27-runtime-poc:unicode", Result: "passed"}) == nil {
				t.Fatal("incomplete historical trace supplied combined Apple tier")
			}
		}
		runner := NewRunner()
		if !runner.runCombinedValueBlockRequirement(id, group, title, []string{"unit"}) || len(runner.records) != 0 || runner.results[id].Status != "FAILED" {
			t.Fatal("weaker required tiers accepted")
		}
	}
	runner := NewRunner()
	if runner.runCombinedValueBlockRequirement("F03", "binding", "related only", []string{"unit"}) || len(runner.results) != 0 {
		t.Fatal("related requirement inherited joined value-block proof")
	}
	all, err := LoadAllContracts("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	mapping, err := BuildRequirementsEvidenceMap(all)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"RP23", "BRG14"} {
		if !reflect.DeepEqual(mapping.Mappings[id].TestIDs, combinedValueBlockScenarios[id].testIDs()) {
			t.Fatal("combined scenario inherited a related or old runtime producer", id)
		}
		if len(mapping.Mappings[id].RequiredAssertions) != 0 {
			t.Fatal("historical Unicode trace still supplies combined assertions", id)
		}
	}
	for _, path := range combinedValueBlockEvidence.SourcePaths {
		if _, err := os.Stat(resolveRepoPath(path)); err != nil {
			t.Fatal(err)
		}
	}
}
func TestCombinedValueBlockActualProductionEvidence(t *testing.T) {
	combinedValueBlockActualProductionEvidence(t, []string{"RP23", "BRG14"})
}
func combinedValueBlockActualProductionEvidence(t *testing.T, ids []string) {
	t.Helper()
	all, selected := combinedValueBlockSelectedContracts(t, ids...)
	runner := NewRunner()
	runner.artifactDir = os.Getenv("CHERRI_COMBINED_VALUE_BLOCK_EVIDENCE_OUTPUT")
	runner.runRepairCases(selected.Repair)
	runner.runRecoveryGates(selected.Gates)
	if len(runner.records) != 2*len(ids) || len(runner.results) != len(ids) || len(runner.observations) != 0 {
		t.Fatalf("incomplete independent producers: %+v records=%d", runner.results, len(runner.records))
	}
	for id, result := range runner.results {
		var wantPassed = id == "RP23"
		var wantStatus = "PENDING_EXTERNAL"
		if wantPassed {
			wantStatus = "PASSED"
		}
		if result.Passed != wantPassed || result.Status != wantStatus || !reflect.DeepEqual(result.ExecutedLevels, combinedValueBlockLocalTiers) {
			t.Fatalf("distinct local scope changed %s: %+v", id, result)
		}
	}
	manifest := &EvidenceManifest{SchemaVersion: "1", ImplementationSHA: runner.implSHA, Records: runner.records}
	report, err := ValidateEvidenceManifest(manifest, selected, "local")
	if err != nil || report == nil || report.PassedCases != 1 || report.FailedCases != 0 || report.SkippedCases != 1 || report.ClosureComplete {
		t.Fatalf("local combined proof expanded/rejected: %+v %v", report, err)
	}
	for _, original := range runner.records {
		id := original.Requirements[0]
		if err := validateCombinedValueBlockRecord(id, original); err != nil {
			t.Fatal(err)
		}
		if original.Tier != "unit" {
			continue
		}
		relocated := editorContractCopyRecord(t, original)
		if err := validateCombinedValueBlockRecord(id, relocated); err != nil {
			t.Fatal("unchanged relocated proof rejected", err)
		}
		for _, fault := range []string{"wrong_tier", "wrong_requirement", "cross_tier_producer", "partial_command", "missing_artifact", "dirty_source", "wrong_source_digest", "missing_source_file", "missing_child", "skipped_child", "foreign_package"} {
			t.Run(id+"/"+fault, func(t *testing.T) {
				broken := editorContractCopyRecord(t, original)
				prefix := combinedValueBlockPrefix(id)
				switch fault {
				case "wrong_tier":
					broken.Tier = "ios-runtime"
				case "wrong_requirement":
					broken.Requirements = []string{"F03"}
				case "cross_tier_producer":
					broken.TestID = combinedValueBlockScenarios[id].producer("native-structure")
				case "partial_command":
					broken.Command = productionTestCommand(combinedValueBlockScenarios[id].tests()[:1])
				case "missing_artifact":
					broken.Artifacts = broken.Artifacts[:2]
				case "dirty_source", "wrong_source_digest", "missing_source_file":
					editorContractMutateArtifact(t, &broken, prefix+"-process.json", func(raw []byte) []byte {
						var receipt editorProcessReceipt
						json.Unmarshal(raw, &receipt)
						if fault == "dirty_source" {
							receipt.SourceCleanAfter = false
						} else if fault == "missing_source_file" {
							originalCount := len(receipt.SourceFiles)
							var retained []editorSourceFile
							for _, file := range receipt.SourceFiles {
								if file.Path != "p15_combined_value_blocks_test.go" {
									retained = append(retained, file)
								}
							}
							if len(retained) != originalCount-1 {
								t.Fatal("missing-source receipt control did not remove the oracle")
							}
							receipt.SourceFiles = retained
						} else {
							receipt.SourceFiles[0].SHA256 = strings.Repeat("f", 64)
						}
						out, _ := json.Marshal(receipt)
						return out
					})
				default:
					target := combinedValueBlockEvidence.Events[id][1]
					var changed []byte
					editorContractMutateArtifact(t, &broken, prefix+"-go.jsonl", func(raw []byte) []byte {
						var result bytes.Buffer
						for _, line := range bytes.Split(raw, []byte{'\n'}) {
							if len(bytes.TrimSpace(line)) == 0 {
								continue
							}
							var event map[string]any
							if err := json.Unmarshal(line, &event); err != nil {
								t.Fatal(err)
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
							data, _ := json.Marshal(event)
							result.Write(data)
							result.WriteByte('\n')
						}
						changed = result.Bytes()
						if bytes.Equal(changed, raw) {
							t.Fatal("control did not change raw events")
						}
						return changed
					})
					editorContractMutateArtifact(t, &broken, prefix+"-process.json", func(raw []byte) []byte {
						var receipt editorProcessReceipt
						json.Unmarshal(raw, &receipt)
						receipt.RawSHA256 = editorSHA(changed)
						receipt.RawBytes = len(changed)
						data, _ := json.Marshal(receipt)
						return data
					})
				}
				err := validateCombinedValueBlockRecord(id, broken)
				if err == nil {
					t.Fatalf("accepted combined evidence mutation %s", fault)
				}
				if fault == "missing_source_file" && !strings.Contains(err.Error(), "source manifest mismatch") {
					t.Fatal("missing-source receipt control failed before the intended source gate", err)
				}
			})
		}
	}
	final, finalErr := ValidateEvidenceManifest(manifest, selected, "final")
	if finalErr == nil || final == nil || final.ClosureComplete {
		t.Fatal("local combined proof closed final release")
	}
	auditPath := filepath.Join(t.TempDir(), "audit.json")
	if err = writeExecutionAudit(runner, all, auditPath); err != nil {
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
	if err = json.Unmarshal(raw, &audit); err != nil {
		t.Fatal(err)
	}
	if len(audit.Rows) != 179 || audit.ClosureComplete {
		t.Fatal("combined producer changed immutable accounting")
	}
	for _, row := range audit.Rows {
		if row.Status == "PASS" && row.ID != "RP23" {
			t.Fatal("unproved combined or related requirement received full acceptance", row.ID)
		}
		if _, executed := runner.results[row.ID]; executed {
			if !row.PositiveAndNegative || !reflect.DeepEqual(row.TestNames, combinedValueBlockScenarios[row.ID].tests()) {
				t.Fatal("actual combined scope lost")
			}
		} else if row.PositiveAndNegative {
			t.Fatal("related requirement inherited combined evidence", row.ID)
		}
	}
	proof, _ := json.MarshalIndent(map[string]any{"source": runner.implSHA, "report": report, "results": runner.results, "records": runner.records}, "", "  ")
	if runner.artifactDir != "" {
		if err = os.WriteFile(filepath.Join(runner.artifactDir, "combined-value-block-evidence.json"), append(proof, '\n'), 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("Distinct %v actual local execution on %s; four unit/native records, RP23 local-only contract PASS, BRG14 pending Apple runtime; no F04 or related-ID inheritance", ids, runner.implSHA)
}

func TestCombinedValueBlockIncompleteF04Scope(t *testing.T) {
	var all, err = LoadAllContracts("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	var found = false
	for _, row := range all.Acceptance.Cases {
		if row.ID != "F04" {
			continue
		}
		found = true
		var spec = combinedValueBlockScenarios["F04"]
		if spec.Group != row.Group || spec.Title != row.Requirement || !reflect.DeepEqual(parseRequiredTiers(row.MinimumTestLevel), spec.Required) {
			t.Fatal("F04 immutable clause changed")
		}
	}
	if !found {
		t.Fatal("missing F04 contract")
	}
	var events = combinedValueBlockEvidence.Events["F04"]
	if len(events) != 46 || !sort.StringsAreSorted(events) {
		t.Fatal("changed fixed F04 observation inventory")
	}
	var seen = map[string]bool{}
	for _, event := range events {
		if seen[event] || !(event == combinedValueBlockScenarios["F04"].Unit || event == combinedValueBlockScenarios["F04"].Native || strings.HasPrefix(event, combinedValueBlockScenarios["F04"].Unit+"/") || strings.HasPrefix(event, combinedValueBlockScenarios["F04"].Native+"/")) {
			t.Fatal("duplicate or foreign F04 observation event")
		}
		seen[event] = true
	}
	var mapping, mapErr = BuildRequirementsEvidenceMap(all)
	if mapErr != nil {
		t.Fatal(mapErr)
	}
	if !reflect.DeepEqual(mapping.Mappings["F04"].TestIDs, combinedValueBlockScenarios["F04"].testIDs()) || len(mapping.Mappings["F04"].RequiredAssertions) != 0 {
		t.Fatal("F04 inherited historical loop/runtime assertion")
	}
	for _, tier := range []string{"unit", "native-structure", "ios-runtime", "ios-ui"} {
		var record = EvidenceRecord{Requirements: []string{"F04"}, Tier: tier, TestID: combinedValueBlockScenarios["F04"].producer(tier), Result: "passed"}
		if err = validateCombinedValueBlockRecord("F04", record); err == nil || !strings.Contains(err.Error(), "nested List and Optional") {
			t.Fatal("bounded facets granted complete F04 credit", err)
		}
	}
}
func TestCombinedValueBlockF04ActualObservation(t *testing.T) {
	var runner = NewRunner()
	runner.artifactDir = os.Getenv("CHERRI_COMBINED_VALUE_BLOCK_EVIDENCE_OUTPUT")
	var spec = combinedValueBlockScenarios["F04"]
	if !runner.runCombinedValueBlockRequirement("F04", spec.Group, spec.Title, spec.Required) {
		t.Fatal("F04 observer absent")
	}
	var result = runner.results["F04"]
	if result.Passed || result.Status != "NOT_RUN" || len(result.ExecutedLevels) != 0 || len(runner.records) != 0 || len(runner.observations) != 1 || !runner.observations[0].Passed {
		t.Fatalf("F04 supported facets failed or acquired full credit: %+v observations=%+v", result, runner.observations)
	}
	var observed combinedValueBlockFacetObservation
	if err := json.Unmarshal([]byte(runner.observations[0].Detail), &observed); err != nil {
		t.Fatal(err)
	}
	if observed.Schema != "cherri.value-loop-facets/v1" || observed.ImplementationSHA != runner.implSHA || observed.Requirement != "F04" || observed.Result != "DIAGNOSTIC_OBSERVATION" ||
		observed.CompleteRequirementExecuted || observed.FormalAcceptanceGranted || observed.Scope != spec.Scope || !reflect.DeepEqual(observed.Tests, spec.tests()) || runner.observations[0].CoverageGap != combinedValueBlockF04Gap {
		t.Fatal("unbound F04 subset observation")
	}
	if err := validateCombinedValueBlockArtifacts("F04", runner.implSHA, observed.Artifacts); err != nil {
		t.Fatal(err)
	}
	for _, tier := range combinedValueBlockLocalTiers {
		t.Run("actual_subset_cannot_supply_"+tier, func(t *testing.T) {
			var forged = EvidenceRecord{Requirements: []string{"F04"}, Tier: tier, ImplementationSHA: runner.implSHA, TestID: spec.producer(tier),
				Result: "passed", Command: productionTestCommand(spec.tests()), Artifacts: observed.Artifacts,
				Assertions: map[string]string{"scope": spec.Scope, "required_production_tests": strings.Join(spec.tests(), "\n"), "required_go_events": strings.Join(combinedValueBlockEvidence.Events["F04"], "\n")}}
			if err := validateCombinedValueBlockRecord("F04", forged); err == nil || !strings.Contains(err.Error(), "complete F04 scenario is NOT_RUN") {
				t.Fatal("actual subset upgraded to complete F04", err)
			}
		})
	}
	if runner.artifactDir != "" {
		var raw, err = json.MarshalIndent(map[string]any{"source": runner.implSHA, "result": result, "observation": observed}, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(runner.artifactDir, "combined-value-block-f04-observation.json"), append(raw, '\n'), 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("F04 actual supported semantic/native facets on %s verified, 0 acceptance records, complete requirement NOT_RUN: %s", runner.implSHA, combinedValueBlockF04Gap)
}
