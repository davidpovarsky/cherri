package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestSupplementalRealModuleResolutionProductionEvidence(t *testing.T) {
	var all, err = LoadAllContracts("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	var selected = &ContractsSet{Acceptance: &ContractFile{}, Repair: &RepairContractFile{}, Gates: &GatesContractFile{}}
	for _, scenario := range all.Repair.Cases {
		if scenario.ID == "RP27" {
			selected.Repair.Cases = append(selected.Repair.Cases, scenario)
		}
	}
	selected.Repair.CaseCount = len(selected.Repair.Cases)
	if selected.Repair.CaseCount != 1 {
		t.Fatal("immutable real-module scenario is absent")
	}
	var runner = NewRunner()
	runner.runRepairCases(selected.Repair)
	var result = runner.results["RP27"]
	if len(runner.results) != 1 || len(runner.records) != 3 || len(runner.observations) != 0 ||
		!result.Passed || result.Status != "PASSED" ||
		!reflect.DeepEqual(result.ExecutedLevels, []string{"unit", "service", "migration"}) {
		t.Fatalf("actual module resolution scenario incomplete: %+v records=%d observations=%d", result, len(runner.records), len(runner.observations))
	}
	var scenario = supplementalProductionScenarios["RP27"]
	for _, record := range runner.records {
		if !reflect.DeepEqual(record.Requirements, []string{"RP27"}) || record.Result != "passed" ||
			record.ExitCode != 0 || record.ImplementationSHA != runner.implSHA {
			t.Fatalf("real-module execution minted an unrelated receipt: %+v", record)
		}
		if err := validateSupplementalProductionRecord("RP27", record); err != nil {
			t.Fatal(err)
		}
		for _, name := range scenario.TestNames {
			if !strings.Contains(record.Detail, name) {
				t.Fatal("actual producer discovery omitted", name)
			}
		}
	}
	var manifest = &EvidenceManifest{SchemaVersion: "1", ImplementationSHA: runner.implSHA, Records: runner.records}
	var report, validationErr = ValidateEvidenceManifest(manifest, selected, "local")
	if validationErr != nil || report.PassedCases != 1 || report.FailedCases != 0 || report.ClosureComplete {
		t.Fatalf("local module scenario rejected or promoted to closure: %+v %v", report, validationErr)
	}
	var encoded, marshalErr = json.Marshal(manifest)
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}
	for _, fault := range []string{"partial_command", "missing_producer_list", "wrong_test", "stale_sha", "invented_runtime", "missing_migration"} {
		t.Run(fault, func(t *testing.T) {
			var changed EvidenceManifest
			if err := json.Unmarshal(encoded, &changed); err != nil {
				t.Fatal(err)
			}
			switch fault {
			case "partial_command":
				changed.Records[0].Command = productionTestCommand(scenario.TestNames[:1])
			case "missing_producer_list":
				delete(changed.Records[0].Assertions, "required_production_tests")
			case "wrong_test":
				changed.Records[0].TestID = "github.com/electrikmilk/cherri:TestUnrelatedModuleSmoke"
			case "stale_sha":
				changed.Records[0].ImplementationSHA = strings.Repeat("f", 40)
			case "invented_runtime":
				changed.Records[0].Tier = "ios-runtime"
			case "missing_migration":
				changed.Records = changed.Records[:2]
			}
			var rejected, err = ValidateEvidenceManifest(&changed, selected, "local")
			if rejected != nil && (p15VerificationResult(t, rejected, "RP27").Passed || rejected.ClosureComplete) {
				t.Fatal("incomplete real-module scenario was accepted")
			}
			if fault != "missing_migration" && err == nil {
				t.Fatal("detached real-module receipt raised no validation error")
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
		t.Fatal("actual module producer changed the immutable179 inventory")
	}
	for _, row := range audit.Rows {
		if row.ID == "RP27" {
			if row.Status != "PASS" || !row.PositiveAndNegative || !reflect.DeepEqual(row.TestNames, scenario.TestNames) ||
				!reflect.DeepEqual(row.ProductionPath, scenario.ProductionPaths) {
				t.Fatalf("audit lost executed RP27 producer: %+v", row)
			}
		} else if row.Status == "PASS" || row.PositiveAndNegative {
			t.Fatalf("a linked module requirement inherited success: %+v", row)
		}
	}
	var receipt, receiptErr = json.Marshal(map[string]any{"source_sha": runner.implSHA, "report": report,
		"records": runner.records, "scope": "RP27 independently executes actual multi-file namespaces, aliases, cycles, module-init migration and unresolved remote/local dependency controls; no related-ID inheritance, network installation or native-device claim"})
	if receiptErr != nil {
		t.Fatal(receiptErr)
	}
	t.Log("P15_MODULE_RESOLUTION_RECEIPT:" + string(receipt))
}
