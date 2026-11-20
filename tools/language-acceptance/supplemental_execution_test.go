package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Keep an explicit set of implemented producers, but derive record counts and
// required tiers from the immutable contracts. Adding a real producer must add
// its ID here; removing one cannot silently shrink the verification scope.
func p15SupplementalSelection(t *testing.T, all *ContractsSet) (*ContractsSet, int) {
	t.Helper()
	var expectedIDs = strings.Fields(`
RP01 RP02 RP03 RP04 RP05 RP09 RP10 RP13 RP27 RP28 RP30 RP31 RP32 RP34 RP35 RP36 RP37 RP38
BRG02 BRG03 BRG04 BRG05 BRG07 BRG09 BRG17 BRG20 BRG32
`)
	if len(supplementalProductionScenarios) != len(expectedIDs) {
		t.Fatal("supplemental producer inventory changed without explicit accounting")
	}
	for _, id := range expectedIDs {
		if _, found := supplementalProductionScenarios[id]; !found {
			t.Fatalf("required supplemental producer %s is absent", id)
		}
	}
	var selected = &ContractsSet{Acceptance: &ContractFile{}, Repair: &RepairContractFile{}, Gates: &GatesContractFile{}}
	var requiredRecords int
	for _, scenario := range all.Repair.Cases {
		if producer, found := supplementalProductionScenarios[scenario.ID]; found {
			var tiers = parseRequiredTiers(strings.Join(scenario.RequiredTestLevels, "+"))
			if !reflect.DeepEqual(producer.Tiers, tiers) {
				t.Fatalf("%s producer tiers differ from immutable repair contract: got %v want %v", scenario.ID, producer.Tiers, tiers)
			}
			selected.Repair.Cases = append(selected.Repair.Cases, scenario)
			requiredRecords += len(tiers)
		}
	}
	for _, scenario := range all.Gates.Gates {
		if producer, found := supplementalProductionScenarios[scenario.ID]; found {
			var tiers = parseRequiredTiers(strings.Join(scenario.MinimumEvidenceTiers, "+"))
			if !reflect.DeepEqual(producer.Tiers, tiers) {
				t.Fatalf("%s producer tiers differ from immutable recovery contract: got %v want %v", scenario.ID, producer.Tiers, tiers)
			}
			selected.Gates.Gates = append(selected.Gates.Gates, scenario)
			requiredRecords += len(tiers)
		}
	}
	selected.Repair.CaseCount = len(selected.Repair.Cases)
	selected.Gates.GateCount = len(selected.Gates.Gates)
	if selected.Repair.CaseCount+selected.Gates.GateCount != len(expectedIDs) {
		t.Fatal("supplemental producer is not an immutable repair/recovery ID")
	}
	return selected, requiredRecords
}

