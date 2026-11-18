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

func TestSchemaGenerationProductionEvidence(t *testing.T) {
	all, err := LoadAllContracts("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	selected := &ContractFile{}
	for _, c := range all.Acceptance.Cases {
		if c.ID == "U02" {
			selected.Cases = append(selected.Cases, c)
		}
	}
	selected.CaseCount = len(selected.Cases)
	if selected.CaseCount != 1 {
		t.Fatal("missing immutable U02 contract")
	}
	contracts := &ContractsSet{Acceptance: selected, Repair: &RepairContractFile{}, Gates: &GatesContractFile{}}
	runner := NewRunner()
	runner.runSchemaGenerationRequirement()
	if len(runner.results) != 1 || len(runner.records) != 2 || len(runner.observations) != 0 {
		t.Fatalf("unexpected generation scope: results=%d records=%d observations=%d", len(runner.results), len(runner.records), len(runner.observations))
	}
	c := selected.Cases[0]
	result := runner.results["U02"]
	if !result.Passed || result.Status != "PASSED" || result.Requirement != c.Requirement || result.TestLevel != c.MinimumTestLevel || !reflect.DeepEqual(result.ExecutedLevels, []string{"unit", "generation"}) {
		t.Fatalf("U02 did not execute its contract: %+v", result)
	}
	tests := schemaProductionTests["U02"]
	for _, record := range runner.records {
		if !reflect.DeepEqual(record.Requirements, []string{"U02"}) || record.ImplementationSHA != runner.implSHA || record.Tier != "unit" && record.Tier != "generation" || record.Result != "passed" || record.ExitCode != 0 || record.TestID != "github.com/electrikmilk/cherri:"+tests[0] || !reflect.DeepEqual(record.Command, productionTestCommand(tests)) || !strings.Contains(record.Detail, tests[0]) {
			t.Fatalf("detached generation receipt: %+v", record)
		}
	}
	manifest := &EvidenceManifest{SchemaVersion: "1", ImplementationSHA: runner.implSHA, Records: runner.records}
	report, err := ValidateEvidenceManifest(manifest, contracts, "local")
	if err != nil || report.PassedCases != 1 || report.FailedCases != 0 || report.SkippedCases != 0 || report.ClosureComplete {
		t.Fatalf("bounded U02 report invalid/expanded: %+v %v", report, err)
	}
	for _, mutation := range []string{"wrong_producer", "missing_generation", "wrong_contract"} {
		broken := *manifest
		broken.Records = append([]EvidenceRecord{}, manifest.Records...)
		switch mutation {
		case "wrong_producer":
			broken.Records[0].TestID = "tools/language-acceptance:runUpstream"
		case "missing_generation":
			broken.Records = nil
			for _, record := range manifest.Records {
				if record.Tier != "generation" {
					broken.Records = append(broken.Records, record)
				}
			}
		case "wrong_contract":
			broken.Records[0].Requirements = []string{"U01"}
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
		if row.ID == "U02" {
			if row.Status != "PASS" || !row.PositiveAndNegative || !reflect.DeepEqual(row.TestNames, tests) {
				t.Fatalf("lost U02 producer: %+v", row)
			}
		} else if row.Status == "PASS" {
			t.Fatalf("unexecuted requirement inherited U02 success: %+v", row)
		}
	}
	status, statusErr := exec.Command("git", "-C", resolveRepoRoot(), "status", "--porcelain", "--untracked-files=normal").Output()
	receipt, err := json.Marshal(map[string]any{"source_sha": runner.implSHA, "source_tree_clean": statusErr == nil && len(status) == 0, "scope": "Only U02 actual production generation and causal mutation checks; no platform or release closure", "report": report, "records": runner.records})
	if err != nil {
		t.Fatal(err)
	}
	t.Log("P15_SCHEMA_GENERATION_RECEIPT:" + string(receipt))
}

func TestSchemaPropagationProductionEvidence(t *testing.T) {
	all, err := LoadAllContracts("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	selected := &ContractFile{}
	for _, c := range all.Acceptance.Cases {
		if c.ID == "U03" {
			selected.Cases = append(selected.Cases, c)
		}
	}
	selected.CaseCount = len(selected.Cases)
	if selected.CaseCount != 1 {
		t.Fatal("missing immutable U03 contract")
	}
	runner := NewRunner()
	runner.runSchemaPropagationRequirement()
	result := runner.results["U03"]
	c := selected.Cases[0]
	if !result.Passed || result.Requirement != c.Requirement || result.TestLevel != c.MinimumTestLevel || len(runner.records) != 1 {
		t.Fatalf("invalid propagation evidence: %+v", result)
	}
	record := runner.records[0]
	if record.TestID != "github.com/electrikmilk/cherri:TestUpstreamPropagationRehearsal" || record.ImplementationSHA != runner.implSHA || record.Tier != "integration" || !reflect.DeepEqual(record.Command, productionTestCommand(schemaProductionTests["U03"])) {
		t.Fatalf("detached propagation receipt: %+v", record)
	}
	manifest := &EvidenceManifest{SchemaVersion: "1", ImplementationSHA: runner.implSHA, Records: runner.records}
	contracts := &ContractsSet{Acceptance: selected, Repair: &RepairContractFile{}, Gates: &GatesContractFile{}}
	report, err := ValidateEvidenceManifest(manifest, contracts, "local")
	if err != nil || report.PassedCases != 1 || report.ClosureComplete {
		t.Fatalf("invalid bounded U03 report: %+v %v", report, err)
	}
	t.Logf("P15_SCHEMA_PROPAGATION_RECEIPT: source=%s; actual generation/analyzer/service/facet rejection and restore; integration=PASS; release=false", runner.implSHA)
}
