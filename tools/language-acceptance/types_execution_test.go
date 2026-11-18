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

func TestTypeLiteralProductionEvidence(t *testing.T) {
	testTypeProductionEvidence(t, "T02", (*Runner).runTypeLiteralRequirement, "P15_TYPE_LITERAL_RECEIPT:", "Only T02 production unit contract; no native execution, external type guarantee, other T requirement or release closure")
}
func TestTypeRecordProductionEvidence(t *testing.T) {
	testTypeProductionEvidence(t, "T06", (*Runner).runTypeRecordRequirement, "P15_TYPE_RECORD_RECEIPT:", "Only T06 production unit contract; static rejection of unvalidated external record guarantees, no runtime validator, native execution or release closure")
}
func TestTypeCardinalityProductionEvidence(t *testing.T) {
	testTypeProductionEvidence(t, "T08", (*Runner).runTypeCardinalityRequirement, "P15_TYPE_CARDINALITY_RECEIPT:", "Only T08 unit and service contract; native metadata and emitted wire are inspected locally, no Apple runtime, platform availability or release closure")
}

func testTypeProductionEvidence(t *testing.T, id string, run func(*Runner), receiptLabel, scope string) {
	t.Helper()
	all, err := LoadAllContracts("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	selected := &ContractFile{}
	for _, contract := range all.Acceptance.Cases {
		if contract.ID == id {
			selected.Cases = append(selected.Cases, contract)
		}
	}
	selected.CaseCount = len(selected.Cases)
	if selected.CaseCount != 1 {
		t.Fatalf("missing immutable %s contract", id)
	}
	contracts := &ContractsSet{Acceptance: selected, Repair: &RepairContractFile{}, Gates: &GatesContractFile{}}
	runner := NewRunner()
	run(runner)
	tiers := parseRequiredTiers(selected.Cases[0].MinimumTestLevel)
	if len(runner.results) != 1 || len(runner.records) != len(tiers) || len(runner.observations) != 0 {
		t.Fatalf("unexpected types scope: results=%d records=%d observations=%d", len(runner.results), len(runner.records), len(runner.observations))
	}
	contract := selected.Cases[0]
	result := runner.results[id]
	if !result.Passed || result.Status != "PASSED" || result.Requirement != contract.Requirement || result.TestLevel != contract.MinimumTestLevel || !reflect.DeepEqual(result.ExecutedLevels, tiers) {
		t.Fatalf("%s did not execute its contract: %+v", id, result)
	}
	tests := typeProductionTests[id]
	for i, record := range runner.records {
		producer := tests[0]
		if id == "T08" && tiers[i] == "service" {
			producer = tests[1]
		}
		if !reflect.DeepEqual(record.Requirements, []string{id}) || record.ImplementationSHA != runner.implSHA || record.Tier != tiers[i] || record.Result != "passed" || record.ExitCode != 0 || record.TestID != "github.com/electrikmilk/cherri:"+producer || !reflect.DeepEqual(record.Command, productionTestCommand(tests)) {
			t.Fatalf("detached production receipt: %+v", record)
		}
		for _, name := range tests {
			if !strings.Contains(record.Detail, name) {
				t.Fatalf("missing named test outcome %s", name)
			}
		}
	}
	manifest := &EvidenceManifest{SchemaVersion: "1", ImplementationSHA: runner.implSHA, Records: runner.records}
	report, err := ValidateEvidenceManifest(manifest, contracts, "local")
	if err != nil || report.PassedCases != 1 || report.FailedCases != 0 || report.SkippedCases != 0 || report.ClosureComplete {
		t.Fatalf("bounded %s report invalid/expanded: %+v %v", id, report, err)
	}
	mutations := []string{"wrong_producer", "missing_record", "wrong_contract"}
	if len(tiers) > 1 {
		mutations = append(mutations, "missing_service_tier")
	}
	for _, mutation := range mutations {
		broken := *manifest
		broken.Records = append([]EvidenceRecord{}, manifest.Records...)
		switch mutation {
		case "wrong_producer":
			broken.Records[0].TestID = "tools/language-acceptance:runTypes"
		case "missing_record":
			broken.Records = nil
		case "wrong_contract":
			broken.Records[0].Requirements = []string{"T01"}
		case "missing_service_tier":
			broken.Records = broken.Records[:1]
		}
		invalid, validationError := ValidateEvidenceManifest(&broken, contracts, "local")
		if validationError == nil && invalid.PassedCases != 0 {
			t.Errorf("%s mutation minted evidence", mutation)
		}
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
		t.Fatal("bounded producer altered formal accounting")
	}
	for _, row := range audit.Rows {
		if row.ID == id {
			if row.Status != "PASS" || !row.PositiveAndNegative || !reflect.DeepEqual(row.TestNames, tests) {
				t.Fatalf("lost type producer: %+v", row)
			}
		} else if row.Status == "PASS" {
			t.Fatalf("unexecuted requirement inherited type success: %+v", row)
		}
	}
	status, statusErr := exec.Command("git", "-C", resolveRepoRoot(), "status", "--porcelain", "--untracked-files=normal").Output()
	receipt, err := json.Marshal(map[string]any{"source_sha": runner.implSHA, "source_tree_clean": statusErr == nil && len(status) == 0, "scope": scope, "report": report, "records": runner.records})
	if err != nil {
		t.Fatal(err)
	}
	t.Log(receiptLabel + string(receipt))
}