func TestSupplementalProductionScenarioInventory(t *testing.T) {
	var all, err = LoadAllContracts("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	var selected, _ = p15SupplementalSelection(t, all)
	_ = p15AppendAssetAuthorizationSelection(t, all, selected)
	_ = p15AppendAIEvaluationPartialSelection(t, all, selected)
}

func TestSupplementalAcceptanceProductionEvidence(t *testing.T) {
	var all, err = LoadAllContracts("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	var selected, requiredRecords = p15SupplementalSelection(t, all)
	requiredRecords += p15AppendAssetAuthorizationSelection(t, all, selected)
	var requiredCases = selected.Repair.CaseCount + selected.Gates.GateCount
	requiredRecords += p15AppendAIEvaluationPartialSelection(t, all, selected)

	var runner = NewRunner()
	runner.runRepairCases(selected.Repair)
	runner.runRecoveryGates(selected.Gates)
	if len(runner.results) != selected.Repair.CaseCount+selected.Gates.GateCount || len(runner.records) != requiredRecords || len(runner.observations) != 0 {
		t.Fatalf("unexpected supplemental scope: results=%d records=%d observations=%d",
			len(runner.results), len(runner.records), len(runner.observations))
	}
	for id, result := range runner.results {
		if scenario, found := supplementalProductionScenarios[id]; found {
			if !result.Passed || result.Status != "PASSED" ||
				!reflect.DeepEqual(result.ExecutedLevels, scenario.Tiers) {
				t.Fatalf("%s complete supplemental scenario did not pass: %+v", id, result)
			}
		} else if id == "RP29" {
			if !result.Passed || result.Status != "PASSED" || !reflect.DeepEqual(result.ExecutedLevels, assetAuthorizationScenario.Tiers) {
				t.Fatalf("asset authorization lost actual security/service/benchmark proof: %+v", result)
			}
		} else if id == "RP50" {
			if result.Passed || result.Status != "PENDING_EXTERNAL" || !reflect.DeepEqual(result.ExecutedLevels, aiEvaluationHarnessScenario.Tiers) {
				t.Fatalf("partial AI controls lost the unexecuted evaluation tier: %+v", result)
			}
		} else if result.Passed || result.Status != "NOT_RUN" || len(result.ExecutedLevels) != 0 {
			t.Fatalf("unexecuted supplemental scenario inherited a pass: %+v", result)
		}
	}
	for _, record := range runner.records {
		if len(record.Requirements) != 1 || record.Result != "passed" || record.ExitCode != 0 ||
			record.ImplementationSHA != runner.implSHA {
			t.Fatalf("invalid production receipt: %+v", record)
		}
		var id = record.Requirements[0]
		var scenario = supplementalProductionScenarios[id]
		if id == "RP29" {
			scenario = assetAuthorizationScenario
			if err := validateEditorPerformanceRecord(id, record); err != nil {
				t.Fatal(err)
			}
		}
		if id == "RP50" {
			scenario = aiEvaluationHarnessScenario
			if err := validateAIEvaluationHarnessRecord(id, record); err != nil {
				t.Fatal(err)
			}
		}
		if err := validateSupplementalProductionRecord(id, record); err != nil {
			t.Fatal(err)
		}
		for _, name := range scenario.TestNames {
			if !strings.Contains(record.Detail, name) {
				t.Fatalf("production discovery receipt missing %s: %+v", name, record)
			}
		}
	}
	var manifest = &EvidenceManifest{SchemaVersion: "1", ImplementationSHA: runner.implSHA, Records: runner.records}
	var report, reportErr = ValidateEvidenceManifest(manifest, selected, "local")
	if reportErr != nil || report.PassedCases != requiredCases || report.FailedCases != 0 ||
		report.SkippedCases != 1 || report.ClosureComplete {
		t.Fatalf("supplemental evidence rejected or expanded into release closure: %+v err=%v", report, reportErr)
	}

	var intakeRecordIndex = -1
	for index, record := range manifest.Records {
		if record.Requirements[0] == "RP36" {
			intakeRecordIndex = index
			break
		}
	}
	if intakeRecordIndex < 0 {
		t.Fatal("independent intake receipt missing")
	}

	// A producer selection, its complete required tiers, and its source SHA are
	// inseparable. Neither a linked test ID nor one positive intake case suffices.
	for _, mutation := range []struct {
		name  string
		apply func(*EvidenceManifest)
	}{
		{"missing_native_tier", func(m *EvidenceManifest) {
			var records []EvidenceRecord
			for _, record := range m.Records {
				if record.Requirements[0] != "BRG03" || record.Tier != "native-structure" {
					records = append(records, record)
				}
			}
			m.Records = records
		}},
		{"wrong_producer", func(m *EvidenceManifest) {
			m.Records[intakeRecordIndex].TestID = "github.com/electrikmilk/cherri:TestAlphaRenamingPasses"
		}},
		{"partial_intake_selection", func(m *EvidenceManifest) {
			m.Records[intakeRecordIndex].Command = productionTestCommand([]string{"TestP15R06ProductionImportFormats"})
		}},
		{"zero_selection_command", func(m *EvidenceManifest) {
			m.Records[intakeRecordIndex].Command = productionTestCommand([]string{"TestSupplementalMissingProducer"})
		}},
		{"missing_selection_assertion", func(m *EvidenceManifest) {
			delete(m.Records[intakeRecordIndex].Assertions, "required_production_tests")
		}},
		{"stale_source", func(m *EvidenceManifest) {
			m.Records[intakeRecordIndex].ImplementationSHA = strings.Repeat("f", 40)
		}},
		{"invented_runtime_tier", func(m *EvidenceManifest) {
			m.Records[intakeRecordIndex].Tier = "ios-runtime"
		}},
	} {
		t.Run(mutation.name, func(t *testing.T) {
			var encoded, err = json.Marshal(manifest)
			if err != nil {
				t.Fatal(err)
			}
			var changed EvidenceManifest
			if err := json.Unmarshal(encoded, &changed); err != nil {
				t.Fatal(err)
			}
			mutation.apply(&changed)
			if _, err := ValidateEvidenceManifest(&changed, selected, "local"); err == nil {
				t.Fatal("invalid supplemental receipt was accepted")
			}
		})
	}

	var auditPath = filepath.Join(t.TempDir(), "execution-audit.json")
	if err := writeExecutionAudit(runner, all, auditPath); err != nil {
		t.Fatal(err)
	}
	var data, readErr = os.ReadFile(auditPath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	var audit struct {
		ClosureComplete bool                `json:"closure_complete"`
		Rows            []ExecutionAuditRow `json:"rows"`
	}
	if err := json.Unmarshal(data, &audit); err != nil {
		t.Fatal(err)
	}
	if audit.ClosureComplete || len(audit.Rows) != 179 {
		t.Fatal("bounded execution changed the 179-obligation accounting")
	}
	for _, row := range audit.Rows {
		if scenario, found := supplementalProductionScenarios[row.ID]; found {
			if row.Status != "PASS" || !row.PositiveAndNegative ||
				!reflect.DeepEqual(row.TestNames, scenario.TestNames) ||
				!reflect.DeepEqual(row.ProductionPath, scenario.ProductionPaths) {
				t.Fatalf("audit lost the actual supplemental producer: %+v", row)
			}
		} else if row.ID == "RP29" {
			if row.Status != "PASS" || !row.PositiveAndNegative ||
				!reflect.DeepEqual(row.Tier, assetAuthorizationScenario.Tiers) ||
				!reflect.DeepEqual(row.TestNames, assetAuthorizationScenario.TestNames) ||
				!reflect.DeepEqual(row.ProductionPath, assetAuthorizationScenario.ProductionPaths) {
				t.Fatalf("audit lost actual asset authorization producer: %+v", row)
			}
		} else if row.ID == "RP50" {
			if row.Status != "BLOCKED_EXTERNAL" || row.PositiveAndNegative ||
				!reflect.DeepEqual(row.Tier, aiEvaluationHarnessScenario.Tiers) ||
				!reflect.DeepEqual(row.TestNames, aiEvaluationHarnessScenario.TestNames) {
				t.Fatalf("audit promoted partial AI controls: %+v", row)
			}
		} else if row.Status == "PASS" || row.PositiveAndNegative {
			t.Fatalf("unexecuted obligation inherited supplemental success: %+v", row)
		}
	}

	var status, statusErr = exec.Command("git", "-C", resolveRepoRoot(), "status", "--porcelain", "--untracked-files=normal").Output()
	var receipt, receiptErr = json.Marshal(map[string]any{
		"source_sha": runner.implSHA, "source_tree_clean": statusErr == nil && len(status) == 0,
		"scope":  "Explicit supplemental producer inventory with immutable contract tiers, including RP04 checked compilation and RP34 actual failed/restored migration; RP01/RP02/RP03/BRG04 actual verification contracts; BRG02/BRG03 strict comparator; BRG07 direct reference chains; RP13 named variadic lists; RP35 native preservation; RP36 intake; RP37 allowed-edit and failed-signing gates. No linked-ID inheritance, live signing success, or release closure.",
		"report": report, "records": runner.records,
	})
	if receiptErr != nil {
		t.Fatal(receiptErr)
	}
	t.Log("P15_SUPPLEMENTAL_RECEIPT:" + string(receipt))
}

func TestSupplementalAcceptanceMissingProducerFailsClosed(t *testing.T) {
	for _, required := range [][]string{
		nil,
		{"TestSupplementalMissingProducer"},
		{"TestP15BRG02ReferenceTopologyProduction", "TestSupplementalMissingProducer"},
	} {
		var passed, detail = executeNamedProductionTestsInPackage("./internal/shortcutcompare", required)
		if passed {
			t.Fatalf("missing discovery produced a pass: %q %s", required, detail)
		}
		var runner = NewRunner()
		runner.Record("BRG02", "recovery_gate", "negative discovery control",
			"unit+native-structure", []string{"unit", "native-structure"}, passed, "PASSED", detail)
		if result := runner.results["BRG02"]; result.Passed || result.Status != "FAILED" {
			t.Fatalf("missing discovery was promoted to acceptance: %+v", result)
		}
		for _, record := range runner.records {
			if record.Result != "failed" || record.ExitCode == 0 {
				t.Fatalf("missing discovery created a successful receipt: %+v", record)
			}
		}
	}
}

func TestSupplementalAcceptanceCannotChangeRequiredTiers(t *testing.T) {
	var runner = NewRunner()
	if runner.runSupplementalProduction("BRG03", "recovery_gate", "incomplete claim", []string{"unit"}) {
		t.Fatal("a weakened contract was executed as the complete supplemental requirement")
	}
	if len(runner.records) != 0 || len(runner.results) != 0 {
		t.Fatal("unexecuted altered contract minted evidence")
	}
}

func TestSupplementalVerificationProductionEvidence(t *testing.T) {
	var all, err = LoadAllContracts("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	var wanted = map[string]bool{"RP01": true, "RP02": true, "RP03": true, "BRG04": true}
	var selected = &ContractsSet{Acceptance: &ContractFile{}, Repair: &RepairContractFile{}, Gates: &GatesContractFile{}}
	for _, scenario := range all.Repair.Cases {
		if wanted[scenario.ID] {
			selected.Repair.Cases = append(selected.Repair.Cases, scenario)
		}
	}
	for _, scenario := range all.Gates.Gates {
		if wanted[scenario.ID] {
			selected.Gates.Gates = append(selected.Gates.Gates, scenario)
		}
	}
	selected.Repair.CaseCount = len(selected.Repair.Cases)
	selected.Gates.GateCount = len(selected.Gates.Gates)
	if selected.Repair.CaseCount != 3 || selected.Gates.GateCount != 1 {
		t.Fatal("independent verification scenario selection drifted")
	}

	var runner = NewRunner()
	runner.runRepairCases(selected.Repair)
	runner.runRecoveryGates(selected.Gates)
	if len(runner.results) != 4 || len(runner.records) != 7 || len(runner.observations) != 0 {
		t.Fatalf("unexpected verifier scope: results=%d records=%d observations=%d", len(runner.results), len(runner.records), len(runner.observations))
	}
	for id, result := range runner.results {
		var scenario = supplementalProductionScenarios[id]
		if !wanted[id] || !result.Passed || result.Status != "PASSED" || !reflect.DeepEqual(result.ExecutedLevels, scenario.Tiers) {
			t.Fatalf("complete verifier scenario did not pass: %+v", result)
		}
	}
	for _, record := range runner.records {
		if len(record.Requirements) != 1 || record.Result != "passed" || record.ExitCode != 0 || record.ImplementationSHA != runner.implSHA {
			t.Fatalf("invalid verifier production receipt: %+v", record)
		}
		var id = record.Requirements[0]
		if err := validateSupplementalProductionRecord(id, record); err != nil {
			t.Fatal(err)
		}
		for _, name := range supplementalProductionScenarios[id].TestNames {
			if !strings.Contains(record.Detail, name) {
				t.Fatalf("production discovery receipt missing %s: %+v", name, record)
			}
		}
	}
	var manifest = &EvidenceManifest{SchemaVersion: "1", ImplementationSHA: runner.implSHA, Records: runner.records}
	var report, reportErr = ValidateEvidenceManifest(manifest, selected, "local")
	if reportErr != nil || report.PassedCases != 4 || report.FailedCases != 0 || report.SkippedCases != 0 || report.ClosureComplete {
		t.Fatalf("independent verifier receipts rejected or expanded into release closure: %+v err=%v", report, reportErr)
	}
	var encoded, marshalErr = json.Marshal(manifest)
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}
	for _, id := range []string{"RP01", "RP02", "RP03", "BRG04"} {
		t.Run("reject_partial_producer_"+id, func(t *testing.T) {
			var changed EvidenceManifest
			if err := json.Unmarshal(encoded, &changed); err != nil {
				t.Fatal(err)
			}
			for index := range changed.Records {
				if changed.Records[index].Requirements[0] == id {
					delete(changed.Records[index].Assertions, "required_production_tests")
					break
				}
			}
			var rejected, err = ValidateEvidenceManifest(&changed, selected, "local")
			if err == nil || p15VerificationResult(t, rejected, id).Passed || rejected.ClosureComplete {
				t.Fatal("partial producer selection was accepted")
			}
		})
	}

	var auditPath = filepath.Join(t.TempDir(), "execution-audit.json")
	if err := writeExecutionAudit(runner, all, auditPath); err != nil {
		t.Fatal(err)
	}
	var data, readErr = os.ReadFile(auditPath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	var audit struct {
		ClosureComplete bool                `json:"closure_complete"`
		Rows            []ExecutionAuditRow `json:"rows"`
	}
	if err := json.Unmarshal(data, &audit); err != nil {
		t.Fatal(err)
	}
	if audit.ClosureComplete || len(audit.Rows) != 179 {
		t.Fatal("bounded verifier execution changed the original 179 obligations")
	}
	for _, row := range audit.Rows {
		if wanted[row.ID] {
			var scenario = supplementalProductionScenarios[row.ID]
			if row.Status != "PASS" || !row.PositiveAndNegative || !reflect.DeepEqual(row.TestNames, scenario.TestNames) || !reflect.DeepEqual(row.ProductionPath, scenario.ProductionPaths) {
				t.Fatalf("audit lost the actual verifier producer: %+v", row)
			}
		} else if row.Status == "PASS" || row.PositiveAndNegative {
			t.Fatalf("unexecuted obligation inherited verifier success: %+v", row)
		}
	}
	var status, statusErr = exec.Command("git", "-C", resolveRepoRoot(), "status", "--porcelain", "--untracked-files=normal").Output()
	var receipt, receiptErr = json.Marshal(map[string]any{
		"source_sha": runner.implSHA, "source_tree_clean": statusErr == nil && len(status) == 0,
		"scope":  "RP01/RP02/RP03/BRG04 real verifier, CLI and fresh-clone contract intake. Synthetic validator fixtures are not native/runtime evidence for other requirements. No release closure.",
		"report": report, "records": runner.records,
	})
	if receiptErr != nil {
		t.Fatal(receiptErr)
	}
	t.Log("P15_VERIFICATION_RECEIPT:" + string(receipt))
}

func TestSupplementalReferenceAndVariadicProductionEvidence(t *testing.T) {
	var all, err = LoadAllContracts("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	var wanted = map[string]bool{"RP13": true, "BRG07": true}
	var selected = &ContractsSet{Acceptance: &ContractFile{}, Repair: &RepairContractFile{}, Gates: &GatesContractFile{}}
	for _, scenario := range all.Repair.Cases {
		if wanted[scenario.ID] {
			selected.Repair.Cases = append(selected.Repair.Cases, scenario)
		}
	}
	for _, scenario := range all.Gates.Gates {
		if wanted[scenario.ID] {
			selected.Gates.Gates = append(selected.Gates.Gates, scenario)
		}
	}
	selected.Repair.CaseCount = len(selected.Repair.Cases)
	selected.Gates.GateCount = len(selected.Gates.Gates)
	if selected.Repair.CaseCount != 1 || selected.Gates.GateCount != 1 {
		t.Fatal("independent reference/variadic scenario selection drifted")
	}

	var runner = NewRunner()
	runner.runRepairCases(selected.Repair)
	runner.runRecoveryGates(selected.Gates)
	if len(runner.results) != 2 || len(runner.records) != 4 || len(runner.observations) != 0 {
		t.Fatalf("unexpected reference/variadic scope: results=%d records=%d observations=%d", len(runner.results), len(runner.records), len(runner.observations))
	}
	for id, result := range runner.results {
		var scenario = supplementalProductionScenarios[id]
		if !wanted[id] || !result.Passed || result.Status != "PASSED" || !reflect.DeepEqual(result.ExecutedLevels, scenario.Tiers) {
			t.Fatalf("complete reference/variadic scenario did not pass: %+v", result)
		}
	}
	for _, record := range runner.records {
		if len(record.Requirements) != 1 || record.Result != "passed" || record.ExitCode != 0 || record.ImplementationSHA != runner.implSHA {
			t.Fatalf("invalid reference/variadic production receipt: %+v", record)
		}
		var id = record.Requirements[0]
		if err := validateSupplementalProductionRecord(id, record); err != nil {
			t.Fatal(err)
		}
		for _, name := range supplementalProductionScenarios[id].TestNames {
			if !strings.Contains(record.Detail, name) {
				t.Fatalf("production discovery receipt missing %s: %+v", name, record)
			}
		}
	}
	var manifest = &EvidenceManifest{SchemaVersion: "1", ImplementationSHA: runner.implSHA, Records: runner.records}
	var report, reportErr = ValidateEvidenceManifest(manifest, selected, "local")
	if reportErr != nil || report.PassedCases != 2 || report.FailedCases != 0 || report.SkippedCases != 0 || report.ClosureComplete {
		t.Fatalf("independent reference/variadic receipts rejected or expanded into release closure: %+v err=%v", report, reportErr)
	}
	var encoded, marshalErr = json.Marshal(manifest)
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}
	for _, id := range []string{"RP13", "BRG07"} {
		for _, fault := range []string{"partial_selection", "missing_assertion", "wrong_producer", "stale_source", "invented_runtime", "missing_native_tier"} {
			t.Run(id+"/"+fault, func(t *testing.T) {
				var changed EvidenceManifest
				if err := json.Unmarshal(encoded, &changed); err != nil {
					t.Fatal(err)
				}
				var index = -1
				for i, record := range changed.Records {
					if record.Requirements[0] == id {
						index = i
						break
					}
				}
				if index < 0 {
					t.Fatal("executed scenario has no receipt")
				}
				switch fault {
				case "partial_selection":
					changed.Records[index].Command = productionTestCommand(supplementalProductionScenarios[id].TestNames[:1])
				case "missing_assertion":
					delete(changed.Records[index].Assertions, "required_production_tests")
				case "wrong_producer":
					changed.Records[index].TestID = "github.com/electrikmilk/cherri:TestAlphaRenamingPasses"
				case "stale_source":
					changed.Records[index].ImplementationSHA = strings.Repeat("f", 40)
				case "invented_runtime":
					changed.Records[index].Tier = "ios-runtime"
				case "missing_native_tier":
					// A roundtrip includes structure under the established tier
					// relation. Remove the strongest required native tier so the
					// remaining weaker receipt cannot complete the scenario.
					var removedTier = "native-structure"
					if id == "BRG07" {
						removedTier = "native-roundtrip"
					}
					var kept []EvidenceRecord
					for _, record := range changed.Records {
						if record.Requirements[0] != id || record.Tier != removedTier {
							kept = append(kept, record)
						}
					}
					changed.Records = kept
				}
				var rejected, validationErr = ValidateEvidenceManifest(&changed, selected, "local")
				if p15VerificationResult(t, rejected, id).Passed || rejected.ClosureComplete {
					t.Fatal("incomplete or detached native producer was accepted")
				}
				if fault != "missing_native_tier" && validationErr == nil {
					t.Fatal("malformed producer receipt did not raise a validation error")
				}
			})
		}
	}

	var auditPath = filepath.Join(t.TempDir(), "execution-audit.json")
	if err := writeExecutionAudit(runner, all, auditPath); err != nil {
		t.Fatal(err)
	}
	var data, readErr = os.ReadFile(auditPath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	var audit struct {
		ClosureComplete bool                `json:"closure_complete"`
		Rows            []ExecutionAuditRow `json:"rows"`
	}
	if err := json.Unmarshal(data, &audit); err != nil {
		t.Fatal(err)
	}
	if audit.ClosureComplete || len(audit.Rows) != 179 {
		t.Fatal("bounded reference/variadic execution changed the original 179 obligations")
	}
	for _, row := range audit.Rows {
		if wanted[row.ID] {
			var scenario = supplementalProductionScenarios[row.ID]
			if row.Status != "PASS" || !row.PositiveAndNegative || !reflect.DeepEqual(row.TestNames, scenario.TestNames) || !reflect.DeepEqual(row.ProductionPath, scenario.ProductionPaths) {
				t.Fatalf("audit lost the actual reference/variadic producer: %+v", row)
			}
		} else if row.Status == "PASS" || row.PositiveAndNegative {
			t.Fatalf("unexecuted obligation inherited reference/variadic success: %+v", row)
		}
	}
	var status, statusErr = exec.Command("git", "-C", resolveRepoRoot(), "status", "--porcelain", "--untracked-files=normal").Output()
	var receipt, receiptErr = json.Marshal(map[string]any{
		"source_sha": runner.implSHA, "source_tree_clean": statusErr == nil && len(status) == 0,
		"scope":  "RP13 independent zero/one/several/nested named-list scenarios with strict wire and negative controls; BRG07 actual codec and XML/binary import/recompile for action/system/variable references and native extensions. No Apple runtime or release closure.",
		"report": report, "records": runner.records,
	})
	if receiptErr != nil {
		t.Fatal(receiptErr)
	}
	t.Log("P15_REFERENCE_VARIADIC_RECEIPT:" + string(receipt))
}
