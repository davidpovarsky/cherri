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
	all, err := LoadAllContracts("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	selected := &ContractFile{}
	for _, c := range all.Acceptance.Cases {
		if c.ID == "T02" {
			selected.Cases = append(selected.Cases, c)
		}
	}
	selected.CaseCount = len(selected.Cases)
	if selected.CaseCount != 1 {
		t.Fatal("missing immutable T02 contract")
	}
	contracts := &ContractsSet{Acceptance: selected, Repair: &RepairContractFile{}, Gates: &GatesContractFile{}}
	runner := NewRunner()
	runner.runTypes()
	if len(runner.results) != 9 || len(runner.records) != 1 || len(runner.observations) != 8 {
		t.Fatalf("unexpected types scope: results=%d records=%d observations=%d", len(runner.results), len(runner.records), len(runner.observations))
	}
	c := selected.Cases[0]
	result := runner.results["T02"]
	if !result.Passed || result.Status != "PASSED" || result.Requirement != c.Requirement || result.TestLevel != c.MinimumTestLevel || !reflect.DeepEqual(result.ExecutedLevels, []string{"unit"}) {
		t.Fatalf("T02 did not execute its contract: %+v", result)
	}
	record := runner.records[0]
	tests := typeProductionTests["T02"]
	if !reflect.DeepEqual(record.Requirements, []string{"T02"}) || record.ImplementationSHA != runner.implSHA || record.Tier != "unit" || record.Result != "passed" || record.ExitCode != 0 ||
		record.TestID != "github.com/electrikmilk/cherri:"+tests[0] || !reflect.DeepEqual(record.Command, productionTestCommand(tests)) || !strings.Contains(record.Detail, tests[0]) {
		t.Fatalf("detached production receipt: %+v", record)
	}
	manifest := &EvidenceManifest{SchemaVersion: "1", ImplementationSHA: runner.implSHA, Records: runner.records}
	report, err := ValidateEvidenceManifest(manifest, contracts, "local")
	if err != nil || report.PassedCases != 1 || report.FailedCases != 0 || report.SkippedCases != 0 || report.ClosureComplete {
		t.Fatalf("bounded T02 report invalid/expanded: %+v %v", report, err)
	}
	for _, mutation := range []string{"wrong_producer", "missing_record", "wrong_contract"} {
		broken := *manifest
		broken.Records = append([]EvidenceRecord{}, manifest.Records...)
		switch mutation {
		case "wrong_producer":
			broken.Records[0].TestID = "tools/language-acceptance:runTypes"
		case "missing_record":
			broken.Records = nil
		case "wrong_contract":
			broken.Records[0].Requirements = []string{"T01"}
		}
		if _, err := ValidateEvidenceManifest(&broken, contracts, "local"); err == nil {
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
		if row.ID == "T02" {
			if row.Status != "PASS" || !row.PositiveAndNegative || !reflect.DeepEqual(row.TestNames, tests) {
				t.Fatalf("lost T02 producer: %+v", row)
			}
		} else if row.Status == "PASS" {
			t.Fatalf("unexecuted requirement inherited T02 success: %+v", row)
		}
	}
	status, statusErr := exec.Command("git", "-C", resolveRepoRoot(), "status", "--porcelain", "--untracked-files=normal").Output()
	receipt, err := json.Marshal(map[string]any{"source_sha": runner.implSHA, "source_tree_clean": statusErr == nil && len(status) == 0, "scope": "Only T02 production unit contract; no native execution, external type guarantee, other T requirement or release closure", "report": report, "records": runner.records})
	if err != nil {
		t.Fatal(err)
	}
	t.Log("P15_TYPE_LITERAL_RECEIPT:" + string(receipt))
}
