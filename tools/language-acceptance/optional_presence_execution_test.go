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

func TestTypeOptionalPresenceBoundedEvidence(t *testing.T) {
	all, err := LoadAllContracts("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	selected := &ContractFile{}
	for _, c := range all.Acceptance.Cases {
		if c.ID == "T05" {
			selected.Cases = append(selected.Cases, c)
		}
	}
	selected.CaseCount = len(selected.Cases)
	if selected.CaseCount != 1 {
		t.Fatal("missing immutable T05 contract")
	}
	contracts := &ContractsSet{Acceptance: selected, Repair: &RepairContractFile{}, Gates: &GatesContractFile{}}
	runner := NewRunner()
	// Exercise the retained historical three-test producer directly. The public
	// T05 entrypoint now runs the complete, separately validated cohort.
	runner.runTypeProduction("T05", "Optional presence differs from optional value and JSON null")
	result := runner.results["T05"]
	contract := selected.Cases[0]
	if result.Passed || result.Status != "NOT_RUN" || result.Requirement != contract.Requirement || result.TestLevel != contract.MinimumTestLevel || len(runner.results) != 1 || len(runner.records) != 0 || len(runner.observations) != 1 {
		t.Fatalf("bounded producer minted full JSON-null proof: %+v %+v", result, runner.records)
	}
	observation := runner.observations[0]
	if !observation.Passed || observation.ID != "T05" || !strings.Contains(observation.CoverageGap, "JSON null") {
		t.Fatalf("actual partial execution failed or lost gap: %+v", observation)
	}
	tests := typeProductionTests["T05"]
	for _, name := range tests {
		if !strings.Contains(observation.Detail, name) {
			t.Fatal("missing actual producer outcome " + name)
		}
	}
	manifest := &EvidenceManifest{SchemaVersion: "1", ImplementationSHA: runner.implSHA, Records: runner.records}
	report, err := ValidateEvidenceManifest(manifest, contracts, "local")
	if report == nil || report.PassedCases != 0 || report.ClosureComplete {
		t.Fatalf("absent complete evidence promoted: %+v %v", report, err)
	}
	for _, fault := range []string{"full_relabel", "missing_native", "cross_tier", "wrong_producer"} {
		t.Run(fault, func(t *testing.T) {
			records := []EvidenceRecord{
				{EvidenceID: "p15-partial-unit", Requirements: []string{"T05"}, Tier: "unit", ImplementationSHA: runner.implSHA, TestID: "github.com/electrikmilk/cherri:" + tests[0], Command: productionTestCommand(tests), ExitCode: 0, Result: "passed", Assertions: map[string]string{"bounded_scope": "actual ordinary values only"}},
				{EvidenceID: "p15-partial-native", Requirements: []string{"T05"}, Tier: "native-structure", ImplementationSHA: runner.implSHA, TestID: "github.com/electrikmilk/cherri:" + tests[1], Command: productionTestCommand(tests), ExitCode: 0, Result: "passed", Assertions: map[string]string{"bounded_scope": "actual ordinary values only"}},
			}
			switch fault {
			case "missing_native":
				records = records[:1]
			case "cross_tier":
				records[1].TestID = records[0].TestID
			case "wrong_producer":
				for i := range records {
					records[i].TestID = "tools/language-acceptance:runTypes"
				}
			}
			fake := &EvidenceManifest{SchemaVersion: "1", ImplementationSHA: runner.implSHA, Records: records}
			got, validationError := ValidateEvidenceManifest(fake, contracts, "local")
			if validationError == nil || (got != nil && (got.PassedCases != 0 || got.ClosureComplete)) {
				t.Fatalf("%s turned partial tests into full T05: %+v %v", fault, got, validationError)
			}
		})
	}
	auditPath := filepath.Join(t.TempDir(), "audit.json")
	if err := writeExecutionAudit(runner, all, auditPath); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(auditPath)
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
		t.Fatal("bounded T05 changed formal accounting")
	}
	found := false
	for _, row := range audit.Rows {
		if row.Status == "PASS" {
			t.Fatalf("partial execution minted PASS: %+v", row)
		}
		if row.ID == "T05" {
			found = true
			if row.Status != "UNVERIFIED" || row.Blocker == "" || !reflect.DeepEqual(row.TestNames, tests) {
				t.Fatalf("partial source audit lost explicit scope: %+v", row)
			}
		}
	}
	if !found {
		t.Fatal("T05 absent from audit")
	}
	status, statusErr := exec.Command("git", "-C", resolveRepoRoot(), "status", "--porcelain", "--untracked-files=normal").Output()
	receipt, err := json.Marshal(map[string]any{"source_sha": runner.implSHA, "source_tree_clean": statusErr == nil && len(status) == 0, "scope": "Actual authored presence/empty/false/zero native fields and unsupported-null rejection only; full T05 JSON-null/optional-container semantics remain UNVERIFIED, no Apple runtime or closure", "command": productionTestCommand(tests), "observations": runner.observations, "report": report, "records": runner.records})
	if err != nil {
		t.Fatal(err)
	}
	t.Log("P15_TYPE_OPTIONAL_PRESENCE_RECEIPT:" + string(receipt))
}
