package main

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestPublicParserRouteProductionEvidence(t *testing.T) {
	all, err := LoadAllContracts("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	selected := &ContractFile{}
	for _, entry := range all.Acceptance.Cases {
		if entry.ID == "U04" {
			selected.Cases = append(selected.Cases, entry)
		}
	}
	selected.CaseCount = len(selected.Cases)
	if selected.CaseCount != 1 {
		t.Fatal("U04 missing or duplicated")
	}
	contracts := &ContractsSet{Acceptance: selected, Repair: &RepairContractFile{}, Gates: &GatesContractFile{}}
	runner := NewRunner()
	runner.runPublicParserRouteRequirement()
	result := runner.results["U04"]
	if !result.Passed || result.Status != "PASSED" || result.Requirement != selected.Cases[0].Requirement ||
		result.TestLevel != selected.Cases[0].MinimumTestLevel || len(runner.records) != 1 || len(runner.observations) != 0 {
		t.Fatalf("public-route producer did not execute its exact contract: %+v", result)
	}
	record := runner.records[0]
	tests := upstreamRouteProductionTests["U04"]
	if record.Tier != "integration" || record.ImplementationSHA != runner.implSHA ||
		record.TestID != "github.com/electrikmilk/cherri:"+tests[0] ||
		!reflect.DeepEqual(record.Command, productionTestCommand(tests)) {
		t.Fatalf("detached public CLI receipt: %+v", record)
	}
	manifest := &EvidenceManifest{SchemaVersion: "1", ImplementationSHA: runner.implSHA, Records: runner.records}
	report, err := ValidateEvidenceManifest(manifest, contracts, "local")
	if err != nil || report.PassedCases != 1 || report.FailedCases != 0 || report.SkippedCases != 0 || report.ClosureComplete {
		t.Fatalf("bounded U04 evidence was rejected or promoted to release: %+v %v", report, err)
	}
	for _, mutation := range []string{"wrong_test", "wrong_source", "missing_record"} {
		t.Run(mutation, func(t *testing.T) {
			broken := *manifest
			broken.Records = append([]EvidenceRecord(nil), manifest.Records...)
			switch mutation {
			case "wrong_test":
				broken.Records[0].TestID = "tools/language-acceptance:parser-smoke"
			case "wrong_source":
				broken.Records[0].ImplementationSHA = "0000000000000000000000000000000000000000"
			case "missing_record":
				broken.Records = nil
			}
			changed, validationErr := ValidateEvidenceManifest(&broken, contracts, "local")
			if validationErr == nil && changed.PassedCases != 0 {
				t.Fatalf("false U04 evidence passed: %+v", changed)
			}
		})
	}
	data, err := json.Marshal(map[string]any{"source_sha": runner.implSHA, "contract_sha256": ExpectedContractSHA256,
		"scope":  "Actual default/explicit V2 and explicit Legacy CLI source routes only; no native execution or release claim",
		"report": report, "records": runner.records})
	if err != nil {
		t.Fatal(err)
	}
	t.Log("P15_U04_RECEIPT:" + string(data))
}
