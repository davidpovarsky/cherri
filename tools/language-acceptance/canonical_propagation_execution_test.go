package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestP15BRG24ProductionEvidence(t *testing.T) {
	all, err := LoadAllContracts("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	selected := &ContractsSet{Acceptance: &ContractFile{}, Repair: &RepairContractFile{}, Gates: &GatesContractFile{}}
	for _, gate := range all.Gates.Gates {
		if gate.ID == "BRG24" {
			selected.Gates.Gates = append(selected.Gates.Gates, gate)
		}
	}
	selected.Gates.GateCount = len(selected.Gates.Gates)
	scenario := supplementalProductionScenarios["BRG24"]
	if selected.Gates.GateCount != 1 || selected.Gates.Gates[0].Title != "Real canonical behavior propagation" ||
		!reflect.DeepEqual(parseRequiredTiers(strings.Join(selected.Gates.Gates[0].MinimumEvidenceTiers, "+")), scenario.Tiers) {
		t.Fatal("BRG24 producer differs from immutable original scenario")
	}
	runner := NewRunner()
	runner.runRecoveryGates(selected.Gates)
	result, found := runner.results["BRG24"]
	if !found || len(runner.results) != 1 || len(runner.records) != 3 || len(runner.observations) != 0 ||
		!result.Passed || result.Status != "PASSED" || !reflect.DeepEqual(result.ExecutedLevels, scenario.Tiers) {
		t.Fatalf("complete source propagation not actually executed: %+v", result)
	}
	for _, record := range runner.records {
		if record.ImplementationSHA != runner.implSHA || !reflect.DeepEqual(record.Requirements, []string{"BRG24"}) ||
			record.Result != "passed" || record.ExitCode != 0 || !strings.Contains(record.Detail, scenario.TestNames[0]) {
			t.Fatal("source propagation receipt lacks actual observed scope")
		}
		if err := validateSupplementalProductionRecord("BRG24", record); err != nil {
			t.Fatal(err)
		}
	}
	manifest := &EvidenceManifest{SchemaVersion: "1", ImplementationSHA: runner.implSHA, Records: runner.records}
	report, err := ValidateEvidenceManifest(manifest, selected, "local")
	if err != nil || report.PassedCases != 1 || report.FailedCases != 0 || report.SkippedCases != 0 || report.ClosureComplete {
		t.Fatalf("bounded BRG24 manifest failed or inherited closure: %+v %v", report, err)
	}
	encoded, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	for _, fault := range []string{"zero_selection", "wrong_producer", "missing_assertion", "stale_source", "missing_repository", "missing_integration", "missing_native", "invented_runtime", "wrong_requirement"} {
		t.Run(fault, func(t *testing.T) {
			var changed EvidenceManifest
			if err := json.Unmarshal(encoded, &changed); err != nil {
				t.Fatal(err)
			}
			missing := ""
			switch fault {
			case "zero_selection":
				changed.Records[0].Command = productionTestCommand([]string{"TestBRG24NoActualProducer"})
			case "wrong_producer":
				changed.Records[0].TestID = "github.com/electrikmilk/cherri:TestAlphaRenamingPasses"
			case "missing_assertion":
				delete(changed.Records[0].Assertions, "required_production_tests")
			case "stale_source":
				changed.Records[0].ImplementationSHA = strings.Repeat("f", 40)
			case "missing_repository":
				missing = "repository"
			case "missing_integration":
				missing = "integration"
			case "missing_native":
				missing = "native-structure"
			case "invented_runtime":
				changed.Records[0].Tier = "ios-runtime"
			case "wrong_requirement":
				changed.Records[0].Requirements = []string{"BRG05"}
			}
			if missing != "" {
				kept := make([]EvidenceRecord, 0)
				for _, record := range changed.Records {
					if record.Tier != missing {
						kept = append(kept, record)
					}
				}
				changed.Records = kept
			}
			rejected, validationErr := ValidateEvidenceManifest(&changed, selected, "local")
			if p15VerificationResult(t, rejected, "BRG24").Passed || rejected.ClosureComplete {
				t.Fatal("detached or incomplete BRG24 evidence passed")
			}
			if missing == "" && validationErr == nil {
				t.Fatal("invalid BRG24 evidence did not fail validation")
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
	var audit struct {
		ClosureComplete bool                `json:"closure_complete"`
		Rows            []ExecutionAuditRow `json:"rows"`
	}
	if err := json.Unmarshal(raw, &audit); err != nil {
		t.Fatal(err)
	}
	if audit.ClosureComplete || len(audit.Rows) != 179 {
		t.Fatal("bounded BRG24 selection changed full original accounting")
	}
	for _, row := range audit.Rows {
		if row.ID == "BRG24" {
			if row.Status != "PASS" || !row.PositiveAndNegative || !reflect.DeepEqual(row.TestNames, scenario.TestNames) || !reflect.DeepEqual(row.ProductionPath, scenario.ProductionPaths) {
				t.Fatalf("BRG24 audit detached from actual producer: %+v", row)
			}
		} else if row.Status == "PASS" || row.PositiveAndNegative {
			t.Fatalf("unexecuted row inherited BRG24 evidence: %+v", row)
		}
	}
	receipt, err := json.Marshal(map[string]any{"source_sha": runner.implSHA, "scope": "Actual owned detached-worktree canonical source edits, rebuilt Legacy/V2 native outputs and restoration only; no Apple runtime or linked-ID inheritance.", "report": report, "records": runner.records})
	if err != nil {
		t.Fatal(err)
	}
	t.Log("P15_CANONICAL_PROPAGATION_PRODUCER_RECEIPT:" + string(receipt))
}
