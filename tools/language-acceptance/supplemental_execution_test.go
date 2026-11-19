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

func TestSupplementalAcceptanceProductionEvidence(t *testing.T) {
	var all, err = LoadAllContracts("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	var selected = &ContractsSet{
		Acceptance: &ContractFile{},
		Repair:     &RepairContractFile{},
		Gates:      &GatesContractFile{},
	}
	for _, scenario := range all.Repair.Cases {
		if _, ok := supplementalProductionScenarios[scenario.ID]; ok {
			selected.Repair.Cases = append(selected.Repair.Cases, scenario)
		}
	}
	for _, scenario := range all.Gates.Gates {
		if _, ok := supplementalProductionScenarios[scenario.ID]; ok {
			selected.Gates.Gates = append(selected.Gates.Gates, scenario)
		}
	}
	selected.Repair.CaseCount = len(selected.Repair.Cases)
	selected.Gates.GateCount = len(selected.Gates.Gates)
	if selected.Repair.CaseCount != 6 || selected.Gates.GateCount != 3 {
		t.Fatal("supplemental scenario selection drifted")
	}

	var runner = NewRunner()
	runner.runRepairCases(all.Repair)
	runner.runRecoveryGates(all.Gates)
	if len(runner.results) != 85 || len(runner.records) != 15 || len(runner.observations) != 0 {
		t.Fatalf("unexpected supplemental scope: results=%d records=%d observations=%d",
			len(runner.results), len(runner.records), len(runner.observations))
	}
	for id, result := range runner.results {
		if scenario, found := supplementalProductionScenarios[id]; found {
			if !result.Passed || result.Status != "PASSED" ||
				!reflect.DeepEqual(result.ExecutedLevels, scenario.Tiers) {
				t.Fatalf("%s complete supplemental scenario did not pass: %+v", id, result)
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
	if reportErr != nil || report.PassedCases != 9 || report.FailedCases != 0 ||
		report.SkippedCases != 0 || report.ClosureComplete {
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
		} else if row.Status == "PASS" || row.PositiveAndNegative {
			t.Fatalf("unexecuted obligation inherited supplemental success: %+v", row)
		}
	}

	var status, statusErr = exec.Command("git", "-C", resolveRepoRoot(), "status", "--porcelain", "--untracked-files=normal").Output()
	var receipt, receiptErr = json.Marshal(map[string]any{
		"source_sha": runner.implSHA, "source_tree_clean": statusErr == nil && len(status) == 0,
		"scope":  "RP01/RP02/RP03/BRG04 actual verification contracts; BRG02/BRG03 strict comparator; RP35 native preservation; RP36 intake; RP37 allowed-edit and failed-signing gates. No linked-ID inheritance, live signing success, or release closure.",
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
