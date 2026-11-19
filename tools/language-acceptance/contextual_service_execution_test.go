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

func TestP15RP38ContextualServiceProductionEvidence(t *testing.T) {
	all, err := LoadAllContracts("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	selected := &ContractsSet{Acceptance: &ContractFile{}, Repair: &RepairContractFile{}, Gates: &GatesContractFile{}}
	for _, scenario := range all.Repair.Cases {
		if scenario.ID == "RP38" {
			selected.Repair.Cases = append(selected.Repair.Cases, scenario)
		}
	}
	selected.Repair.CaseCount = len(selected.Repair.Cases)
	if selected.Repair.CaseCount != 1 {
		t.Fatal("immutable RP38 scenario missing")
	}
	runner := NewRunner()
	runner.runRepairCases(selected.Repair)
	if len(runner.results) != 1 || len(runner.records) != 2 || len(runner.observations) != 0 {
		t.Fatalf("wrong bounded service/protocol scope: results=%d records=%d observations=%d", len(runner.results), len(runner.records), len(runner.observations))
	}
	scenario := supplementalProductionScenarios["RP38"]
	result := runner.results["RP38"]
	if !result.Passed || result.Status != "PASSED" || !reflect.DeepEqual(result.ExecutedLevels, scenario.Tiers) {
		t.Fatalf("actual nine-test producer failed: %+v", result)
	}
	for _, record := range runner.records {
		if err := validateSupplementalProductionRecord("RP38", record); err != nil {
			t.Fatal(err)
		}
		if len(record.Requirements) != 1 || record.Requirements[0] != "RP38" || record.Result != "passed" || record.ExitCode != 0 || record.ImplementationSHA != runner.implSHA {
			t.Fatalf("detached service/protocol record: %+v", record)
		}
		for _, name := range scenario.TestNames {
			if !strings.Contains(record.Detail, name) {
				t.Fatalf("undiscovered production test: %s", name)
			}
		}
	}
	manifest := &EvidenceManifest{SchemaVersion: "1", ImplementationSHA: runner.implSHA, Records: runner.records}
	report, err := ValidateEvidenceManifest(manifest, selected, "local")
	if err != nil || report.PassedCases != 1 || report.FailedCases != 0 || report.ClosureComplete {
		t.Fatalf("actual RP38 rejected or promoted to release closure: %+v %v", report, err)
	}
	encoded, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	for _, fault := range []string{"capability_only", "missing_protocol", "missing_selection", "stale_source", "invented_runtime", "linked_original", "wrong_producer"} {
		t.Run(fault, func(t *testing.T) {
			var changed EvidenceManifest
			if err := json.Unmarshal(encoded, &changed); err != nil {
				t.Fatal(err)
			}
			switch fault {
			case "capability_only":
				changed.Records[0].Command = productionTestCommandInPackage(scenario.PackagePath, []string{"TestLSPServerInitializeAndDidOpen"})
			case "missing_protocol":
				var kept []EvidenceRecord
				for _, record := range changed.Records {
					if record.Tier != "protocol" {
						kept = append(kept, record)
					}
				}
				changed.Records = kept
			case "missing_selection":
				delete(changed.Records[0].Assertions, "required_production_tests")
			case "stale_source":
				changed.Records[0].ImplementationSHA = strings.Repeat("f", 40)
			case "invented_runtime":
				changed.Records[0].Tier = "ios-runtime"
			case "linked_original":
				changed.Records[0].Requirements = []string{"E02"}
			case "wrong_producer":
				changed.Records[0].TestID = "github.com/electrikmilk/cherri/internal/language/lsp:TestLSPServerInitializeAndDidOpen"
			}
			rejected, validationErr := ValidateEvidenceManifest(&changed, selected, "local")
			if p15VerificationResult(t, rejected, "RP38").Passed || rejected.ClosureComplete {
				t.Fatal("incomplete or forged protocol receipt accepted")
			}
			if fault != "missing_protocol" && validationErr == nil {
				t.Fatal("malformed producer did not raise validation error")
			}
		})
	}
	auditPath := filepath.Join(t.TempDir(), "execution-audit.json")
	if err := writeExecutionAudit(runner, all, auditPath); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatal(err)
	}
	var audit struct {
		ClosureComplete bool
		Rows            []ExecutionAuditRow
	}
	// Decode the actual serialized fields without relying on Go's field-name
	// normalization for the underscore-separated closure_complete key.
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(data, &envelope); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(envelope["rows"], &audit.Rows); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(envelope["closure_complete"], &audit.ClosureComplete); err != nil {
		t.Fatal(err)
	}
	if audit.ClosureComplete || len(audit.Rows) != 179 {
		t.Fatal("bounded producer changed the 179 obligations")
	}
	for _, row := range audit.Rows {
		if row.ID == "RP38" {
			if row.Status != "PASS" || !row.PositiveAndNegative || !reflect.DeepEqual(row.TestNames, scenario.TestNames) || !reflect.DeepEqual(row.ProductionPath, scenario.ProductionPaths) {
				t.Fatalf("audit lost actual producer: %+v", row)
			}
		} else if row.Status == "PASS" || row.PositiveAndNegative {
			t.Fatalf("unexecuted obligation inherited service success: %+v", row)
		}
	}
	status, statusErr := exec.Command("git", "-C", resolveRepoRoot(), "status", "--porcelain", "--untracked-files=normal").Output()
	receipt, err := json.Marshal(map[string]any{"source_sha": runner.implSHA, "source_tree_clean": statusErr == nil && len(status) == 0, "scope": "RP38 actual framed LSP and shared service; nine discovered tests, scoped labels/variables, exact UTF-16 requests/ranges, diagnostic multiplicity, typed signatures, malformed framing, error responses, transport failures. No iOS editor, original-ID inheritance, or release closure.", "report": report, "records": runner.records})
	if err != nil {
		t.Fatal(err)
	}
	t.Log("P15_RP38_SERVICE_PROTOCOL_RECEIPT:" + string(receipt))
}
