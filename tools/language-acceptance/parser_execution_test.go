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

func TestParserAcceptanceProductionEvidence(t *testing.T) {
	all, err := LoadAllContracts("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	selected := &ContractFile{}
	for _, c := range all.Acceptance.Cases {
		if _, ok := parserProductionTests[c.ID]; ok {
			selected.Cases = append(selected.Cases, c)
		}
	}
	selected.CaseCount = len(selected.Cases)
	if selected.CaseCount != 5 {
		t.Fatalf("parser selection drifted from five actual contracts: %d", selected.CaseCount)
	}
	contracts := &ContractsSet{Acceptance: selected, Repair: &RepairContractFile{}, Gates: &GatesContractFile{}}
	runner := NewRunner()
	runner.runParser()
	if len(runner.results) != 5 || len(runner.records) != 7 || len(runner.observations) != 0 {
		t.Fatalf("unexpected evidence scope: results=%d records=%d observations=%d", len(runner.results), len(runner.records), len(runner.observations))
	}
	for _, c := range selected.Cases {
		result := runner.results[c.ID]
		if !result.Passed || result.Status != "PASSED" || result.Requirement != c.Requirement ||
			result.TestLevel != c.MinimumTestLevel || !reflect.DeepEqual(result.ExecutedLevels, parseRequiredTiers(c.MinimumTestLevel)) {
			t.Fatalf("%s did not execute its complete declared contract: %+v", c.ID, result)
		}
	}
	for _, record := range runner.records {
		if len(record.Requirements) != 1 || record.Result != "passed" || record.ExitCode != 0 ||
			record.ImplementationSHA != runner.implSHA || record.Tier != "unit" && record.Tier != "service" {
			t.Fatalf("invented parser evidence: %+v", record)
		}
		tests := parserProductionTests[record.Requirements[0]]
		if record.TestID != "github.com/electrikmilk/cherri:"+tests[0] ||
			!reflect.DeepEqual(record.Command, productionTestCommand(tests)) ||
			!strings.Contains(record.Detail, tests[0]) {
			t.Fatalf("receipt is detached from executed producer: %+v", record)
		}
	}

	manifest := &EvidenceManifest{SchemaVersion: "1", ImplementationSHA: runner.implSHA, Records: runner.records}
	report, err := ValidateEvidenceManifest(manifest, contracts, "local")
	if err != nil || report.PassedCases != 5 || report.FailedCases != 0 || report.SkippedCases != 0 || report.ClosureComplete {
		t.Fatalf("five-contract local evidence rejected or expanded into release closure: report=%+v err=%v", report, err)
	}

	// Removing the actual service tier must not leave P02 satisfied.
	missingService := *manifest
	missingService.Records = nil
	for _, record := range manifest.Records {
		if record.Requirements[0] != "P02" || record.Tier != "service" {
			missingService.Records = append(missingService.Records, record)
		}
	}
	mutated, err := ValidateEvidenceManifest(&missingService, contracts, "local")
	if err == nil || mutated.PassedCases == 5 {
		t.Fatal("missing service evidence mutation passed")
	}

	wrongProducer := *manifest
	wrongProducer.Records = append([]EvidenceRecord{}, manifest.Records...)
	wrongProducer.Records[0].TestID = "github.com/electrikmilk/cherri:TestP15P04ReturnTerminators"
	if _, err := ValidateEvidenceManifest(&wrongProducer, contracts, "local"); err == nil {
		t.Fatal("wrong production test identity passed")
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
		if tests, ok := parserProductionTests[row.ID]; ok {
			if row.Status != "PASS" || !row.PositiveAndNegative || !reflect.DeepEqual(row.TestNames, tests) {
				t.Fatalf("parser execution audit lost actual producer: %+v", row)
			}
		} else if row.Status == "PASS" {
			t.Fatalf("unexecuted obligation inherited parser success: %+v", row)
		}
		if row.ID == "U04" && (row.Status != "UNVERIFIED" || !strings.Contains(row.Blocker, "D005")) {
			t.Fatalf("ambiguous historical U04 contract was silently closed: %+v", row)
		}
	}

	status, statusErr := exec.Command("git", "-C", resolveRepoRoot(), "status", "--porcelain", "--untracked-files=normal").Output()
	receipt, err := json.Marshal(map[string]any{
		"source_sha": runner.implSHA, "source_tree_clean": statusErr == nil && len(status) == 0,
		"scope":  "Only P01-P05 production unit/service contracts; no Swift UI, Apple runtime, linked repair, or release closure",
		"report": report, "records": runner.records,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Log("P15_PARSER_RECEIPT:" + string(receipt))
}

func TestParserAcceptanceMissingProducerFailsClosed(t *testing.T) {
	for _, selected := range [][]string{nil, {"TestP15ParserProducerThatDoesNotExist"}} {
		passed, detail := executeNamedProductionTests(selected)
		if passed {
			t.Fatalf("missing production test created success: %q %s", selected, detail)
		}
		runner := NewRunner()
		runner.Record("P01", "parser", "negative discovery control", "unit", []string{"unit"}, passed, "PASSED", detail)
		if runner.results["P01"].Passed || runner.results["P01"].Status != "FAILED" {
			t.Fatal("failed discovery was converted into formal success")
		}
		for _, record := range runner.records {
			if record.Result != "failed" || record.ExitCode == 0 {
				t.Fatal("failed discovery minted passing evidence")
			}
		}
	}
}
